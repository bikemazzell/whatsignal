# WhatSignal Requirements

## 1. Introduction
WhatSignal is a one-to-one chat bridge between WhatsApp and Signal. It listens for incoming WhatsApp messages (text, image, video, audio), forwards them to a local Signal-CLI daemon, and vice-versa—allowing reply/quote in Signal to route back to the original WhatsApp sender.

## 2. Scope  
- Initial support: one-on-one chats only  
- Future: group chats (out of scope for initial release)  

## 3. Functional Requirements  

### 3.1 WhatsApp → Signal  
- Receive incoming WhatsApp messages via webhook (prefer) or polling fallback  
- Support text, images, video, audio, documents  
- Download media from WhatsApp API and re-upload to Signal respecting Signal size limits  
- Forward to Signal-CLI daemon, tagging each payload with:  
  - Original sender's number/name  
  - Timestamp  
- Use compact JSON-encoded header for metadata with clear visual separator:
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
- All JSON metadata fields are mandatory for consistent message processing

### 3.2 Signal → WhatsApp  
- Listen on Signal-CLI JSON-RPC daemon for incoming messages  
- Detect replies/quotes: use timestamp correlation and embedded header metadata to match to original WhatsApp message  
- Fetch reply text/media and invoke WhatsApp HTTP API to send back to the correct WhatsApp chatId  
- Support media attachments (images, videos, documents, audio) when replying to messages
- Properly handle and convert media formats for compatibility between platforms
- When a reply cannot be confidently correlated to the original message:
  - Notify the sender that the reply could not be processed
  - Log the event with appropriate error details
  - Offer option to send as a new message instead of reply

### 3.3 Mapping & Persistence  
- Persist mappings (WhatsApp messageId ↔ Signal messageId) in a lightweight store (e.g. SQLite)  
- Configurable retention window (default: 30 days)  

### 3.4 Loop Prevention  
- Append a hidden metadata flag to all forwarded messages to identify origin  
- Discard any incoming event that bears the bridge's own metadata  

### 3.5 Bot Commands
- Support command interface (e.g., `@bot help`) for controlling bridge behavior
- Include commands for status, reconnection, and configuration adjustments
- Allow users to query message history and bridge statistics

### 3.6 Media Handling & Caching
- Implement efficient media file caching to avoid unnecessary re-downloads and re-uploads
- Cache media files based on hash/fingerprint for a configurable period
- Apply appropriate size and type conversions to ensure compatibility between platforms
- Implement cleanup routines for cached media beyond retention period
- Support optional lazy loading of media (download only when explicitly requested)
- Enforce Signal platform media size limits:
  - Videos: Maximum 100 MB per file
  - GIFs: Maximum 25 MB per file
  - Images: Maximum 8 MB per file
  - Other documents: Follow Signal's default limits
- Apply size reduction/compression when media exceeds platform limits

### 3.7 Service Reliability & Edge Cases
- Handle duplicate message detection and prevention (WhatsApp occasionally sends duplicates)
- Implement message delivery confirmation with timeout and retry mechanism
- Support reconnection and session restoration after connection failures
- Handle rate limiting gracefully (especially for Signal)
- Properly validate JIDs/phone numbers before message sending
- Implement appropriate error handling for platform-specific issues:
  - Reply failures to specific numbers
  - Contact sync issues
  - Certificate validation and proxy traversal
  - Memory management and CPU utilization monitoring
- Utilize native delivery confirmation mechanisms:
  - Monitor WhatsApp delivery receipts/read receipts
  - Track Signal delivery confirmations
  - Update message status in database upon confirmation
  - Handle cases where delivery status is unavailable or times out

## 4. Non-Functional Requirements  

### 4.1 Performance & Reliability  
- Throughput: up to 10 messages/minute  
- On send failure: retry with exponential back-off (configurable parameters in settings.json)  
- Graceful shutdown: drain in-flight messages and persist state
- Resource monitoring and alerting for excessive memory or CPU usage
- Circuit breaker pattern for failing external services

### 4.2 Configuration  
- Single WhatsApp number ↔ single Signal number  
- All settings (API credentials, webhook port, retry/back-off, retention window) in `settings.json`  
- Avoid hard-coding secrets; support overriding via environment variables  
- Persist WhatsApp session information to avoid repeated authentication
- Auto-reconnect on service disruptions with configurable retry intervals

### 4.3 Language & Libraries  
- Primary: Go (target ≥1.22)  
- Fallback: Python (3.10+)  
- Use well-maintained HTTP, JSON, logging, and CLI-RPC client libraries  
- Follow clean-architecture principles (separate transport, business logic, persistence)  

