package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type Registry struct {
	mu          sync.Mutex
	startedAt   time.Time
	requests    map[string]int64
	durationSum map[string]float64
}

type Snapshot struct {
	UptimeSeconds int64
	TotalRequests int64
	ErrorRequests int64
	Routes        []RouteMetric
}

type RouteMetric struct {
	Method         string
	Path           string
	Status         string
	Requests       int64
	DurationSum    float64
	AverageLatency float64
	Error          bool
}

func NewRegistry() *Registry {
	return &Registry{
		startedAt:   time.Now(),
		requests:    make(map[string]int64),
		durationSum: make(map[string]float64),
	}
}

func (r *Registry) Observe(method string, path string, status int, duration time.Duration) {
	if r == nil {
		return
	}
	key := metricKey(method, path, status)
	r.mu.Lock()
	r.requests[key]++
	r.durationSum[key] += duration.Seconds()
	r.mu.Unlock()
}

func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		r.write(w)
	})
}

func (r *Registry) Snapshot() Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()

	snapshot := Snapshot{UptimeSeconds: int64(time.Since(r.startedAt).Seconds())}
	keys := sortedKeys(r.requests)
	for _, key := range keys {
		method, path, status := splitMetricKey(key)
		requests := r.requests[key]
		durationSum := r.durationSum[key]
		route := RouteMetric{
			Method:      method,
			Path:        path,
			Status:      status,
			Requests:    requests,
			DurationSum: durationSum,
			Error:       strings.HasPrefix(status, "4") || strings.HasPrefix(status, "5"),
		}
		if requests > 0 {
			route.AverageLatency = durationSum / float64(requests)
		}
		snapshot.TotalRequests += requests
		if route.Error {
			snapshot.ErrorRequests += requests
		}
		snapshot.Routes = append(snapshot.Routes, route)
	}
	sort.Slice(snapshot.Routes, func(i, j int) bool {
		if snapshot.Routes[i].Requests == snapshot.Routes[j].Requests {
			return snapshot.Routes[i].Path < snapshot.Routes[j].Path
		}
		return snapshot.Routes[i].Requests > snapshot.Routes[j].Requests
	})
	return snapshot
}

func (r *Registry) write(w http.ResponseWriter) {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, _ = fmt.Fprintf(w, "# HELP app_uptime_seconds Application uptime in seconds.\n")
	_, _ = fmt.Fprintf(w, "# TYPE app_uptime_seconds gauge\n")
	_, _ = fmt.Fprintf(w, "app_uptime_seconds %.0f\n", time.Since(r.startedAt).Seconds())

	_, _ = fmt.Fprintf(w, "# HELP http_requests_total Total HTTP requests.\n")
	_, _ = fmt.Fprintf(w, "# TYPE http_requests_total counter\n")
	keys := sortedKeys(r.requests)
	for _, key := range keys {
		method, path, status := splitMetricKey(key)
		_, _ = fmt.Fprintf(w, "http_requests_total{method=%q,path=%q,status=%q} %d\n", method, path, status, r.requests[key])
	}

	_, _ = fmt.Fprintf(w, "# HELP http_request_duration_seconds_sum Total HTTP request duration seconds.\n")
	_, _ = fmt.Fprintf(w, "# TYPE http_request_duration_seconds_sum counter\n")
	keys = sortedKeysFloat(r.durationSum)
	for _, key := range keys {
		method, path, status := splitMetricKey(key)
		_, _ = fmt.Fprintf(w, "http_request_duration_seconds_sum{method=%q,path=%q,status=%q} %.6f\n", method, path, status, r.durationSum[key])
	}
}

func sortedKeys(values map[string]int64) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysFloat(values map[string]float64) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func metricKey(method string, path string, status int) string {
	return method + "\x00" + sanitizePath(path) + "\x00" + fmt.Sprintf("%d", status)
}

func splitMetricKey(key string) (string, string, string) {
	parts := strings.Split(key, "\x00")
	if len(parts) != 3 {
		return "unknown", "unknown", "0"
	}
	return parts[0], parts[1], parts[2]
}

func sanitizePath(path string) string {
	if path == "" {
		return "/"
	}
	return path
}
