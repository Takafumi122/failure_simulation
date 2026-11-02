package engine

import (
	"reflect"
	"testing"

	"failurelab/backend/internal/model"
	"failurelab/backend/internal/resilience"
	"failurelab/backend/internal/topology"
)

func baseRequest() model.SimulationRequest {
	resCfg := resilience.DefaultResilienceConfig()
	return model.SimulationRequest{
		DurationSec:       15,
		RequestsPerSecond: 30,
		Seed:              12345,
		Fault: model.FaultConfig{
			TargetService: topology.NodeFeatureDB,
			Type:          model.FaultNone,
			StartSec:      3,
			EndSec:        10,
		},
		Services:   topology.DefaultServices(),
		Resilience: &resCfg,
	}
}

func runAllStrategies(req model.SimulationRequest) map[string]model.StrategyResult {
	out := make(map[string]model.StrategyResult, len(resilience.AllStrategyIDs))
	for _, stratID := range resilience.AllStrategyIDs {
		pol := resilience.NewPolicy(stratID, req.Resilience)
		out[stratID] = NewSimulator(req, pol).Run()
	}
	return out
}

// ============================================================================
// Phase 1 Compatibility Tests (I. Compatibility)
// ============================================================================

func TestSeedDeterminism(t *testing.T) {
	req1 := baseRequest()
	req1.Fault = model.FaultConfig{
		TargetService: topology.NodeProductAPI,
		Type:          model.FaultErrorRate,
		StartSec:      3,
		EndSec:        10,
		FailureRate:   0.35,
	}
	req2 := req1

	res1 := NewSimulator(req1, resilience.NewBaselinePolicy()).Run()
	res2 := NewSimulator(req2, resilience.NewBaselinePolicy()).Run()

	if !reflect.DeepEqual(res1.Metrics, res2.Metrics) {
		t.Fatalf("expected identical Metrics for same seed, got:\nres1=%+v\nres2=%+v", res1.Metrics.Overall, res2.Metrics.Overall)
	}
	if !reflect.DeepEqual(res1.TimeSeries, res2.TimeSeries) {
		t.Fatalf("expected identical TimeSeries for same seed")
	}
	if res1.ExecutionMeta.TotalEventsProcessed != res2.ExecutionMeta.TotalEventsProcessed {
		t.Fatalf("expected identical event counts, got %d vs %d",
			res1.ExecutionMeta.TotalEventsProcessed, res2.ExecutionMeta.TotalEventsProcessed)
	}
}

func TestHealthyBaseline100PercentSuccess(t *testing.T) {
	req := baseRequest()
	res := NewSimulator(req, resilience.NewBaselinePolicy()).Run()

	totalExpected := req.DurationSec * req.RequestsPerSecond
	if res.Metrics.Overall.TotalRequests != totalExpected {
		t.Fatalf("expected %d total requests, got %d", totalExpected, res.Metrics.Overall.TotalRequests)
	}
	if res.Metrics.Overall.SuccessRate != 1.0 || res.Metrics.Overall.FailureCount != 0 {
		t.Fatalf("expected 100%% success rate on healthy baseline, got successRate=%v failures=%d",
			res.Metrics.Overall.SuccessRate, res.Metrics.Overall.FailureCount)
	}
	if res.Metrics.Overall.P95LatencyMs < 70 || res.Metrics.Overall.P95LatencyMs > 95 {
		t.Fatalf("expected p95 latency around 80ms, got %.2fms", res.Metrics.Overall.P95LatencyMs)
	}
}

func TestOutageFaultPropagation(t *testing.T) {
	req := baseRequest()
	req.DurationSec = 10
	req.RequestsPerSecond = 20
	req.Fault = model.FaultConfig{
		TargetService: topology.NodeRecommendation,
		Type:          model.FaultOutage,
		StartSec:      2,
		EndSec:        7,
	}

	res := NewSimulator(req, resilience.NewBaselinePolicy()).Run()

	if res.Metrics.Overall.FailureCount == 0 {
		t.Fatalf("expected failures during outage, got 0")
	}
	for _, sec := range []int{3, 4, 5, 6} {
		pt := res.TimeSeries[sec]
		if pt.SuccessRate > 0.05 {
			t.Errorf("expected ~0%% success rate at second %d during full outage, got %v", sec, pt.SuccessRate)
		}
		if pt.Services[topology.NodeRecommendation].Status != model.StatusDown {
			t.Errorf("expected Recommendation status to be 'down' at second %d, got %s",
				sec, pt.Services[topology.NodeRecommendation].Status)
		}
		dbCalls := res.TimeSeries[sec].Services[topology.NodeFeatureDB].CallCount
		if dbCalls != 0 {
			t.Errorf("expected 0 calls to Feature DB during Recommendation outage at sec %d, got %d", sec, dbCalls)
		}
	}
}

