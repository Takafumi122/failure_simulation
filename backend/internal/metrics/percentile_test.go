package metrics

import "testing"

func TestPercentileCalculation(t *testing.T) {
	// Empty slice returns 0
	if got := Percentile(nil, 95); got != 0 {
		t.Fatalf("expected 0 for empty slice, got %v", got)
	}

	// Single element returns that element
	if got := Percentile([]float64{42.5}, 95); got != 42.5 {
		t.Fatalf("expected 42.5 for single element, got %v", got)
	}

	// Unsorted values 1..100
	values := make([]float64, 100)
	for i := 0; i < 100; i++ {
		// Insert in reverse order to verify internal sorting
		values[i] = float64(100 - i)
	}

	p50 := Percentile(values, 50)
	if p50 < 50.0 || p50 > 51.0 {
		t.Errorf("expected p50 around 50.5, got %v", p50)
	}

	p95 := Percentile(values, 95)
	if p95 < 95.0 || p95 > 96.0 {
		t.Errorf("expected p95 around 95.05, got %v", p95)
	}

	p99 := Percentile(values, 99)
	if p99 < 99.0 || p99 > 100.0 {
		t.Errorf("expected p99 around 99.01, got %v", p99)
	}

	avg := Average(values)
	if avg != 50.5 {
		t.Errorf("expected average 50.5, got %v", avg)
	}
}
