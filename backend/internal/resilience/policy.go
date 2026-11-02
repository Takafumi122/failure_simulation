package resilience

import (
	"fmt"
	"math"

	"failurelab/backend/internal/model"
	"failurelab/backend/internal/topology"
)

const (
	StrategyBaseline            = "baseline"
	StrategyRetry               = "retry"
	StrategyCircuitBreaker      = "circuit_breaker"
	StrategyRetryCircuitBreaker = "retry_circuit_breaker"
	StrategyFailFast            = "fail_fast"
)

// AllStrategyIDs lists the 5 canonical comparison strategies in order.
var AllStrategyIDs = []string{
	StrategyBaseline,
	StrategyRetry,
	StrategyCircuitBreaker,
	StrategyRetryCircuitBreaker,
	StrategyFailFast,
}

// DefaultResilienceConfig returns the default Phase 2 resilience parameters.
func DefaultResilienceConfig() model.ResilienceConfig {
	return model.ResilienceConfig{
		TargetCaller: topology.NodeRecommendation,
		TargetCallee: topology.NodeFeatureDB,
		Retry: model.RetryConfig{
			MaxRetries:        3,
			InitialBackoffMs:  25,
			MaxBackoffMs:      250,
			BackoffMultiplier: 2.0,
			Jitter:            true,
		},
		CircuitBreaker: model.CircuitBreakerConfig{
			ConsecutiveFailuresThreshold: 5,
			OpenTimeoutMs:                3000,
			HalfOpenMaxCalls:             2,
			HalfOpenSuccessThreshold:     2,
		},
		FailFastTimeoutMs: 120,
	}
}

// NormalizeResilienceConfig fills in safe defaults for any missing fields.
func NormalizeResilienceConfig(cfg *model.ResilienceConfig) model.ResilienceConfig {
	def := DefaultResilienceConfig()
	if cfg == nil {
		return def
	}
	res := *cfg
	if res.TargetCaller == "" {
		res.TargetCaller = def.TargetCaller
	}
	if res.TargetCallee == "" {
		res.TargetCallee = def.TargetCallee
	}
	if res.Retry.MaxRetries < 0 {
		res.Retry.MaxRetries = def.Retry.MaxRetries
	}
	if res.Retry.InitialBackoffMs <= 0 {
		res.Retry.InitialBackoffMs = def.Retry.InitialBackoffMs
	}
	if res.Retry.MaxBackoffMs <= 0 {
		res.Retry.MaxBackoffMs = def.Retry.MaxBackoffMs
	}
	if res.Retry.BackoffMultiplier < 1.0 {
		res.Retry.BackoffMultiplier = def.Retry.BackoffMultiplier
	}
	if res.CircuitBreaker.ConsecutiveFailuresThreshold <= 0 {
		res.CircuitBreaker.ConsecutiveFailuresThreshold = def.CircuitBreaker.ConsecutiveFailuresThreshold
	}
	if res.CircuitBreaker.OpenTimeoutMs <= 0 {
		res.CircuitBreaker.OpenTimeoutMs = def.CircuitBreaker.OpenTimeoutMs
	}
	if res.CircuitBreaker.HalfOpenMaxCalls <= 0 {
		res.CircuitBreaker.HalfOpenMaxCalls = def.CircuitBreaker.HalfOpenMaxCalls
	}
	if res.CircuitBreaker.HalfOpenSuccessThreshold <= 0 {
		res.CircuitBreaker.HalfOpenSuccessThreshold = def.CircuitBreaker.HalfOpenSuccessThreshold
	}
	if res.FailFastTimeoutMs <= 0 {
		res.FailFastTimeoutMs = def.FailFastTimeoutMs
	}
	return res
}

// IsRetryableReason returns true if the failure reason is eligible for retry.
// Circuit Breaker Open rejections (ReasonCircuitOpen) are explicitly non-retryable.
func IsRetryableReason(reason model.FailureReason) bool {
	switch reason {
	case model.ReasonFaultOutage,
		model.ReasonFaultError,
		model.ReasonBaseError,
		model.ReasonQueueOverflow,
		model.ReasonTimeout,
		model.ReasonDownstreamError:
		return true
	case model.ReasonCircuitOpen, model.ReasonNone:
		return false
	default:
		return false
	}
}

