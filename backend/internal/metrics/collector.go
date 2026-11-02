package metrics

import (
	"failurelab/backend/internal/fault"
	"failurelab/backend/internal/model"
	"failurelab/backend/internal/topology"
)

// SpanOutcome represents the completed outcome of a request attempt at a single service (or client root).
type SpanOutcome struct {
	RequestID     int
	ServiceID     string
	Attempt       int
	ArrivalSec    int
	CompletionSec int
	LatencyMs     float64
	Success       bool
	Reason        model.FailureReason
}

// serviceAccumulator stores raw observations for a single service.
type serviceAccumulator struct {
	callCount                int
	primaryCallCount         int
	retryCallCount           int
	successCount             int
	failureCount             int
	timeoutCount             int
	queueOverflowCount       int
	circuitBreakerRejections int
	latencies                []float64
	successLatencies         []float64
	failureLatencies         []float64
	peakConcurrent           int
	peakQueueLength          int
	lastCircuitState         model.CircuitState
}

// Collector aggregates spans, concurrency watermarks, retries, CB rejections, and time-series buckets.
type Collector struct {
	durationSec  int
	injector     *fault.Injector
	targetCallee string

	rootOutcomes []SpanOutcome
	overallPeak  map[string]*serviceAccumulator

	// Per-second buckets: index [0 .. durationSec-1]
	rootBySec    [][]SpanOutcome
	serviceBySec []map[string]*serviceAccumulator
	latBySec     []map[string][]float64
}

// NewCollector initializes a metrics collector for durationSec seconds.
func NewCollector(durationSec int, injector *fault.Injector, targetCallee string) *Collector {
	if durationSec < 1 {
		durationSec = 1
	}
	if targetCallee == "" {
		targetCallee = topology.NodeFeatureDB
	}
	c := &Collector{
		durationSec:  durationSec,
		injector:     injector,
		targetCallee: targetCallee,
		overallPeak:  make(map[string]*serviceAccumulator),
		rootBySec:    make([][]SpanOutcome, durationSec),
		serviceBySec: make([]map[string]*serviceAccumulator, durationSec),
		latBySec:     make([]map[string][]float64, durationSec),
	}

	for _, id := range topology.OrderedServiceIDs {
		c.overallPeak[id] = &serviceAccumulator{lastCircuitState: model.CircuitClosed}
	}
	for sec := 0; sec < durationSec; sec++ {
		c.serviceBySec[sec] = make(map[string]*serviceAccumulator)
		c.latBySec[sec] = make(map[string][]float64)
		for _, id := range topology.OrderedServiceIDs {
			c.serviceBySec[sec][id] = &serviceAccumulator{lastCircuitState: model.CircuitClosed}
		}
	}
	return c
}

func (c *Collector) clampSec(timestampMs float64) int {
	sec := int(timestampMs / 1000.0)
	if sec < 0 {
		return 0
	}
	if sec >= c.durationSec {
		return c.durationSec - 1
	}
	return sec
}

// RecordConcurrency updates the peak worker and queue watermarks for a service at timestampMs.
func (c *Collector) RecordConcurrency(serviceID string, timestampMs float64, activeWorkers int, queueLen int) {
	acc, ok := c.overallPeak[serviceID]
	if !ok {
		return
	}
	if activeWorkers > acc.peakConcurrent {
		acc.peakConcurrent = activeWorkers
	}
	if queueLen > acc.peakQueueLength {
		acc.peakQueueLength = queueLen
	}

	sec := c.clampSec(timestampMs)
	secAcc := c.serviceBySec[sec][serviceID]
	if activeWorkers > secAcc.peakConcurrent {
		secAcc.peakConcurrent = activeWorkers
	}
	if queueLen > secAcc.peakQueueLength {
		secAcc.peakQueueLength = queueLen
	}
}

// RecordCircuitRejection records a call rejected by the Circuit Breaker before reaching calleeID.
// Note: A CB rejection does NOT increment calleeID.callCount because the downstream service was protected.
func (c *Collector) RecordCircuitRejection(calleeID string, timestampMs float64) {
	acc, ok := c.overallPeak[calleeID]
	if !ok {
		return
	}
	acc.circuitBreakerRejections++

	sec := c.clampSec(timestampMs)
	secAcc := c.serviceBySec[sec][calleeID]
	secAcc.circuitBreakerRejections++
}

