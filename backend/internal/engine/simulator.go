package engine

import (
	"container/heap"
	"fmt"

	"failurelab/backend/internal/fault"
	"failurelab/backend/internal/metrics"
	"failurelab/backend/internal/model"
	"failurelab/backend/internal/resilience"
	"failurelab/backend/internal/topology"
)

const (
	// DefaultMaxEvents guards against infinite loops or event explosion.
	DefaultMaxEvents = 500000
	// MaxRecordedLogEvents caps the number of detailed event logs returned in JSON to keep payloads responsive.
	MaxRecordedLogEvents = 1500
)

// Span tracks the lifecycle of a single service invocation (attempt) within a request trace.
type Span struct {
	SpanID            int64
	RequestID         int
	ServiceID         string
	ParentSpanID      int64 // 0 if called directly by client root
	ArrivalTimeMs     float64
	StartTimeMs       float64
	DeadlineMs        float64
	PostWorkMs        float64
	Attempt           int
	ActiveChildSpanID int64 // Tracks the currently expected downstream child attempt
	HoldingWorker     bool
	InQueue           bool
	Completed         bool
	EvalShouldFail    bool
	EvalFailureReason model.FailureReason
}

// serviceRuntimeState holds the mutable worker and queue state for a service during simulation.
type serviceRuntimeState struct {
	config        model.ServiceConfig
	activeWorkers int
	waitQueue     []int64 // FIFO queue of SpanIDs waiting for a worker
}

// Simulator executes a discrete-event simulation in virtual time.
type Simulator struct {
	req           model.SimulationRequest
	policy        resilience.Policy
	injector      *fault.Injector
	collector     *metrics.Collector
	coordRNG      *DeterministicRNG
	pq            *EventPriorityQueue
	services      map[string]*serviceRuntimeState
	spans         map[int64]*Span
	rootArrivalMs map[int]float64
	rootCompleted map[int]bool

	nextSeqID          int64
	nextSpanID         int64
	maxEvents          int
	eventsCount        int
	truncated          bool
	recordedLogs       []model.SimulationEventLog
	breakerTransitions []model.BreakerTransition
}

// NewSimulator constructs a deterministic simulator instance for the given request and strategy policy.
func NewSimulator(req model.SimulationRequest, policy resilience.Policy) *Simulator {
	mergedServices := topology.DefaultServices()
	for id, cfg := range req.Services {
		if _, ok := mergedServices[id]; ok {
			mergedServices[id] = cfg
		}
	}
	req.Services = mergedServices

	maxEv := req.MaxEvents
	if maxEv <= 0 || maxEv > DefaultMaxEvents {
		maxEv = DefaultMaxEvents
	}

	if policy == nil {
		policy = resilience.NewBaselinePolicy()
	}
	_, targetCallee := policy.TargetEdge()

	inj := fault.NewInjector(req.Fault)
	col := metrics.NewCollector(req.DurationSec, inj, targetCallee)
	coordRNG := NewDeterministicRNG(req.Seed)

	srvState := make(map[string]*serviceRuntimeState, len(topology.OrderedServiceIDs))
	for _, id := range topology.OrderedServiceIDs {
		cfg := mergedServices[id]
		if cfg.MaxConcurrent <= 0 {
			cfg.MaxConcurrent = 10
		}
		if cfg.QueueLimit < 0 {
			cfg.QueueLimit = 0
		}
		if cfg.TimeoutMs <= 0 {
			cfg.TimeoutMs = 1000
		}
		srvState[id] = &serviceRuntimeState{
			config:    cfg,
			waitQueue: make([]int64, 0, cfg.QueueLimit),
		}
	}

	return &Simulator{
		req:                req,
		policy:             policy,
		injector:           inj,
		collector:          col,
		coordRNG:           coordRNG,
		pq:                 NewEventQueue(),
		services:           srvState,
		spans:              make(map[int64]*Span, req.DurationSec*req.RequestsPerSecond*4),
		rootArrivalMs:      make(map[int]float64, req.DurationSec*req.RequestsPerSecond),
		rootCompleted:      make(map[int]bool, req.DurationSec*req.RequestsPerSecond),
		maxEvents:          maxEv,
		recordedLogs:       make([]model.SimulationEventLog, 0, 512),
		breakerTransitions: make([]model.BreakerTransition, 0, 16),
	}
}

