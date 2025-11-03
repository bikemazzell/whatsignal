package media

import (
	"path/filepath"
	"strings"
	"whatsignal/internal/models"
)

// Router provides centralized media type detection and validation
type Router interface {
	// GetMediaType returns the media type (image, video, voice, document) for a file path
	GetMediaType(path string) string
	// IsImageAttachment checks if the file is an image based on extension
	IsImageAttachment(path string) bool
	// IsVideoAttachment checks if the file is a video based on extension
	IsVideoAttachment(path string) bool
	// IsVoiceAttachment checks if the file is a voice/audio file based on extension
	IsVoiceAttachment(path string) bool
	// IsDocumentAttachment checks if the file is a document based on extension
	IsDocumentAttachment(path string) bool
	// GetMaxSizeForMediaType returns the maximum allowed size in bytes for a media type
	GetMaxSizeForMediaType(mediaType string) int64
}

type router struct {
	config models.MediaConfig
}

// NewRouter creates a new Router instance
func NewRouter(config models.MediaConfig) Router {
	return &router{
		config: config,
	}
}

func (r *router) GetMediaType(path string) string {
	switch {
	case r.IsImageAttachment(path):
		return "image"
	case r.IsVideoAttachment(path):
		return "video"
	case r.IsVoiceAttachment(path):
		return "voice"
	default:
		return "document" // Default everything else to document
	}
}

func (r *router) IsImageAttachment(path string) bool {
	return r.hasAllowedExtension(path, r.config.AllowedTypes.Image)
}

func (r *router) IsVideoAttachment(path string) bool {
	return r.hasAllowedExtension(path, r.config.AllowedTypes.Video)
}

func (r *router) IsVoiceAttachment(path string) bool {
	return r.hasAllowedExtension(path, r.config.AllowedTypes.Voice)
}

func (r *router) IsDocumentAttachment(path string) bool {
	return r.hasAllowedExtension(path, r.config.AllowedTypes.Document)
}

func (r *router) GetMaxSizeForMediaType(mediaType string) int64 {
	const bytesPerMB = 1024 * 1024
	switch mediaType {
	case "image":
		return int64(r.config.MaxSizeMB.Image * bytesPerMB)
	case "video":
		return int64(r.config.MaxSizeMB.Video * bytesPerMB)
	case "voice":
		return int64(r.config.MaxSizeMB.Voice * bytesPerMB)
	case "document":
		return int64(r.config.MaxSizeMB.Document * bytesPerMB)
	default:
		return int64(r.config.MaxSizeMB.Document * bytesPerMB)
	}
}

// hasAllowedExtension checks if the file path has an extension that matches any of the allowed extensions
func (r *router) hasAllowedExtension(path string, allowedExtensions []string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	for _, allowedExt := range allowedExtensions {
		// Support both ".png" and "png" style entries in config
		normAllowed := strings.TrimPrefix(strings.ToLower(allowedExt), ".")
		if ext == normAllowed {
			return true
		}
	}
	return false
}
