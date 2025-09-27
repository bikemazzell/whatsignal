# WhatsSignal Integration Tests

This directory contains comprehensive integration tests for the WhatsSignal application. These tests validate end-to-end functionality using real components where possible.

## Overview

The integration test suite is designed to:
- Test complete message flows from WhatsApp to Signal and vice versa
- Validate multi-channel routing capabilities
- Test media processing and storage
- Verify error recovery and resilience
- Validate security and authentication flows
- Ensure proper resource cleanup and isolation

## Architecture

### Test Infrastructure

- **helpers.go** - Core test infrastructure and utilities
- **environment.go** - Enhanced test environment management with isolation modes
- **fixtures.go** - Predefined test data and scenarios

### Test Execution

- **run-tests.sh** - Test runner script supporting both mock and Docker modes
- **docker-compose.yml** - Optional external dependencies for comprehensive testing

### Test Data

- **test-data/** - Configuration files for Docker services
  - `prometheus.yml` - Metrics collection configuration
  - `nginx.conf` - Reverse proxy configuration
  - `postgres-init.sql` - Database initialization scripts

## Quick Start

### Basic Testing (Mocks Only)

```bash
# Run all integration tests with mocked dependencies
./integration_test/run-tests.sh

# Run specific test pattern
./integration_test/run-tests.sh TestEndToEnd

# Run with verbose output
./integration_test/run-tests.sh -v
```

### Comprehensive Testing (With Docker)

```bash
# Run with real external services
./integration_test/run-tests.sh --docker

# Run specific test with Docker services
./integration_test/run-tests.sh --docker TestMultiChannel

# Keep services running for debugging
./integration_test/run-tests.sh --docker --no-cleanup
```

## Test Environment Modes

### Isolation Modes

1. **IsolationNone** - Shared resources (fastest, least realistic)
2. **IsolationProcess** - Separate database and files per environment
3. **IsolationStrict** - Complete isolation for each test

### Docker Services

When using `--docker`, the following services are available:

| Service | Port | Purpose |
|---------|------|---------|
| WAHA (WhatsApp) | 3000 | Real WhatsApp API |
| Signal-CLI | 8080 | Real Signal messaging |
| PostgreSQL | 5432 | Alternative database backend |
| Redis | 6379 | Caching and session storage |
| Prometheus | 9090 | Metrics collection |
| Jaeger | 16686 | Distributed tracing |
| MinIO | 9000 | S3-compatible storage |
| Nginx | 8081 | Load balancing and proxy |

## Test Structure

### Standard Test Fixtures

The test suite includes predefined fixtures for:
- **Contacts** - Standard test contacts (Alice, Bob, groups, blocked users)
- **Message Mappings** - Various message states and types
- **Webhooks** - WhatsApp webhook payloads for different scenarios
- **Configurations** - Test configurations for different scenarios

### Test Scenarios

Predefined test scenarios include:
- **Simple Message Flow** - Basic end-to-end messaging
- **Multi-Channel Routing** - Multiple sessions and destinations
- **Media Processing** - Image and file handling
- **Error Recovery** - Failure and retry scenarios
- **Status Tracking** - Message acknowledgments and delivery

## Writing Integration Tests

### Basic Test Structure

```go
func TestMyIntegration(t *testing.T) {
    // Create test environment
    env := NewTestEnvironment(t)
    defer env.Cleanup()
    
    // Set up test data
    fixtures := env.GetFixtures()
    contacts := fixtures.Contacts()
    
    // Populate database
    err := env.PopulateWithFixtures()
    require.NoError(t, err)
    
    // Run test scenario
    // ... test logic here ...
    
    // Verify results
    // ... assertions here ...
}
```

### Using Environment Manager

```go
func TestWithEnvironmentManager(t *testing.T) {
    manager := NewEnvironmentManager()
    defer manager.CleanupAll()
    
    env := manager.CreateEnvironment(t, "test_scenario")
    
    // Test with managed environment
    // ... test logic here ...
}
```

### Advanced Features

```go
func TestAdvancedScenario(t *testing.T) {
    env := NewTestEnvironment(t)
    defer env.Cleanup()
    
    // Set isolation mode
    env.SetIsolationMode(IsolationStrict)
    
    // Create test media
    imagePath := env.CreateTestImageFile("test.png")
    
    // Mock HTTP endpoints
    env.AddHTTPHandler("/custom", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("custom response"))
    })
    
    // Wait for conditions
    ready := env.WaitForCondition(func() bool {
        return env.VerifyDatabaseConnection() == nil
    }, 30*time.Second, 1*time.Second)
    
    require.True(t, ready, "Database should be ready")
}
```

## Performance Testing

### Benchmarks

Integration tests can include performance benchmarks:

```go
func BenchmarkMessageThroughput(b *testing.B) {
    env := NewTestEnvironment(testing.TB(b))
    defer env.Cleanup()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // Benchmark message processing
    }
}
```

### Load Testing

Use fixtures to generate large datasets:

```go
func TestHighVolume(t *testing.T) {
    env := NewTestEnvironment(t)
    defer env.Cleanup()
    
    fixtures := env.GetFixtures()
    contacts := fixtures.RandomTestData(10000)
    
    // Test with high volume data
}
```

## Debugging

### Service Logs

When using Docker services, check logs:

```bash
# View all service logs
docker-compose -f integration_test/docker-compose.yml logs

# View specific service logs
docker-compose -f integration_test/docker-compose.yml logs waha
docker-compose -f integration_test/docker-compose.yml logs signal-cli
```

### Test Environment Stats

```go
stats := env.GetEnvironmentStats()
t.Logf("Environment stats: %+v", stats)
```

### Verbose Output

Use verbose mode for detailed test output:

```bash
./integration_test/run-tests.sh -v TestMyTest
```

## CI/CD Integration

The integration tests are designed to work in CI environments:

```yaml
# Example GitHub Actions step
- name: Run Integration Tests
  run: |
    ./integration_test/run-tests.sh --timeout 15m
  env:
    WHATSIGNAL_TEST_MODE: true
```

For Docker-based testing in CI:

```yaml
- name: Run Integration Tests with Docker
  run: |
    ./integration_test/run-tests.sh --docker --timeout 20m
```

## Best Practices

### Test Isolation

- Always use `defer env.Cleanup()` to ensure resource cleanup
- Use appropriate isolation modes for your test requirements
- Avoid shared state between tests

### Resource Management

- Set reasonable timeouts for long-running operations
- Use context cancellation for graceful shutdown
- Monitor resource usage in performance tests

### Error Handling

- Test both success and failure scenarios
- Verify error messages and types
- Test recovery mechanisms

### Maintainability

- Use fixtures for consistent test data
- Group related tests in focused files
- Document complex test scenarios
- Keep tests simple and focused

## Troubleshooting

### Common Issues

1. **Port conflicts** - Ensure ports 3000, 8080, etc. are available
2. **Docker not running** - Start Docker daemon before using --docker flag
3. **Permission errors** - Check file permissions for test directories
4. **Resource limits** - Increase Docker memory/CPU limits for complex tests

### Test Failures

1. Check service health: `docker-compose ps`
2. Review logs: `docker-compose logs [service]`
3. Verify network connectivity between services
4. Check environment variables and configuration

### Performance Issues

1. Use appropriate isolation modes
2. Limit test data size for faster execution
3. Use parallel execution where appropriate
4. Monitor resource usage during tests

## Contributing

When adding new integration tests:

1. Follow the existing test structure
2. Add appropriate fixtures for new scenarios
3. Ensure proper cleanup and resource management
4. Update this documentation for new features
5. Test both mock and Docker modes when applicable