func (s *Simulator) schedule(timestampMs float64, evType model.EventType, reqID int, spanID int64, serviceID string, attempt int, reason model.FailureReason) {
	s.nextSeqID++
	ev := &SimEvent{
		SequenceID:  s.nextSeqID,
		TimestampMs: timestampMs,
		Type:        evType,
		RequestID:   reqID,
		SpanID:      spanID,
		ServiceID:   serviceID,
		Attempt:     attempt,
		Reason:      reason,
	}
	heap.Push(s.pq, ev)
}

func (s *Simulator) createSpan(reqID int, serviceID string, parentSpanID int64, arrivalMs float64, parentDeadlineMs float64, attemptTimeoutOverrideMs float64, attempt int) *Span {
	s.nextSpanID++
	cfg := s.services[serviceID].config
	timeoutMs := cfg.TimeoutMs
	if attemptTimeoutOverrideMs > 0 && attemptTimeoutOverrideMs < timeoutMs {
		timeoutMs = attemptTimeoutOverrideMs
	}
	ownDeadline := arrivalMs + timeoutMs
	if parentDeadlineMs > 0 && parentDeadlineMs < ownDeadline {
		ownDeadline = parentDeadlineMs
	}
	sp := &Span{
		SpanID:        s.nextSpanID,
		RequestID:     reqID,
		ServiceID:     serviceID,
		ParentSpanID:  parentSpanID,
		ArrivalTimeMs: arrivalMs,
		DeadlineMs:    ownDeadline,
		Attempt:       attempt,
	}
	s.spans[sp.SpanID] = sp
	return sp
}

func (s *Simulator) recordBreakerTransition(tr *model.BreakerTransition) {
	if tr == nil {
		return
	}
	s.nextSeqID++
	tr.SequenceID = s.nextSeqID
	s.breakerTransitions = append(s.breakerTransitions, *tr)
	s.collector.RecordCircuitStateAt(tr.CalleeID, tr.TimestampMs, tr.ToState)
	s.recordLog(tr.TimestampMs, 0, tr.CalleeID, 0, model.EventCircuitStateChange, 0, model.ReasonNone,
		fmt.Sprintf("Circuit Breaker [%s] %s -> %s (%s)", tr.Edge, tr.FromState, tr.ToState, tr.Reason))
}

func (s *Simulator) recordLog(timestampMs float64, reqID int, serviceID string, attempt int, evType model.EventType, latencyMs float64, reason model.FailureReason, msg string) {
	if len(s.recordedLogs) >= MaxRecordedLogEvents {
		// Prioritize keeping circuit transitions, failures, timeouts, and queue events if log buffer is getting full
		if evType != model.EventCircuitStateChange &&
			evType != model.EventRequestFailure &&
			evType != model.EventRequestTimeout &&
			evType != model.EventQueueEnqueued &&
			evType != model.EventRetryBackoffComplete {
			return
		}
		if len(s.recordedLogs) >= MaxRecordedLogEvents+400 {
			return
		}
	}

	var qLen, workers int
	if st, ok := s.services[serviceID]; ok {
		qLen = len(st.waitQueue)
		workers = st.activeWorkers
	}

	sec := int(timestampMs / 1000.0)
	if sec < 0 {
		sec = 0
	}

	s.recordedLogs = append(s.recordedLogs, model.SimulationEventLog{
		SequenceID:    s.nextSeqID,
		TimestampMs:   metrics.Round2(timestampMs),
		Second:        sec,
		RequestID:     reqID,
		Attempt:       attempt,
		ServiceID:     serviceID,
		EventType:     evType,
		LatencyMs:     metrics.Round2(latencyMs),
		QueueLength:   qLen,
		ActiveWorkers: workers,
		Reason:        reason,
		Message:       msg,
	})
}