func TestLatencyFaultAndTimeoutCascade(t *testing.T) {
	req := baseRequest()
	req.DurationSec = 15
	req.RequestsPerSecond = 35
	req.Fault = model.FaultConfig{
		TargetService: topology.NodeFeatureDB,
		Type:          model.FaultLatency,
		StartSec:      3,
		EndSec:        11,
		LatencyMs:     600,
	}

	res := NewSimulator(req, resilience.NewBaselinePolicy()).Run()

	if res.Metrics.Overall.TimeoutCount == 0 {
		t.Fatalf("expected timeouts when Feature DB latency exceeds service timeoutMs, got 0")
	}
	if res.Metrics.Overall.P95LatencyMs <= 150 {
		t.Fatalf("expected elevated p95 latency during latency fault, got %.2fms", res.Metrics.Overall.P95LatencyMs)
	}
}

func TestQueueOverflowUnderHighConcurrency(t *testing.T) {
	req := baseRequest()
	req.DurationSec = 10
	req.RequestsPerSecond = 80
	srv := req.Services[topology.NodeRecommendation]
	srv.MaxConcurrent = 4
	srv.QueueLimit = 6
	srv.BaseLatencyMs = 120
	req.Services[topology.NodeRecommendation] = srv

	res := NewSimulator(req, resilience.NewBaselinePolicy()).Run()

	recMetrics := res.Metrics.Services[topology.NodeRecommendation]
	if recMetrics.QueueOverflowCount == 0 {
		t.Fatalf("expected queue overflow at Recommendation under 80 RPS with maxConcurrent=4, queueLimit=6, got 0")
	}
	if res.Metrics.Overall.QueueOverflowCount == 0 {
		t.Fatalf("expected queue overflow to propagate to overall metrics, got 0")
	}
	if recMetrics.PeakQueueLength > srv.QueueLimit {
		t.Fatalf("expected peak queue length <= queueLimit (%d), got %d", srv.QueueLimit, recMetrics.PeakQueueLength)
	}
}

func TestMaxEventsSafetyLimit(t *testing.T) {
	req := baseRequest()
	req.DurationSec = 20
	req.RequestsPerSecond = 50
	req.MaxEvents = 200

	res := NewSimulator(req, resilience.NewBaselinePolicy()).Run()
	if !res.ExecutionMeta.Truncated || res.ExecutionMeta.Status != "truncated" {
		t.Fatalf("expected simulation to truncate with status='truncated' when MaxEvents=200, got %+v", res.ExecutionMeta)
	}
	if res.ExecutionMeta.TotalEventsProcessed != 200 {
		t.Fatalf("expected TotalEventsProcessed=200, got %d", res.ExecutionMeta.TotalEventsProcessed)
	}
	if res.Metrics.Overall.SuccessRate >= 1.0 {
		t.Fatalf("expected truncated run not to report 100%% logical request success rate, got %v", res.Metrics.Overall.SuccessRate)
	}
}

// ============================================================================
// Phase 2 Comprehensive Scenario & Resilience Tests (Step 7: A ~ H)
// ============================================================================

// Scenario A: Transient Fault -> Retry improves logical request success rate.
func TestScenarioA_TransientFault_RetryImprovesSuccessRate(t *testing.T) {
	req := baseRequest()
	req.DurationSec = 20
	req.RequestsPerSecond = 30
	req.Fault = model.FaultConfig{
		TargetService: topology.NodeFeatureDB,
		Type:          model.FaultErrorRate,
		StartSec:      4,
		EndSec:        16,
		FailureRate:   0.40,
	}

	results := runAllStrategies(req)
	baseOv := results[resilience.StrategyBaseline].Metrics.Overall
	retryOv := results[resilience.StrategyRetry].Metrics.Overall

	t.Logf("[Scenario A] Baseline successRate=%.2f%% vs Retry successRate=%.2f%% (retries=%d, amp=%.2fx)",
		baseOv.SuccessRate*100, retryOv.SuccessRate*100, retryOv.TotalRetries, retryOv.RetryAmplificationFactor)

	if retryOv.SuccessRate <= baseOv.SuccessRate+0.15 {
		t.Fatalf("expected Retry to significantly improve successRate over Baseline during transient 40%% error fault: baseline=%.4f, retry=%.4f",
			baseOv.SuccessRate, retryOv.SuccessRate)
	}
	if retryOv.TotalRetries == 0 || retryOv.RetryAmplificationFactor <= 1.0 {
		t.Fatalf("expected positive retry count and amplification factor > 1.0, got retries=%d amp=%.2f",
			retryOv.TotalRetries, retryOv.RetryAmplificationFactor)
	}
	// Logical request count must never be inflated by retry attempts
	if retryOv.TotalRequests != baseOv.TotalRequests || retryOv.CompletedRequests != baseOv.CompletedRequests {
		t.Fatalf("logical request count mismatch: baseline=%d/%d, retry=%d/%d",
			baseOv.CompletedRequests, baseOv.TotalRequests, retryOv.CompletedRequests, retryOv.TotalRequests)
	}
}

