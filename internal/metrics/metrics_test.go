package metrics

import (
	"testing"
	"time"
)

func TestRegistry_IncrementCounter(t *testing.T) {
	registry := NewRegistry()

	// Test basic counter increment
	registry.IncrementCounter("test_counter", nil, "Test counter")

	metrics := registry.GetAllMetrics()
	counters := metrics.Counters

	if counter, exists := counters["test_counter"]; !exists {
		t.Fatal("Expected counter 'test_counter' to exist")
	} else if counter.Value != 1 {
		t.Fatalf("Expected counter value to be 1, got %f", counter.Value)
	}

	// Test increment with labels
	labels := map[string]string{"status": "success"}
	registry.IncrementCounter("test_counter", labels, "Test counter")

	metrics = registry.GetAllMetrics()
	counters = metrics.Counters

	labeledKey := "test_counter_status:success"
	if counter, exists := counters[labeledKey]; !exists {
		t.Fatal("Expected labeled counter to exist")
	} else if counter.Value != 1 {
		t.Fatalf("Expected labeled counter value to be 1, got %f", counter.Value)
	}

	// Test multiple increments
	registry.IncrementCounter("test_counter", labels, "Test counter")

	metrics = registry.GetAllMetrics()
	counters = metrics.Counters

	if counter, exists := counters[labeledKey]; !exists {
		t.Fatal("Expected labeled counter to exist")
	} else if counter.Value != 2 {
		t.Fatalf("Expected labeled counter value to be 2, got %f", counter.Value)
	}
}

func TestRegistry_AddToCounter(t *testing.T) {
	registry := NewRegistry()

	// Test adding custom values
	registry.AddToCounter("test_add_counter", 5.5, nil, "Test add counter")

	metrics := registry.GetAllMetrics()
	counters := metrics.Counters

	if counter, exists := counters["test_add_counter"]; !exists {
		t.Fatal("Expected counter 'test_add_counter' to exist")
	} else if counter.Value != 5.5 {
		t.Fatalf("Expected counter value to be 5.5, got %f", counter.Value)
	}

	// Test adding to existing counter
	registry.AddToCounter("test_add_counter", 2.3, nil, "Test add counter")

	metrics = registry.GetAllMetrics()
	counters = metrics.Counters

	if counter, exists := counters["test_add_counter"]; !exists {
		t.Fatal("Expected counter to exist")
	} else if counter.Value != 7.8 {
		t.Fatalf("Expected counter value to be 7.8, got %f", counter.Value)
	}
}

func TestRegistry_RecordTimer(t *testing.T) {
	registry := NewRegistry()

	// Test basic timer recording
	duration := 100 * time.Millisecond
	registry.RecordTimer("test_timer", duration, nil, "Test timer")

	metrics := registry.GetAllMetrics()
	timers := metrics.Timers

	if timer, exists := timers["test_timer"]; !exists {
		t.Fatal("Expected timer 'test_timer' to exist")
	} else {
		if timer.Count != 1 {
			t.Fatalf("Expected timer count to be 1, got %d", timer.Count)
		}
		expectedMs := float64(duration.Nanoseconds()) / 1e6
		if timer.Sum != expectedMs {
			t.Fatalf("Expected timer sum to be %f, got %f", expectedMs, timer.Sum)
		}
		if timer.Min != expectedMs {
			t.Fatalf("Expected timer min to be %f, got %f", expectedMs, timer.Min)
		}
		if timer.Max != expectedMs {
			t.Fatalf("Expected timer max to be %f, got %f", expectedMs, timer.Max)
		}
		if timer.Average != expectedMs {
			t.Fatalf("Expected timer average to be %f, got %f", expectedMs, timer.Average)
		}
	}

	// Test multiple recordings
	duration2 := 200 * time.Millisecond
	registry.RecordTimer("test_timer", duration2, nil, "Test timer")

	metrics = registry.GetAllMetrics()
	timers = metrics.Timers

	if timer, exists := timers["test_timer"]; !exists {
		t.Fatal("Expected timer to exist")
	} else {
		if timer.Count != 2 {
			t.Fatalf("Expected timer count to be 2, got %d", timer.Count)
		}

		expectedMs1 := float64(duration.Nanoseconds()) / 1e6
		expectedMs2 := float64(duration2.Nanoseconds()) / 1e6
		expectedSum := expectedMs1 + expectedMs2
		expectedAvg := expectedSum / 2

		if timer.Sum != expectedSum {
			t.Fatalf("Expected timer sum to be %f, got %f", expectedSum, timer.Sum)
		}
		if timer.Average != expectedAvg {
			t.Fatalf("Expected timer average to be %f, got %f", expectedAvg, timer.Average)
		}
		if timer.Min != expectedMs1 {
			t.Fatalf("Expected timer min to be %f, got %f", expectedMs1, timer.Min)
		}
		if timer.Max != expectedMs2 {
			t.Fatalf("Expected timer max to be %f, got %f", expectedMs2, timer.Max)
		}
	}
}

