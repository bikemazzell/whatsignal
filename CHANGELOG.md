# Changelog

All notable changes to WhatSignal will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.53.0] - 24-06-2025

### Added
- **Signal voice recording detection**: Automatic detection of voice recordings without file extensions
  - File signature detection using binary headers (OGG "OggS" signature detection)
  - Content-based file type detection when file extensions are missing
  - Proper routing of Signal voice recordings to WhatsApp `/api/sendVoice` endpoint
- **Auto-reply to last sender**: When responding without quoting a message, automatically replies to the most recent WhatsApp contact
  - Database query optimization for finding latest message mappings
  - Eliminates "New thread creation is not yet supported" warnings for natural conversation flow
  - Maintains conversation context across multiple WhatsApp contacts
- **Message reactions forwarding**: Signal reactions (👍, ❤️, etc.) are now forwarded to WhatsApp
  - React and remove reactions between platforms
  - Proper message correlation for reaction targeting
  - Full emoji support for reaction forwarding

### Fixed
- **Signal voice recordings without extensions**: Files like `signal-attachments/P59DFxKqtUuf3KdZB2cp` now properly detected as voice messages
  - Fixed "file type . is not allowed" errors for extensionless voice recordings
  - Enhanced media validation to default unknown files to documents instead of rejecting them
  - Improved content detection algorithm with binary file signature recognition
- **Auto-reply logic**: Eliminated incorrect message routing when replying without quotes
  - Fixed database encryption issues in latest message mapping queries
  - Improved message threading for seamless conversation continuation
- **Test coverage**: Enhanced test suite for voice detection and auto-reply functionality

### Changed
- **Media handling**: Unknown file types now default to document processing instead of being rejected
- **Database interface**: Added `GetLatestMessageMapping` method for improved message correlation
- **Error handling**: More graceful fallback behavior for unsupported file types

## [0.51.0] - 23-06-2025

### Added
- Session health monitoring with automatic restart for WAHA disconnections
- Session status endpoint at `/session/status` for monitoring
- Configuration-driven timeouts and intervals
- Server timeout configuration (`readTimeoutSec`, `writeTimeoutSec`, `idleTimeoutSec`)
- Signal attachments directory configuration (`attachmentsDir`)
- Complete media support for Signal to WhatsApp: images, videos, documents, voice messages
- Automatic media type detection and proper WAHA endpoint routing
- **Photo attachment support for WhatsApp to Signal forwarding**
  - URL download functionality for WhatsApp media URLs
  - HTTP client with 30-second timeout for reliable downloads
  - Content-Type detection from HTTP headers and file extensions
  - Comprehensive caching system to prevent duplicate downloads
  - Support for JPEG, PNG, GIF, WebP, MP4, MOV, OGG, AAC, PDF, DOC, DOCX
- **Fixed Signal to WhatsApp photo forwarding**
  - Proper base64 encoding of attachment file data
  - Content-Type detection for all common file types
  - Filename extraction from full file paths
  - Comprehensive error handling for file operations
- **JFIF image format support** for WhatsApp media forwarding
- **Mutex synchronization** for Signal-CLI operations to prevent race conditions

### Fixed
- Contact sync startup failures when WAHA session not ready
- Session getting stuck in bad states (OPENING, STOPPED)
- Hardcoded timeout values replaced with configurable constants
- Signal attachment path resolution using absolute paths
- Media processing for Signal attachments (images, videos, voice, documents)
- **Critical bug: Signal attachments were sent with empty data field**
  - Signal client now properly base64 encodes file content
  - Content-Type detection works for all media types
  - Filename extraction from paths instead of sending full paths
- **WhatsApp media URL download failures**
  - Added robust error handling for network timeouts
  - Proper validation of downloaded file types and sizes
  - Fallback mechanisms for content type detection
- **CRITICAL: WhatsApp photo forwarding to Signal completely broken (401 authentication errors)**
  - Fixed WAHA media URL authentication by adding required `X-Api-Key` header
  - Added support for `.jfif` image format in media handler and Signal client
  - Corrected Signal-CLI REST API attachment format (simple base64 strings vs object format)
  - Increased HTTP client timeout to 60 seconds for long-polling operations
- **Signal polling breakdown after sending messages**
  - Added mutex synchronization to prevent concurrent Signal-CLI send/receive operations
  - Fixed race condition that caused "context deadline exceeded" errors
  - Ensured continuous bidirectional message flow without polling interruption

## [0.50.0] - 20-06-2025

### Added
- Initial release of WhatSignal
- One-to-one chat bridging between WhatsApp and Signal
- Smart contact management with name display
- Comprehensive media support (images, videos, documents, voice)
- Database encryption at rest
- Docker deployment with pre-built images
- Health monitoring endpoint with version information
- Automated setup and deployment scripts
- Contact caching and sync functionality
- Message reply correlation
- Configurable data retention
- Webhook authentication
- Path traversal protection

### Security
- Field-level database encryption
- Deterministic encryption for message lookups
- Non-root Docker containers
- Secure secret generation in deployment

[0.53.0]: https://github.com/bikemazzell/whatsignal/releases/tag/v0.53.0
[0.51.0]: https://github.com/bikemazzell/whatsignal/releases/tag/v0.51.0
[0.50.0]: https://github.com/bikemazzell/whatsignal/releases/tag/v0.50.0