// Policy defines the interface for service call resilience strategies across call boundaries.
type Policy interface {
	ID() string
	Name() string
	TargetEdge() (callerID string, calleeID string)
	// AttemptTimeoutOverride returns a >0 timeout in ms if the policy overrides per-attempt timeout for caller->callee.
	AttemptTimeoutOverride(callerID string, calleeID string) float64
	// AllowCall checks whether a downstream call from callerID to calleeID is allowed at currentTimeMs.
	// If the Circuit Breaker transitions (e.g. OPEN -> HALF_OPEN), transition is non-nil.
	AllowCall(callerID string, calleeID string, currentTimeMs float64) (allowed bool, reason model.FailureReason, transition *model.BreakerTransition)
	// OnResult records the outcome of an attempt on callerID->calleeID and decides whether to schedule a retry.
	// jitterDraw is a deterministic uniform float64 in [0.0, 1.0).
	OnResult(callerID string, calleeID string, currentTimeMs float64, success bool, reason model.FailureReason, attempt int, jitterDraw float64) (shouldRetry bool, backoffMs float64, transition *model.BreakerTransition)
	// CurrentCircuitState returns the current Circuit Breaker state for a target service (if applicable).
	CurrentCircuitState(calleeID string) model.CircuitState
}

// edgeBreaker holds the isolated state machine for a single caller->callee boundary.
type edgeBreaker struct {
	callerID            string
	calleeID            string
	cfg                 model.CircuitBreakerConfig
	state               model.CircuitState
	consecutiveFailures int
	openedAtMs          float64
	halfOpenInFlight    int
	halfOpenSuccesses   int
}

func newEdgeBreaker(callerID, calleeID string, cfg model.CircuitBreakerConfig) *edgeBreaker {
	return &edgeBreaker{
		callerID: callerID,
		calleeID: calleeID,
		cfg:      cfg,
		state:    model.CircuitClosed,
	}
}

func (b *edgeBreaker) makeTransition(now float64, from, to model.CircuitState, reason string) *model.BreakerTransition {
	sec := int(now / 1000.0)
	if sec < 0 {
		sec = 0
	}
	return &model.BreakerTransition{
		TimestampMs: math.Round(now*100) / 100,
		Second:      sec,
		CallerID:    b.callerID,
		CalleeID:    b.calleeID,
		Edge:        fmt.Sprintf("%s -> %s", b.callerID, b.calleeID),
		FromState:   from,
		ToState:     to,
		Reason:      reason,
	}
}

func (b *edgeBreaker) allowCall(now float64) (bool, model.FailureReason, *model.BreakerTransition) {
	switch b.state {
	case model.CircuitClosed:
		return true, model.ReasonNone, nil

	case model.CircuitOpen:
		if now-b.openedAtMs >= b.cfg.OpenTimeoutMs {
			prev := b.state
			b.state = model.CircuitHalfOpen
			b.halfOpenInFlight = 1
			b.halfOpenSuccesses = 0
			tr := b.makeTransition(now, prev, model.CircuitHalfOpen,
				fmt.Sprintf("open timeout (%.0fms) elapsed; probing downstream in HALF_OPEN", b.cfg.OpenTimeoutMs))
			return true, model.ReasonNone, tr
		}
		return false, model.ReasonCircuitOpen, nil

	case model.CircuitHalfOpen:
		if b.halfOpenInFlight < b.cfg.HalfOpenMaxCalls {
			b.halfOpenInFlight++
			return true, model.ReasonNone, nil
		}
		return false, model.ReasonCircuitOpen, nil

	default:
		return true, model.ReasonNone, nil
	}
}

