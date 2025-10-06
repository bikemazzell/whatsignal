package constants

// Default timeout values used by client packages
const (
	DefaultHTTPTimeoutSec          = 30
	DefaultSignalHTTPTimeoutSec    = 60
	DefaultWhatsAppTimeoutMs       = 30000
	DefaultWhatsAppRetryCount      = 3
	DefaultMediaDownloadTimeoutSec = 30
)

// File size constants used by media packages
const (
	BytesPerMegabyte         = 1024 * 1024
	DefaultMaxImageSizeMB    = 5
	DefaultMaxVideoSizeMB    = 100
	DefaultMaxDocumentSizeMB = 100
	DefaultMaxVoiceSizeMB    = 16
	MimeDetectionBufferSize  = 512
)

// Validation and security constants used by packages
const (
	MaxMessageIDLength   = 256
	MaxSessionNameLength = 64
	MinPhoneNumberLength = 10
)

// File permission constants
const (
	DefaultFilePermissions      = 0600
	DefaultDirectoryPermissions = 0750
)

// Channel and buffer size constants
const (
	SignalDownloadChannelSize = 1
	DefaultDevServerPort      = 3000
)

// Timing constants used by packages
const (
	TypingDurationPerCharMs = 50
	MaxTypingDurationSec    = 3
	DefaultBackoffInitialMs = 500
	DefaultBackoffMaxSec    = 5
)

// Media file type constants
var (
	DefaultImageTypes    = []string{"jpg", "jpeg", "png", "gif"}
	DefaultVideoTypes    = []string{"mp4", "mov"}
	DefaultDocumentTypes = []string{"pdf", "doc", "docx"}
	DefaultVoiceTypes    = []string{"ogg"}
)
