# WhatSignal Requirements

## 1. Introduction
WhatSignal is a one-to-one chat bridge between WhatsApp and Signal. It listens for incoming WhatsApp messages (text, image, video, audio), forwards them to a local Signal-CLI daemon, and vice-versa—allowing reply/quote in Signal to route back to the original WhatsApp sender.

## 2. Scope  
- ✅ **IMPLEMENTED**: One-on-one chat bridging
- 🔄 **FUTURE**: Group chat bridging (out of scope for initial release)  

## 3. Functional Requirements  

### 3.1 WhatsApp → Signal ✅ **IMPLEMENTED**
- ✅ Receive incoming WhatsApp messages via webhook (prefer) or polling fallback  
- ✅ Support text, images, video, audio, documents  
- ✅ Download media from WhatsApp API and re-upload to Signal respecting Signal size limits  
- ✅ Forward to Signal-CLI daemon, tagging each payload with:  
  - Original sender's number/name  
  - Timestamp  
- ✅ Use compact JSON-encoded header for metadata with clear visual separator:
  ```
  {
    "sender": "John Doe",
    "chat": "Friends Group",
    "time": "2023-06-15T13:45:21Z",
    "msgId": "wa-123456789",
    "threadId": "thread-987654321"
  }
  ---
  Actual message content here
  ```
- ✅ All JSON metadata fields are mandatory for consistent message processing

### 3.2 Signal → WhatsApp ✅ **IMPLEMENTED**
- ✅ Listen on Signal-CLI JSON-RPC daemon for incoming messages  
- ✅ Detect replies/quotes: use timestamp correlation and embedded header metadata to match to original WhatsApp message  
- ✅ Fetch reply text/media and invoke WhatsApp HTTP API to send back to the correct WhatsApp chatId  
- ✅ Support media attachments (images, videos, documents, audio) when replying to messages
- ✅ Properly handle and convert media formats for compatibility between platforms
- ✅ When a reply cannot be confidently correlated to the original message:
  - ✅ Log the event with appropriate error details
  - 🔄 **PARTIAL**: Notify the sender that the reply could not be processed (logged but not user-facing)
  - 🔄 **FUTURE**: Offer option to send as a new message instead of reply

### 3.3 Mapping & Persistence ✅ **IMPLEMENTED**
- ✅ Persist mappings (WhatsApp messageId ↔ Signal messageId) in SQLite database
- ✅ Configurable retention window (default: 30 days)  
- ✅ **ENHANCED**: Database encryption at rest with AES-256-GCM

### 3.4 Loop Prevention 🔄 **PARTIAL**
- 🔄 **PLANNED**: Append a hidden metadata flag to all forwarded messages to identify origin  
- 🔄 **PLANNED**: Discard any incoming event that bears the bridge's own metadata  

### 3.5 Bot Commands 🔄 **FUTURE**
- 🔄 **PLANNED**: Support command interface (e.g., `@bot help`) for controlling bridge behavior
- 🔄 **PLANNED**: Include commands for status, reconnection, and configuration adjustments
- 🔄 **PLANNED**: Allow users to query message history and bridge statistics

### 3.6 Media Handling & Caching ✅ **IMPLEMENTED**
- ✅ Implement efficient media file caching to avoid unnecessary re-downloads and re-uploads
- ✅ Cache media files based on hash/fingerprint for a configurable period
- ✅ Apply appropriate size and type conversions to ensure compatibility between platforms
- ✅ Implement cleanup routines for cached media beyond retention period
- 🔄 **PARTIAL**: Support optional lazy loading of media (basic implementation)
- ✅ Enforce platform media size limits:
  - Videos: Maximum 100 MB per file
  - GIFs: Maximum 25 MB per file
  - Images: Maximum 5 MB per file (updated from 8MB)
  - Documents: Maximum 100 MB per file
  - Voice: Maximum 16 MB per file
- 🔄 **FUTURE**: Apply size reduction/compression when media exceeds platform limits

### 3.7 Service Reliability & Edge Cases ✅ **MOSTLY IMPLEMENTED**
- 🔄 **PARTIAL**: Handle duplicate message detection and prevention (basic implementation)
- ✅ Implement message delivery confirmation with timeout and retry mechanism
- ✅ Support reconnection and session restoration after connection failures
- ✅ Handle rate limiting gracefully (especially for Signal)
- ✅ Properly validate JIDs/phone numbers before message sending
- ✅ Implement appropriate error handling for platform-specific issues:
  - ✅ Reply failures to specific numbers
  - ✅ Contact sync issues
  - ✅ Certificate validation and proxy traversal
  - ✅ Memory management and CPU utilization monitoring
- ✅ Utilize native delivery confirmation mechanisms:
  - ✅ Monitor WhatsApp delivery receipts/read receipts
  - ✅ Track Signal delivery confirmations
  - ✅ Update message status in database upon confirmation
  - ✅ Handle cases where delivery status is unavailable or times out

## 4. Non-Functional Requirements  

