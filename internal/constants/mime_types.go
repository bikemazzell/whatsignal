package constants

// MimeTypes maps file extensions to their corresponding MIME types
var MimeTypes = map[string]string{
	// Image formats
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".jfif": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",

	// Video formats
	".mp4": "video/mp4",
	".mov": "video/quicktime",
	".avi": "video/x-msvideo",

	// Document formats
	".pdf":  "application/pdf",
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".txt":  "text/plain",
	".rtf":  "application/rtf",

	// Audio formats
	".ogg": "audio/ogg",
	".mp3": "audio/mpeg",
	".wav": "audio/wav",
	".aac": "audio/aac",
	".m4a": "audio/mp4",
}

// DefaultMimeType is the fallback MIME type for unknown file extensions
const DefaultMimeType = "application/octet-stream"

// Default file extensions for different media categories
const (
	DefaultImageExtension = "jpg"
	DefaultVideoExtension = "mp4"
	DefaultAudioExtension = "ogg" // Signal's default
)

// Default allowed file types for media configuration
var (
	DefaultImageTypes    = []string{"jpg", "jpeg", "png", "gif"}
	DefaultVideoTypes    = []string{"mp4", "mov"}
	DefaultDocumentTypes = []string{"pdf", "doc", "docx"}
	DefaultVoiceTypes    = []string{"ogg"}
)

// ContentTypeToExtension maps partial content type matches to file extensions
var ContentTypeToExtension = map[string]string{
	// Audio content type mappings
	"audio/ogg":  "ogg",
	"audio/mpeg": "mp3",
	"audio/mp3":  "mp3",
	"audio/aac":  "aac",
	"audio/m4a":  "m4a",
	"audio/mp4":  "m4a",

	// Image content type mappings
	"image/jpeg": "jpg",
	"image/jpg":  "jpg",
	"image/png":  "png",
	"image/gif":  "gif",
	"image/webp": "webp",

	// Video content type mappings
	"video/mp4":       "mp4",
	"video/mov":       "mov",
	"video/quicktime": "mov",
	"video/avi":       "avi",
	"video/x-msvideo": "avi",
}

// FileSignatures maps file format signatures to extensions
var FileSignatures = map[string]string{
	"OggS":              "ogg",  // OGG file signature
	"ID3":               "mp3",  // MP3 ID3v2 tag
	"GIF87a":            "gif",  // GIF87a signature
	"GIF89a":            "gif",  // GIF89a signature
	"RIFF":              "webp", // WebP/RIFF container (needs additional check)
	"%PDF":              "pdf",  // PDF signature
	"\x89PNG\r\n\x1a\n": "png",  // PNG signature
}

// MimeTypeToExtension maps MIME types to their primary file extensions
var MimeTypeToExtension = map[string]string{
	// Image formats
	"image/jpeg":    ".jpg",
	"image/png":     ".png",
	"image/gif":     ".gif",
	"image/webp":    ".webp",
	"image/svg+xml": ".svg",

	// Video formats
	"video/mp4":       ".mp4",
	"video/quicktime": ".mov",
	"video/mov":       ".mov",
	"video/x-msvideo": ".avi",

	// Document formats
	"application/pdf":    ".pdf",
	"application/msword": ".doc",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": ".docx",
	"text/plain":      ".txt",
	"application/rtf": ".rtf",

	// Audio formats
	"audio/ogg":  ".ogg",
	"audio/mpeg": ".mp3",
	"audio/wav":  ".wav",
	"audio/aac":  ".aac",
	"audio/mp4":  ".m4a",
}
