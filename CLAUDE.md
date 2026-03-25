# CLAUDE.md — Project instructions for Claude Code

## Lessons Learned

### Testing: Reproduce failures, not just successes
- When a user reports a bug, the first test must reproduce the **failure condition**, not just confirm the happy path works. A test that always passes proves nothing about the bug.
- "All tests pass" does not mean "the bug is fixed." Ask: "What production conditions differ from the test environment?" Signal-cli version differences, timing, message volume, and race conditions under load are common culprits.
- When the test infrastructure itself has defects (e.g., static mock responses), fixing the infrastructure and seeing tests pass only proves the infrastructure is fixed — not that the production code is correct.

### Testing: No brittle sleeps
- Never use `time.Sleep` for synchronization in tests. Use polling helpers that check for expected state (e.g., "wait until WhatsApp send count reaches N"). Sleeps cause flaky tests on slow CI runners and waste time on fast ones.

### Release: Version bump must be the last commit
- All fixes, including ones discovered during validation (like the deploy script bug), must be committed BEFORE running `bump-version.sh`. A tag that doesn't include a fix committed after it is a process error. If a bug is found after tagging, bump again — don't leave the tag stale.

### Lock ordering: Document it
- When test infrastructure uses multiple mutexes, document the lock acquisition order. Acquiring lock B while holding lock A is safe only if no code path acquires A while holding B. Undocumented lock ordering causes deadlocks that surface only under load.

### Test assertions: Assert the thing you care about
- A test named `TestReactionRouting_TargetsCorrectMessage` must assert that the correct message was targeted. Asserting only that "no error occurred" or "the count is 1" doesn't verify routing correctness. If the assertion can't be made due to API limitations, document why and mark it as a known gap.

### Ship diagnostics with every new code path
- When adding a new code path (e.g., WebSocket receive alongside HTTP polling), always include the same diagnostic logging as the existing path. The HTTP polling path had raw envelope logging for missing quotes; the WebSocket path was shipped without it, making production debugging impossible. Every new path needs equal observability from day one.

### Upstream bug: signal-cli-rest-api does not serialize quotes (issue #818)
- Filed: https://github.com/bbernhard/signal-cli-rest-api/issues/818
- signal-cli v0.97 does not include the `quote` field in received DataMessages, in both native and json-rpc modes, with disappearing messages on or off. The Signal protocol protobuf includes the quote but signal-cli does not serialize it to JSON.
- This breaks `extractMappingFromQuotedText` which relies on `msg.QuotedMessage.Text` to extract the sender name and route replies correctly.
- Until this is fixed upstream, fallback routing (last active chat) is the only routing mechanism. The "Quote a message to reply to a specific chat" warning was removed because it's misleading.
- FIXED in v0.98 (signal-cli v0.14.1). Quote field is now present in received DataMessages via WebSocket. The existing routing pipeline works correctly. Require signal-cli-rest-api >= 0.98.

### Infrastructure: signal-cli-rest-api MODE is a critical decision with tradeoffs
- `MODE=native` strips quote fields from signal-cli's JSON output, causing all quoted replies to lose their quote and trigger fallback routing. The bridge cannot detect quoted messages in native mode.
- `MODE=json-rpc` preserves full message metadata (quotes, reactions, mentions) but has historically caused **complete message sending/receiving failures** in production. The switch from json-rpc to native was made deliberately in v1.1.3 because json-rpc was unreliable.
- `AUTO_RECEIVE_SCHEDULE` is incompatible with json-rpc mode (fatal error on startup). Must be removed when switching to json-rpc.
- **Expect instability after switching to json-rpc.** Monitor closely. If messages stop flowing, this is the likely cause. The previous failure mode was total — not partial. Have a rollback plan ready (switch back to native + re-add AUTO_RECEIVE_SCHEDULE).
- If json-rpc proves unreliable again, the alternative is to stay on native mode and implement quote detection via a different mechanism (e.g., tracking recent outbound messages and matching reply text patterns, or using signal-cli's dbus interface).

## Release checklist

- Always check for the latest Go version and update ALL of the following before finishing a commit or release:
  - `go.mod` — `go` and `toolchain` directives
  - `Dockerfile` — base image `golang:X.Y.Z-alpine`
  - `.github/workflows/*.yml` — every `GO_VERSION` env var and any hardcoded `go-version:` values
- Run `go mod tidy` with the new toolchain after updating.
- After bumping Go, rebuild any locally installed Go tools (staticcheck, golangci-lint, etc.) with the new toolchain so local CI matches GitHub Actions.
- The `scripts/bump-version.sh` script creates and pushes a git tag (`vX.Y.Z`) along with the commit. Both the branch and the tag must be pushed to origin for a complete release.
- `scripts/bump-version.sh` requires git push auth (HTTPS credentials or SSH). If it cannot push, do all local steps manually (bump VERSION, commit, tag), then hand off to the user with: `git push origin main && git push origin vX.Y.Z`
