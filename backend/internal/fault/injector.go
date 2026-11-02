package fault

import (
	"failurelab/backend/internal/model"
)

// EvaluationResult holds the fault evaluation outcome for a service processing step.
type EvaluationResult struct {
	FaultActive   bool
	ShouldFail    bool
	FailureReason model.FailureReason
	ExtraDelayMs  float64
}

// Injector evaluates whether a fault applies to a given service at a specific virtual timestamp.
type Injector struct {
	config model.FaultConfig
}

// NewInjector creates a new fault injector.
func NewInjector(cfg model.FaultConfig) *Injector {
	return &Injector{config: cfg}
}

// IsActiveAt returns true if the configured fault is active for the service at timestampMs.
func (inj *Injector) IsActiveAt(serviceID string, timestampMs float64) bool {
	if inj.config.Type == model.FaultNone || inj.config.Type == "" {
		return false
	}
	if inj.config.TargetService != serviceID {
		return false
	}
	sec := timestampMs / 1000.0
	return sec >= inj.config.StartSec && sec < inj.config.EndSec
}

// Evaluate determines if a request attempt at serviceID at timestampMs fails or incurs extra latency,
// using deterministic uniform values in [0.0, 1.0) provided by the caller.
func (inj *Injector) Evaluate(serviceID string, baseFailureRate float64, timestampMs float64, faultDraw float64, baseDraw float64) EvaluationResult {
	res := EvaluationResult{}
	active := inj.IsActiveAt(serviceID, timestampMs)
	res.FaultActive = active

	if active {
		switch inj.config.Type {
		case model.FaultOutage:
			res.ShouldFail = true
			res.FailureReason = model.ReasonFaultOutage
			return res
		case model.FaultLatency:
			res.ExtraDelayMs = inj.config.LatencyMs
		case model.FaultErrorRate:
			effectiveRate := inj.config.FailureRate
			if effectiveRate <= 0 {
				effectiveRate = 0.5
			}
			if faultDraw < effectiveRate {
				res.ShouldFail = true
				res.FailureReason = model.ReasonFaultError
				return res
			}
		}
	}

	// Evaluate base failure rate if not already failed
	if baseFailureRate > 0 {
		if baseDraw < baseFailureRate {
			res.ShouldFail = true
			res.FailureReason = model.ReasonBaseError
			return res
		}
	}

	return res
}