### 4.1 Performance & Reliability ✅ **IMPLEMENTED**
- ✅ Throughput: up to 10 messages/minute  
- ✅ On send failure: retry with exponential back-off (configurable parameters in config.json)  
- ✅ Graceful shutdown: drain in-flight messages and persist state
- ✅ Resource monitoring and alerting for excessive memory or CPU usage
- ✅ Circuit breaker pattern for failing external services

### 4.2 Configuration ✅ **IMPLEMENTED**
- ✅ Single WhatsApp number ↔ single Signal number  
- ✅ All settings (API credentials, webhook port, retry/back-off, retention window) in `config.json`  
- ✅ Avoid hard-coding secrets; support overriding via environment variables  
- ✅ Persist WhatsApp session information to avoid repeated authentication
- ✅ Auto-reconnect on service disruptions with configurable retry intervals

### 4.3 Language & Libraries ✅ **IMPLEMENTED**
- ✅ Primary: Go (≥1.22)  
- ✅ Use well-maintained HTTP, JSON, logging, and CLI-RPC client libraries  
- ✅ Follow clean-architecture principles (separate transport, business logic, persistence)  

### 4.4 Security ✅ **ENHANCED IMPLEMENTATION**
- ✅ Store API credentials and webhook secrets securely (file permissions, environment variables)  
- ✅ Validate and sanitize incoming webhook payloads  
- ✅ Verify webhook signatures (`X-Waha-Signature-256` for WAHA, `X-Signal-Signature-256` for Signal)
- ✅ Rate-limit or authenticate webhooks to prevent abuse  
- ✅ Log sensitive data (e.g. message content, numbers) only if explicitly enabled
- ✅ **ENHANCED**: Database field-level encryption with AES-256-GCM
- ✅ **ENHANCED**: Comprehensive security scanning with `govulncheck` and `gosec`
- ✅ **ENHANCED**: Path validation to prevent directory traversal attacks

### 4.5 Logging & Monitoring ✅ **IMPLEMENTED**
- ✅ Structured logging (JSON) with levels (DEBUG, INFO, WARN, ERROR)  
- ✅ Basic operational metrics for service health monitoring
- ✅ Health-check endpoint (`/health`)  

## 5. Architecture Overview ✅ **IMPLEMENTED**
```
[WhatsApp API] → Webhook/Poller → WhatSignal Core → Signal-CLI JSON-RPC  
[Signal-CLI JSON-RPC] → WhatSignal Core → WhatsApp HTTP API  
```  
- ✅ **Transport Layer**: HTTP server for WhatsApp webhooks, JSON-RPC client for Signal  
- ✅ **Core Logic**: message transformation, metadata tagging, mapping persistence, loop filter  
- ✅ **Persistence**: local embedded SQLite store for mappings and configuration  

## 6. Interfaces ✅ **IMPLEMENTED**

### 6.1 WhatsApp HTTP API (Waha) ✅ **IMPLEMENTED**
- ✅ Endpoints: `/api/sendText`, `/api/sendImage`, `/api/sendVideo`, `/api/sendFile`, `/api/sendVoice`
- ✅ Webhook receiver at `/webhook/whatsapp`  
- ✅ **ENHANCED**: Full WAHA API compliance with typing simulation and seen status

### 6.2 Signal-CLI JSON-RPC ✅ **IMPLEMENTED**
- ✅ Methods used: `send`, `receive`, `register` (for device initialization/check, not full registration)
- ✅ Persistent daemon running locally on configurable port/socket
- ✅ WhatSignal client uses configured phone number and device name
- ✅ `InitializeDevice` method in WhatSignal's client performs an initial check/communication with the daemon
- ✅ Authentication via Bearer token if `signal.authToken` is configured
- ✅ All client method calls from WhatSignal include `context.Context`
- ✅ HTTP client used by WhatSignal for JSON-RPC calls is configurable (e.g. for timeouts)

## 7. Data Model ✅ **IMPLEMENTED**

- **MessageMapping** ✅ **IMPLEMENTED**
  - ✅ `whatsappChatId`, `whatsappMessageId`  
  - ✅ `signalMessageId`, `signalTimestamp`  
  - ✅ `forwardedAt`  
  - ✅ `deliveryStatus` (enum: pending, sent, delivered, read, failed)
  - ✅ `mediaPath` (optional, for cached media references)
  - ✅ **ENHANCED**: `mediaType` field for media classification
  - ✅ **ENHANCED**: `createdAt`, `updatedAt` timestamps

- **Database Schema Principles** ✅ **IMPLEMENTED**
  - ✅ Simple indexing strategy focused on common lookup patterns:
    - Index on `whatsappMessageId` for reply correlation
    - Index on `signalMessageId` for delivery status updates
    - Composite index on `(whatsappChatId, forwardedAt)` for conversation retrieval
  - ✅ Keep schema minimal but sufficient for core functionality
  - ✅ Use SQLite with write-ahead logging for reliability and performance
  - ✅ Implement automated pruning of old records based on retention policy

