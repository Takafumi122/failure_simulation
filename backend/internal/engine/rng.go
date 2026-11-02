package engine

import "hash/fnv"

// RNGPurpose distinguishes different random draws for the same (requestID, serviceID, attempt).
type RNGPurpose uint64

const (
	PurposeArrivalJitter RNGPurpose = 1
	PurposeFaultEval     RNGPurpose = 2
	PurposeBaseFaultEval RNGPurpose = 3
	PurposeLatencyJitter RNGPurpose = 4
	PurposeRetryJitter   RNGPurpose = 5
)

// DeterministicRNG generates coordinate-indexed pseudo-random numbers in [0.0, 1.0).
// Because draws depend strictly on (seed, reqID, serviceID, attempt, purpose) rather than
// sequential state consumption, adding retries or circuit breakers in one strategy never
// shifts or perturbs the random numbers seen by other requests or services.
type DeterministicRNG struct {
	seed uint64
}

// NewDeterministicRNG creates a coordinate-based deterministic RNG from seed.
func NewDeterministicRNG(seed int64) *DeterministicRNG {
	return &DeterministicRNG{
		seed: uint64(seed),
	}
}

// Float64 returns a deterministic uniform float64 in [0.0, 1.0) for the given coordinates.
func (r *DeterministicRNG) Float64(reqID int, serviceID string, attempt int, purpose RNGPurpose) float64 {
	h := fnv.New64a()
	var buf [32]byte
	putUint64(buf[0:8], r.seed)
	putUint64(buf[8:16], uint64(reqID))
	putUint64(buf[16:24], uint64(attempt))
	putUint64(buf[24:32], uint64(purpose))
	_, _ = h.Write(buf[:])
	_, _ = h.Write([]byte(serviceID))

	// SplitMix64 finalizer for high statistical quality
	z := h.Sum64() + 0x9e3779b97f4a7c15
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	z = z ^ (z >> 31)

	// Convert top 53 bits to float64 in [0.0, 1.0)
	return float64(z>>11) / (1 << 53)
}

func putUint64(b []byte, v uint64) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
	b[4] = byte(v >> 32)
	b[5] = byte(v >> 40)
	b[6] = byte(v >> 48)
	b[7] = byte(v >> 56)
}
