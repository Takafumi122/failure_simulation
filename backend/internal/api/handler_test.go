package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"failurelab/backend/internal/model"
	"failurelab/backend/internal/topology"
)

func TestGetScenariosEndpoint(t *testing.T) {
	h := NewHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/scenarios", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp model.ScenariosResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.EngineVersion == "" {
		t.Errorf("expected non-empty engineVersion in scenarios response")
	}
	if len(resp.Topology.Nodes) != 5 {
		t.Errorf("expected 5 topology nodes, got %d", len(resp.Topology.Nodes))
	}
	if len(resp.Scenarios) != 5 {
		t.Errorf("expected 5 preset gallery scenarios, got %d", len(resp.Scenarios))
	}

	// Verify /healthz endpoint
	reqHealth := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recHealth := httptest.NewRecorder()
	h.ServeHTTP(recHealth, reqHealth)
	if recHealth.Code != http.StatusOK {
		t.Fatalf("expected /healthz status 200, got %d", recHealth.Code)
	}
}

func TestCompareSimulationsEndpointSuccess(t *testing.T) {
	h := NewHandler()
	payload := model.SimulationRequest{
		DurationSec:       10,
		RequestsPerSecond: 20,
		Seed:              42,
		Fault: model.FaultConfig{
			TargetService: topology.NodeFeatureDB,
			Type:          model.FaultLatency,
			StartSec:      2,
			EndSec:        6,
			LatencyMs:     300,
		},
		Services: topology.DefaultServices(),
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/simulations/compare", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.CompareResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal compare response: %v", err)
	}
	if len(resp.Results) != 5 {
		t.Fatalf("expected 5 strategy results when strategies is omitted, got %d", len(resp.Results))
	}
	res := resp.Results[0]
	if res.StrategyID != "baseline" {
		t.Errorf("expected first strategyId 'baseline', got %q", res.StrategyID)
	}
	if len(res.TimeSeries) != 10 {
		t.Errorf("expected 10 timeSeries points, got %d", len(res.TimeSeries))
	}
	if res.Metrics.Overall.TotalRequests != 200 {
		t.Errorf("expected 200 totalRequests, got %d", res.Metrics.Overall.TotalRequests)
	}
}

func TestCompareSimulationsValidationErrors(t *testing.T) {
	h := NewHandler()

	cases := []struct {
		name string
		body string
	}{
		{
			name: "invalid JSON",
			body: `{bad json`,
		},
		{
			name: "negative durationSec",
			body: `{"durationSec": -5, "requestsPerSecond": 20}`,
		},
		{
			name: "excessive RPS",
			body: `{"durationSec": 10, "requestsPerSecond": 99999}`,
		},
		{
			name: "invalid fault target",
			body: `{"durationSec": 10, "requestsPerSecond": 20, "fault": {"targetService": "unknown-db", "type": "outage", "startSec": 1, "endSec": 5}}`,
		},
		{
			name: "invalid fault window startSec >= endSec",
			body: `{"durationSec": 10, "requestsPerSecond": 20, "fault": {"targetService": "feature-db", "type": "outage", "startSec": 8, "endSec": 4}}`,
		},
		{
			name: "invalid latencyMs for latency fault",
			body: `{"durationSec": 10, "requestsPerSecond": 20, "fault": {"targetService": "feature-db", "type": "latency", "startSec": 2, "endSec": 6, "latencyMs": 0}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/simulations/compare", bytes.NewReader([]byte(tc.body)))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("case %q: expected 400 Bad Request, got %d (%s)", tc.name, rec.Code, rec.Body.String())
			}
		})
	}
}
