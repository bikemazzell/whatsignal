# WhatsApp-Signal Bridge TODO

## Edge Case Testing

### Completed Edge Case Tests
- ✓ Encryption edge cases (very large data, binary data, concurrent access, tamper detection)
- ✓ Database edge cases (concurrent operations, transaction rollback, SQL injection protection)
- ✓ Rate limiting edge cases (burst traffic, sustained load, memory cleanup, race conditions)

### Remaining Edge Cases to Test

#### Network and API Edge Cases
- Network partition handling during message transmission
- API timeout scenarios (WhatsApp/Signal APIs becoming unresponsive)
- Partial message delivery recovery
- WebSocket connection stability under poor network conditions
- Handling of malformed API responses
- Recovery from temporary network failures
- Connection pool exhaustion

#### Message Processing Edge Cases
- Message queue overflow scenarios
- Very large message handling (>10MB)
- Handling of malformed message formats
- Unicode and emoji edge cases in different positions
- Message ordering during high concurrency
- Duplicate message detection and handling
- Message loop prevention

#### Resource Management Edge Cases
- Memory leak detection under long-running operations
- Disk space exhaustion during media downloads
- CPU spike handling during encryption/decryption
- File descriptor exhaustion
- Database connection pool exhaustion
- Goroutine leak detection

#### Security Edge Cases
- Timing attack resistance in authentication
- Path traversal in media handling (partially tested)
- XXE attacks in XML processing (if any)
- SSRF prevention in URL handling
- Resource exhaustion attacks (zip bombs, etc.)

#### Error Recovery Edge Cases
- Cascading failure prevention
- Circuit breaker behavior under sustained failures
- Graceful degradation when services are unavailable
- Recovery from corrupted state
- Handling of persistent retries
- Dead letter queue management

#### Performance Edge Cases
- Behavior under extreme load (>10k messages/second)
- Performance degradation curves
- Latency spikes during garbage collection
- Impact of large contact lists on sync
- Batch processing optimization limits

#### Configuration Edge Cases
- Invalid configuration combinations
- Configuration hot-reloading edge cases
- Environment variable precedence issues
- Missing required configuration with defaults
- Configuration validation completeness



## Unit test code coverage improvements

### Current Coverage Status
Packages below 95% coverage that need improvement:

  - cmd/migrate: 0.0% → 95% (needs tests for migration command)
  - cmd/whatsignal: 76.4% → 95% (18.6% gap)
  - pkg/signal: 81.9% → 95% (13.1% gap)
  - internal/database: Failed tests → 95% (needs fixing + more tests)
  - pkg/whatsapp: 80.6% → 95% (14.4% gap)
  - pkg/media: 82.6% → 95% (12.4% gap)
  - internal/service: 85.8% → 95% (9.2% gap)

Already meeting 95%+ coverage:
  - internal/security: 95.8% ✓
  - internal/config: 96.2% ✓
  - internal/migrations: 100.0% ✓
  - internal/models: 100.0% ✓
  - pkg/signal/types: 100.0% ✓
  - pkg/whatsapp/types: 100.0% ✓

### Specific Areas Needing Tests

#### cmd/whatsignal (76.4% → 95%)
- [ ] Error paths in main.go initialization
- [ ] Server shutdown error handling
- [ ] Rate limiter middleware edge cases
- [ ] Webhook signature verification failures
- [ ] Configuration validation error paths
- [ ] Signal device initialization failures

#### pkg/signal (81.9% → 95%)
- [ ] RPC connection failures
- [ ] Message parsing errors
- [ ] Attachment upload/download failures
- [ ] Retry mechanism edge cases
- [ ] Context cancellation handling

#### internal/database (Failed → 95%)
- [ ] Fix failing tests first
- [ ] Transaction rollback scenarios
- [ ] Concurrent access patterns
- [ ] Migration failure handling
- [ ] Encryption/decryption errors

#### pkg/whatsapp (80.6% → 95%)
- [ ] API error responses
- [ ] Session management failures
- [ ] Media upload errors
- [ ] Contact sync edge cases
- [ ] Webhook parsing failures

#### pkg/media (82.6% → 95%)
- [ ] File system errors
- [ ] Invalid media types
- [ ] Size limit enforcement
- [ ] Concurrent download handling
- [ ] Cleanup failures

#### internal/service (85.8% → 95%)
- [ ] Bridge service error paths
- [ ] Message routing failures
- [ ] Session monitoring errors
- [ ] Poller lifecycle edge cases
- [ ] Contact service failures


## Group chat support

## Dynamic Session-Based Signal Polling Implementation

### Current State
- Single Signal poller with one worker (`sp.wg.Add(1)`)
- Fixed polling regardless of WAHA session count
- One intermediary Signal number for all sessions

### Required Implementation

