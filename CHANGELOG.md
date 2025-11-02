# Changelog

All notable changes to WhatSignal will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).


## [1.1.14]

### Fixed
- **Critical: Duplicate logging bug** - Fixed severe log corruption causing duplicate log entries for webhook requests
  - Root cause 1: Bridge service was creating its own unconfigured logger instance instead of using the shared application logger
  - Root cause 2: Webhook endpoints had both global `ObservabilityMiddleware` and `WebhookObservabilityMiddleware` applied, causing every webhook request to be logged twice
  - Solution: Modified `NewBridge()` to accept a logger parameter and pass the configured logger from main application
  - Solution: Removed global middleware from webhook routes, keeping only the webhook-specific middleware for better context
  - Impact: Eliminates duplicate log entries, ensures consistent log formatting across all components, and improves log readability
  - Updated all test files to pass test loggers to bridge constructor

## [1.1.13] - 2025-10-24

### Added
- **WhatsApp group chat support** - Messages from WhatsApp groups now forward to Signal with format "Sender in GroupName: message"
- **Group message reply support** - Signal users can reply to group messages, responses are sent back to the WhatsApp group
- **Comprehensive integration tests** - Added full test coverage for group message bidirectional flow and reply functionality

## [1.1.12] - 2025-10-07

### Fixed
- **Signal polling timeout errors** - Removed hardcoded 45s context timeout causing "context deadline exceeded" errors; fixed race condition in Start(); added ticker reset for consistent intervals
- **Trace IDs showing as all zeros** - Fixed trace ID generation when OpenTelemetry disabled; now generates unique legacy IDs
- **Type assertion panic** - Added proper type checking before asserting SignalClient type to prevent crashes
- **Goroutine leaks** - Fixed scheduler cleanup and Signal poller defer handling

### Added
- **Signal poller improvements** - Config validation, smart error classification (retryable vs non-retryable), jittered exponential backoff, graceful degradation tracking, enhanced metrics
- **Session monitoring** - Automatic detection/recovery of WhatsApp sessions stuck in STARTING status (configurable timeout: 30s default)
- **OpenTelemetry optimization** - Comprehensive refactor with nil logger protection, idempotent shutdown, config validation, 7 new test cases
- **SessionMonitor optimization** - Session name caching, O(1) status lookups, fixed race conditions, better goroutine lifecycle management

## [1.1.11] - 2025-10-06

### Fixed
- **Critical: Multi-session message forwarding** - Fixed Signal → WhatsApp message forwarding failures for non-default sessions
  - Issue: Messages from Signal to WhatsApp were failing with "context deadline exceeded" errors when sent from the 2nd (non-default) session
  - Root cause: WhatsApp session was stuck in "STARTING" status instead of "WORKING" status in WAHA API
  - Added session status validation before sending messages to prevent timeouts when sessions aren't ready
  - Implemented `GetSessionStatusByName()` and `validateSessionStatus()` methods to check session health
  - Improved error messages to indicate session status issues: "session 'X' is not ready (status: Y). Please ensure the WhatsApp session is authenticated and in WORKING state"
  - Increased default WhatsApp API timeout from 10 seconds to 30 seconds to handle slow WAHA responses during session initialization or high load
  - Added session-aware logging to include session name in all Signal-to-WhatsApp message forwarding operations
  - Updated documentation to reflect new 30-second default timeout value

## [1.1.10] - 2025-10-04

### Fixed
- **Critical: Bidirectional media forwarding** - Fixed WhatsApp → Signal media forwarding failure in Docker deployments
Docker internal hostname and the port matches
  - Added special handling for Docker deployments where services use internal hostnames but generate URLs with external IPs
  - Extended validation to support both WAHA and Signal-CLI services on the same host with different ports
  - Security: Only allows IP addresses or Docker internal hostnames with matching ports, blocks external domains
  - Added comprehensive test coverage for Docker hostname/external IP scenarios

## [1.1.9] - 2025-09-30

