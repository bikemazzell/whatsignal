package config

import (
	"context"
	"os"
	"sync"
	"time"

	"whatsignal/internal/models"

	"github.com/sirupsen/logrus"
)

// ConfigWatcher watches for configuration file changes and reloads configuration
type ConfigWatcher struct {
	configPath string
	logger     *logrus.Logger
	mu         sync.RWMutex
	config     *models.Config
	callbacks  []func(*models.Config)
}

// NewConfigWatcher creates a new configuration watcher
func NewConfigWatcher(configPath string, logger *logrus.Logger) *ConfigWatcher {
	return &ConfigWatcher{
		configPath: configPath,
		logger:     logger,
		callbacks:  make([]func(*models.Config), 0),
	}
}

// Start begins watching the configuration file for changes using polling
func (cw *ConfigWatcher) Start(ctx context.Context) error {
	// Load initial configuration
	config, err := LoadConfig(cw.configPath)
	if err != nil {
		return err
	}

	cw.mu.Lock()
	cw.config = config
	cw.mu.Unlock()

	// Get initial file modification time
	stat, err := os.Stat(cw.configPath)
	if err != nil {
		return err
	}
	lastModTime := stat.ModTime()

	cw.logger.WithField("path", cw.configPath).Info("Configuration watcher started")

	ticker := time.NewTicker(5 * time.Second) // Check every 5 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			cw.logger.Info("Configuration watcher stopping")
			return nil

		case <-ticker.C:
			stat, err := os.Stat(cw.configPath)
			if err != nil {
				cw.logger.WithError(err).Error("Failed to stat configuration file")
				continue
			}

			if stat.ModTime().After(lastModTime) {
				cw.logger.Debug("Configuration file changed")
				lastModTime = stat.ModTime()

				// Small delay to ensure file write is complete
				time.Sleep(100 * time.Millisecond)
				cw.reloadConfig()
			}
		}
	}
}

// GetConfig returns the current configuration (thread-safe)
func (cw *ConfigWatcher) GetConfig() *models.Config {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return cw.config
}

// OnConfigChange registers a callback to be called when configuration changes
func (cw *ConfigWatcher) OnConfigChange(callback func(*models.Config)) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.callbacks = append(cw.callbacks, callback)
}

// reloadConfig reloads the configuration from file
func (cw *ConfigWatcher) reloadConfig() {
	newConfig, err := LoadConfig(cw.configPath)
	if err != nil {
		cw.logger.WithError(err).Error("Failed to reload configuration")
		return
	}

	cw.mu.Lock()
	oldConfig := cw.config
	cw.config = newConfig
	callbacks := make([]func(*models.Config), len(cw.callbacks))
	copy(callbacks, cw.callbacks)
	cw.mu.Unlock()

	cw.logger.Info("Configuration reloaded successfully")

	// Notify all registered callbacks
	for _, callback := range callbacks {
		go func(cb func(*models.Config)) {
			defer func() {
				if r := recover(); r != nil {
					cw.logger.WithField("panic", r).Error("Config change callback panicked")
				}
			}()
			cb(newConfig)
		}(callback)
	}

	// Log significant changes
	cw.logConfigChanges(oldConfig, newConfig)
}

// logConfigChanges logs notable configuration changes
func (cw *ConfigWatcher) logConfigChanges(old, new *models.Config) {
	if old == nil {
		return
	}

	if old.RetentionDays != new.RetentionDays {
		cw.logger.WithFields(logrus.Fields{
			"old": old.RetentionDays,
			"new": new.RetentionDays,
		}).Info("Retention days changed")
	}

	if old.Server.CleanupIntervalHours != new.Server.CleanupIntervalHours {
		cw.logger.WithFields(logrus.Fields{
			"old": old.Server.CleanupIntervalHours,
			"new": new.Server.CleanupIntervalHours,
		}).Info("Cleanup interval changed")
	}

	if len(old.Channels) != len(new.Channels) {
		cw.logger.WithFields(logrus.Fields{
			"old_count": len(old.Channels),
			"new_count": len(new.Channels),
		}).Info("Number of channels changed")
	}
}
