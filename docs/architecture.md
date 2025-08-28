# WhatSignal Architecture

## Overview

WhatSignal is a bridge service that enables bidirectional communication between WhatsApp and Signal messaging platforms. It's built as a Go application with a modular, microservices-oriented architecture that ensures scalability, security, and reliability.

## System Architecture

### High-Level Architecture

```mermaid
graph TB
    subgraph "External Services"
        WA[WhatsApp Users]
        SIG[Signal Users]
    end
    
    subgraph "API Layer"
        WAHA[WAHA API<br/>WhatsApp HTTP API]
        SIGAPI[Signal-CLI REST API<br/>Signal Interface]
    end
    
    subgraph "WhatSignal Core"
        WS[WhatSignal Bridge Service]
    end
    
    subgraph "Data Layer"
        DB[(SQLite Database<br/>Encrypted Storage)]
        MEDIA[Media Cache<br/>File System]
        ATTACHMENTS[Signal Attachments<br/>File System]
    end
    
    WA <--> WAHA
    SIG <--> SIGAPI
    WAHA <--> WS
    SIGAPI <--> WS
    WS <--> DB
    WS <--> MEDIA
    WS <--> ATTACHMENTS
```

## Component Architecture

### Core Components

```mermaid
graph TD
    subgraph "API Layer"
        HTTP[HTTP Server<br/>Gorilla Mux]
        WH[Webhook Handler]
        RL[Rate Limiter]
        SEC[Security Middleware]
    end
    
    subgraph "Service Layer"
        MS[Message Service]
        BR[Message Bridge]
        CM[Channel Manager]
        CS[Contact Service]
        SP[Signal Poller]
        SM[Session Monitor]
        SCH[Scheduler]
    end
    
    subgraph "Client Layer"
        WAC[WhatsApp Client]
        SC[Signal Client]
        MH[Media Handler]
    end
    
    subgraph "Data Access Layer"
        DB[Database Service]
        ENC[Encryption Service]
        CACHE[Media Cache]
    end
    
    HTTP --> WH
    HTTP --> RL
    HTTP --> SEC
    WH --> MS
    MS --> BR
    BR --> CM
    BR --> CS
    MS --> SP
    MS --> SM
    MS --> SCH
    BR --> WAC
    BR --> SC
    BR --> MH
    MS --> DB
    DB --> ENC
    MH --> CACHE
```

## Message Flow

### WhatsApp to Signal Flow

```mermaid
sequenceDiagram
    participant WU as WhatsApp User
    participant WAHA as WAHA API
    participant WH as Webhook Handler
    participant MS as Message Service
    participant BR as Bridge
    participant CM as Channel Manager
    participant CS as Contact Service
    participant SC as Signal Client
    participant SU as Signal User
    
    WU->>WAHA: Send Message
    WAHA->>WH: POST /webhook/whatsapp
    WH->>WH: Validate HMAC Signature
    WH->>MS: HandleWhatsAppMessage()
    MS->>CS: GetContactDisplayName()
    CS-->>MS: Contact Name
    MS->>CM: GetSignalDestination()
    CM-->>MS: Destination Number
    MS->>BR: ForwardToSignal()
    BR->>SC: SendMessage()
    SC->>SU: Deliver Message
    BR->>MS: Save Mapping
    MS->>MS: Store in Database
```

### Signal to WhatsApp Flow

```mermaid
sequenceDiagram
    participant SU as Signal User
    participant SC as Signal Client
    participant SP as Signal Poller
    participant MS as Message Service
    participant BR as Bridge
    participant CM as Channel Manager
    participant WAC as WhatsApp Client
    participant WAHA as WAHA API
    participant WU as WhatsApp User
    
    SU->>SC: Send Message
    SP->>SC: PollMessages()
    SC-->>SP: New Messages
    SP->>MS: ProcessSignalMessage()
    MS->>CM: GetWhatsAppSession()
    CM-->>MS: Session Name
    MS->>MS: Check Reply Context
    MS->>BR: ForwardToWhatsApp()
    BR->>WAC: SendMessage()
    WAC->>WAHA: API Request
    WAHA->>WU: Deliver Message
    BR->>MS: Save Mapping
    MS->>MS: Store in Database
```

## Directory Structure

```
whatsignal/
├── cmd/                      # Application entry points
│   ├── whatsignal/          # Main application
│   │   ├── main.go          # Entry point
│   │   ├── server.go        # HTTP server
│   │   └── security.go      # Security middleware
│   └── migrate/             # Database migration tool
│
├── internal/                 # Private application code
│   ├── config/              # Configuration management
│   ├── constants/           # Application constants
│   ├── database/            # Database layer
│   │   ├── database.go      # DB operations
│   │   ├── encryption.go    # Data encryption
│   │   └── queries.go       # SQL queries
│   ├── migrations/          # Database migrations
│   ├── models/              # Data models
│   ├── security/            # Security utilities
│   └── service/             # Business logic
│       ├── bridge.go        # Message bridging
│       ├── channel_manager.go # Multi-channel support
│       ├── contact_service.go # Contact management
│       ├── message_service.go # Message handling
│       ├── scheduler.go     # Background tasks
│       ├── session_monitor.go # Session health
│       └── signal_poller.go # Signal message polling
│
├── pkg/                      # Public packages
│   ├── media/               # Media handling
│   ├── signal/              # Signal client
│   │   ├── client.go        # Signal API client
│   │   └── types/           # Signal types
│   └── whatsapp/            # WhatsApp client
│       ├── client.go        # WAHA API client
│       ├── session.go       # Session management
│       ├── webhook.go       # Webhook handling
│       └── types/           # WhatsApp types
│
├── scripts/                  # Deployment scripts
├── docs/                     # Documentation
└── docker-compose.yml        # Container orchestration
```

