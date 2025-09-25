package metrics

import (
	"fmt"
	"sync"
	"time"
)

// MetricType represents the type of metric
type MetricType string

const (
	Counter   MetricType = "counter"
	Timer     MetricType = "timer"
	Histogram MetricType = "histogram"
	Gauge     MetricType = "gauge"
)

// Metric represents a single metric with its metadata
type Metric struct {
	Name        string            `json:"name"`
	Type        MetricType        `json:"type"`
	Value       float64           `json:"value"`
	Count       int64             `json:"count,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Description string            `json:"description,omitempty"`
	LastUpdate  time.Time         `json:"last_update"`
}

// TimerMetric stores timing information
type TimerMetric struct {
	Count   int64   `json:"count"`
	Sum     float64 `json:"sum_ms"`
	Min     float64 `json:"min_ms"`
	Max     float64 `json:"max_ms"`
	Average float64 `json:"avg_ms"`
	P95     float64 `json:"p95_ms,omitempty"`
	P99     float64 `json:"p99_ms,omitempty"`
	samples []float64
}

// Registry manages all metrics in memory
type Registry struct {
	mu        sync.RWMutex
	counters  map[string]*Metric
	timers    map[string]*TimerMetric
	gauges    map[string]*Metric
	startTime time.Time
}

// NewRegistry creates a new metrics registry
func NewRegistry() *Registry {
	return &Registry{
		counters:  make(map[string]*Metric),
		timers:    make(map[string]*TimerMetric),
		gauges:    make(map[string]*Metric),
		startTime: time.Now(),
	}
}

// Global registry instance
var globalRegistry = NewRegistry()

// GetRegistry returns the global registry instance
func GetRegistry() *Registry {
	return globalRegistry
}

// IncrementCounter increments a counter metric
func (r *Registry) IncrementCounter(name string, labels map[string]string, description string) {
	r.AddToCounter(name, 1, labels, description)
}

// AddToCounter adds a value to a counter metric
func (r *Registry) AddToCounter(name string, value float64, labels map[string]string, description string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := r.metricKey(name, labels)
	if counter, exists := r.counters[key]; exists {
		counter.Value += value
		counter.LastUpdate = time.Now()
	} else {
		r.counters[key] = &Metric{
			Name:        name,
			Type:        Counter,
			Value:       value,
			Labels:      copyLabels(labels),
			Description: description,
			LastUpdate:  time.Now(),
		}
	}
}

// RecordTimer records a timing measurement
func (r *Registry) RecordTimer(name string, duration time.Duration, labels map[string]string, description string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := r.metricKey(name, labels)
	durationMs := float64(duration.Nanoseconds()) / 1e6

	if timer, exists := r.timers[key]; exists {
		timer.Count++
		timer.Sum += durationMs
		timer.samples = append(timer.samples, durationMs)

		if durationMs < timer.Min || timer.Min == 0 {
			timer.Min = durationMs
		}
		if durationMs > timer.Max {
			timer.Max = durationMs
		}

		timer.Average = timer.Sum / float64(timer.Count)

		// Keep only last 1000 samples for percentile calculation
		if len(timer.samples) > 1000 {
			timer.samples = timer.samples[len(timer.samples)-1000:]
		}

		// Calculate percentiles if we have enough samples
		if len(timer.samples) >= 10 {
			timer.P95 = r.calculatePercentile(timer.samples, 0.95)
			timer.P99 = r.calculatePercentile(timer.samples, 0.99)
		}
	} else {
		r.timers[key] = &TimerMetric{
			Count:   1,
			Sum:     durationMs,
			Min:     durationMs,
			Max:     durationMs,
			Average: durationMs,
			samples: []float64{durationMs},
		}
	}
}

// SetGauge sets a gauge metric value
func (r *Registry) SetGauge(name string, value float64, labels map[string]string, description string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := r.metricKey(name, labels)
	r.gauges[key] = &Metric{
		Name:        name,
		Type:        Gauge,
		Value:       value,
		Labels:      copyLabels(labels),
		Description: description,
		LastUpdate:  time.Now(),
	}
}

// GetAllMetrics returns all metrics in a structured format
func (r *Registry) GetAllMetrics() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := map[string]interface{}{
		"counters":  make(map[string]*Metric),
		"timers":    make(map[string]*TimerMetric),
		"gauges":    make(map[string]*Metric),
		"uptime_ms": time.Since(r.startTime).Milliseconds(),
		"timestamp": time.Now().Unix(),
	}

	// Copy counters
	for key, counter := range r.counters {
		result["counters"].(map[string]*Metric)[key] = counter
	}

	// Copy timers
	for key, timer := range r.timers {
		result["timers"].(map[string]*TimerMetric)[key] = timer
	}

	// Copy gauges
	for key, gauge := range r.gauges {
		result["gauges"].(map[string]*Metric)[key] = gauge
	}

	return result
}

// metricKey generates a unique key for a metric with labels
func (r *Registry) metricKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}

	key := name
	for k, v := range labels {
		key += fmt.Sprintf("_%s:%s", k, v)
	}
	return key
}

// calculatePercentile calculates the specified percentile from samples
func (r *Registry) calculatePercentile(samples []float64, percentile float64) float64 {
	if len(samples) == 0 {
		return 0
	}

	// Simple percentile calculation (would use sort in production)
	sorted := make([]float64, len(samples))
	copy(sorted, samples)

	// Bubble sort for simplicity (consider using sort.Float64s for production)
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			if sorted[j] > sorted[j+1] {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	index := int(float64(len(sorted)) * percentile)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

// copyLabels creates a copy of the labels map
func copyLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return nil
	}

	copy := make(map[string]string)
	for k, v := range labels {
		copy[k] = v
	}
	return copy
}

// Convenience functions for global registry

// IncrementCounter increments a counter in the global registry
func IncrementCounter(name string, labels map[string]string, description string) {
	globalRegistry.IncrementCounter(name, labels, description)
}

// AddToCounter adds to a counter in the global registry
func AddToCounter(name string, value float64, labels map[string]string, description string) {
	globalRegistry.AddToCounter(name, value, labels, description)
}

// RecordTimer records timing in the global registry
func RecordTimer(name string, duration time.Duration, labels map[string]string, description string) {
	globalRegistry.RecordTimer(name, duration, labels, description)
}

// SetGauge sets a gauge in the global registry
func SetGauge(name string, value float64, labels map[string]string, description string) {
	globalRegistry.SetGauge(name, value, labels, description)
}

// GetAllMetrics returns all metrics from the global registry
func GetAllMetrics() map[string]interface{} {
	return globalRegistry.GetAllMetrics()
}
