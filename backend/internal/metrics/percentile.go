package metrics

import (
	"math"
	"sort"
)

// Percentile calculates the p-th percentile (0 <= p <= 100) from a slice of values.
// It uses the nearest-rank / linear interpolation method on a sorted copy.
func Percentile(values []float64, p float64) float64 {
	n := len(values)
	if n == 0 {
		return 0
	}
	sorted := make([]float64, n)
	copy(sorted, values)
	sort.Float64s(sorted)

	if p <= 0 {
		return Round2(sorted[0])
	}
	if p >= 100 {
		return Round2(sorted[n-1])
	}

	// Rank index in [0, n-1]
	pos := (p / 100.0) * float64(n-1)
	lower := int(math.Floor(pos))
	upper := int(math.Ceil(pos))
	if lower == upper {
		return Round2(sorted[lower])
	}
	weight := pos - float64(lower)
	val := sorted[lower]*(1.0-weight) + sorted[upper]*weight
	return Round2(val)
}

// Average calculates the arithmetic mean of values.
func Average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return Round2(sum / float64(len(values)))
}

// Round2 rounds a float64 to 2 decimal places.
func Round2(val float64) float64 {
	return math.Round(val*100) / 100
}

// Round4 rounds a float64 to 4 decimal places.
func Round4(val float64) float64 {
	return math.Round(val*10000) / 10000
}