### Fixed
- **Media download IP validation** - Fixed private IP rejection for self-hosted WAHA deployments
  - Issue: Media downloads from WAHA running on private IPs (192.168.x.x, 10.x.x.x) were incorrectly rejected

## [1.1.9] - 2025-09-27

### Testing Infrastructure
- **Comprehensive integration test suite** - Complete end-to-end testing framework with real components
  - Added `integration_test/` directory with test environment management, fixtures, and utilities
  - Multiple testing modes: mock services (fast), Docker services (comprehensive), performance benchmarks
  - Real component testing: SQLite databases, HTTP servers, file systems, network operations
  - CI/CD integration with dedicated GitHub Actions workflow for integration tests
  - Makefile targets: `test-integration`, `test-integration-docker`, `test-integration-perf`
  - Comprehensive test data fixtures and scenarios for multi-channel message routing
  - Docker Compose setup for external services (Signal-CLI, WAHA, PostgreSQL, Redis, Prometheus)
  - Proper resource isolation, cleanup, and artifact collection for debugging
  - **Fixed database migration handling** - Robust migration path resolution for test environments
  - **Security hardening** - Fixed file permissions (0750 for dirs, 0600 for files) and error handling

## [1.1.9] - 2025-09-26

### Health Monitoring & Metrics
- **Enhanced health check endpoints** - Comprehensive dependency health monitoring (G12)
  - Added health check methods to database, WhatsApp API, and Signal API clients
  - Enhanced `/health` endpoint with individual dependency status reporting
  - Returns structured JSON with overall health status and individual service states
  - Proper HTTP status codes (200 for healthy, 503 for unhealthy dependencies)
  - Full test coverage for all health check scenarios including mock implementations
- **Cache performance metrics** - Advanced cache monitoring and observability (G15)
  - Added contact cache hit/miss/refresh metrics using internal metrics system
  - Metrics: `contact_cache_hits_total`, `contact_cache_misses_total`, `contact_cache_refreshes_total`
  - Integrated with existing metrics registry for centralized collection
  - Full test coverage with comprehensive cache behavior validation

### Go-Specific Issues
- **Structured error handling system** - Comprehensive error management with codes and context
  - Added `internal/errors/types.go` with `AppError` type supporting error codes, causes, context, and retry flags
  - Added `internal/errors/helpers.go` with convenience functions for common error scenarios (API, database, validation, etc.)
  - Added HTTP status code mapping and standardized JSON error responses
  - Full test coverage with 15+ test cases validating error creation, context handling, and serialization
- **Feature flags system** - Runtime feature toggling for safer deployments
  - Added `internal/features/flags.go` with thread-safe flag management and 16 predefined flags
  - Added `internal/features/config.go` supporting environment variables with `WHATSIGNAL_FEATURE_` prefix
  - Categories: core, API, security, and experimental features with global enable/disable overrides
  - Full test coverage with 10+ test cases validating flag operations, configuration loading, and concurrency safety
- **API versioning strategy** - Semantic versioning with backward compatibility
  - Added `internal/versioning/version.go` with `APIVersion` struct supporting semantic versioning and feature tracking
  - Added `internal/versioning/middleware.go` with HTTP middleware for version negotiation via headers and URL paths
  - Support for version compatibility checking, feature gates, and deprecation warnings
  - Full test coverage with 25+ test cases validating version parsing, compatibility, and middleware behavior

## [1.1.8] - 2025-09-25

- **Exponential backoff retry logic** - Implemented configurable exponential backoff utility with jitter support
  - Added `internal/retry/backoff.go` with comprehensive retry logic including context cancellation and custom retry predicates
  - Replaced linear database connection backoff with exponential backoff in main.go
  - Supports configurable initial delay, max delay, multiplier, max attempts, and jitter options
  - Full test coverage with 9 test cases validating all backoff scenarios
- **Enhanced request/response logging middleware** - Created detailed debugging middleware with privacy protection
  - Added `internal/middleware/detailed_logging.go` with configurable request/response capture
  - Privacy-aware logging with configurable body size limits and sensitive header masking
  - Supports selective endpoint skipping and comprehensive debugging capabilities
