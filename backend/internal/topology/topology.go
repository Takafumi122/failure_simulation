package topology

import "failurelab/backend/internal/model"

const (
	NodeClient         = "client"
	NodeAPIGateway     = "api-gateway"
	NodeProductAPI     = "product-api"
	NodeRecommendation = "recommendation"
	NodeFeatureDB      = "feature-db"
)

// OrderedServiceIDs returns the ordered service chain after client.
var OrderedServiceIDs = []string{
	NodeAPIGateway,
	NodeProductAPI,
	NodeRecommendation,
	NodeFeatureDB,
}

// AllNodeIDs returns all nodes including client in topological order.
var AllNodeIDs = []string{
	NodeClient,
	NodeAPIGateway,
	NodeProductAPI,
	NodeRecommendation,
	NodeFeatureDB,
}

// ServiceName returns the human-readable name for a service ID.
func ServiceName(id string) string {
	switch id {
	case NodeClient:
		return "Client"
	case NodeAPIGateway:
		return "API Gateway"
	case NodeProductAPI:
		return "Product API"
	case NodeRecommendation:
		return "Recommendation"
	case NodeFeatureDB:
		return "Feature DB"
	default:
		return id
	}
}

// DownstreamOf returns the downstream service ID in the fixed chain, or "" if terminal.
func DownstreamOf(serviceID string) string {
	switch serviceID {
	case NodeClient:
		return NodeAPIGateway
	case NodeAPIGateway:
		return NodeProductAPI
	case NodeProductAPI:
		return NodeRecommendation
	case NodeRecommendation:
		return NodeFeatureDB
	default:
		return ""
	}
}

// IsValidFaultTarget checks if the given service ID is a valid backend service for fault injection.
func IsValidFaultTarget(serviceID string) bool {
	for _, id := range OrderedServiceIDs {
		if id == serviceID {
			return true
		}
	}
	return false
}

// DefaultServices returns the default service configurations for the fixed topology.
func DefaultServices() map[string]model.ServiceConfig {
	return map[string]model.ServiceConfig{
		NodeAPIGateway: {
			BaseLatencyMs: 10,
			FailureRate:   0.0,
			MaxConcurrent: 50,
			QueueLimit:    100,
			TimeoutMs:     1200,
		},
		NodeProductAPI: {
			BaseLatencyMs: 20,
			FailureRate:   0.0,
			MaxConcurrent: 35,
			QueueLimit:    70,
			TimeoutMs:     900,
		},
		NodeRecommendation: {
			BaseLatencyMs: 35,
			FailureRate:   0.0,
			MaxConcurrent: 25,
			QueueLimit:    50,
			TimeoutMs:     650,
		},
		NodeFeatureDB: {
			BaseLatencyMs: 15,
			FailureRate:   0.0,
			MaxConcurrent: 20,
			QueueLimit:    40,
			TimeoutMs:     450,
		},
	}
}