### 4.4 Security  
- Store API credentials and webhook secrets securely (file permissions, optional secret manager)  
- Validate and sanitize incoming webhook payloads  
- Rate-limit or authenticate webhooks to prevent abuse  
- Log sensitive data (e.g. message content, numbers) only if explicitly enabled

### 4.5 Logging & Monitoring  
- Structured logging (JSON) with levels (INFO, WARN, ERROR)  
- Basic operational metrics for service health monitoring
- Health-check endpoint (`/health`)  

## 5. Architecture Overview  
```
[WhatsApp API] → Webhook/Poller → WhatSignal Core → Signal-CLI JSON-RPC  
[Signal-CLI JSON-RPC] → WhatSignal Core → WhatsApp HTTP API  
```  
- **Transport Layer**: HTTP server for WhatsApp webhooks, JSON-RPC client for Signal  
- **Core Logic**: message transformation, metadata tagging, mapping persistence, loop filter  
- **Persistence**: local embedded store for mappings and configuration  

## 6. Interfaces  

### 6.1 WhatsApp HTTP API (Waha)  
- Endpoints: `/api/sendText`, `/api/sendMedia`, webhook receiver at `/webhook/whatsapp`  

### 6.2 Signal-CLI JSON-RPC  
- Methods: `send`, `receive`, `register`, `event subscribe`  
- Persistent daemon running locally on configurable port/socket
- Support for device registration and management
- Proper session handling and persistence
- Media type validation and conversion
- Size limit enforcement per platform
- Reply correlation with metadata
- Group message support (planned)

## 7. Data Model  

- **MessageMapping**  
  - `whatsappChatId`, `whatsappMessageId`  
  - `signalMessageId`, `signalTimestamp`  
  - `forwardedAt`  
  - `deliveryStatus` (enum: sent, delivered, read, failed)
  - `mediaPath` (optional, for cached media references)

- **Database Schema Principles**
  - Simple indexing strategy focused on common lookup patterns:
    - Index on `whatsappMessageId` for reply correlation
    - Index on `signalMessageId` for delivery status updates
    - Composite index on `(whatsappChatId, forwardedAt)` for conversation retrieval
  - Keep schema minimal but sufficient for core functionality
  - Use SQLite with write-ahead logging for reliability and performance
  - Implement automated pruning of old records based on retention policy

- **Config (settings.json)**  
  ```json
  {
    "whatsapp": {
      "apiBaseUrl": "...",
      "webhookSecret": "...",
      "pollIntervalSec": 30
    },
    "signal": {
      "rpcUrl": "http://localhost:<port>",
      "authToken": null
    },
    "retry": {
      "initialBackoffMs": 1000,
      "maxBackoffMs": 60000,
      "maxAttempts": 5
    },
    "retentionDays": 30
  }
  ```

## 8. Error Handling & Retries  
- Wrap all outbound calls with retry logic and jitter  
- On unrecoverable errors, log and move on to avoid blocking other messages  
- Expose a dead-letter queue for manual inspection  

## 9. Testing Strategy  
- **Unit tests**: mock WhatsApp client, mock Signal-CLI JSON-RPC client, core logic  
- **Integration tests**: containerized end-to-end flows using lightweight HTTP stub for WhatsApp and a real Signal-CLI daemon instance  
- **Key Test Scenarios**:
  - Message format encoding/decoding validation
  - Metadata extraction and proper JSON formatting
  - Media download, conversion, and upload flows
  - Database read/write operations and query performance
  - Reply correlation mechanisms
  - Error handling and retry logic
  - Message delivery confirmation tracking
  - Connection disruption and recovery
- Code coverage target ≥80%  

## 10. Deployment  
- Dockerized service with multi-stage build, minimal base image (e.g. Alpine)  
- Expose ports: HTTP webhook, metrics  
- Deploy via container orchestrator (Docker Compose / Kubernetes)  

## 11. Future Enhancements  
- Group chat bridging  
- Multi-account support  
- Web UI for monitoring and configuration
- Enhanced authentication and security mechanisms:
  - Webhook endpoint authentication
  - Credential rotation 
  - Secure credential storage solutions
- Comprehensive metrics collection and visualization:
  - Message throughput statistics
  - Delivery success rates
  - Performance metrics (CPU/memory usage)
  - Latency measurements
- API version compatibility management

## 12. References
- WhatsApp REST API:
- https://github.com/devlikeapro/waha

- Signal CLI:
- https://github.com/AsamK/signal-cli

- Previous effort at bridging:
- https://github.com/meinto/whatsapp-signal-bridge

## 13. Legal Considerations
- Include appropriate disclaimers about unofficial API usage
- License under permissive open source license (e.g., MIT)
- Document compliance considerations for WhatsApp and Signal terms of service
