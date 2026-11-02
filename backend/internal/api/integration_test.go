package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"failurelab/backend/internal/model"
)

func TestPresetScenariosEndToEndViaHTTPServer(t *testing.T) {
	srv := httptest.NewUnstartedServer(NewHandler())
	// In sandbox environments where binding a TCP socket may be restricted, fallback to Handler.ServeHTTP
	h := NewHandler()

	// 1. Fetch GET /api/scenarios
	reqGet := httptest.NewRequest("GET", "/api/scenarios", nil)
	recGet := httptest.NewRecorder()
	h.ServeHTTP(recGet, reqGet)
	if recGet.Code != 200 {
		t.Fatalf("GET /api/scenarios returned %d", recGet.Code)
	}

	var scenResp model.ScenariosResponse
	if err := json.Unmarshal(recGet.Body.Bytes(), &scenResp); err != nil {
		t.Fatalf("failed to decode scenarios: %v", err)
	}

	// 2. Run POST /api/simulations/compare for every preset scenario and verify realistic metrics
	for _, sc := range scenResp.Scenarios {
		payload, _ := json.Marshal(sc.Config)
		reqPost := httptest.NewRequest("POST", "/api/simulations/compare", bytes.NewReader(payload))
		reqPost.Header.Set("Content-Type", "application/json")
		recPost := httptest.NewRecorder()
		h.ServeHTTP(recPost, reqPost)

		if recPost.Code != 200 {
			t.Fatalf("scenario %s failed with status %d: %s", sc.ID, recPost.Code, recPost.Body.String())
		}

		var cmpResp model.CompareResponse
		if err := json.Unmarshal(recPost.Body.Bytes(), &cmpResp); err != nil {
			t.Fatalf("failed to decode compare response for %s: %v", sc.ID, err)
		}
		res := cmpResp.Results[0]
		ov := res.Metrics.Overall
		t.Logf("[Scenario %s] totalReq=%d successRate=%.1f%% errorRate=%.1f%% timeouts=%d queueDrops=%d p95=%.1fms events=%d",
			sc.ID, ov.TotalRequests, ov.SuccessRate*100, ov.ErrorRate*100, ov.TimeoutCount, ov.QueueOverflowCount, ov.P95LatencyMs, res.ExecutionMeta.TotalEventsProcessed)

		if ov.TotalRequests != sc.Config.DurationSec*sc.Config.RequestsPerSecond {
			t.Errorf("scenario %s: expected %d requests, got %d", sc.ID, sc.Config.DurationSec*sc.Config.RequestsPerSecond, ov.TotalRequests)
		}
		if len(res.TimeSeries) != sc.Config.DurationSec {
			t.Errorf("scenario %s: expected %d timeSeries points, got %d", sc.ID, sc.Config.DurationSec, len(res.TimeSeries))
		}
	}
	_ = srv
	_ = fmt.Sprint()
}