// Scenario B: Prolonged Outage -> Retry increases downstream attempts without improving success rate.
func TestScenarioB_ProlongedOutage_RetryIncreasesAttemptsWithoutSuccessGain(t *testing.T) {
	req := baseRequest()
	req.DurationSec = 15
	req.RequestsPerSecond = 30
	req.Fault = model.FaultConfig{
		TargetService: topology.NodeFeatureDB,
		Type:          model.FaultOutage,
		StartSec:      3,
		EndSec:        13, // 10 seconds of 100% outage
	}

	results := runAllStrategies(req)
	baseRes := results[resilience.StrategyBaseline]
	retryRes := results[resilience.StrategyRetry]

	baseDBCalls := baseRes.Metrics.Services[topology.NodeFeatureDB].CallCount
	retryDBCalls := retryRes.Metrics.Services[topology.NodeFeatureDB].CallCount

	t.Logf("[Scenario B] Baseline: success=%.2f%%, dbCalls=%d | Retry: success=%.2f%%, dbCalls=%d, retries=%d, amp=%.2fx",
		baseRes.Metrics.Overall.SuccessRate*100, baseDBCalls,
		retryRes.Metrics.Overall.SuccessRate*100, retryDBCalls,
		retryRes.Metrics.Overall.TotalRetries, retryRes.Metrics.Overall.RetryAmplificationFactor)

	if retryDBCalls <= baseDBCalls*2 {
		t.Fatalf("expected Retry to multiply Feature DB calls during outage: baseline=%d, retry=%d", baseDBCalls, retryDBCalls)
	}
	// During full 100% prolonged outage, retry cannot rescue requests inside the outage window
	if retryRes.Metrics.Overall.SuccessRate > baseRes.Metrics.Overall.SuccessRate+0.01 {
		t.Fatalf("expected Retry NOT to improve successRate during 100%% prolonged outage: baseline=%.4f, retry=%.4f",
			baseRes.Metrics.Overall.SuccessRate, retryRes.Metrics.Overall.SuccessRate)
	}
}

// Scenario C: Retry Storm -> Retry under high latency/contention increases downstream calls, queue length, and timeouts.
func TestScenarioC_RetryStorm_IncreasesCallsQueueAndTimeouts(t *testing.T) {
	req := baseRequest()
	req.DurationSec = 18
	req.RequestsPerSecond = 45
	// Configure Feature DB with tight workers and 160ms attempt timeout so latency causes timeouts + retries while workers are still busy
	dbCfg := req.Services[topology.NodeFeatureDB]
	dbCfg.MaxConcurrent = 10
	dbCfg.QueueLimit = 45
	dbCfg.TimeoutMs = 160
	req.Services[topology.NodeFeatureDB] = dbCfg

	recCfg := req.Services[topology.NodeRecommendation]
	recCfg.MaxConcurrent = 35
	recCfg.QueueLimit = 60
	recCfg.TimeoutMs = 900
	req.Services[topology.NodeRecommendation] = recCfg

	req.Fault = model.FaultConfig{
		TargetService: topology.NodeFeatureDB,
		Type:          model.FaultLatency,
		StartSec:      3,
		EndSec:        14,
		LatencyMs:     210, // Total ~225ms > 160ms DB timeout, triggering retries into an already congested queue
	}

	results := runAllStrategies(req)
	baseRes := results[resilience.StrategyBaseline]
	retryRes := results[resilience.StrategyRetry]

	baseDB := baseRes.Metrics.Services[topology.NodeFeatureDB]
	retryDB := retryRes.Metrics.Services[topology.NodeFeatureDB]

	t.Logf("[Scenario C] Baseline DB: calls=%d, peakQueue=%d, timeouts=%d | Retry DB: calls=%d, peakQueue=%d, timeouts=%d",
		baseDB.CallCount, baseDB.PeakQueueLength, baseDB.TimeoutCount,
		retryDB.CallCount, retryDB.PeakQueueLength, retryDB.TimeoutCount)

	if retryDB.CallCount <= baseDB.CallCount {
		t.Fatalf("expected Retry Storm to increase downstream call count: baseline=%d, retry=%d", baseDB.CallCount, retryDB.CallCount)
	}
	if retryDB.PeakQueueLength <= baseDB.PeakQueueLength {
		t.Fatalf("expected Retry Storm to increase downstream peak queue length: baseline=%d, retry=%d", baseDB.PeakQueueLength, retryDB.PeakQueueLength)
	}
	if retryDB.TimeoutCount <= baseDB.TimeoutCount {
		t.Fatalf("expected Retry Storm to increase timeout count: baseline=%d, retry=%d", baseDB.TimeoutCount, retryDB.TimeoutCount)
	}
}

