package middleware

import (
	"net/http"
	"time"

	"file-storage-server/internal/metrics"
)

type metricsResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *metricsResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func Metrics(registry *metrics.Registry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorder := &metricsResponseWriter{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()
			next.ServeHTTP(recorder, r)
			registry.Observe(r.Method, r.URL.Path, recorder.status, time.Since(start))
		})
	}
}