// Run executes the simulation from t=0 until the event queue is exhausted or maxEvents is reached.
func (s *Simulator) Run() model.StrategyResult {
	totalRequests := s.req.DurationSec * s.req.RequestsPerSecond
	intervalMs := 1000.0 / float64(s.req.RequestsPerSecond)

	// 1. Generate deterministic client request arrivals across durationSec using Coordinate RNG
	reqID := 0
	for sec := 0; sec < s.req.DurationSec; sec++ {
		baseSecMs := float64(sec) * 1000.0
		for i := 0; i < s.req.RequestsPerSecond; i++ {
			reqID++
			jitterDraw := s.coordRNG.Float64(reqID, topology.NodeClient, 1, PurposeArrivalJitter)
			jitter := jitterDraw * intervalMs * 0.5
			arrivalMs := baseSecMs + float64(i)*intervalMs + jitter
			maxSecMs := float64(sec+1)*1000.0 - 0.01
			if arrivalMs > maxSecMs {
				arrivalMs = maxSecMs
			}
			s.rootArrivalMs[reqID] = arrivalMs

			rootSpan := s.createSpan(reqID, topology.NodeAPIGateway, 0, arrivalMs, 0, 0, 1)
			s.schedule(arrivalMs, model.EventRequestArrival, reqID, rootSpan.SpanID, topology.NodeAPIGateway, 1, model.ReasonNone)
		}
	}

	// 2. Process discrete events in strict virtual time order
	for s.pq.Len() > 0 {
		if s.eventsCount >= s.maxEvents {
			s.truncated = true
			break
		}
		ev := heap.Pop(s.pq).(*SimEvent)
		s.eventsCount++

		sp, exists := s.spans[ev.SpanID]
		if !exists || sp.Completed {
			continue
		}

		switch ev.Type {
		case model.EventRequestArrival:
			s.handleArrival(ev, sp)
		case model.EventProcessingStart:
			s.handleProcessingStart(ev, sp)
		case model.EventDownstreamCall, model.EventRetryBackoffComplete:
			s.handleDownstreamCall(ev, sp)
		case model.EventProcessingComplete:
			s.handleProcessingComplete(ev, sp)
		case model.EventRequestFailure:
			s.handleRequestFailure(ev, sp)
		case model.EventRequestTimeout:
			s.handleRequestTimeout(ev, sp)
		}
	}

	simMetrics, timeSeries := s.collector.BuildFinal(totalRequests, s.req.Services, s.req.Fault, s.truncated)

	status := "completed"
	if s.truncated {
		status = "truncated"
	}
	unfinished := totalRequests - len(s.rootCompleted)
	if unfinished < 0 {
		unfinished = 0
	}

	return model.StrategyResult{
		StrategyID:         s.policy.ID(),
		StrategyName:       s.policy.Name(),
		Metrics:            simMetrics,
		TimeSeries:         timeSeries,
		BreakerTransitions: s.breakerTransitions,
		Events:             s.recordedLogs,
		ExecutionMeta: model.ExecutionMeta{
			EngineVersion:        model.EngineVersion,
			Status:               status,
			TotalEventsProcessed: s.eventsCount,
			RecordedEventCount:   len(s.recordedLogs),
			Truncated:            s.truncated,
			UnfinishedRequests:   unfinished,
			MaxEventsLimit:       s.maxEvents,
			Seed:                 s.req.Seed,
		},
	}
}

