package main

import (
	"encoding/json"
	"net/http"

	"whatsignal/internal/metrics"
	"whatsignal/internal/tracing"

	"github.com/sirupsen/logrus"
)

// handleMetrics returns current application metrics
func (s *Server) handleMetrics() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestInfo := tracing.GetRequestInfo(r.Context())

		s.logger.WithFields(logrus.Fields{
			"request_id": requestInfo.RequestID,
			"trace_id":   requestInfo.TraceID,
			"endpoint":   "/metrics",
		}).Debug("Serving metrics endpoint")

		// Get all metrics from the global registry
		allMetrics := metrics.GetAllMetrics()

		// Set appropriate headers
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		// Encode and send metrics
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")

		if err := encoder.Encode(allMetrics); err != nil {
			s.logger.WithFields(logrus.Fields{
				"request_id": requestInfo.RequestID,
				"trace_id":   requestInfo.TraceID,
				"error":      err,
			}).Error("Failed to encode metrics response")

			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		s.logger.WithFields(logrus.Fields{
			"request_id": requestInfo.RequestID,
			"trace_id":   requestInfo.TraceID,
			"endpoint":   "/metrics",
		}).Debug("Metrics endpoint served successfully")
	}
}