func TestRegistry_SetGauge(t *testing.T) {
	registry := NewRegistry()

	// Test basic gauge setting
	registry.SetGauge("test_gauge", 42.5, nil, "Test gauge")

	metrics := registry.GetAllMetrics()
	gauges := metrics.Gauges

	if gauge, exists := gauges["test_gauge"]; !exists {
		t.Fatal("Expected gauge 'test_gauge' to exist")
	} else if gauge.Value != 42.5 {
		t.Fatalf("Expected gauge value to be 42.5, got %f", gauge.Value)
	}

	// Test gauge update
	registry.SetGauge("test_gauge", 100.0, nil, "Test gauge")

	metrics = registry.GetAllMetrics()
	gauges = metrics.Gauges

	if gauge, exists := gauges["test_gauge"]; !exists {
		t.Fatal("Expected gauge to exist")
	} else if gauge.Value != 100.0 {
		t.Fatalf("Expected gauge value to be 100.0, got %f", gauge.Value)
	}
}

func TestRegistry_MetricKey(t *testing.T) {
	registry := NewRegistry()

	// Test key without labels
	key := registry.metricKey("test_metric", nil)
	if key != "test_metric" {
		t.Fatalf("Expected key to be 'test_metric', got '%s'", key)
	}

	// Test key with labels
	labels := map[string]string{
		"status": "success",
		"type":   "webhook",
	}
	key = registry.metricKey("test_metric", labels)

	// Key should contain all labels (order may vary)
	if key != "test_metric_status:success_type:webhook" && key != "test_metric_type:webhook_status:success" {
		t.Fatalf("Unexpected metric key: %s", key)
	}
}

func TestRegistry_PercentileCalculation(t *testing.T) {
	registry := NewRegistry()

	// Record enough samples to trigger percentile calculation
	samples := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
		60 * time.Millisecond,
		70 * time.Millisecond,
		80 * time.Millisecond,
		90 * time.Millisecond,
		100 * time.Millisecond,
	}

	for _, duration := range samples {
		registry.RecordTimer("percentile_test", duration, nil, "Percentile test")
	}

	metrics := registry.GetAllMetrics()
	timers := metrics.Timers

	if timer, exists := timers["percentile_test"]; !exists {
		t.Fatal("Expected timer to exist")
	} else {
		if timer.Count != int64(len(samples)) {
			t.Fatalf("Expected timer count to be %d, got %d", len(samples), timer.Count)
		}

		// Check that percentiles are calculated (should be non-zero)
		if timer.P95 <= 0 {
			t.Fatal("Expected P95 to be calculated")
		}
		if timer.P99 <= 0 {
			t.Fatal("Expected P99 to be calculated")
		}

		// P95 should be greater than P99 is not true since P99 is higher percentile
		// P99 should be >= P95
		if timer.P99 < timer.P95 {
			t.Fatalf("Expected P99 (%f) to be >= P95 (%f)", timer.P99, timer.P95)
		}
	}
}

func TestGlobalRegistry(t *testing.T) {
	// Test global registry functions
	IncrementCounter("global_test", nil, "Global test")
	AddToCounter("global_add", 5.0, nil, "Global add test")
	RecordTimer("global_timer", 50*time.Millisecond, nil, "Global timer test")
	SetGauge("global_gauge", 123.45, nil, "Global gauge test")

	metrics := GetAllMetrics()

	// Check that all metrics were recorded
	counters := metrics.Counters
	timers := metrics.Timers
	gauges := metrics.Gauges

	if _, exists := counters["global_test"]; !exists {
		t.Fatal("Expected global counter to exist")
	}
	if _, exists := counters["global_add"]; !exists {
		t.Fatal("Expected global add counter to exist")
	}
	if _, exists := timers["global_timer"]; !exists {
		t.Fatal("Expected global timer to exist")
	}
	if _, exists := gauges["global_gauge"]; !exists {
		t.Fatal("Expected global gauge to exist")
	}

	// Check metadata
	if metrics.UptimeMs < 0 {
		t.Fatal("Expected uptime_ms to be non-negative")
	}
	if metrics.Timestamp == 0 {
		t.Fatal("Expected timestamp to be present")
	}
}

func TestCopyLabels(t *testing.T) {
	original := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	copy := copyLabels(original)

	// Check that copy contains same data
	if len(copy) != len(original) {
		t.Fatal("Copy should have same length as original")
	}

	for k, v := range original {
		if copy[k] != v {
			t.Fatalf("Expected copy[%s] to be %s, got %s", k, v, copy[k])
		}
	}

	// Check that modifying copy doesn't affect original
	copy["key3"] = "value3"

	if _, exists := original["key3"]; exists {
		t.Fatal("Modifying copy should not affect original")
	}

	// Test nil input
	nilCopy := copyLabels(nil)
	if nilCopy != nil {
		t.Fatal("Copy of nil should be nil")
	}
}
