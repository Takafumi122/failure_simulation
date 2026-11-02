package model

// EngineVersion identifies the deterministic simulation engine release.
const EngineVersion = "v3.0.0-des"

// FaultType defines the type of fault injected into a service.
type FaultType string

const (
	FaultNone      FaultType = "none"
	FaultOutage    FaultType = "outage"
	FaultLatency   FaultType = "latency"
	FaultErrorRate FaultType = "error_rate"
)

// EventType defines discrete simulation events.
type EventType string

const (
	EventRequestArrival       EventType = "REQUEST_ARRIVAL"
	EventQueueEnqueued        EventType = "QUEUE_ENQUEUED"
	EventProcessingStart      EventType = "PROCESSING_START"
	EventDownstreamCall       EventType = "DOWNSTREAM_CALL"
	EventRetryBackoffComplete EventType = "RETRY_BACKOFF_COMPLETE"
	EventProcessingComplete   EventType = "PROCESSING_COMPLETE"
	EventRequestFailure       EventType = "REQUEST_FAILURE"
	EventRequestTimeout       EventType = "REQUEST_TIMEOUT"
	EventCircuitStateChange   EventType = "CIRCUIT_STATE_CHANGE"
)

// FailureReason describes why a request or attempt failed.
type FailureReason string

const (
	ReasonNone            FailureReason = ""
	ReasonFaultOutage     FailureReason = "fault_outage"
	ReasonFaultError      FailureReason = "fault_error"
	ReasonBaseError       FailureReason = "base_error"
	ReasonQueueOverflow   FailureReason = "queue_overflow"
	ReasonTimeout         FailureReason = "timeout"
	ReasonDownstreamError FailureReason = "downstream_error"
	ReasonCircuitOpen     FailureReason = "circuit_open"
)

// CircuitState represents the state of a Circuit Breaker.
type CircuitState string

const (
	CircuitClosed   CircuitState = "CLOSED"
	CircuitOpen     CircuitState = "OPEN"
	CircuitHalfOpen CircuitState = "HALF_OPEN"
)

// ServiceStatus represents the health status of a service at a given time or overall.
type ServiceStatus string

const (
	StatusHealthy  ServiceStatus = "healthy"
	StatusDegraded ServiceStatus = "degraded"
	StatusCritical ServiceStatus = "critical"
	StatusDown     ServiceStatus = "down"
)

// ServiceConfig holds per-service simulation parameters.
type ServiceConfig struct {
	BaseLatencyMs float64 `json:"baseLatencyMs"`
	FailureRate   float64 `json:"failureRate"`
	MaxConcurrent int     `json:"maxConcurrent"`
	QueueLimit    int     `json:"queueLimit"`
	TimeoutMs     float64 `json:"timeoutMs"`
}

// FaultConfig specifies when and how a fault is injected.
type FaultConfig struct {
	TargetService string    `json:"targetService"`
	Type          FaultType `json:"type"`
	StartSec      float64   `json:"startSec"`
	EndSec        float64   `json:"endSec"`
	LatencyMs     float64   `json:"latencyMs,omitempty"`
	FailureRate   float64   `json:"failureRate,omitempty"`
}

// RetryConfig holds configuration for the Retry strategy.
type RetryConfig struct {
	MaxRetries        int     `json:"maxRetries"`
	InitialBackoffMs  float64 `json:"initialBackoffMs"`
	MaxBackoffMs      float64 `json:"maxBackoffMs"`
	BackoffMultiplier float64 `json:"backoffMultiplier"`
	Jitter            bool    `json:"jitter"`
}

// CircuitBreakerConfig holds configuration for the Circuit Breaker strategy.
type CircuitBreakerConfig struct {
	ConsecutiveFailuresThreshold int     `json:"consecutiveFailuresThreshold"`
	OpenTimeoutMs                float64 `json:"openTimeoutMs"`
	HalfOpenMaxCalls             int     `json:"halfOpenMaxCalls"`
	HalfOpenSuccessThreshold     int     `json:"halfOpenSuccessThreshold"`
}

// ResilienceConfig holds parameters for Phase 2 resilience policies.
type ResilienceConfig struct {
	TargetCaller      string               `json:"targetCaller"`
	TargetCallee      string               `json:"targetCallee"`
	Retry             RetryConfig          `json:"retry"`
	CircuitBreaker    CircuitBreakerConfig `json:"circuitBreaker"`
	FailFastTimeoutMs float64              `json:"failFastTimeoutMs"`
}

// SimulationRequest represents the input payload for running a simulation comparison.
type SimulationRequest struct {
	ScenarioID        string                   `json:"scenarioId,omitempty"`
	DurationSec       int                      `json:"durationSec"`
	RequestsPerSecond int                      `json:"requestsPerSecond"`
	Seed              int64                    `json:"seed"`
	Fault             FaultConfig              `json:"fault"`
	Services          map[string]ServiceConfig `json:"services"`
	Resilience        *ResilienceConfig        `json:"resilience,omitempty"`
	Strategies        []string                 `json:"strategies,omitempty"`
	MaxEvents         int                      `json:"maxEvents,omitempty"`
}