func (b *edgeBreaker) onResult(now float64, success bool, reason model.FailureReason) *model.BreakerTransition {
	// Calls rejected by the breaker itself do not mutate breaker probe counters
	if reason == model.ReasonCircuitOpen {
		return nil
	}

	switch b.state {
	case model.CircuitClosed:
		if success {
			b.consecutiveFailures = 0
			return nil
		}
		b.consecutiveFailures++
		if b.consecutiveFailures >= b.cfg.ConsecutiveFailuresThreshold {
			prev := b.state
			b.state = model.CircuitOpen
			b.openedAtMs = now
			b.halfOpenInFlight = 0
			b.halfOpenSuccesses = 0
			return b.makeTransition(now, prev, model.CircuitOpen,
				fmt.Sprintf("consecutive failures reached threshold (%d) due to %s", b.consecutiveFailures, reason))
		}
		return nil

	case model.CircuitHalfOpen:
		if b.halfOpenInFlight > 0 {
			b.halfOpenInFlight--
		}
		if success {
			b.halfOpenSuccesses++
			if b.halfOpenSuccesses >= b.cfg.HalfOpenSuccessThreshold {
				prev := b.state
				b.state = model.CircuitClosed
				b.consecutiveFailures = 0
				b.halfOpenInFlight = 0
				b.halfOpenSuccesses = 0
				return b.makeTransition(now, prev, model.CircuitClosed,
					fmt.Sprintf("probe succeeded (%d/%d consecutive successes in HALF_OPEN)", b.cfg.HalfOpenSuccessThreshold, b.cfg.HalfOpenSuccessThreshold))
			}
			return nil
		}
		// Any failure in HALF_OPEN immediately trips back to OPEN
		prev := b.state
		b.state = model.CircuitOpen
		b.openedAtMs = now
		b.halfOpenInFlight = 0
		b.halfOpenSuccesses = 0
		return b.makeTransition(now, prev, model.CircuitOpen,
			fmt.Sprintf("probe call failed in HALF_OPEN (%s); reopening circuit", reason))

	case model.CircuitOpen:
		// An already-in-flight call from before the breaker opened finished late
		if !success {
			b.openedAtMs = now
		}
		return nil
	}
	return nil
}

// ConfigurablePolicy implements all 5 strategies (baseline, retry, circuit_breaker, retry_circuit_breaker, fail_fast).
type ConfigurablePolicy struct {
	strategyID    string
	strategyName  string
	cfg           model.ResilienceConfig
	enableRetry   bool
	enableCB      bool
	enableFailFast bool
	breakers      map[string]*edgeBreaker // keyed by "caller->callee"
}

func edgeKey(callerID, calleeID string) string {
	return callerID + "->" + calleeID
}

// NewPolicy creates a Policy for the given strategyID and ResilienceConfig.
func NewPolicy(strategyID string, rawCfg *model.ResilienceConfig) Policy {
	cfg := NormalizeResilienceConfig(rawCfg)
	p := &ConfigurablePolicy{
		strategyID: strategyID,
		cfg:        cfg,
		breakers:   make(map[string]*edgeBreaker),
	}

	switch strategyID {
	case StrategyRetry:
		p.strategyName = "Retry (Exponential Backoff + Jitter)"
		p.enableRetry = true
	case StrategyCircuitBreaker:
		p.strategyName = "Circuit Breaker (Fail-Fast on Consecutive Failures)"
		p.enableCB = true
	case StrategyRetryCircuitBreaker:
		p.strategyName = "Retry + Circuit Breaker"
		p.enableRetry = true
		p.enableCB = true
	case StrategyFailFast:
		p.strategyName = fmt.Sprintf("Fail-Fast (Short Attempt Timeout %.0fms)", cfg.FailFastTimeoutMs)
		p.enableFailFast = true
	default:
		p.strategyID = StrategyBaseline
		p.strategyName = "Baseline (No Retry / No Circuit Breaker)"
	}

	if p.enableCB {
		key := edgeKey(cfg.TargetCaller, cfg.TargetCallee)
		p.breakers[key] = newEdgeBreaker(cfg.TargetCaller, cfg.TargetCallee, cfg.CircuitBreaker)
	}

	return p
}