// RecordCircuitStateAt updates the circuit state snapshot for a service at a given virtual time.
func (c *Collector) RecordCircuitStateAt(calleeID string, timestampMs float64, state model.CircuitState) {
	sec := c.clampSec(timestampMs)
	for s := sec; s < c.durationSec; s++ {
		if acc, ok := c.serviceBySec[s][calleeID]; ok {
			acc.lastCircuitState = state
		}
	}
}

// RecordServiceSpan records the completion or failure of an actual attempt on a backend service.
func (c *Collector) RecordServiceSpan(outcome SpanOutcome) {
	acc, ok := c.overallPeak[outcome.ServiceID]
	if !ok {
		return
	}
	acc.callCount++
	if outcome.Attempt > 1 {
		acc.retryCallCount++
	} else {
		acc.primaryCallCount++
	}
	acc.latencies = append(acc.latencies, outcome.LatencyMs)
	if outcome.Success {
		acc.successCount++
		acc.successLatencies = append(acc.successLatencies, outcome.LatencyMs)
	} else {
		acc.failureCount++
		acc.failureLatencies = append(acc.failureLatencies, outcome.LatencyMs)
		if outcome.Reason == model.ReasonTimeout {
			acc.timeoutCount++
		}
		if outcome.Reason == model.ReasonQueueOverflow {
			acc.queueOverflowCount++
		}
	}

	sec := outcome.ArrivalSec
	if sec < 0 {
		sec = 0
	}
	if sec >= c.durationSec {
		sec = c.durationSec - 1
	}
	secAcc := c.serviceBySec[sec][outcome.ServiceID]
	secAcc.callCount++
	if outcome.Attempt > 1 {
		secAcc.retryCallCount++
	} else {
		secAcc.primaryCallCount++
	}
	c.latBySec[sec][outcome.ServiceID] = append(c.latBySec[sec][outcome.ServiceID], outcome.LatencyMs)
	if outcome.Success {
		secAcc.successCount++
	} else {
		secAcc.failureCount++
		if outcome.Reason == model.ReasonTimeout {
			secAcc.timeoutCount++
		}
		if outcome.Reason == model.ReasonQueueOverflow {
			secAcc.queueOverflowCount++
		}
	}
}

// RecordRootOutcome records the end-to-end logical client request outcome.
func (c *Collector) RecordRootOutcome(outcome SpanOutcome) {
	c.rootOutcomes = append(c.rootOutcomes, outcome)
	sec := outcome.ArrivalSec
	if sec < 0 {
		sec = 0
	}
	if sec >= c.durationSec {
		sec = c.durationSec - 1
	}
	c.rootBySec[sec] = append(c.rootBySec[sec], outcome)
}

// ClassifyStatus determines the health status of a service based on error rate and queue saturation.
func ClassifyStatus(callCount int, errorRate float64, peakQueue int, queueLimit int, faultOutageActive bool) model.ServiceStatus {
	if faultOutageActive || (callCount > 0 && errorRate >= 0.95) {
		return model.StatusDown
	}
	if errorRate >= 0.25 || (queueLimit > 0 && float64(peakQueue)/float64(queueLimit) >= 0.85) {
		return model.StatusCritical
	}
	if errorRate > 0.01 || (queueLimit > 0 && float64(peakQueue)/float64(queueLimit) >= 0.30) {
		return model.StatusDegraded
	}
	return model.StatusHealthy
}