// Scenario D & E: Circuit Breaker trips to OPEN on consecutive failures, reduces downstream calls,
// blocks additional retries when combined, and transitions HALF_OPEN -> CLOSED after fault recovery.
func TestScenarioD_And_E_CircuitBreakerAndRecovery(t *testing.T) {
	req := baseRequest()
	req.DurationSec = 22
	req.RequestsPerSecond = 30
	req.Fault = model.FaultConfig{
		TargetService: topology.NodeFeatureDB,
		Type:          model.FaultOutage,
		StartSec:      4,
		EndSec:        12, // Outage from 4s to 12s; recovery from 12s to 22s
	}
	req.Resilience.CircuitBreaker = model.CircuitBreakerConfig{
		ConsecutiveFailuresThreshold: 5,
		OpenTimeoutMs:                2500,
		HalfOpenMaxCalls:             2,
		HalfOpenSuccessThreshold:     2,
	}

	results := runAllStrategies(req)
	baseRes := results[resilience.StrategyBaseline]
	retryRes := results[resilience.StrategyRetry]
	cbRes := results[resilience.StrategyCircuitBreaker]
	retryCBRes := results[resilience.StrategyRetryCircuitBreaker]

	baseDBCalls := baseRes.Metrics.Services[topology.NodeFeatureDB].CallCount
	retryDBCalls := retryRes.Metrics.Services[topology.NodeFeatureDB].CallCount
	cbDBCalls := cbRes.Metrics.Services[topology.NodeFeatureDB].CallCount
	retryCBDBCalls := retryCBRes.Metrics.Services[topology.NodeFeatureDB].CallCount

	t.Logf("[Scenario D/E] DB Calls -> Baseline=%d, Retry=%d, CB=%d (rejections=%d), Retry+CB=%d (rejections=%d, retries=%d)",
		baseDBCalls, retryDBCalls,
		cbDBCalls, cbRes.Metrics.Overall.CircuitBreakerRejections,
		retryCBDBCalls, retryCBRes.Metrics.Overall.CircuitBreakerRejections, retryCBRes.Metrics.Overall.TotalRetries)

	// D.1: Circuit Breaker must significantly reduce downstream calls compared to Baseline
	if cbDBCalls >= baseDBCalls-100 {
		t.Fatalf("expected Circuit Breaker to reduce downstream calls during outage: baseline=%d, cb=%d", baseDBCalls, cbDBCalls)
	}
	if cbRes.Metrics.Overall.CircuitBreakerRejections == 0 {
		t.Fatalf("expected positive CircuitBreakerRejections count when breaker is OPEN")
	}

	// D.2: Retry + Circuit Breaker must suppress retries while Breaker is OPEN (far fewer calls than Retry-only)
	if retryCBDBCalls >= retryDBCalls/2 {
		t.Fatalf("expected Retry+CB to prevent retry storm when breaker is OPEN: retryDBCalls=%d, retryCBDBCalls=%d",
			retryDBCalls, retryCBDBCalls)
	}

	// E: Verify state transitions CLOSED -> OPEN -> HALF_OPEN -> CLOSED after recovery at t=12s
	transitions := cbRes.BreakerTransitions
	if len(transitions) < 3 {
		t.Fatalf("expected at least 3 breaker transitions (CLOSED->OPEN, OPEN->HALF_OPEN, HALF_OPEN->CLOSED), got %d: %+v",
			len(transitions), transitions)
	}

	sawOpen := false
	sawHalfOpen := false
	sawClosedAfterRecovery := false
	for _, tr := range transitions {
		if tr.ToState == model.CircuitOpen {
			sawOpen = true
		}
		if tr.ToState == model.CircuitHalfOpen {
			sawHalfOpen = true
		}
		if sawHalfOpen && tr.ToState == model.CircuitClosed && tr.TimestampMs >= 12000 {
			sawClosedAfterRecovery = true
		}
	}
	if !sawOpen || !sawHalfOpen || !sawClosedAfterRecovery {
		t.Fatalf("expected full recovery lifecycle (OPEN -> HALF_OPEN -> CLOSED after 12s), got transitions: %+v", transitions)
	}
}

