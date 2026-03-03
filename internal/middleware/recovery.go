package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/sirupsen/logrus"
)

// RecoveryMiddleware catches panics in HTTP handlers, logs a stack trace, and returns HTTP 500.
func RecoveryMiddleware(logger *logrus.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.WithFields(logrus.Fields{
						"panic": rec,
						"stack": string(debug.Stack()),
						"path":  r.URL.Path,
					}).Error("Recovered from panic in HTTP handler")

					w.WriteHeader(http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