## Data Model

### Core Database Schema

```mermaid
erDiagram
    MESSAGE_MAPPINGS {
        string id PK
        string whatsapp_chat_id
        string whatsapp_msg_id
        string signal_msg_id
        timestamp signal_timestamp
        timestamp forwarded_at
        string delivery_status
        string media_path
        string session_name
    }
    
    CONTACTS {
        string id PK
        string phone_number
        string name
        string push_name
        string encrypted_name
        timestamp last_updated
        string session_name
    }
    
    MESSAGE_MAPPINGS ||--o| CONTACTS : "references"
```

## Security Architecture

### Security Layers

```mermaid
graph TD
    subgraph "External Security"
        HMAC[HMAC Webhook Validation]
        RATE[Rate Limiting]
        PATH[Path Traversal Protection]
    end
    
    subgraph "Application Security"
        AUTH[API Key Authentication]
        VAL[Input Validation]
        SAN[Data Sanitization]
    end
    
    subgraph "Data Security"
        ENC[Database Encryption at Rest]
        DET[Deterministic Encryption<br/>for Lookups]
        CLEAN[Automated Data Cleanup]
    end
    
    HMAC --> AUTH
    RATE --> AUTH
    PATH --> AUTH
    AUTH --> VAL
    VAL --> SAN
    SAN --> ENC
    ENC --> DET
    DET --> CLEAN
```

## Deployment Architecture

### Container Architecture

```mermaid
graph LR
    subgraph "Docker Network"
        subgraph "WhatSignal Container"
            WS[WhatSignal<br/>Port 8082]
            VOL1[Config Volume]
            VOL2[Data Volume]
        end
        
        subgraph "WAHA Container"
            WAHA[WAHA API<br/>Port 3000]
            VOL3[WAHA Data]
        end
        
        subgraph "Signal-CLI Container"
            SIG[Signal-CLI API<br/>Port 8080]
            VOL4[Signal Data]
        end
    end
    
    WS <--> WAHA
    WS <--> SIG
    VOL1 -.-> WS
    VOL2 -.-> WS
    VOL3 -.-> WAHA
    VOL4 -.-> SIG
```

## Key Design Decisions

### 1. **Microservices Architecture**
- Separate concerns between WhatsApp (WAHA), Signal (Signal-CLI), and bridge logic
- Each service can be scaled and maintained independently
- Clear API boundaries between components

### 2. **Multi-Channel Support**
- Channel Manager maps WhatsApp sessions to Signal destinations
- Supports multiple concurrent WhatsApp-Signal conversation pairs
- Session-based routing for enterprise deployments

### 3. **Database Design**
- SQLite for simplicity and portability
- Encrypted storage for sensitive data
- Deterministic encryption for efficient lookups
- Message mapping table for bidirectional correlation

### 4. **Media Handling**
- Centralized media cache with automatic cleanup
- Support for multiple file formats (images, videos, documents, voice)
- Intelligent WAHA version detection for video compatibility
- Binary file type detection using content signatures

### 5. **Security First**
- HMAC validation for webhooks
- Rate limiting on API endpoints
- Path traversal protection
- Database encryption at rest
- Automated data retention policies

### 6. **Reliability Features**
- Message delivery tracking
- Retry mechanisms with exponential backoff
- Session health monitoring
- Graceful error handling
- Comprehensive logging

### 7. **Performance Optimizations**
- Contact caching (24-hour TTL)
- Startup contact sync
- Connection pooling
- Concurrent message processing
- Efficient database indexing

## Integration Points

### External APIs

1. **WAHA API (WhatsApp)**
   - REST API for WhatsApp operations
   - Webhook for incoming messages
   - Session management endpoints
   - Media upload/download

2. **Signal-CLI REST API**
   - REST API for Signal operations
   - Message sending/receiving
   - Attachment handling
   - Device registration

### Internal APIs

1. **Health Check Endpoint**
   - `/health` - System status
   - `/session/status` - Session health

2. **Webhook Endpoints**
   - `/webhook/whatsapp` - WAHA webhooks
   - HMAC signature validation
   - Rate limiting protection

## Scalability Considerations

1. **Horizontal Scaling**
   - Stateless application design
   - Database can be migrated to PostgreSQL
   - Media storage can use S3/object storage

2. **Performance**
   - Concurrent message processing
   - Efficient caching strategies
   - Optimized database queries
   - Connection pooling

3. **Monitoring**
   - Structured JSON logging
   - Health check endpoints
   - Metrics collection ready
   - Error tracking and alerting

## Future Architecture Considerations

1. **Message Queue Integration**
   - Add Redis/RabbitMQ for async processing
   - Better handling of high message volumes
   - Improved reliability with message persistence

2. **API Gateway**
   - Centralized authentication
   - Rate limiting at edge
   - Request routing and load balancing

3. **Observability**
   - OpenTelemetry integration
   - Distributed tracing
   - Metrics aggregation
   - Log centralization

4. **Multi-Region Support**
   - Database replication
   - CDN for media files
   - Regional failover

## Technology Stack

- **Language**: Go 1.22+
- **Web Framework**: Gorilla Mux
- **Database**: SQLite (with encryption)
- **Container**: Docker & Docker Compose
- **External Services**: WAHA, Signal-CLI REST API
- **Security**: HMAC, AES encryption, bcrypt
- **Logging**: Logrus (structured JSON)
- **Testing**: Go testing package (>80% coverage)