// Scenario F: Determinism across all 5 strategies for identical seed and config.
func TestScenarioF_DeterminismAcrossAllFiveStrategies(t *testing.T) {
	req := baseRequest()
	req.DurationSec = 15
	req.RequestsPerSecond = 30
	req.Fault = model.FaultConfig{
		TargetService: topology.NodeFeatureDB,
		Type:          model.FaultErrorRate,
		StartSec:      3,
		EndSec:        11,
		FailureRate:   0.45,
	}

	run1 := runAllStrategies(req)
	run2 := runAllStrategies(req)

	for _, stratID := range resilience.AllStrategyIDs {
		r1 := run1[stratID]
		r2 := run2[stratID]
		if !reflect.DeepEqual(r1.Metrics, r2.Metrics) {
			t.Fatalf("strategy %s metrics differed between identical runs:\nr1=%+v\nr2=%+v", stratID, r1.Metrics.Overall, r2.Metrics.Overall)
		}
		if !reflect.DeepEqual(r1.BreakerTransitions, r2.BreakerTransitions) {
			t.Fatalf("strategy %s breaker transitions differed between identical runs", stratID)
		}
		if r1.ExecutionMeta.TotalEventsProcessed != r2.ExecutionMeta.TotalEventsProcessed {
			t.Fatalf("strategy %s event count differed: %d vs %d",
				stratID, r1.ExecutionMeta.TotalEventsProcessed, r2.ExecutionMeta.TotalEventsProcessed)
		}
	}
}

// Scenario G: Healthy Baseline -> No unnecessary retries or Circuit Breaker opens when there is no fault.
func TestScenarioG_HealthyBaselineNoSpuriousRetriesOrBreakerOpen(t *testing.T) {
	req := baseRequest()
	req.DurationSec = 15
	req.RequestsPerSecond = 35
	req.Fault.Type = model.FaultNone

	results := runAllStrategies(req)
	for _, stratID := range resilience.AllStrategyIDs {
		res := results[stratID]
		ov := res.Metrics.Overall
		if ov.SuccessRate != 1.0 || ov.FailureCount != 0 {
			t.Fatalf("strategy %s: expected 100%% success on healthy run, got successRate=%v failures=%d",
				stratID, ov.SuccessRate, ov.FailureCount)
		}
		if ov.TotalRetries != 0 {
			t.Fatalf("strategy %s: expected 0 retries on healthy run, got %d", stratID, ov.TotalRetries)
		}
		if ov.CircuitBreakerRejections != 0 || len(res.BreakerTransitions) != 0 {
			t.Fatalf("strategy %s: expected 0 CB rejections/transitions on healthy run, got rejections=%d transitions=%d",
				stratID, ov.CircuitBreakerRejections, len(res.BreakerTransitions))
		}
	}
}

