# WhatsApp-Signal Bridge TODO


# Next TODOs
- implement message reactions (e.g. if a User reacts to a message with 👍, that should be forwarded to WhatsApp via `/api/reaction` PUT command)
- check that sending an image, a file, voice message, and video all work (e.g. /api/sendImage, /api/sendFile, /api/sendVoice, /api/sendVideo)
- check that any videos are converted to WhatsApp understandable format (e.g. may need to use /api/{sesion}/media/convert/
- check that any voice responses are converted to WhatsApp understandable format (e.g. may need to use /api/{sesion}/media/convert/voice POST command)video POST command)
- make sure that URL previews work (if enabled, i.e. "linkPreview": true in /api/sendText body)


-improve code coverage to at least 90% across all packages; currently they are at:
  - cmd/whatsignal: 78.8%
  - internal/config: 96.4%
  - internal/database: 79.3%
  - internal/migrations: 100.0%
  - internal/models: 100.0%
  - internal/security: 95.8%
  - internal/service: 79.7%
  - pkg/media: 84.9%
  - pkg/whatsapp: 72.2%
  - pkg/whatsapp/types: 100.0%
  - pkg/signal/types: 100.0%



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