// NewBaselinePolicy creates a default BaselinePolicy for backward compatibility with Phase 1 tests.
func NewBaselinePolicy() Policy {
	return NewPolicy(StrategyBaseline, nil)
}

// ResolvePolicy resolves a strategy ID with default config.
func ResolvePolicy(strategyID string) Policy {
	return NewPolicy(strategyID, nil)
}

func (p *ConfigurablePolicy) ID() string {
	return p.strategyID
}

func (p *ConfigurablePolicy) Name() string {
	return p.strategyName
}

func (p *ConfigurablePolicy) TargetEdge() (string, string) {
	return p.cfg.TargetCaller, p.cfg.TargetCallee
}

func (p *ConfigurablePolicy) isTargetEdge(callerID, calleeID string) bool {
	return callerID == p.cfg.TargetCaller && calleeID == p.cfg.TargetCallee
}

func (p *ConfigurablePolicy) AttemptTimeoutOverride(callerID, calleeID string) float64 {
	if p.enableFailFast && p.isTargetEdge(callerID, calleeID) {
		return p.cfg.FailFastTimeoutMs
	}
	return 0
}

func (p *ConfigurablePolicy) AllowCall(callerID, calleeID string, currentTimeMs float64) (bool, model.FailureReason, *model.BreakerTransition) {
	if !p.enableCB || !p.isTargetEdge(callerID, calleeID) {
		return true, model.ReasonNone, nil
	}
	key := edgeKey(callerID, calleeID)
	b, ok := p.breakers[key]
	if !ok {
		b = newEdgeBreaker(callerID, calleeID, p.cfg.CircuitBreaker)
		p.breakers[key] = b
	}
	return b.allowCall(currentTimeMs)
}

func (p *ConfigurablePolicy) OnResult(callerID, calleeID string, currentTimeMs float64, success bool, reason model.FailureReason, attempt int, jitterDraw float64) (bool, float64, *model.BreakerTransition) {
	if !p.isTargetEdge(callerID, calleeID) {
		return false, 0, nil
	}

	var tr *model.BreakerTransition
	if p.enableCB {
		key := edgeKey(callerID, calleeID)
		if b, ok := p.breakers[key]; ok {
			tr = b.onResult(currentTimeMs, success, reason)
		}
	}

	if success || !p.enableRetry {
		return false, 0, tr
	}

	// Check if retry is allowed:
	// 1. Must not exceed MaxRetries (attempt is 1-based, so attempt 1 can retry if MaxRetries >= 1)
	if attempt > p.cfg.Retry.MaxRetries {
		return false, 0, tr
	}
	// 2. Reason must be retryable (ReasonCircuitOpen is never retryable)
	if !IsRetryableReason(reason) {
		return false, 0, tr
	}
	// 3. If Circuit Breaker is enabled and is currently OPEN, do NOT schedule additional retries
	if p.enableCB {
		key := edgeKey(callerID, calleeID)
		if b, ok := p.breakers[key]; ok && b.state == model.CircuitOpen {
			return false, 0, tr
		}
	}

	// Calculate Exponential Backoff + Deterministic Equal Jitter
	exp := math.Pow(p.cfg.Retry.BackoffMultiplier, float64(attempt-1))
	backoff := p.cfg.Retry.InitialBackoffMs * exp
	if backoff > p.cfg.Retry.MaxBackoffMs {
		backoff = p.cfg.Retry.MaxBackoffMs
	}
	if p.cfg.Retry.Jitter {
		// Equal jitter in [0.5 * backoff, 1.0 * backoff]
		backoff = backoff * (0.5 + 0.5*jitterDraw)
	}
	if backoff < 1.0 {
		backoff = 1.0
	}

	return true, math.Round(backoff*100) / 100, tr
}

func (p *ConfigurablePolicy) CurrentCircuitState(calleeID string) model.CircuitState {
	if !p.enableCB || calleeID != p.cfg.TargetCallee {
		return model.CircuitClosed
	}
	key := edgeKey(p.cfg.TargetCaller, p.cfg.TargetCallee)
	if b, ok := p.breakers[key]; ok {
		return b.state
	}
	return model.CircuitClosed
}