#### 1. WAHA Session Discovery
- **API Endpoint**: `GET /api/sessions/` 
- **Documentation**: https://waha.devlike.pro/docs/how-to/sessions/
- **Implementation**:
  - Create WAHA session client in `pkg/whatsapp/`
  - Add `GetSessions()` method to WhatsApp client interface
  - Parse session response to extract active session names
  - Handle authentication if required

#### 2. Dynamic Polling Manager
- **Replace**: Current single `SignalPoller` 
- **With**: `SignalPollingManager` that manages multiple workers
- **Features**:
  - Maintain map of `sessionName -> SignalPoller`
  - Start/stop individual pollers per session
  - Coordinate polling across sessions with one Signal relay number
  - Handle session lifecycle events

#### 3. Session Lifecycle Management
- **Session Discovery**:
  - Periodic polling of `/api/sessions/` to detect changes
  - Compare current active sessions with known sessions
  - Create workers for new sessions
  - Destroy workers for removed sessions

- **Webhook Integration**:
  - Monitor WAHA webhooks for session lifecycle events
  - Events: session started, session stopped, session error
  - Trigger worker creation/destruction based on events

#### 4. Architecture Changes

##### 4.1 New Components
```
SignalPollingManager
├── SessionMonitor (WAHA session discovery)
├── PollerRegistry (manage active pollers)
└── SessionLifecycleHandler (webhook events)
```

##### 4.2 Updated Components
- **MessageService**: Remove `PollSignalMessages`, delegate to manager
- **SignalPoller**: Add session context, update worker count logic
- **Main**: Replace single poller with polling manager

#### 5. Configuration Updates
```json
{
  "signal": {
    "pollingEnabled": true,
    "pollIntervalSec": 5,
    "pollTimeoutSec": 10,
    "sessionDiscoveryIntervalSec": 30,
    "maxConcurrentSessions": 10
  },
  "whatsapp": {
    "sessionApiEnabled": true,
    "sessionApiPath": "/api/sessions/"
  }
}
```

#### 6. Implementation Steps

##### Phase 1: WAHA Session Client
- [ ] Create `pkg/whatsapp/sessions.go`
- [ ] Add `SessionInfo` struct for API response
- [ ] Implement `GetSessions()` method in WhatsApp client
- [ ] Add session discovery configuration
- [ ] Unit tests for session discovery

##### Phase 2: Polling Manager
- [ ] Create `internal/service/signal_polling_manager.go`
- [ ] Implement session-to-poller mapping
- [ ] Add worker lifecycle management
- [ ] Replace single poller in main.go
- [ ] Update message routing for multi-session

##### Phase 3: Dynamic Scaling
- [ ] Implement session monitoring loop
- [ ] Add session change detection
- [ ] Handle worker creation/destruction
- [ ] Add metrics/logging for session changes

##### Phase 4: Webhook Integration
- [ ] Extend webhook handlers for session events
- [ ] Add session lifecycle event processing
- [ ] Implement real-time session updates
- [ ] Test session start/stop scenarios

##### Phase 5: Testing & Optimization
- [ ] Load testing with multiple sessions
- [ ] Memory leak testing for worker lifecycle
- [ ] Performance optimization for high session count
- [ ] Documentation updates

#### 7. Technical Considerations

##### 7.1 Message Routing
- **Question**: How to route Signal messages to correct WAHA session?
- **Current**: All messages go through one bridge
- **Proposed**: Session-aware message mapping
  - Store `sessionName` in message mappings
  - Route replies to originating session

##### 7.2 Resource Management
- **Memory**: Each poller has its own goroutine and resources
- **Connections**: Limit concurrent Signal API connections
- **Backoff**: Independent retry logic per session

##### 7.3 Error Handling
- **Session Failures**: Don't stop all polling if one session fails
- **API Errors**: Graceful degradation when session API unavailable
- **Recovery**: Automatic session rediscovery after failures

#### 8. Migration Strategy
- [ ] Maintain backward compatibility with single session
- [ ] Feature flag for dynamic polling (`enableDynamicPolling`)
- [ ] Gradual rollout with monitoring
- [ ] Fallback to single poller if session discovery fails

#### 9. Monitoring & Observability
- [ ] Metrics for active sessions and pollers
- [ ] Logging for session lifecycle events
- [ ] Health checks for individual session pollers
- [ ] Alert on session discovery failures

### Success Criteria
1. **Scalability**: Automatically scale polling workers with WAHA sessions
2. **Reliability**: Individual session failures don't affect others
3. **Performance**: No degradation with multiple sessions
4. **Maintainability**: Clean separation of session and polling concerns
5. **Observability**: Clear visibility into session and polling status

### Future Enhancements
- **Multi-Signal Support**: Different Signal numbers per session
- **Load Balancing**: Distribute sessions across multiple Signal relays
- **Session Prioritization**: Different polling intervals per session importance
- **Auto-scaling**: Dynamic adjustment based on message volume