- **OpenTelemetry distributed tracing support** - Full integration with Jaeger and stdout exporters
  - Added `internal/tracing/opentelemetry.go` with TracingManager for centralized trace management
  - Integration with existing legacy tracing system for backward compatibility
  - Configurable exporters (jaeger, stdout, console) with proper resource attribution
  - Context propagation and span management throughout the application
- **Package structure violations fixed** - Resolved architectural import violations
  - Created `pkg/constants/` package with shared defaults and MIME types to eliminate internal imports
  - Moved CircuitBreaker from internal to `pkg/circuitbreaker/` for proper architectural layering
  - Fixed pkg->internal import violations while maintaining clean architecture boundaries
- **Comprehensive performance benchmarks** - Added benchmark coverage for all critical components
  - Created `internal/retry/backoff_bench_test.go` with 8 backoff performance benchmarks
  - Created `internal/database/database_bench_test.go` with 9 database operation benchmarks
  - Created `pkg/circuitbreaker/circuit_breaker_bench_test.go` with 10 circuit breaker benchmarks
  - Benchmarks cover success/failure scenarios, concurrent access, state transitions, and mixed operations

### Code Quality and Type Safety Improvements (Go-Specific Issues G1-G5, G16-G17)
- **Reduced interface{} usage** by ~75% to improve type safety
  - Replaced `map[string]interface{}` with proper struct types in metrics system (`MetricsSnapshot`)
  - Created concrete request types for WhatsApp API (`SeenRequest`, `TypingRequest`, `SendMessageRequest`, `MediaMessageRequest`)
  - Eliminated unsafe map indexing patterns in test code
- **Fixed ignored error handling** - Added proper error logging for previously ignored errors
  - Fixed file cleanup errors in media handler with proper error logging
  - Added error logging for HTTP response body Close() failures
  - Improved error handling in typing cleanup operations
- **Added missing struct field tags** for better serialization and validation
  - Added JSON and validation tags to `ClientConfig` struct with proper validation rules
  - Added JSON tags to `RequestInfo` struct for debugging/logging support
  - Added JSON tags to error types (`ValidationError`, `ConfigError`) for API responses