func (s *Simulator) handleArrival(ev *SimEvent, sp *Span) {
	now := ev.TimestampMs
	st := s.services[sp.ServiceID]

	// Schedule deadline timeout check for this span attempt
	s.schedule(sp.DeadlineMs, model.EventRequestTimeout, sp.RequestID, sp.SpanID, sp.ServiceID, sp.Attempt, model.ReasonTimeout)

	// Check worker availability
	if st.activeWorkers < st.config.MaxConcurrent {
		st.activeWorkers++
		sp.HoldingWorker = true
		s.collector.RecordConcurrency(sp.ServiceID, now, st.activeWorkers, len(st.waitQueue))
		s.schedule(now, model.EventProcessingStart, sp.RequestID, sp.SpanID, sp.ServiceID, sp.Attempt, model.ReasonNone)
		return
	}

	// Workers full -> check waitQueue capacity
	if len(st.waitQueue) < st.config.QueueLimit {
		st.waitQueue = append(st.waitQueue, sp.SpanID)
		sp.InQueue = true
		s.collector.RecordConcurrency(sp.ServiceID, now, st.activeWorkers, len(st.waitQueue))
		s.recordLog(now, sp.RequestID, sp.ServiceID, sp.Attempt, model.EventQueueEnqueued, 0, model.ReasonNone,
			fmt.Sprintf("Req #%d (att %d) queued at %s (queue=%d/%d)", sp.RequestID, sp.Attempt, topology.ServiceName(sp.ServiceID), len(st.waitQueue), st.config.QueueLimit))
		return
	}

	// Queue limit exceeded -> immediate overflow failure
	s.collector.RecordConcurrency(sp.ServiceID, now, st.activeWorkers, len(st.waitQueue))
	s.schedule(now, model.EventRequestFailure, sp.RequestID, sp.SpanID, sp.ServiceID, sp.Attempt, model.ReasonQueueOverflow)
}

func (s *Simulator) handleProcessingStart(ev *SimEvent, sp *Span) {
	now := ev.TimestampMs
	st := s.services[sp.ServiceID]
	sp.StartTimeMs = now

	// If deadline already expired while waiting in queue, fail immediately as timeout
	if now >= sp.DeadlineMs {
		s.schedule(now, model.EventRequestTimeout, sp.RequestID, sp.SpanID, sp.ServiceID, sp.Attempt, model.ReasonTimeout)
		return
	}

	// Evaluate fault injection and base failure rate deterministically using Coordinate RNG
	faultDraw := s.coordRNG.Float64(sp.RequestID, sp.ServiceID, sp.Attempt, PurposeFaultEval)
	baseDraw := s.coordRNG.Float64(sp.RequestID, sp.ServiceID, sp.Attempt, PurposeBaseFaultEval)
	eval := s.injector.Evaluate(sp.ServiceID, st.config.FailureRate, now, faultDraw, baseDraw)
	if eval.ShouldFail && eval.FailureReason == model.ReasonFaultOutage {
		// Immediate failure on service outage (minimal 0.5ms overhead)
		s.schedule(now+0.5, model.EventRequestFailure, sp.RequestID, sp.SpanID, sp.ServiceID, sp.Attempt, eval.FailureReason)
		return
	}

	// Calculate local service processing time with deterministic +-10% jitter
	latJitterDraw := s.coordRNG.Float64(sp.RequestID, sp.ServiceID, sp.Attempt, PurposeLatencyJitter)
	jitterFactor := 0.9 + latJitterDraw*0.2
	localMs := st.config.BaseLatencyMs*jitterFactor + eval.ExtraDelayMs
	if localMs < 0.5 {
		localMs = 0.5
	}

	sp.EvalShouldFail = eval.ShouldFail
	sp.EvalFailureReason = eval.FailureReason

	downstreamID := topology.DownstreamOf(sp.ServiceID)
	if downstreamID == "" || eval.ShouldFail {
		// Terminal node (Feature DB) or local error before downstream call
		if eval.ShouldFail {
			s.schedule(now+localMs, model.EventRequestFailure, sp.RequestID, sp.SpanID, sp.ServiceID, sp.Attempt, eval.FailureReason)
		} else {
			s.schedule(now+localMs, model.EventProcessingComplete, sp.RequestID, sp.SpanID, sp.ServiceID, sp.Attempt, model.ReasonNone)
		}
		return
	}

	// Non-terminal node: split local work into pre-downstream (60%) and post-downstream (40%)
	preWorkMs := localMs * 0.6
	sp.PostWorkMs = localMs * 0.4
	s.schedule(now+preWorkMs, model.EventDownstreamCall, sp.RequestID, sp.SpanID, sp.ServiceID, 1, model.ReasonNone)
}