- **Config (config.json)** ✅ **IMPLEMENTED**
  ```json
  {
    "whatsapp": {
      "api_base_url": "http://localhost:3000",
      "api_key": "your-waha-api-key",
      "session_name": "default",
      "timeout_ms": 10000,
      "retry_count": 3,
      "webhook_secret": "your-whatsapp-webhook-secret"
    },
    "signal": {
      "rpc_url": "http://localhost:8080/jsonrpc",
      "auth_token": "your-signal-auth-token",
      "phone_number": "+1234567890",
      "device_name": "whatsignal-device",
      "webhook_secret": "your-signal-webhook-secret"
    },
    "retry": {
      "initial_backoff_ms": 1000,
      "max_backoff_ms": 60000,
      "max_attempts": 5
    },
    "retentionDays": 30,
    "database": {
      "path": "./whatsignal.db"
    },
    "media": {
      "cache_dir": "./media_cache",
      "maxSizeMB": {
        "image": 5,
        "video": 100,
        "gif": 25,
        "document": 100,
        "voice": 16
      },
      "allowedTypes": {
        "image": ["jpg", "jpeg", "png"],
        "video": ["mp4", "mov"],
        "document": ["pdf", "doc", "docx"],
        "voice": ["ogg"]
      }
    }
  }
  ```

## 8. Error Handling & Retries ✅ **IMPLEMENTED**
- ✅ Wrap all outbound calls with retry logic and jitter  
- ✅ On unrecoverable errors, log and move on to avoid blocking other messages  
- 🔄 **FUTURE**: Expose a dead-letter queue for manual inspection  

## 9. Testing Strategy ✅ **IMPLEMENTED**
- ✅ **Unit tests**: mock WhatsApp client, mock Signal-CLI JSON-RPC client, core logic  
- ✅ **Integration tests**: containerized end-to-end flows using lightweight HTTP stub for WhatsApp and a real Signal-CLI daemon instance  
- ✅ **Key Test Scenarios**:
  - ✅ Message format encoding/decoding validation
  - ✅ Metadata extraction and proper JSON formatting
  - ✅ Media download, conversion, and upload flows
  - ✅ Database read/write operations and query performance
  - ✅ Reply correlation mechanisms
  - ✅ Error handling and retry logic
  - ✅ Message delivery confirmation tracking
  - ✅ Connection disruption and recovery
  - ✅ **ENHANCED**: Encryption/decryption testing
  - ✅ **ENHANCED**: Security vulnerability testing
- ✅ Code coverage target ≥80% (currently achieved)

## 10. Deployment ✅ **IMPLEMENTED**
- ✅ Dockerized service with multi-stage build, minimal base image (Alpine)  
- ✅ Expose ports: HTTP webhook (8082), health endpoint
- ✅ Deploy via container orchestrator (Docker Compose / Kubernetes)  
- ✅ **ENHANCED**: Production-ready Docker Compose with security hardening

## 11. Future Enhancements 🔄 **ROADMAP**
- 🔄 **PLANNED**: Group chat bridging  
- 🔄 **PLANNED**: Multi-account support  
- 🔄 **PLANNED**: Web UI for monitoring and configuration
- ✅ **IMPLEMENTED**: Enhanced authentication and security mechanisms:
  - ✅ Webhook endpoint authentication
  - ✅ Database encryption
  - 🔄 **PLANNED**: Credential rotation automation
  - 🔄 **PLANNED**: Secure credential storage solutions
- 🔄 **PLANNED**: Comprehensive metrics collection and visualization:
  - 🔄 **PLANNED**: Message throughput statistics
  - 🔄 **PLANNED**: Delivery success rates
  - 🔄 **PLANNED**: Performance metrics (CPU/memory usage)
  - 🔄 **PLANNED**: Latency measurements
- 🔄 **PLANNED**: API version compatibility management

## 12. References
- WhatsApp REST API: https://github.com/devlikeapro/waha
- Signal CLI: https://github.com/AsamK/signal-cli
- Previous effort at bridging: https://github.com/meinto/whatsapp-signal-bridge

## 13. Legal Considerations ✅ **IMPLEMENTED**
- ✅ Include appropriate disclaimers about unofficial API usage
- ✅ License under permissive open source license (MIT)
- ✅ Document compliance considerations for WhatsApp and Signal terms of service

## 14. Implementation Status Summary

### ✅ **FULLY IMPLEMENTED**
- Core message bridging (WhatsApp ↔ Signal)
- Media handling and caching
- Database persistence with encryption
- Configuration management
- Error handling and retries
- Comprehensive testing
- Security hardening
- Docker deployment
- Documentation

### 🔄 **PARTIALLY IMPLEMENTED**
- Loop prevention (basic structure in place)
- Duplicate message detection (basic implementation)
- Advanced media processing (compression/resizing)

### 🔄 **PLANNED FOR FUTURE**
- Bot commands interface
- Group chat support
- Web UI for monitoring
- Advanced metrics and monitoring
- Dead-letter queue
- Multi-account support