- **Enhanced rate limiting with client feedback headers**
  - Added standard rate limit headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`
  - Added `Retry-After` header for rate-limited requests
  - Implemented `RateLimitInfo` struct for detailed rate limit status
- **Improved request/response size limits** for better DoS protection
  - Extended size limits to all request methods with bodies (POST, PUT, PATCH)
  - Enhanced content-type validation for webhook endpoints
  - Improved error messages and logging for oversized requests

### Security
- Enhanced rate limiting feedback improves client behavior and debugging
- Extended request size limits provide better protection against DoS attacks
- Improved error handling reduces silent failures and potential security issues

### Changed
- Metrics API now returns structured `MetricsSnapshot` instead of `map[string]interface{}`
- WhatsApp client uses typed request structs instead of generic maps
- Rate limiter provides detailed limit information through `AllowWithInfo()` method
- Security middleware validates content-type for more endpoint types

## [1.1.8] - 2025-09-25

### Code Quality
- Consolidated hard-coded default values into centralized constants
- Eliminated code duplication in media type detection with shared MediaRouter abstraction
- Standardized logging patterns and field names across the codebase
- Added comprehensive configuration validation for numeric ranges and retry settings
- Made encryption salts configurable via environment variables (backward compatible)
- Eliminated duplicate media processing wrapper methods in bridge service
- Implemented parallel contact sync with bounded concurrency for faster startup
- Added retry logic for database operations with exponential backoff and smart error detection
- **Configurable cleanup intervals** - Database cleanup scheduler interval now configurable via `server.cleanupIntervalHours`
- **Configuration hot-reload capability** - Added polling-based configuration watcher for runtime config changes
- **Extracted webhook JSON field constants** - Centralized hardcoded JSON field names in webhook processing

### Observability and Monitoring  
- **Comprehensive metrics collection system** with counters, timers, and gauges for operational insights
- **Request tracing and correlation IDs** for debugging multi-session deployments and request flows
- **Privacy-aware structured logging** with automatic masking of sensitive data (phone numbers, chat IDs, message IDs)
- **HTTP middleware for automatic instrumentation** of all API endpoints with timing and status metrics
- **Webhook-specific observability** with dedicated metrics for processing time, success/failure rates
- **Signal polling metrics** with retry attempts, success rates, and timing measurements
- **Metrics endpoint** (`/metrics`) exposing operational data in JSON format for monitoring systems
- **Enhanced message processing logging** with tracing context and privacy-compliant field masking

### Added
- `internal/metrics/` - Lightweight embedded metrics system with counters, timers, and percentiles
- `internal/tracing/` - Request correlation and distributed tracing support
- `internal/privacy/` - Privacy-aware data masking utilities for sensitive information
- `internal/middleware/` - HTTP observability middleware for automatic request instrumentation
- `internal/config/watcher.go` - Configuration hot-reload capability with polling-based file watching
- Metrics for webhook requests, failures, Signal polls, and message processing operations
- Request tracing with unique correlation IDs propagated through all operations
- Privacy masking for phone numbers, chat IDs, message IDs, and session names in logs
- Configurable cleanup scheduler interval via `server.cleanupIntervalHours` configuration
- Webhook JSON field name constants in `internal/models/webhooks.go`

### Changed
- File permissions and PBKDF2 iterations now use named constants instead of magic numbers
- Media type detection logic centralized in `internal/media/router.go` 
- Logging field names standardized (e.g., `ip` → `remote_ip`) with documented standards in `internal/service/logging_standards.go`
- Encryption salts can now be set via `WHATSIGNAL_ENCRYPTION_SALT` and `WHATSIGNAL_ENCRYPTION_LOOKUP_SALT`
- HTTP endpoints now include observability middleware for automatic metrics collection and request tracing
- Bridge service message processing enhanced with structured logging and performance metrics
- Cleanup scheduler constructor now accepts configurable interval hours parameter
- Scheduler service updated to use configurable cleanup intervals instead of hardcoded constants

## [1.1.7] - 2025-09-24

### Fixed
- **Critical**: Fixed unbounded memory growth in rate limiter with periodic cleanup mechanism
- **Critical**: Added database connection pooling to prevent connection exhaustion under load

### Added
- Externalized all hard-coded configuration values with sensible defaults
- Comprehensive input validation and bounds checking for all numeric configurations
- Circuit breaker pattern for external API calls (WhatsApp/Signal APIs)
- Graceful degradation for external service failures with fallback to cached data
- Structured error handling with error codes and retryable flags
- Docker volume path configuration via environment variable

### Security
- Fixed potential goroutine leaks in Signal attachment downloads
- Added proper context cancellation and cleanup for all background operations
- Enhanced rate limiter with configurable limits and cleanup intervals

### Changed
- Server timeouts now configurable (read/write/idle)
- Database connection pool settings externalized
- Rate limiting configuration moved to config file
- All magic numbers replaced with named constants

## [1.1.6] - 2025-09-22

### Fixed
- **Critical**: Fixed voice message relay failure in Docker deployments
  - Resolved "failed to process media: resolved to disallowed IP: 172.x.x.x" error
  - Enhanced URL rewriting to handle Docker internal network IPs (172.x.x.x/12 range)
  - Updated media validation to allow Docker internal IPs when they match WAHA port and external WAHA host is configured
  - Voice messages from WhatsApp now properly forward to Signal in containerized environments

### Security
- Maintained security restrictions while allowing legitimate Docker internal network access
- Docker internal IPs are only allowed when:
  - They have the same port as the configured WAHA base URL
  - The WAHA base URL is a real IP address (not a domain name)
  - The WAHA IP is not a loopback address

### Testing
- Added comprehensive tests for Docker internal IP handling in media processing
- Added tests for URL rewriting functionality with Docker internal networks
- Added validation tests for Docker deployment scenarios

### Build
- Fixed Docker build failures caused by Alpine package repository connectivity issues
- Enhanced Dockerfile with proper repository configuration and error handling
- Improved build reliability for multi-platform Docker images (linux/amd64, linux/arm64)

### Configuration
- Added default timeout (10 seconds) and retry count (3) for WhatsApp API requests
- Enhanced error messages for contact sync failures to indicate missing API key
- Improved session status validation before attempting contact sync
- Added better diagnostics for WAHA API authentication issues

### Network Security
- **CRITICAL FIX**: Resolved Docker network access restrictions blocking WAHA and Signal API connections
- Implemented intelligent Docker internal hostname detection (single-word hostnames without dots)
- Enhanced media URL rewriting to automatically handle any Docker internal hostname
- Updated URL validation to allow Docker internal hostnames that will be rewritten to external hosts
- Added configurable environment variable overrides (WHATSAPP_API_URL, SIGNAL_RPC_URL) for external access
- Removed hardcoded service names in favor of heuristic-based detection
- Fixed issue where applications running in Docker containers couldn't access external services due to internal network restrictions

## [1.1.5] - 2025-09-04

### Testing
- Coverage improvements and new tests:
  - Verified and extended `pkg/whatsapp/client_media_test.go` covering all media session methods, error paths, and payload validation
  - Confirmed comprehensive SSRF tests in `pkg/media/validate_url_test.go`
  - Confirmed Signal attachment processing tests exist in `pkg/signal/client_attachment_test.go`
  - Added `cmd/whatsignal/getclientip_test.go` covering client IP extraction scenarios
  - Verified extensive migration parser tests in `internal/migrations/migrations_test.go`
  - Added `internal/service/message_service_thread_test.go` to validate GetMessageThread behavior and error handling
  - Extended `internal/service/session_monitor_test.go` with unhealthy status triggers, rapid state changes, and wait error cases
  - Added `pkg/whatsapp/client_core_edge_test.go` to harden RestartSession, GetAllContacts, SendTextWithSession optional flows, and sendReactionRequest

  - Added decrypt-error coverage tests for DB queries in `internal/database/database_edge_cases_test.go` (SignalID, LatestByChatID, Latest)

### Security
- Strengthened rate limiting accuracy by testing client IP extraction across proxy scenarios (XFF, X-Real-IP, IPv4/IPv6, malformed headers)


## [1.1.4] - 2025-09-03

### Testing
- **Comprehensive test coverage improvements**: Added extensive unit tests for critical zero-coverage functions
  - Created `pkg/whatsapp/client_media_test.go` with 28+ test cases for media session functions (SendImageWithSession, SendVoiceWithSession, SendVideoWithSession, SendDocumentWithSession)
  - Created `pkg/media/validate_url_test.go` with 60+ security test cases for URL validation and SSRF protection
  - Created `pkg/signal/client_attachment_test.go` with comprehensive attachment processing tests including HTTP downloads, timeout handling, and integration scenarios
  - Added 50+ test cases for migration number parser in `internal/migrations/migrations_test.go`
  - All new tests validate happy paths, error conditions, edge cases, security concerns, and boundary conditions

### Changed
- **Database migrations**: Simplified migration system to single initial schema
  - Removed `002_add_session_name.sql` and integrated session_name column into initial schema
  - Consolidated migration logic for cleaner deployment
- **Docker Compose**: Updated volume mounts for better organization
  - Changed data volume from `./data/whatsignal:/app/data` to `./data:/app/data`
  - Ensures consistent directory structure across deployments
- **Migration tool**: Removed standalone `cmd/migrate/main.go` as migrations are now handled automatically

### Fixed
- **Test infrastructure**: Fixed server setup bug in attachment extraction tests
  - Corrected condition for unified server creation when testing failed downloads
  - Failed attachment downloads now correctly return empty attachment lists instead of creating empty files
- **Database tests**: Enhanced edge case testing for message mapping queries
  - Added comprehensive tests for GetLatestMessageMappingByWhatsAppChatID
  - Improved coverage for empty database scenarios and concurrent access patterns

### Security
- **Media URL validation**: Achieved comprehensive test coverage (60+ tests) for SSRF protection mechanisms
  - Tests validate scheme restrictions, host validation, IP literal blocking, DNS resolution checks
  - Security bypass attempt tests including URL encoding, redirects, and obfuscation techniques

### Documentation
- **Docker security**: Updated security documentation to reflect current volume mount structure
- **README**: Updated installation instructions with current docker-compose configuration

## [1.1.3] - 29-08-2025

### Security
- **Request Size Limits**: Added comprehensive request size validation with configurable limits (default 5MB)
- **Session Handling**: Removed legacy default-session fallback and deprecated handlers; explicit session names are now required (breaking change)
- **Docker Security**: Complete Docker image hardening with distroless base, pinned SHAs, and security-first deployment
  - Migrated to distroless image (gcr.io/distroless/static-debian12:nonroot)
  - Pinned base images with SHA digests for supply-chain security
  - Added read-only filesystem with minimal writable mounts
  - Implemented capability dropping, non-root user (uid=65532), and resource constraints

### CI/CD
- **Security Gates**: Comprehensive GitHub Actions workflow with security scanning
  - Added gosec, govulncheck, golangci-lint, staticcheck integration
  - Dependency scanning with vulnerability checks
  - Docker image security scanning with Trivy
  - License compliance checking
- **Makefile**: Enhanced with security, coverage, and CI targets

### Testing
- **Test Stability**: Fixed timeout issues in test suite
  - Added WHATSIGNAL_TEST_MODE environment variable to skip typing delays in tests
  - Optimized database tests (reduced TestDatabase_LargeDataSet from 10k to 500 records)
  - Fixed rate limiter test timing issues
- **Coverage**: All database tests now pass; working toward 95% coverage target

### Fixed
- Eliminated data race between Server.Start and Server.Shutdown by guarding server pointer with RWMutex; test suite now passes with -race and CI build-and-test gate is green

### Documentation
- **Security Guide**: Added comprehensive Docker security documentation (docs/docker-security.md)
- **Deployment**: Created security-hardened docker-compose.security.yml with best practices
- **Build Toolchain**: Upgraded Go toolchain to 1.24.6 in go.mod, CI, and Dockerfile to address stdlib advisory GO-2025-3849 and keep scanners green

## [1.1.2] - 28-08-2025

### Security
- Webhook hardening for WAHA:
  - Enforced request body size limits via http.MaxBytesReader (default 5MB; configurable)
  - Added timestamp skew validation for replay protection (default 300s; configurable)
  - Removed raw webhook body logging on decode errors; only logs body length at debug level

- Database encryption safety:
  - Encryption is now mandatory; WHATSIGNAL_ENCRYPTION_SECRET (>=32 chars) is required at runtime; plaintext mode removed
  - Introduced HMAC-SHA256 lookup hashing with independent derived key (EncryptionLookupSalt)
  - Stored values continue to use random-nonce AES-GCM; removed deterministic lookup encryption
  - Updated DB layer to store and query via *_hash fields; added indexes

- Media SSRF prevention:
  - Added URL validation for media downloads; only allow WAHA base host
  - Blocked IP literals and private/loopback/link-local resolutions
  - Preserved testability by allowing httptest servers when WAHA base URL is unset

### Changed
- WhatsApp client logging:
  - Replaced fmt.Printf with structured logrus logs (warn/info/debug) and sanitized fields
  - Removed commented debug fmt.Printf blocks for payload/endpoint
- cmd/migrate: swapped fmt.Print* for log.Print* for consistency
- Version CLI output consolidated into a single fmt.Printf (intentional stdout behavior)

### Tests
- Added test to ensure invalid JSON webhook does not log raw body content
- Extended signature/TS skew tests for webhooks
- Full build and test verification

## [1.1.1] - 28-08-2025

### Security
- Added replay protection for WAHA webhooks with timestamp skew validation
  - New server.webhookMaxSkewSec config (default 300 seconds)
  - Backwards-compatible env override WHATSIGNAL_WEBHOOK_MAX_SKEW_SEC still honored
- Hardened database encryption initialization
  - If encryption disabled and no secret, DB operates in plaintext without error
  - If encryption enabled but secret missing, startup errors as before

### Documentation
- Updated configuration guide with new server.webhookMaxSkewSec option

## [1.1.0] - 26-06-2025

### Security
- **CRITICAL**: Enhanced input validation and sanitization across all endpoints
  - Added comprehensive phone number validation (E.164 format required)
  - Added message ID validation with length limits and character restrictions
  - Added session name validation with alphanumeric-only requirements
  - Enhanced webhook payload validation for both WhatsApp and Signal endpoints
- **HIGH**: Improved file path security and directory traversal protection
  - Enhanced path validation in all file operations
  - Added validation to media handler, signal client, and configuration loader
  - Improved file permissions (0600 for files, 0750 for directories)
- **MEDIUM**: Added rate limiting and security middleware
  - Implemented IP-based rate limiting (100 requests/minute per IP)
  - Added security headers (X-Content-Type-Options, X-Frame-Options, etc.)
  - Enhanced webhook authentication with better error handling
- **LOW**: Improved error handling and logging security
  - Fixed unhandled errors in HTTP response handling
  - Enhanced random number generation with proper error checking
  - Added security-focused logging with IP tracking

### Added
- **WAHA version detection**: Intelligent handling of video messages based on WAHA version
  - Automatic detection of WAHA Plus vs Core for video compatibility
  - Version-specific fallback logic for unsupported video formats in WAHA Core
  - Caching of version detection results for performance
- **Session-aware messaging**: All messaging operations now support session context
  - Session-specific message sending for better channel management
  - Enhanced error handling with session context
  - Improved test coverage for session-based operations
- **Signal CLI simplification**: Removed authentication token requirement
  - Simplified Signal CLI configuration (no auth tokens needed)
  - Updated documentation and examples
  - Cleaner REST API integration
- **Privacy protection system**: Comprehensive logging sanitization
  - Phone number sanitization in logs (show only last 4 digits)
  - Message ID truncation for privacy
  - Content hiding in non-verbose mode
  - WhatsApp message ID sanitization with phone number masking
- **Security validation functions**: New comprehensive input validation system
  - `ValidatePhoneNumber()` - E.164 format validation
  - `ValidateMessageID()` - Length and character validation
  - `ValidateSessionName()` - Alphanumeric validation
- **Rate limiting system**: IP-based request throttling for webhook endpoints
- **Security middleware**: Comprehensive security headers and content-type validation
- **Enhanced configuration security**: Production mode validation and webhook secret requirements

### Changed
- **Video message handling**: Automatic fallback from video to document for WAHA Core
  - Smart detection prevents 422 errors when sending videos to unsupported WAHA versions
  - Maintains functionality across different WAHA deployment types
- **Test data sanitization**: All test cases now use safe placeholder data
  - Removed real phone numbers and IP addresses from test files
  - Added consistent test patterns for privacy protection
- **Encryption implementation**: Added security annotations for deterministic encryption
  - Documented intentional use of deterministic nonces for searchable encryption
  - Added #nosec annotations with explanations for legitimate security patterns
- **Test data**: Updated all test cases to use realistic phone numbers and validation-compliant data
- **Configuration validation**: Enhanced security checks for production deployments

### Fixed
- **Session-aware method implementations**: All mock interfaces updated for compatibility
  - Fixed missing session-aware methods in test mocks
  - Resolved interface compliance issues in unit tests
  - Enhanced test coverage for session-based functionality
- **Authorization header handling**: Updated Signal CLI tests to remove auth token expectations
  - Fixed test failures related to removed authentication requirements
  - Consistent handling across all Signal CLI operations
- **Channel manager initialization**: Fixed nil pointer errors in tests
  - Proper channel configuration in test setups
  - Enhanced error handling for channel operations
- **File permissions**: Changed from 0755/0644 to 0750/0600 for better security
- **Error handling**: Fixed multiple unhandled error cases identified by security analysis
- **Input validation**: Resolved all input validation gaps in webhook handlers
- **Security annotations**: Added #nosec annotations with explanations for all false positive security warnings

## [1.0.0] - 25-06-2025

### Added
- **Code organization and maintainability improvements**: Major refactoring to improve code structure and constants management
  - Created centralized constants package (`internal/constants/`) for all hardcoded values
  - Added `defaults.go` with numeric constants, timeouts, size limits, and conversion factors
  - Added `mime_types.go` with MIME type mappings, file extensions, and content type detection
  - Added `queries.go` with all SQL queries as named constants for better maintainability
  - Moved default file type arrays to constants for configuration validation

### Changed
- **SQL query organization**: Extracted all inline SQL queries from `database.go` into named constants
  - Improved code readability by removing large SQL blocks from functions
  - Enhanced maintainability with centralized query management
  - Consistent naming convention for all database queries
- **Constants consolidation**: Moved all magic numbers and hardcoded strings to centralized constants
  - Eliminated hardcoded timeout values (30 seconds, 1024*1024 bytes, etc.)
  - Centralized MIME type mappings and file extension handling
  - Moved default media type configurations to constants
- **Code quality improvements**: Removed debugging messages that were used for troubleshooting specific issues
  - Cleaned up production code by removing temporary debug logging
  - Preserved functional logging while removing troubleshooting artifacts

### Technical Debt Reduction
- **Breaking changes**: This is a major version bump due to significant internal restructuring
- **Improved maintainability**: All configuration values, SQL queries, and constants are now centrally managed
- **Enhanced developer experience**: Easier to modify timeouts, limits, and queries without hunting through code
- **Better testing**: Constants can be easily adjusted for testing scenarios

## [0.54.0] - 24-06-2025

### Added
- **Message deletion forwarding**: When a message is deleted in Signal, it is now automatically deleted in WhatsApp
  - Real-time deletion event detection from Signal CLI REST API
  - Proper handling of Signal `remoteDelete` events in message processing
  - Database lookup by Signal message ID for accurate message mapping
  - WAHA API integration for WhatsApp message deletion using correct message ID format
  - Comprehensive error handling and validation for deletion operations

### Fixed
- **WAHA API response parsing**: Fixed critical issue where WhatsApp message IDs were not being extracted properly
  - Updated response parsing to handle actual WAHA API format with nested `_serialized` field
  - Added support for both `id._serialized` and `_data.id._serialized` response structures
  - Proper extraction of WhatsApp message IDs in format `true_chatId@c.us_messageId`
  - Fixed empty message ID issue that prevented message deletion from working
- **Signal deletion event detection**: Enhanced Signal CLI message processing to detect `remoteDelete` events
  - Added `RemoteDelete` field to Signal REST message structure
  - Proper parsing of deletion timestamps and target message identification
  - Improved message filtering to process deletion events correctly

### Changed
- **Database queries**: Updated message deletion lookup to use `GetMessageMappingBySignalID` instead of generic mapping lookup
- **Response structures**: Added `WAHAMessageResponse` type to properly handle complex WAHA API responses
- **Error messages**: Improved validation and error reporting for message deletion operations

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

[1.1.0]: https://github.com/bikemazzell/whatsignal/releases/tag/v1.1.0
[1.0.0]: https://github.com/bikemazzell/whatsignal/releases/tag/v1.0.0
[0.54.0]: https://github.com/bikemazzell/whatsignal/releases/tag/v0.54.0
[0.53.0]: https://github.com/bikemazzell/whatsignal/releases/tag/v0.53.0
[0.51.0]: https://github.com/bikemazzell/whatsignal/releases/tag/v0.51.0
[0.50.0]: https://github.com/bikemazzell/whatsignal/releases/tag/v0.50.0