// BuildFinal computes the final SimulationMetrics and TimeSeriesPoint slice.
func (c *Collector) BuildFinal(totalGeneratedRequests int, serviceConfigs map[string]model.ServiceConfig, faultCfg model.FaultConfig, truncated bool) (model.SimulationMetrics, []model.TimeSeriesPoint) {
	var rootLatencies, rootSuccessLatencies, rootFailureLatencies []float64
	var successCount, failureCount, timeoutCount, queueOverflowCount int

	for _, o := range c.rootOutcomes {
		rootLatencies = append(rootLatencies, o.LatencyMs)
		if o.Success {
			successCount++
			rootSuccessLatencies = append(rootSuccessLatencies, o.LatencyMs)
		} else {
			failureCount++
			rootFailureLatencies = append(rootFailureLatencies, o.LatencyMs)
			if o.Reason == model.ReasonTimeout {
				timeoutCount++
			}
			if o.Reason == model.ReasonQueueOverflow {
				queueOverflowCount++
			}
		}
	}

	completed := len(c.rootOutcomes)
	// If simulation was truncated by MaxEvents before all requests completed,
	// unfinished requests are counted as failures rather than ignored.
	unfinished := 0
	if totalGeneratedRequests > completed {
		unfinished = totalGeneratedRequests - completed
		if truncated {
			failureCount += unfinished
		}
	}

	denominator := completed
	if truncated && totalGeneratedRequests > 0 {
		denominator = totalGeneratedRequests
	}

	var successRate, errorRate float64
	if denominator > 0 {
		successRate = Round4(float64(successCount) / float64(denominator))
		errorRate = Round4(float64(failureCount) / float64(denominator))
	}

	var totalDownstreamCalls, totalRetries, totalCBRejections int
	var maxWorkerUtil float64
	var maxQueueLen int

	serviceMetrics := make(map[string]model.ServiceMetrics, len(topology.OrderedServiceIDs))
	for _, id := range topology.OrderedServiceIDs {
		acc := c.overallPeak[id]
		totalDownstreamCalls += acc.callCount
		totalRetries += acc.retryCallCount
		totalCBRejections += acc.circuitBreakerRejections

		var sRate, eRate float64
		if acc.callCount > 0 {
			sRate = Round4(float64(acc.successCount) / float64(acc.callCount))
			eRate = Round4(float64(acc.failureCount) / float64(acc.callCount))
		}
		cfg := serviceConfigs[id]
		var workerUtil float64
		if cfg.MaxConcurrent > 0 {
			workerUtil = Round4(float64(acc.peakConcurrent) / float64(cfg.MaxConcurrent))
			if workerUtil > 1.0 {
				workerUtil = 1.0
			}
		}
		if workerUtil > maxWorkerUtil {
			maxWorkerUtil = workerUtil
		}
		if acc.peakQueueLength > maxQueueLen {
			maxQueueLen = acc.peakQueueLength
		}

		status := ClassifyStatus(acc.callCount, eRate, acc.peakQueueLength, cfg.QueueLimit, false)
		serviceMetrics[id] = model.ServiceMetrics{
			ServiceID:                id,
			ServiceName:              topology.ServiceName(id),
			CallCount:                acc.callCount,
			PrimaryCallCount:         acc.primaryCallCount,
			RetryCallCount:           acc.retryCallCount,
			SuccessCount:             acc.successCount,
			FailureCount:             acc.failureCount,
			TimeoutCount:             acc.timeoutCount,
			QueueOverflowCount:       acc.queueOverflowCount,
			CircuitBreakerRejections: acc.circuitBreakerRejections,
			SuccessRate:              sRate,
			ErrorRate:                eRate,
			AvgLatencyMs:             Average(acc.latencies),
			P50LatencyMs:             Percentile(acc.latencies, 50),
			P95LatencyMs:             Percentile(acc.latencies, 95),
			P99LatencyMs:             Percentile(acc.latencies, 99),
			SuccessAvgLatencyMs:      Average(acc.successLatencies),
			SuccessP95LatencyMs:      Percentile(acc.successLatencies, 95),
			FailureAvgLatencyMs:      Average(acc.failureLatencies),
			FailureP95LatencyMs:      Percentile(acc.failureLatencies, 95),
			PeakConcurrent:           acc.peakConcurrent,
			MaxConcurrent:            cfg.MaxConcurrent,
			WorkerUtilization:        workerUtil,
			PeakQueueLength:          acc.peakQueueLength,
			Status:                   status,
		}
	}

	// Calculate Retry Amplification Factor on the target callee (or overall if primary calls > 0)
	retryAmp := 1.0
	if targetAcc, ok := c.overallPeak[c.targetCallee]; ok && targetAcc.primaryCallCount > 0 {
		retryAmp = Round2(float64(targetAcc.callCount) / float64(targetAcc.primaryCallCount))
	} else if totalDownstreamCalls > totalRetries && (totalDownstreamCalls-totalRetries) > 0 {
		primaryTotal := totalDownstreamCalls - totalRetries
		retryAmp = Round2(float64(totalDownstreamCalls) / float64(primaryTotal))
	}

	targetEdgeCalls := 0
	if targetAcc, ok := c.overallPeak[c.targetCallee]; ok {
		targetEdgeCalls = targetAcc.callCount
	}

	overall := model.OverallMetrics{
		TotalRequests:            totalGeneratedRequests,
		CompletedRequests:        completed,
		SuccessCount:             successCount,
		FailureCount:             failureCount,
		TimeoutCount:             timeoutCount,
		QueueOverflowCount:       queueOverflowCount,
		SuccessRate:              successRate,
		ErrorRate:                errorRate,
		AvgLatencyMs:             Average(rootLatencies),
		P50LatencyMs:             Percentile(rootLatencies, 50),
		P95LatencyMs:             Percentile(rootLatencies, 95),
		P99LatencyMs:             Percentile(rootLatencies, 99),
		SuccessAvgLatencyMs:      Average(rootSuccessLatencies),
		SuccessP95LatencyMs:      Percentile(rootSuccessLatencies, 95),
		FailureAvgLatencyMs:      Average(rootFailureLatencies),
		FailureP95LatencyMs:      Percentile(rootFailureLatencies, 95),
		TotalDownstreamCalls:     totalDownstreamCalls,
		TargetEdgeCalls:          targetEdgeCalls,
		TotalRetries:             totalRetries,
		RetryAmplificationFactor: retryAmp,
		CircuitBreakerRejections: totalCBRejections,
		MaxWorkerUtilization:     maxWorkerUtil,
		MaxQueueLength:           maxQueueLen,
	}

	timeSeries := make([]model.TimeSeriesPoint, c.durationSec)
	for sec := 0; sec < c.durationSec; sec++ {
		midMs := float64(sec)*1000.0 + 500.0
		outcomes := c.rootBySec[sec]
		var secLat []float64
		var sCount, fCount, tCount, qCount int
		for _, o := range outcomes {
			secLat = append(secLat, o.LatencyMs)
			if o.Success {
				sCount++
			} else {
				fCount++
				if o.Reason == model.ReasonTimeout {
					tCount++
				}
				if o.Reason == model.ReasonQueueOverflow {
					qCount++
				}
			}
		}
		tot := len(outcomes)
		var sRate, eRate float64
		if tot > 0 {
			sRate = Round4(float64(sCount) / float64(tot))
			eRate = Round4(float64(fCount) / float64(tot))
		} else {
			sRate = 1.0
			eRate = 0.0
		}

		anyFaultActive := false
		var secRetries, secCBRejects int
		secServices := make(map[string]model.ServiceSecondMetrics, len(topology.OrderedServiceIDs))
		for _, id := range topology.OrderedServiceIDs {
			sAcc := c.serviceBySec[sec][id]
			sLats := c.latBySec[sec][id]
			secRetries += sAcc.retryCallCount
			secCBRejects += sAcc.circuitBreakerRejections

			var srvSRate, srvERate float64
			if sAcc.callCount > 0 {
				srvSRate = Round4(float64(sAcc.successCount) / float64(sAcc.callCount))
				srvERate = Round4(float64(sAcc.failureCount) / float64(sAcc.callCount))
			} else {
				srvSRate = 1.0
				srvERate = 0.0
			}
			fActive := c.injector.IsActiveAt(id, midMs)
			if fActive {
				anyFaultActive = true
			}
			outageNow := fActive && faultCfg.Type == model.FaultOutage
			status := ClassifyStatus(sAcc.callCount, srvERate, sAcc.peakQueueLength, serviceConfigs[id].QueueLimit, outageNow)

			secServices[id] = model.ServiceSecondMetrics{
				ServiceID:                id,
				CallCount:                sAcc.callCount,
				RetryCount:               sAcc.retryCallCount,
				SuccessCount:             sAcc.successCount,
				FailureCount:             sAcc.failureCount,
				TimeoutCount:             sAcc.timeoutCount,
				QueueOverflowCount:       sAcc.queueOverflowCount,
				CircuitBreakerRejections: sAcc.circuitBreakerRejections,
				SuccessRate:              srvSRate,
				ErrorRate:                srvERate,
				AvgLatencyMs:             Average(sLats),
				P95LatencyMs:             Percentile(sLats, 95),
				PeakConcurrent:           sAcc.peakConcurrent,
				PeakQueueLength:          sAcc.peakQueueLength,
				FaultActive:              fActive,
				CircuitState:             sAcc.lastCircuitState,
				Status:                   status,
			}
		}

		timeSeries[sec] = model.TimeSeriesPoint{
			Second:                   sec,
			TotalRequests:            tot,
			SuccessCount:             sCount,
			FailureCount:             fCount,
			TimeoutCount:             tCount,
			QueueOverflowCount:       qCount,
			RetryCount:               secRetries,
			CircuitBreakerRejections: secCBRejects,
			SuccessRate:              sRate,
			ErrorRate:                eRate,
			AvgLatencyMs:             Average(secLat),
			P95LatencyMs:             Percentile(secLat, 95),
			FaultActive:              anyFaultActive,
			Services:                 secServices,
		}
	}

	return model.SimulationMetrics{
		Overall:  overall,
		Services: serviceMetrics,
	}, timeSeries
}
