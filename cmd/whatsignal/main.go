package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/whatsignal/internal/config"
	"github.com/whatsignal/internal/database"
	"github.com/whatsignal/internal/service"
	"github.com/whatsignal/pkg/media"
	signalcli "github.com/whatsignal/pkg/signal"
	"github.com/whatsignal/pkg/whatsapp"
)

func main() {
	configPath := flag.String("config", "config.json", "Path to configuration file")
	dbPath := flag.String("db", "whatsignal.db", "Path to SQLite database")
	cacheDir := flag.String("cache", "cache", "Path to media cache directory")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	waClient := whatsapp.NewClient(cfg.WhatsApp.APIBaseURL)
	sigClient := signalcli.NewClient(cfg.Signal.RPCURL)

	mediaHandler, err := media.NewHandler(*cacheDir)
	if err != nil {
		log.Fatalf("Failed to initialize media handler: %v", err)
	}

	bridge := service.NewBridge(
		db,
		waClient,
		sigClient,
		mediaHandler,
		service.RetryConfig{
			InitialBackoff: cfg.Retry.InitialBackoffMs,
			MaxBackoff:     cfg.Retry.MaxBackoffMs,
			MaxAttempts:    cfg.Retry.MaxAttempts,
		},
	)

	// Start cleanup goroutine
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			if err := bridge.CleanupOldRecords(cfg.RetentionDays); err != nil {
				log.Printf("Failed to cleanup old records: %v", err)
			}
		}
	}()

	// Start Signal message receiver
	go func() {
		for {
			messages, err := sigClient.ReceiveMessages(30)
			if err != nil {
				log.Printf("Failed to receive Signal messages: %v", err)
				time.Sleep(time.Second * 5)
				continue
			}

			for _, msg := range messages {
				if err := bridge.HandleSignalMessage(&msg); err != nil {
					log.Printf("Failed to handle Signal message: %v", err)
				}
			}
		}
	}()

	// Setup HTTP server for WhatsApp webhooks
	http.HandleFunc("/webhook/whatsapp", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if r.Header.Get("X-Webhook-Secret") != cfg.WhatsApp.WebhookSecret {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var payload struct {
			ChatID    string `json:"chatId"`
			MessageID string `json:"messageId"`
			Sender    string `json:"sender"`
			Content   string `json:"content"`
			MediaURL  string `json:"mediaUrl,omitempty"`
		}

		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		var mediaPath string
		if payload.MediaURL != "" {
			// Download media file
			resp, err := http.Get(payload.MediaURL)
			if err != nil {
				log.Printf("Failed to download media: %v", err)
			} else {
				defer resp.Body.Close()

				tempFile, err := os.CreateTemp("", "whatsignal-media-*")
				if err != nil {
					log.Printf("Failed to create temp file: %v", err)
				} else {
					defer os.Remove(tempFile.Name())
					if _, err := io.Copy(tempFile, resp.Body); err != nil {
						log.Printf("Failed to save media: %v", err)
					} else {
						mediaPath = tempFile.Name()
					}
				}
			}
		}

		if err := bridge.HandleWhatsAppMessage(
			payload.ChatID,
			payload.MessageID,
			payload.Sender,
			payload.Content,
			mediaPath,
		); err != nil {
			log.Printf("Failed to handle WhatsApp message: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	// Setup health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr: ":8080",
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
	}()

	log.Printf("Starting server on :8080")
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("HTTP server error: %v", err)
	}
}
