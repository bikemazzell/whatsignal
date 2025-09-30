# Testing Guide

## Quick Reference

```bash
# Run unit tests only
make test

# Run unit tests with race detection
make test-race

# Run unit tests with coverage
make coverage

# Run integration tests (mock mode, no Docker)
make test-integration

# Run integration tests with Docker services
make test-integration-docker

# Run all CI checks (formatting, linting, security, tests)
make ci

# Simulate full GitHub Actions workflow locally (RECOMMENDED before pushing)
make ci-integration-workflow
```

## Local CI Workflow Testing

**Before pushing to GitHub**, run:

```bash
make ci-integration-workflow
```

This simulates the exact GitHub Actions `integration-tests-docker` workflow:

1. ✅ Verifies Go installation
2. ✅ Installs dependencies
3. ✅ Builds project
4. ✅ Starts Docker services (PostgreSQL, Redis, WAHA, Signal-CLI, etc.)
5. ✅ Runs full integration test suite
6. ✅ Cleans up Docker resources

### Benefits

- **Fast feedback**: ~2-3 minutes locally vs ~10+ minutes on GitHub
- **No wasted CI minutes**: Test before pushing
- **Better debugging**: Full access to logs and services
- **Offline capable**: Works without internet (after initial Docker image pulls)

### Requirements

- Go 1.24.6+ installed
- Docker and Docker Compose running
- ~8GB free disk space
- Available ports: 3000, 5432, 6379, 8080, 8081, 9000, 9001, 9090, 14250, 16686

## Integration Test Options

### Run with specific pattern
```bash
make test-integration-pattern PATTERN=TestWhatsAppToSignal
```

### Run with verbose output
```bash
make test-integration-verbose
```

### Run performance benchmarks
```bash
make test-integration-perf
```

### Keep Docker services running after tests
```bash
./integration_test/run-tests.sh --docker --no-cleanup
```

Then clean up manually when done:
```bash
cd integration_test
docker compose down -v --remove-orphans
```

## CI/CD Pipeline

The GitHub Actions workflow mirrors `make ci-integration-workflow`:

- `.github/workflows/integration-tests.yml` - Integration tests with Docker
- Runs on: pushes to `main`/`develop`, tags, and manual triggers
- Auto-fails if services don't start healthy
- Uploads logs on failure

## Troubleshooting

### Go not found
```bash
which go
go version
```

### Docker services fail to start
```bash
# Check if ports are in use
sudo lsof -i :3000 :5432 :6379 :8080

# Check Docker daemon
docker version
docker ps
```

### Tests timeout
Increase timeout:
```bash
./integration_test/run-tests.sh --docker --timeout 30m
```

### View service logs
```bash
cd integration_test
docker compose logs waha
docker compose logs postgres
docker compose logs -f  # follow all logs
```

## Best Practices

1. **Always run `make ci-integration-workflow` before pushing**
2. Run `make ci` for quick checks (no Docker required)
3. Use `make test-integration-pattern PATTERN=...` for focused testing
4. Keep Docker images updated: `docker compose pull` in `integration_test/`
5. Clean up regularly: `make clean-integration` and `make docker-clean`