func (s *Simulator) handleDownstreamCall(ev *SimEvent, sp *Span) {
	now := ev.TimestampMs
	downstreamID := topology.DownstreamOf(sp.ServiceID)
	if downstreamID == "" {
		s.schedule(now, model.EventProcessingComplete, sp.RequestID, sp.SpanID, sp.ServiceID, sp.Attempt, model.ReasonNone)
		return
	}

	attempt := ev.Attempt
	if attempt <= 0 {
		attempt = 1
	}

	if ev.Type == model.EventRetryBackoffComplete {
		s.recordLog(now, sp.RequestID, sp.ServiceID, attempt, model.EventRetryBackoffComplete, 0, model.ReasonNone,
			fmt.Sprintf("Req #%d retry attempt #%d firing from %s -> %s", sp.RequestID, attempt, topology.ServiceName(sp.ServiceID), topology.ServiceName(downstreamID)))
	}

	// Consult resilience policy (e.g., Circuit Breaker) for caller -> downstream
	allowed, blockReason, tr := s.policy.AllowCall(sp.ServiceID, downstreamID, now)
	s.recordBreakerTransition(tr)
	if !allowed {
		// Rejected by Circuit Breaker! Record CB rejection metric (does not invoke downstream)
		s.collector.RecordCircuitRejection(downstreamID, now)
		s.recordLog(now, sp.RequestID, sp.ServiceID, attempt, model.EventRequestFailure, 0, blockReason,
			fmt.Sprintf("Req #%d blocked by Circuit Breaker at %s -> %s (%s)", sp.RequestID, topology.ServiceName(sp.ServiceID), topology.ServiceName(downstreamID), blockReason))
		s.schedule(now, model.EventRequestFailure, sp.RequestID, sp.SpanID, sp.ServiceID, sp.Attempt, blockReason)
		return
	}

	attemptTimeoutOverride := s.policy.AttemptTimeoutOverride(sp.ServiceID, downstreamID)
	childSpan := s.createSpan(sp.RequestID, downstreamID, sp.SpanID, now, sp.DeadlineMs, attemptTimeoutOverride, attempt)
	sp.ActiveChildSpanID = childSpan.SpanID
	s.schedule(now, model.EventRequestArrival, sp.RequestID, childSpan.SpanID, downstreamID, attempt, model.ReasonNone)
}