// TopologyNode represents a node in the service graph.
type TopologyNode struct {
	ID            string        `json:"id"`
	Label         string        `json:"label"`
	Role          string        `json:"role"`
	Tier          int           `json:"tier"`
	DefaultConfig ServiceConfig `json:"defaultConfig"`
}

// TopologyEdge represents a directed dependency between two services.
type TopologyEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
}

// Topology holds the static dependency graph.
type Topology struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}

// ScenarioPreset represents a pre-configured scenario for quick demonstration.
type ScenarioPreset struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	ShortTitle  string            `json:"shortTitle"`
	Category    string            `json:"category"`
	Description string            `json:"description"`
	Highlight   string            `json:"highlight"`
	Config      SimulationRequest `json:"config"`
}

// ScenariosResponse is returned by GET /api/scenarios.
type ScenariosResponse struct {
	EngineVersion string           `json:"engineVersion"`
	Topology      Topology         `json:"topology"`
	Scenarios     []ScenarioPreset `json:"scenarios"`
}

// OverallMetrics represents end-to-end simulation metrics observed by the client
// plus Phase 2/3 system-wide resilience and success/failure split latency metrics.
type OverallMetrics struct {
	TotalRequests            int     `json:"totalRequests"`
	CompletedRequests        int     `json:"completedRequests"`
	SuccessCount             int     `json:"successCount"`
	FailureCount             int     `json:"failureCount"`
	TimeoutCount             int     `json:"timeoutCount"`
	QueueOverflowCount       int     `json:"queueOverflowCount"`
	SuccessRate              float64 `json:"successRate"`
	ErrorRate                float64 `json:"errorRate"`
	AvgLatencyMs             float64 `json:"avgLatencyMs"`
	P50LatencyMs             float64 `json:"p50LatencyMs"`
	P95LatencyMs             float64 `json:"p95LatencyMs"`
	P99LatencyMs             float64 `json:"p99LatencyMs"`
	SuccessAvgLatencyMs      float64 `json:"successAvgLatencyMs"`
	SuccessP95LatencyMs      float64 `json:"successP95LatencyMs"`
	FailureAvgLatencyMs      float64 `json:"failureAvgLatencyMs"`
	FailureP95LatencyMs      float64 `json:"failureP95LatencyMs"`
	TotalDownstreamCalls     int     `json:"totalDownstreamCalls"`
	TargetEdgeCalls          int     `json:"targetEdgeCalls"`
	TotalRetries             int     `json:"totalRetries"`
	RetryAmplificationFactor float64 `json:"retryAmplificationFactor"`
	CircuitBreakerRejections int     `json:"circuitBreakerRejections"`
	MaxWorkerUtilization     float64 `json:"maxWorkerUtilization"`
	MaxQueueLength           int     `json:"maxQueueLength"`
}

// ServiceMetrics represents aggregated metrics for a single service across the simulation.
type ServiceMetrics struct {
	ServiceID                string        `json:"serviceId"`
	ServiceName              string        `json:"serviceName"`
	CallCount                int           `json:"callCount"`
	PrimaryCallCount         int           `json:"primaryCallCount"`
	RetryCallCount           int           `json:"retryCallCount"`
	SuccessCount             int           `json:"successCount"`
	FailureCount             int           `json:"failureCount"`
	TimeoutCount             int           `json:"timeoutCount"`
	QueueOverflowCount       int           `json:"queueOverflowCount"`
	CircuitBreakerRejections int           `json:"circuitBreakerRejections"`
	SuccessRate              float64       `json:"successRate"`
	ErrorRate                float64       `json:"errorRate"`
	AvgLatencyMs             float64       `json:"avgLatencyMs"`
	P50LatencyMs             float64       `json:"p50LatencyMs"`
	P95LatencyMs             float64       `json:"p95LatencyMs"`
	P99LatencyMs             float64       `json:"p99LatencyMs"`
	SuccessAvgLatencyMs      float64       `json:"successAvgLatencyMs"`
	SuccessP95LatencyMs      float64       `json:"successP95LatencyMs"`
	FailureAvgLatencyMs      float64       `json:"failureAvgLatencyMs"`
	FailureP95LatencyMs      float64       `json:"failureP95LatencyMs"`
	PeakConcurrent           int           `json:"peakConcurrent"`
	MaxConcurrent            int           `json:"maxConcurrent"`
	WorkerUtilization        float64       `json:"workerUtilization"`
	PeakQueueLength          int           `json:"peakQueueLength"`
	Status                   ServiceStatus `json:"status"`
}

// SimulationMetrics groups overall and per-service metrics.
type SimulationMetrics struct {
	Overall  OverallMetrics            `json:"overall"`
	Services map[string]ServiceMetrics `json:"services"`
}

