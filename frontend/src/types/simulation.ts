export type FaultType = 'none' | 'outage' | 'latency' | 'error_rate';

export type ServiceStatus = 'healthy' | 'degraded' | 'critical' | 'down';

export type CircuitState = 'CLOSED' | 'OPEN' | 'HALF_OPEN';

export type EventType =
  | 'REQUEST_ARRIVAL'
  | 'QUEUE_ENQUEUED'
  | 'PROCESSING_START'
  | 'DOWNSTREAM_CALL'
  | 'RETRY_BACKOFF_COMPLETE'
  | 'PROCESSING_COMPLETE'
  | 'REQUEST_FAILURE'
  | 'REQUEST_TIMEOUT'
  | 'CIRCUIT_STATE_CHANGE';

export interface ServiceConfig {
  baseLatencyMs: number;
  failureRate: number;
  maxConcurrent: number;
  queueLimit: number;
  timeoutMs: number;
}

export interface FaultConfig {
  targetService: string;
  type: FaultType;
  startSec: number;
  endSec: number;
  latencyMs?: number;
  failureRate?: number;
}

export interface RetryConfig {
  maxRetries: number;
  initialBackoffMs: number;
  maxBackoffMs: number;
  backoffMultiplier: number;
  jitter: boolean;
}

export interface CircuitBreakerConfig {
  consecutiveFailuresThreshold: number;
  openTimeoutMs: number;
  halfOpenMaxCalls: number;
  halfOpenSuccessThreshold: number;
}

export interface ResilienceConfig {
  targetCaller: string;
  targetCallee: string;
  retry: RetryConfig;
  circuitBreaker: CircuitBreakerConfig;
  failFastTimeoutMs: number;
}

export interface SimulationRequest {
  scenarioId?: string;
  durationSec: number;
  requestsPerSecond: number;
  seed: number;
  fault: FaultConfig;
  services: Record<string, ServiceConfig>;
  resilience?: ResilienceConfig;
  strategies?: string[];
  maxEvents?: number;
}

export interface TopologyNode {
  id: string;
  label: string;
  role: string;
  tier: number;
  defaultConfig: ServiceConfig;
}

export interface TopologyEdge {
  id: string;
  source: string;
  target: string;
}

export interface Topology {
  nodes: TopologyNode[];
  edges: TopologyEdge[];
}

export interface ScenarioPreset {
  id: string;
  name: string;
  shortTitle?: string;
  category?: string;
  description: string;
  highlight: string;
  config: SimulationRequest;
}

export interface ScenariosResponse {
  engineVersion: string;
  topology: Topology;
  scenarios: ScenarioPreset[];
}

export interface OverallMetrics {
  totalRequests: number;
  completedRequests: number;
  successCount: number;
  failureCount: number;
  timeoutCount: number;
  queueOverflowCount: number;
  successRate: number;
  errorRate: number;
  avgLatencyMs: number;
  p50LatencyMs: number;
  p95LatencyMs: number;
  p99LatencyMs: number;
  successAvgLatencyMs: number;
  successP95LatencyMs: number;
  failureAvgLatencyMs: number;
  failureP95LatencyMs: number;
  totalDownstreamCalls: number;
  targetEdgeCalls: number;
  totalRetries: number;
  retryAmplificationFactor: number;
  circuitBreakerRejections: number;
  maxWorkerUtilization: number;
  maxQueueLength: number;
}

export interface ServiceMetrics {
  serviceId: string;
  serviceName: string;
  callCount: number;
  primaryCallCount: number;
  retryCallCount: number;
  successCount: number;
  failureCount: number;
  timeoutCount: number;
  queueOverflowCount: number;
  circuitBreakerRejections: number;
  successRate: number;
  errorRate: number;
  avgLatencyMs: number;
  p50LatencyMs: number;
  p95LatencyMs: number;
  p99LatencyMs: number;
  successAvgLatencyMs: number;
  successP95LatencyMs: number;
  failureAvgLatencyMs: number;
  failureP95LatencyMs: number;
  peakConcurrent: number;
  maxConcurrent: number;
  workerUtilization: number;
  peakQueueLength: number;
  status: ServiceStatus;
}

export interface SimulationMetrics {
  overall: OverallMetrics;
  services: Record<string, ServiceMetrics>;
}

export interface ServiceSecondMetrics {
  serviceId: string;
  callCount: number;
  retryCount: number;
  successCount: number;
  failureCount: number;
  timeoutCount: number;
  queueOverflowCount: number;
  circuitBreakerRejections: number;
  successRate: number;
  errorRate: number;
  avgLatencyMs: number;
  p95LatencyMs: number;
  peakConcurrent: number;
  peakQueueLength: number;
  faultActive: boolean;
  circuitState?: CircuitState;
  status: ServiceStatus;
}

export interface TimeSeriesPoint {
  second: number;
  totalRequests: number;
  successCount: number;
  failureCount: number;
  timeoutCount: number;
  queueOverflowCount: number;
  retryCount: number;
  circuitBreakerRejections: number;
  successRate: number;
  errorRate: number;
  avgLatencyMs: number;
  p95LatencyMs: number;
  faultActive: boolean;
  services: Record<string, ServiceSecondMetrics>;
}

export interface BreakerTransition {
  sequenceId: number;
  timestampMs: number;
  second: number;
  callerId: string;
  calleeId: string;
  edge: string;
  fromState: CircuitState;
  toState: CircuitState;
  reason: string;
}

export interface SimulationEventLog {
  sequenceId: number;
  timestampMs: number;
  second: number;
  requestId: number;
  attempt?: number;
  serviceId: string;
  eventType: EventType;
  latencyMs?: number;
  queueLength: number;
  activeWorkers: number;
  reason?: string;
  message: string;
}

export interface ExecutionMeta {
  engineVersion: string;
  status: string;
  totalEventsProcessed: number;
  recordedEventCount: number;
  truncated: boolean;
  unfinishedRequests: number;
  maxEventsLimit: number;
  seed: number;
}

export interface StrategyResult {
  strategyId: string;
  strategyName: string;
  metrics: SimulationMetrics;
  timeSeries: TimeSeriesPoint[];
  breakerTransitions: BreakerTransition[];
  events: SimulationEventLog[];
  executionMeta: ExecutionMeta;
}

export interface CompareResponse {
  engineVersion: string;
  request: SimulationRequest;
  topology: Topology;
  results: StrategyResult[];
}