func (s *Simulator) handleProcessingComplete(ev *SimEvent, sp *Span) {
	now := ev.TimestampMs
	sp.Completed = true
	latencyMs := now - sp.ArrivalTimeMs

	s.releaseWorkerAndDrainQueue(sp, now)

	// Record service-level span metric
	s.collector.RecordServiceSpan(metrics.SpanOutcome{
		RequestID:     sp.RequestID,
		ServiceID:     sp.ServiceID,
		Attempt:       sp.Attempt,
		ArrivalSec:    int(sp.ArrivalTimeMs / 1000.0),
		CompletionSec: int(now / 1000.0),
		LatencyMs:     latencyMs,
		Success:       true,
		Reason:        model.ReasonNone,
	})

	if sp.ParentSpanID == 0 {
		// Completed at top-level API Gateway -> Client logical request succeeded
		if !s.rootCompleted[sp.RequestID] {
			s.rootCompleted[sp.RequestID] = true
			rootLat := now - s.rootArrivalMs[sp.RequestID]
			s.collector.RecordRootOutcome(metrics.SpanOutcome{
				RequestID:     sp.RequestID,
				ServiceID:     topology.NodeClient,
				Attempt:       1,
				ArrivalSec:    int(s.rootArrivalMs[sp.RequestID] / 1000.0),
				CompletionSec: int(now / 1000.0),
				LatencyMs:     rootLat,
				Success:       true,
				Reason:        model.ReasonNone,
			})
			if sp.RequestID%15 == 1 {
				s.recordLog(now, sp.RequestID, sp.ServiceID, sp.Attempt, model.EventProcessingComplete, rootLat, model.ReasonNone,
					fmt.Sprintf("Req #%d succeeded end-to-end in %.1fms", sp.RequestID, rootLat))
			}
		}
		return
	}

	parent, ok := s.spans[sp.ParentSpanID]
	if !ok {
		return
	}

	// Notify policy on caller -> callee boundary (e.g., Half-Open probe succeeded -> Closed)
	_, _, tr := s.policy.OnResult(parent.ServiceID, sp.ServiceID, now, true, model.ReasonNone, sp.Attempt, 0)
	s.recordBreakerTransition(tr)

	// Only notify parent span if parent is still waiting for THIS child attempt
	if !parent.Completed && parent.ActiveChildSpanID == sp.SpanID {
		s.schedule(now+parent.PostWorkMs, model.EventProcessingComplete, parent.RequestID, parent.SpanID, parent.ServiceID, parent.Attempt, model.ReasonNone)
	}
}

func (s *Simulator) handleRequestFailure(ev *SimEvent, sp *Span) {
	now := ev.TimestampMs
	sp.Completed = true
	latencyMs := now - sp.ArrivalTimeMs

	s.releaseWorkerAndDrainQueue(sp, now)

	s.collector.RecordServiceSpan(metrics.SpanOutcome{
		RequestID:     sp.RequestID,
		ServiceID:     sp.ServiceID,
		Attempt:       sp.Attempt,
		ArrivalSec:    int(sp.ArrivalTimeMs / 1000.0),
		CompletionSec: int(now / 1000.0),
		LatencyMs:     latencyMs,
		Success:       false,
		Reason:        ev.Reason,
	})

	s.recordLog(now, sp.RequestID, sp.ServiceID, sp.Attempt, model.EventRequestFailure, latencyMs, ev.Reason,
		fmt.Sprintf("Req #%d (att %d) failed at %s (%s, %.1fms)", sp.RequestID, sp.Attempt, topology.ServiceName(sp.ServiceID), ev.Reason, latencyMs))

	s.handleChildOrRootFailure(sp, now, ev.Reason)
}

func (s *Simulator) handleRequestTimeout(ev *SimEvent, sp *Span) {
	now := ev.TimestampMs
	sp.Completed = true
	latencyMs := now - sp.ArrivalTimeMs

	s.releaseWorkerAndDrainQueue(sp, now)

	s.collector.RecordServiceSpan(metrics.SpanOutcome{
		RequestID:     sp.RequestID,
		ServiceID:     sp.ServiceID,
		Attempt:       sp.Attempt,
		ArrivalSec:    int(sp.ArrivalTimeMs / 1000.0),
		CompletionSec: int(now / 1000.0),
		LatencyMs:     latencyMs,
		Success:       false,
		Reason:        model.ReasonTimeout,
	})

	s.recordLog(now, sp.RequestID, sp.ServiceID, sp.Attempt, model.EventRequestTimeout, latencyMs, model.ReasonTimeout,
		fmt.Sprintf("Req #%d (att %d) timed out at %s after %.1fms", sp.RequestID, sp.Attempt, topology.ServiceName(sp.ServiceID), latencyMs))

	s.handleChildOrRootFailure(sp, now, model.ReasonTimeout)
}

