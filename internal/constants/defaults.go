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
)

// Privacy settings
const (
	DefaultPhoneMaskLength = 4
	DefaultMessageIDLength = 8
)