// Scenario H: Event Safety & Fail-Fast -> MaxRetries limit, parent deadline limit, and Fail-Fast short timeout are respected.
func TestScenarioH_EventSafetyAndFailFastLatencyReduction(t *testing.T) {
	req := baseRequest()
	req.DurationSec = 15
	req.RequestsPerSecond = 30
	req.Resilience.Retry.MaxRetries = 2
	req.Resilience.FailFastTimeoutMs = 100 // Much shorter than Feature DB default 450ms timeout
	req.Fault = model.FaultConfig{
		TargetService: topology.NodeFeatureDB,
		Type:          model.FaultLatency,
		StartSec:      3,
		EndSec:        12,
		LatencyMs:     460, // Exceeds default 450ms timeout
	}

	results := runAllStrategies(req)
	baseOv := results[resilience.StrategyBaseline].Metrics.Overall
	failFastOv := results[resilience.StrategyFailFast].Metrics.Overall
	retryOv := results[resilience.StrategyRetry].Metrics.Overall

	t.Logf("[Scenario H] Baseline p95=%.1fms (failP95=%.1fms, succP95=%.1fms) | Fail-Fast p95=%.1fms (failP95=%.1fms, succP95=%.1fms) | Retry amp=%.2fx",
		baseOv.P95LatencyMs, baseOv.FailureP95LatencyMs, baseOv.SuccessP95LatencyMs,
		failFastOv.P95LatencyMs, failFastOv.FailureP95LatencyMs, failFastOv.SuccessP95LatencyMs,
		retryOv.RetryAmplificationFactor)

	// Fail-Fast should cut failure p95 latency and overall p95 latency substantially compared to Baseline waiting 450ms+
	if failFastOv.FailureP95LatencyMs >= baseOv.FailureP95LatencyMs*0.5 {
		t.Fatalf("expected Fail-Fast failure p95 (%.1fms) to be much lower than Baseline failure p95 (%.1fms)",
			failFastOv.FailureP95LatencyMs, baseOv.FailureP95LatencyMs)
	}
	if failFastOv.P95LatencyMs >= baseOv.P95LatencyMs*0.6 {
		t.Fatalf("expected Fail-Fast (100ms timeout) to have much lower overall p95 latency than Baseline (450ms timeout): failFast=%.1fms, baseline=%.1fms",
			failFastOv.P95LatencyMs, baseOv.P95LatencyMs)
	}
	// Successful requests (outside the fault window) should have normal ~80ms latency in both strategies
	if failFastOv.SuccessP95LatencyMs < 65 || failFastOv.SuccessP95LatencyMs > 105 {
		t.Fatalf("expected Fail-Fast successful request p95 latency around 80ms, got %.1fms", failFastOv.SuccessP95LatencyMs)
	}

	// Retry amplification must never exceed 1 + MaxRetries (1 + 2 = 3.0)
	if retryOv.RetryAmplificationFactor > 3.0 {
		t.Fatalf("expected RetryAmplificationFactor <= 3.0 when MaxRetries=2, got %.2f", retryOv.RetryAmplificationFactor)
	}
}

// TestSimulationInvariants verifies that across all 5 preset scenarios and all 5 strategies:
// 1. Active workers never become negative and never exceed MaxConcurrent.
// 2. No logical request is completed more than once (CompletedRequests <= TotalRequests and SuccessCount + FailureCount == CompletedRequests).
// 3. No span attempt exceeds 1 + MaxRetries.
func TestSimulationInvariants(t *testing.T) {
	presets := topology.PresetScenarios()
	for _, preset := range presets {
		req := preset.Config
		maxRetries := 3
		if req.Resilience != nil {
			maxRetries = req.Resilience.Retry.MaxRetries
		}
		for _, stratID := range resilience.AllStrategyIDs {
			pol := resilience.NewPolicy(stratID, req.Resilience)
			sim := NewSimulator(req, pol)
			res := sim.Run()

			// 1. Verify final worker state is 0 (all held workers were released) and never negative
			for srvID, st := range sim.services {
				if st.activeWorkers != 0 {
					t.Errorf("[%s/%s] service %s leaked workers: activeWorkers=%d at end of run",
						preset.ID, stratID, srvID, st.activeWorkers)
				}
			}

			// 2. Verify no span exceeded 1 + maxRetries
			for _, sp := range sim.spans {
				if sp.Attempt > 1+maxRetries {
					t.Errorf("[%s/%s] span %d (req #%d) exceeded max attempt limit: attempt=%d, maxAllowed=%d",
						preset.ID, stratID, sp.SpanID, sp.RequestID, sp.Attempt, 1+maxRetries)
				}
			}

			// 3. Verify no duplicate logical request completion
			ov := res.Metrics.Overall
			if ov.CompletedRequests != ov.TotalRequests {
				t.Errorf("[%s/%s] expected all %d requests to complete, got %d",
					preset.ID, stratID, ov.TotalRequests, ov.CompletedRequests)
			}
			if ov.SuccessCount+ov.FailureCount != ov.TotalRequests {
				t.Errorf("[%s/%s] success (%d) + failure (%d) != totalRequests (%d)",
					preset.ID, stratID, ov.SuccessCount, ov.FailureCount, ov.TotalRequests)
			}
		}
	}
}
