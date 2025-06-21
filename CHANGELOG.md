# Changelog

All notable changes to WhatSignal will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.51.0] - 21-06-2025

### Added
- Session health monitoring with automatic restart for WAHA disconnections
- Session status endpoint at `/session/status` for monitoring
- Configuration-driven timeouts and intervals
- Server timeout configuration (`readTimeoutSec`, `writeTimeoutSec`, `idleTimeoutSec`)

### Fixed
- Contact sync startup failures when WAHA session not ready
- Session getting stuck in bad states (OPENING, STOPPED)
- Hardcoded timeout values replaced with configurable constants

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

[0.50.0]: https://github.com/bikemazzell/whatsignal/releases/tag/v0.50.0