func (s *Simulator) handleChildOrRootFailure(sp *Span, now float64, reason model.FailureReason) {
	if sp.ParentSpanID == 0 {
		if !s.rootCompleted[sp.RequestID] {
			s.rootCompleted[sp.RequestID] = true
			rootLat := now - s.rootArrivalMs[sp.RequestID]
			s.collector.RecordRootOutcome(metrics.SpanOutcome{
				RequestID:     sp.RequestID,
				ServiceID:     topology.NodeClient,
				Attempt:       1,
				ArrivalSec:    int(s.rootArrivalMs[sp.RequestID] / 1000.0),
				CompletionSec: int(now / 1000.0),
				LatencyMs:     rootLat,
				Success:       false,
				Reason:        reason,
			})
		}
		return
	}

	parent, ok := s.spans[sp.ParentSpanID]
	if !ok {
		return
	}

	// Consult policy on caller -> callee boundary (updates Circuit Breaker failure count & checks Retry)
	jitterDraw := s.coordRNG.Float64(sp.RequestID, sp.ServiceID, sp.Attempt, PurposeRetryJitter)
	shouldRetry, backoffMs, tr := s.policy.OnResult(parent.ServiceID, sp.ServiceID, now, false, reason, sp.Attempt, jitterDraw)
	s.recordBreakerTransition(tr)

	// If parent has already timed out or moved on to another attempt, do not schedule retries or duplicate parent failure
	if parent.Completed || parent.ActiveChildSpanID != sp.SpanID {
		return
	}

	if shouldRetry && now+backoffMs < parent.DeadlineMs {
		// Schedule next attempt after backoffMs in virtual time (parent continues holding its worker)
		nextAttempt := sp.Attempt + 1
		s.schedule(now+backoffMs, model.EventRetryBackoffComplete, parent.RequestID, parent.SpanID, parent.ServiceID, nextAttempt, model.ReasonNone)
		return
	}

	// No retry (or retry would exceed parent deadline): propagate failure to parent span
	propReason := model.ReasonDownstreamError
	switch reason {
	case model.ReasonTimeout:
		propReason = model.ReasonTimeout
	case model.ReasonQueueOverflow:
		propReason = model.ReasonQueueOverflow
	case model.ReasonCircuitOpen:
		propReason = model.ReasonCircuitOpen
	}
	s.schedule(now+0.5, model.EventRequestFailure, parent.RequestID, parent.SpanID, parent.ServiceID, parent.Attempt, propReason)
}

func (s *Simulator) releaseWorkerAndDrainQueue(sp *Span, now float64) {
	st := s.services[sp.ServiceID]

	// If the span was still sitting in the waitQueue (e.g., timed out before getting a worker), remove it
	if sp.InQueue {
		sp.InQueue = false
		for i, id := range st.waitQueue {
			if id == sp.SpanID {
				st.waitQueue = append(st.waitQueue[:i], st.waitQueue[i+1:]...)
				break
			}
		}
		s.collector.RecordConcurrency(sp.ServiceID, now, st.activeWorkers, len(st.waitQueue))
		return
	}

	if !sp.HoldingWorker {
		return
	}
	sp.HoldingWorker = false
	st.activeWorkers--
	if st.activeWorkers < 0 {
		st.activeWorkers = 0
	}

	// Dequeue the next non-completed waiting span if available
	for len(st.waitQueue) > 0 && st.activeWorkers < st.config.MaxConcurrent {
		nextSpanID := st.waitQueue[0]
		st.waitQueue = st.waitQueue[1:]
		nextSpan, ok := s.spans[nextSpanID]
		if !ok || nextSpan.Completed {
			continue
		}
		nextSpan.InQueue = false
		st.activeWorkers++
		nextSpan.HoldingWorker = true
		s.schedule(now, model.EventProcessingStart, nextSpan.RequestID, nextSpan.SpanID, nextSpan.ServiceID, nextSpan.Attempt, model.ReasonNone)
	}

	s.collector.RecordConcurrency(sp.ServiceID, now, st.activeWorkers, len(st.waitQueue))
}