// ServiceSecondMetrics holds per-second state for a single service.
type ServiceSecondMetrics struct {
	ServiceID                string        `json:"serviceId"`
	CallCount                int           `json:"callCount"`
	RetryCount               int           `json:"retryCount"`
	SuccessCount             int           `json:"successCount"`
	FailureCount             int           `json:"failureCount"`
	TimeoutCount             int           `json:"timeoutCount"`
	QueueOverflowCount       int           `json:"queueOverflowCount"`
	CircuitBreakerRejections int           `json:"circuitBreakerRejections"`
	SuccessRate              float64       `json:"successRate"`
	ErrorRate                float64       `json:"errorRate"`
	AvgLatencyMs             float64       `json:"avgLatencyMs"`
	P95LatencyMs             float64       `json:"p95LatencyMs"`
	PeakConcurrent           int           `json:"peakConcurrent"`
	PeakQueueLength          int           `json:"peakQueueLength"`
	FaultActive              bool          `json:"faultActive"`
	CircuitState             CircuitState  `json:"circuitState,omitempty"`
	Status                   ServiceStatus `json:"status"`
}

// TimeSeriesPoint holds metrics for a 1-second bucket [second, second+1).
type TimeSeriesPoint struct {
	Second                   int                             `json:"second"`
	TotalRequests            int                             `json:"totalRequests"`
	SuccessCount             int                             `json:"successCount"`
	FailureCount             int                             `json:"failureCount"`
	TimeoutCount             int                             `json:"timeoutCount"`
	QueueOverflowCount       int                             `json:"queueOverflowCount"`
	RetryCount               int                             `json:"retryCount"`
	CircuitBreakerRejections int                             `json:"circuitBreakerRejections"`
	SuccessRate              float64                         `json:"successRate"`
	ErrorRate                float64                         `json:"errorRate"`
	AvgLatencyMs             float64                         `json:"avgLatencyMs"`
	P95LatencyMs             float64                         `json:"p95LatencyMs"`
	FaultActive              bool                            `json:"faultActive"`
	Services                 map[string]ServiceSecondMetrics `json:"services"`
}

// BreakerTransition records a state transition of a Circuit Breaker.
type BreakerTransition struct {
	SequenceID  int64        `json:"sequenceId"`
	TimestampMs float64      `json:"timestampMs"`
	Second      int          `json:"second"`
	CallerID    string       `json:"callerId"`
	CalleeID    string       `json:"calleeId"`
	Edge        string       `json:"edge"`
	FromState   CircuitState `json:"fromState"`
	ToState     CircuitState `json:"toState"`
	Reason      string       `json:"reason"`
}

// SimulationEventLog represents a recorded event for timeline playback and inspection.
type SimulationEventLog struct {
	SequenceID    int64         `json:"sequenceId"`
	TimestampMs   float64       `json:"timestampMs"`
	Second        int           `json:"second"`
	RequestID     int           `json:"requestId"`
	Attempt       int           `json:"attempt,omitempty"`
	ServiceID     string        `json:"serviceId"`
	EventType     EventType     `json:"eventType"`
	LatencyMs     float64       `json:"latencyMs,omitempty"`
	QueueLength   int           `json:"queueLength"`
	ActiveWorkers int           `json:"activeWorkers"`
	Reason        FailureReason `json:"reason,omitempty"`
	Message       string        `json:"message"`
}

// ExecutionMeta provides diagnostic metadata about the simulation run.
type ExecutionMeta struct {
	EngineVersion        string `json:"engineVersion"`
	Status               string `json:"status"` // "completed" or "truncated"
	TotalEventsProcessed int    `json:"totalEventsProcessed"`
	RecordedEventCount   int    `json:"recordedEventCount"`
	Truncated            bool   `json:"truncated"`
	UnfinishedRequests   int    `json:"unfinishedRequests"`
	MaxEventsLimit       int    `json:"maxEventsLimit"`
	Seed                 int64  `json:"seed"`
}

// StrategyResult holds the complete simulation output for one resilience strategy.
type StrategyResult struct {
	StrategyID         string               `json:"strategyId"`
	StrategyName       string               `json:"strategyName"`
	Metrics            SimulationMetrics    `json:"metrics"`
	TimeSeries         []TimeSeriesPoint    `json:"timeSeries"`
	BreakerTransitions []BreakerTransition  `json:"breakerTransitions"`
	Events             []SimulationEventLog `json:"events"`
	ExecutionMeta      ExecutionMeta        `json:"executionMeta"`
}

// CompareResponse is returned by POST /api/simulations/compare.
type CompareResponse struct {
	EngineVersion string            `json:"engineVersion"`
	Request       SimulationRequest `json:"request"`
	Topology      Topology          `json:"topology"`
	Results       []StrategyResult  `json:"results"`
}