func defaultResiliencePtr() *model.ResilienceConfig {
	return &model.ResilienceConfig{
		TargetCaller: NodeRecommendation,
		TargetCallee: NodeFeatureDB,
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

var allFiveStrategies = []string{
	"baseline",
	"retry",
	"circuit_breaker",
	"retry_circuit_breaker",
	"fail_fast",
}

// FixedTopology returns the fixed 5-node dependency graph.
func FixedTopology() model.Topology {
	defaults := DefaultServices()
	return model.Topology{
		Nodes: []model.TopologyNode{
			{
				ID:    NodeClient,
				Label: "Client",
				Role:  "Ingress Traffic Generator",
				Tier:  0,
				DefaultConfig: model.ServiceConfig{
					BaseLatencyMs: 0,
					FailureRate:   0,
					MaxConcurrent: 0,
					QueueLimit:    0,
					TimeoutMs:     1500,
				},
			},
			{
				ID:            NodeAPIGateway,
				Label:         "API Gateway",
				Role:          "Edge Routing & Auth",
				Tier:          1,
				DefaultConfig: defaults[NodeAPIGateway],
			},
			{
				ID:            NodeProductAPI,
				Label:         "Product API",
				Role:          "Core Catalog Aggregator",
				Tier:          2,
				DefaultConfig: defaults[NodeProductAPI],
			},
			{
				ID:            NodeRecommendation,
				Label:         "Recommendation",
				Role:          "Personalization Engine",
				Tier:          3,
				DefaultConfig: defaults[NodeRecommendation],
			},
			{
				ID:            NodeFeatureDB,
				Label:         "Feature DB",
				Role:          "Low-Latency Feature Store",
				Tier:          4,
				DefaultConfig: defaults[NodeFeatureDB],
			},
		},
		Edges: []model.TopologyEdge{
			{ID: "e-client-gw", Source: NodeClient, Target: NodeAPIGateway},
			{ID: "e-gw-product", Source: NodeAPIGateway, Target: NodeProductAPI},
			{ID: "e-product-rec", Source: NodeProductAPI, Target: NodeRecommendation},
			{ID: "e-rec-db", Source: NodeRecommendation, Target: NodeFeatureDB},
		},
	}
}

// PresetScenarios returns the 5 Scenario Gallery presets demonstrating failure propagation and strategy comparison.
func PresetScenarios() []model.ScenarioPreset {
	return []model.ScenarioPreset{
		{
			ID:          "transient-failure",
			Name:        "Transient Failure — Probabilistic DB Errors (35%)",
			ShortTitle:  "Transient Failure",
			Category:    "Retry Wins",
			Description: "Between 6s and 18s, Feature DB experiences a 35% probabilistic error rate without latency spikes. Baseline drops to ~82.5% logical success, whereas Retry with Exponential Backoff + Jitter recovers logical success rate to ~98.9%.",
			Highlight:   "Retry Recovers Transient Errors (+16% Success)",
			Config: model.SimulationRequest{
				ScenarioID:        "transient-failure",
				DurationSec:       25,
				RequestsPerSecond: 35,
				Seed:              42,
				Fault: model.FaultConfig{
					TargetService: NodeFeatureDB,
					Type:          model.FaultErrorRate,
					StartSec:      6,
					EndSec:        18,
					LatencyMs:     0,
					FailureRate:   0.35,
				},
				Services:   DefaultServices(),
				Resilience: defaultResiliencePtr(),
				Strategies: allFiveStrategies,
			},
		},
		{
			ID:          "prolonged-outage",
			Name:        "Prolonged Outage — Complete Downstream Blackout (100%)",
			ShortTitle:  "Prolonged Outage",
			Category:    "Retry Futility vs. CB",
			Description: "From 4s to 20s, Feature DB suffers a 100% outage. Naive Retry multiplies Feature DB call attempts nearly 3x without improving logical success rate at all. Circuit Breaker trips OPEN after 5 consecutive failures and blocks >500 futile downstream calls.",
			Highlight:   "Retry Wastes 3x Calls; CB Sheds Load",
			Config: model.SimulationRequest{
				ScenarioID:        "prolonged-outage",
				DurationSec:       24,
				RequestsPerSecond: 40,
				Seed:              42,
				Fault: model.FaultConfig{
					TargetService: NodeFeatureDB,
					Type:          model.FaultOutage,
					StartSec:      4,
					EndSec:        20,
					LatencyMs:     0,
					FailureRate:   1.0,
				},
				Services:   DefaultServices(),
				Resilience: defaultResiliencePtr(),
				Strategies: allFiveStrategies,
			},
		},
		{
			ID:          "retry-storm",
			Name:        "Retry Storm — High Latency & Queue Saturation (+220ms)",
			ShortTitle:  "Retry Storm",
			Category:    "Queue Overload & Fail-Fast",
			Description: "Under 50 RPS traffic, Feature DB slows down by +220ms (exceeding its 160ms timeout). Retry triggers a Retry Storm that floods the downstream queue, inflates timeouts 3.5x, and exhausts upstream workers—while Fail-Fast (90ms) and Circuit Breaker keep p95 latency low.",
			Highlight:   "Retry Storm Amplifies Queue & Timeouts",
			Config: model.SimulationRequest{
				ScenarioID:        "retry-storm",
				DurationSec:       24,
				RequestsPerSecond: 50,
				Seed:              101,
				Fault: model.FaultConfig{
					TargetService: NodeFeatureDB,
					Type:          model.FaultLatency,
					StartSec:      5,
					EndSec:        17,
					LatencyMs:     220,
					FailureRate:   0.0,
				},
				Services: map[string]model.ServiceConfig{
					NodeAPIGateway:     {BaseLatencyMs: 10, FailureRate: 0, MaxConcurrent: 70, QueueLimit: 100, TimeoutMs: 1400},
					NodeProductAPI:     {BaseLatencyMs: 20, FailureRate: 0, MaxConcurrent: 50, QueueLimit: 80, TimeoutMs: 1100},
					NodeRecommendation: {BaseLatencyMs: 35, FailureRate: 0, MaxConcurrent: 35, QueueLimit: 60, TimeoutMs: 850},
					NodeFeatureDB:      {BaseLatencyMs: 15, FailureRate: 0, MaxConcurrent: 12, QueueLimit: 45, TimeoutMs: 160},
				},
				Resilience: &model.ResilienceConfig{
					TargetCaller: NodeRecommendation,
					TargetCallee: NodeFeatureDB,
					Retry: model.RetryConfig{
						MaxRetries:        3,
						InitialBackoffMs:  20,
						MaxBackoffMs:      150,
						BackoffMultiplier: 2.0,
						Jitter:            true,
					},
					CircuitBreaker: model.CircuitBreakerConfig{
						ConsecutiveFailuresThreshold: 5,
						OpenTimeoutMs:                2500,
						HalfOpenMaxCalls:             2,
						HalfOpenSuccessThreshold:     2,
					},
					FailFastTimeoutMs: 90,
				},
				Strategies: allFiveStrategies,
			},
		},
		{
			ID:          "circuit-breaker-recovery",
			Name:        "Circuit Breaker Recovery — Outage & Self-Healing (CLOSED→OPEN→HALF_OPEN→CLOSED)",
			ShortTitle:  "Circuit Breaker Recovery",
			Category:    "Self-Healing State Machine",
			Description: "Feature DB goes down from 5s to 13s and recovers at 13s. Observe the Circuit Breaker trip OPEN during the outage, periodically probe in HALF_OPEN every 2.5s, and automatically close the circuit once 2 consecutive probe calls succeed after t=13s.",
			Highlight:   "Full CLOSED → OPEN → HALF_OPEN → CLOSED Lifecycle",
			Config: model.SimulationRequest{
				ScenarioID:        "circuit-breaker-recovery",
				DurationSec:       24,
				RequestsPerSecond: 35,
				Seed:              42,
				Fault: model.FaultConfig{
					TargetService: NodeFeatureDB,
					Type:          model.FaultOutage,
					StartSec:      5,
					EndSec:        13,
					LatencyMs:     0,
					FailureRate:   1.0,
				},
				Services: DefaultServices(),
				Resilience: &model.ResilienceConfig{
					TargetCaller: NodeRecommendation,
					TargetCallee: NodeFeatureDB,
					Retry: model.RetryConfig{
						MaxRetries:        3,
						InitialBackoffMs:  25,
						MaxBackoffMs:      200,
						BackoffMultiplier: 2.0,
						Jitter:            true,
					},
					CircuitBreaker: model.CircuitBreakerConfig{
						ConsecutiveFailuresThreshold: 5,
						OpenTimeoutMs:                2500,
						HalfOpenMaxCalls:             2,
						HalfOpenSuccessThreshold:     2,
					},
					FailFastTimeoutMs: 120,
				},
				Strategies: allFiveStrategies,
			},
		},
		{
			ID:          "healthy-system",
			Name:        "Healthy System — Steady-State Baseline (Zero Faults)",
			ShortTitle:  "Healthy System",
			Category:    "Zero Overhead Baseline",
			Description: "Steady-state operation with no injected faults. Verifies that all 5 strategies achieve 100% logical success (~84ms p95 latency) with zero spurious retries and zero Circuit Breaker false positives.",
			Highlight:   "100% Success & Zero Overhead Across All 5 Strategies",
			Config: model.SimulationRequest{
				ScenarioID:        "healthy-system",
				DurationSec:       20,
				RequestsPerSecond: 40,
				Seed:              42,
				Fault: model.FaultConfig{
					TargetService: NodeFeatureDB,
					Type:          model.FaultNone,
					StartSec:      5,
					EndSec:        15,
					LatencyMs:     0,
					FailureRate:   0.0,
				},
				Services:   DefaultServices(),
				Resilience: defaultResiliencePtr(),
				Strategies: allFiveStrategies,
			},
		},
	}
}
