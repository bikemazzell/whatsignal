package constants

// Default polling configuration values
const (
	DefaultSignalPollIntervalSec = 5
	DefaultSignalPollTimeoutSec  = 10
	DefaultRetryBackoffMs        = 1000
	DefaultMaxBackoffMs          = 60000
	DefaultMaxAttempts           = 5
	DefaultRetentionDays         = 30
	DefaultServerPort            = 8082
)

// Default media configuration values
const (
	DefaultMaxImageSizeMB    = 5
	DefaultMaxVideoSizeMB    = 100
	DefaultMaxDocumentSizeMB = 100
	DefaultMaxVoiceSizeMB    = 16
)

// Default timeout values
const (
	DefaultHTTPTimeoutSec             = 30
	DefaultDatabaseRetryAttempts      = 3
	DefaultGracefulShutdownSec        = 30
	DefaultSessionReadyTimeoutSec     = 30
	DefaultSessionHealthCheckSec      = 30
	DefaultSessionMonitorInitDelaySec = 10
	DefaultSessionRestartTimeoutSec   = 30
	DefaultSessionWaitTimeoutSec      = 60
	DefaultBackoffInitialMs           = 500
	DefaultBackoffMaxSec              = 5
	DefaultContactSyncBatchSize       = 100
	DefaultContactSyncDelayMs         = 100
	DefaultServerReadTimeoutSec       = 15
	DefaultServerWriteTimeoutSec      = 15
	DefaultServerIdleTimeoutSec       = 60
	DefaultSessionStatusTimeoutSec    = 5
	DefaultWebhookMaxSkewSec          = 300
	DefaultWebhookMaxBytes            = 5 * 1024 * 1024
	DefaultRateLimitPerMinute         = 100
	DefaultRateLimitCleanupMinutes    = 5
	DefaultDBMaxOpenConnections       = 25
	DefaultDBMaxIdleConnections       = 5
	DefaultDBConnMaxLifetimeSec       = 300 // 5 minutes
	DefaultDBConnMaxIdleTimeSec       = 60  // 1 minute
	DefaultMediaDownloadTimeoutSec    = 30  // 30 seconds
	DefaultSignalHTTPTimeoutSec       = 60  // 60 seconds
)

// Privacy settings
const (
	DefaultPhoneMaskLength = 4
	DefaultMessageIDLength = 8
)

// Time-related constants
const (
	DefaultSignalPollingTimeoutSec = 30
	DefaultWhatsAppPollIntervalSec = 30
	DefaultWhatsAppTimeoutMs       = 30000
	DefaultWhatsAppRetryCount      = 3
	TypingDurationPerCharMs        = 50
	MaxTypingDurationSec           = 3
	CleanupSchedulerIntervalHours  = 24
	DefaultContactCacheHours       = 24
)

// Numeric conversions
const (
	MillisecondsPerSecond = 1000
	SecondsPerDay         = 86400
)

// Size and length constants
const (
	MinPhoneNumberLength       = 10
	MimeDetectionBufferSize    = 512
	MessageIDRandomBytesLength = 16
)

// Network and port constants
const (
	DefaultDevServerPort = 3000
)

// Channel and buffer sizes
const (
	ServerErrorChannelSize    = 1
	SignalDownloadChannelSize = 1
)

// File size and conversion constants
const (
	BytesPerMegabyte          = 1024 * 1024
	DefaultDownloadTimeoutSec = 30
)

// Validation and security constants
const (
	MaxMessageIDLength   = 256 // Maximum allowed message ID length
	MaxSessionNameLength = 64  // Maximum allowed session name length
)

// File permission constants
const (
	DefaultFilePermissions      = 0600 // Read/write for owner only
	DefaultDirectoryPermissions = 0750 // Read/write/execute for owner, read/execute for group
)

// Encryption constants
// DEPRECATED: These default salts are retained for backward compatibility only.
// New deployments should set WHATSIGNAL_ENCRYPTION_SALT and WHATSIGNAL_ENCRYPTION_LOOKUP_SALT
// environment variables with unique, randomly generated salts.
const (
	EncryptionSalt       = "whatsignal-salt-v1"        // Default salt for backward compatibility
	EncryptionLookupSalt = "whatsignal-lookup-salt-v1" // Default lookup salt for backward compatibility
	PBKDF2Iterations     = 100000                      // PBKDF2 iterations for key derivation
)

// Validation limits
const (
	MaxRateLimitPerMinute = 10000 // Maximum allowed rate limit per minute
)
