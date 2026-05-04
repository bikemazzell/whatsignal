#!/bin/bash
#
# Release smoke test: pulls a freshly-built whatsignal image and verifies
# that it accepts a webhook signed in the documented WAHA HMAC format
# (HMAC-SHA512 over the raw body, hex). This catches divergence between
# the verifier and WAHA's actual signing contract. v1.2.47 would fail.
#
# Required env: IMAGE — full image ref including tag (e.g. ghcr.io/foo/bar:1.2.48)
#
set -euo pipefail

if [ -z "${IMAGE:-}" ]; then
    echo "ERROR: IMAGE environment variable required" >&2
    exit 2
fi

CONTAINER_NAME="whatsignal-smoke-$$"
WORK_DIR=$(mktemp -d)
trap 'docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true; rm -rf "$WORK_DIR"' EXIT

WEBHOOK_SECRET="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

# Minimal config the binary will accept on startup.
cat > "$WORK_DIR/config.json" <<EOF
{
  "whatsapp": {"api_base_url": "http://127.0.0.1:1"},
  "signal": {
    "rpc_url": "http://127.0.0.1:1",
    "intermediaryPhoneNumber": "+15555550100",
    "pollingEnabled": false,
    "attachmentsDir": "/app/data/attachments"
  },
  "channels": [
    {"whatsappSessionName": "default", "signalDestinationPhoneNumber": "+15555550101"}
  ],
  "database": {"path": "/app/data/whatsignal.db"},
  "media": {"cache_dir": "/app/data/media"}
}
EOF

echo "Pulling $IMAGE"
docker pull "$IMAGE"

PORT=$(comm -23 <(seq 30000 40000 | sort) <(ss -tan 2>/dev/null | awk 'NR>1 {sub(/.*:/, "", $4); print $4}' | sort -u) | shuf -n 1)
echo "Using port $PORT"

# tmpfs /app/data so the nonroot user in the image can write without
# host-side ownership tweaks. Smoke test does not need data persistence.
docker run -d --name "$CONTAINER_NAME" \
    -p "$PORT:8082" \
    -v "$WORK_DIR/config.json:/app/config.json:ro" \
    --tmpfs /app/data:size=64M,mode=1777 \
    -e WHATSAPP_API_KEY=smoke-test-api-key \
    -e WHATSIGNAL_WHATSAPP_WEBHOOK_SECRET="$WEBHOOK_SECRET" \
    -e WHATSIGNAL_ENCRYPTION_SECRET=smoke-test-encryption-secret-not-for-production-use \
    -e WHATSIGNAL_ENCRYPTION_SALT=smoke-test-salt-32-bytes-fixed-padding-here-aa \
    -e WHATSIGNAL_ENCRYPTION_LOOKUP_SALT=smoke-test-lookup-salt-32-bytes-fixed-padding \
    "$IMAGE"

echo "Waiting for /health..."
for _ in $(seq 1 60); do
    if curl -fsS "http://127.0.0.1:$PORT/health" >/dev/null 2>&1; then
        echo "healthy"
        break
    fi
    sleep 1
done

if ! curl -fsS "http://127.0.0.1:$PORT/health" >/dev/null 2>&1; then
    echo "ERROR: container never became healthy. Logs:" >&2
    docker logs "$CONTAINER_NAME" 2>&1 | tail -50 >&2
    exit 1
fi

# Sign per WAHA contract: HMAC-SHA512(rawBody, secret), lowercase hex.
BODY='{"event":"session.status","payload":{"name":"default","status":"WORKING"},"session":"default"}'
SIG=$(printf '%s' "$BODY" | openssl dgst -sha512 -mac HMAC -macopt "key:$WEBHOOK_SECRET" | awk '{print $NF}')
TS=$(($(date +%s) * 1000))

STATUS=$(curl -sS -o /tmp/smoke-resp -w '%{http_code}' \
    -X POST "http://127.0.0.1:$PORT/webhook/whatsapp" \
    -H 'Content-Type: application/json' \
    -H "X-Webhook-Hmac: $SIG" \
    -H "X-Webhook-Timestamp: $TS" \
    --data-binary "$BODY")

echo "Webhook response status: $STATUS"
echo "Response body: $(cat /tmp/smoke-resp)"

case "$STATUS" in
    200|400)
        # 200 = accepted; 400 = signature OK but JSON shape rejected (acceptable for this test).
        # Either result proves the verifier accepted a WAHA-format signature.
        echo "Smoke test PASSED: verifier accepted WAHA-format signature"
        ;;
    401)
        echo "ERROR: verifier rejected WAHA-format signature. Container would not accept real WAHA webhooks." >&2
        echo "Container logs:" >&2
        docker logs "$CONTAINER_NAME" 2>&1 | tail -30 >&2
        exit 1
        ;;
    *)
        echo "ERROR: unexpected response status $STATUS" >&2
        docker logs "$CONTAINER_NAME" 2>&1 | tail -30 >&2
        exit 1
        ;;
esac

# Negative test: a bad signature must be rejected.
BAD_SIG="00${SIG:2}"
BAD_STATUS=$(curl -sS -o /dev/null -w '%{http_code}' \
    -X POST "http://127.0.0.1:$PORT/webhook/whatsapp" \
    -H 'Content-Type: application/json' \
    -H "X-Webhook-Hmac: $BAD_SIG" \
    -H "X-Webhook-Timestamp: $TS" \
    --data-binary "$BODY")

if [ "$BAD_STATUS" != "401" ]; then
    echo "ERROR: verifier accepted a bad signature; status=$BAD_STATUS" >&2
    exit 1
fi
echo "Smoke test PASSED: bad signature correctly rejected"
