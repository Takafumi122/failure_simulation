package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"failurelab/backend/internal/engine"
	"failurelab/backend/internal/model"
	"failurelab/backend/internal/resilience"
	"failurelab/backend/internal/topology"
)

const (
	MaxAllowedDurationSec = 300
	MaxAllowedRPS         = 500
	MaxAllowedRetries     = 10
	MaxRequestBodyBytes   = 64 * 1024 // 64 KB max JSON payload
)

// ErrorResponse represents a JSON error payload.
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// ipBucket tracks token bucket state per client IP for rate limiting.
type ipBucket struct {
	tokens     float64
	lastRefill time.Time
}

// RateLimiter implements an in-memory per-IP token bucket rate limiter.
type RateLimiter struct {
	mu           sync.Mutex
	buckets      map[string]*ipBucket
	ratePerSec   float64
	burst        float64
	cleanupTimer time.Time
}

// NewRateLimiter creates a token-bucket rate limiter.
func NewRateLimiter(ratePerSec float64, burst float64) *RateLimiter {
	return &RateLimiter{
		buckets:      make(map[string]*ipBucket),
		ratePerSec:   ratePerSec,
		burst:        burst,
		cleanupTimer: time.Now(),
	}
}

func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	// Periodic map cleanup every 5 minutes to prevent unbounded memory growth
	if now.Sub(rl.cleanupTimer) > 5*time.Minute {
		for k, b := range rl.buckets {
			if now.Sub(b.lastRefill) > 10*time.Minute {
				delete(rl.buckets, k)
			}
		}
		rl.cleanupTimer = now
	}

	b, exists := rl.buckets[ip]
	if !exists {
		rl.buckets[ip] = &ipBucket{
			tokens:     rl.burst - 1,
			lastRefill: now,
		}
		return true
	}

	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * rl.ratePerSec
	if b.tokens > rl.burst {
		b.tokens = rl.burst
	}
	b.lastRefill = now

	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return true
	}
	return false
}

// Handler serves the FailureLab HTTP API endpoints and optional static frontend assets.
type Handler struct {
	mux         *http.ServeMux
	rateLimiter *RateLimiter
	staticDir   string
}

// NewHandler creates and registers all HTTP routes with CORS, security headers, and rate limiting.
func NewHandler() http.Handler {
	staticDir := os.Getenv("STATIC_DIR")
	h := &Handler{
		mux:         http.NewServeMux(),
		rateLimiter: NewRateLimiter(5.0, 30.0), // 5 req/sec sustained, burst of 30 per IP
		staticDir:   staticDir,
	}
	h.mux.HandleFunc("GET /api/scenarios", h.handleGetScenarios)
	h.mux.HandleFunc("POST /api/simulations/compare", h.handleCompareSimulations)
	h.mux.HandleFunc("GET /api/health", h.handleHealth)
	h.mux.HandleFunc("GET /healthz", h.handleHealth)

	if staticDir != "" {
		fs := http.FileServer(http.Dir(staticDir))
		h.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeError(w, http.StatusNotFound, "not_found", "API route not found")
				return
			}
			cleanPath := filepath.Clean(r.URL.Path)
			fullPath := filepath.Join(staticDir, cleanPath)
			if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
				fs.ServeHTTP(w, r)
				return
			}
			// SPA fallback to index.html
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
		})
	}

	return h.withMiddleware(h.mux)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (h *Handler) withMiddleware(next http.Handler) http.Handler {
	allowedOriginsEnv := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowOrigin := "*"
		if allowedOriginsEnv != "" && allowedOriginsEnv != "*" {
			allowOrigin = ""
			for _, o := range strings.Split(allowedOriginsEnv, ",") {
				if strings.TrimSpace(o) == origin {
					allowOrigin = origin
					break
				}
			}
		}
		if allowOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Rate limit simulation compare endpoint
		if strings.HasPrefix(r.URL.Path, "/api/simulations/") {
			ip := clientIP(r)
			if !h.rateLimiter.allow(ip) {
				writeError(w, http.StatusTooManyRequests, "rate_limit_exceeded",
					"Too many simulation requests from this IP. Please wait a moment and try again.")
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, msg string, details string) {
	writeJSON(w, status, ErrorResponse{
		Error:   msg,
		Details: details,
	})
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":        "ok",
		"engineVersion": model.EngineVersion,
	})
}

func (h *Handler) handleGetScenarios(w http.ResponseWriter, r *http.Request) {
	resp := model.ScenariosResponse{
		EngineVersion: model.EngineVersion,
		Topology:      topology.FixedTopology(),
		Scenarios:     topology.PresetScenarios(),
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleCompareSimulations(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodyBytes)

	var req model.SimulationRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", fmt.Sprintf("Failed to parse JSON body (max %d bytes): %v", MaxRequestBodyBytes, err))
		return
	}

	if err := ValidateSimulationRequest(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	strategies := req.Strategies
	if len(strategies) == 0 {
		strategies = resilience.AllStrategyIDs
	}

	results := make([]model.StrategyResult, 0, len(strategies))
	for _, stratID := range strategies {
		policy := resilience.NewPolicy(stratID, req.Resilience)
		sim := engine.NewSimulator(req, policy)
		res := sim.Run()
		results = append(results, res)
	}

	resp := model.CompareResponse{
		EngineVersion: model.EngineVersion,
		Request:       req,
		Topology:      topology.FixedTopology(),
		Results:       results,
	}
	writeJSON(w, http.StatusOK, resp)
}

// ValidateSimulationRequest validates simulation parameters and returns an error if invalid.
func ValidateSimulationRequest(req *model.SimulationRequest) error {
	if req.DurationSec <= 0 || req.DurationSec > MaxAllowedDurationSec {
		return fmt.Errorf("durationSec must be between 1 and %d (got %d)", MaxAllowedDurationSec, req.DurationSec)
	}
	if req.RequestsPerSecond <= 0 || req.RequestsPerSecond > MaxAllowedRPS {
		return fmt.Errorf("requestsPerSecond must be between 1 and %d (got %d)", MaxAllowedRPS, req.RequestsPerSecond)
	}
	if req.MaxEvents < 0 || req.MaxEvents > engine.DefaultMaxEvents {
		return fmt.Errorf("maxEvents must be between 0 and %d (got %d)", engine.DefaultMaxEvents, req.MaxEvents)
	}

	if req.Fault.Type == "" {
		req.Fault.Type = model.FaultNone
	}

	switch req.Fault.Type {
	case model.FaultNone, model.FaultOutage, model.FaultLatency, model.FaultErrorRate:
	default:
		return fmt.Errorf("unsupported fault.type: %q (must be none, outage, latency, or error_rate)", req.Fault.Type)
	}

	if req.Fault.Type != model.FaultNone {
		if !topology.IsValidFaultTarget(req.Fault.TargetService) {
			return fmt.Errorf("invalid fault.targetService: %q", req.Fault.TargetService)
		}
		if req.Fault.StartSec < 0 || req.Fault.EndSec <= req.Fault.StartSec || req.Fault.EndSec > float64(req.DurationSec) {
			return fmt.Errorf("fault window [startSec=%.1f, endSec=%.1f] must satisfy 0 <= startSec < endSec <= durationSec (%d)",
				req.Fault.StartSec, req.Fault.EndSec, req.DurationSec)
		}
		if req.Fault.Type == model.FaultLatency && (req.Fault.LatencyMs <= 0 || req.Fault.LatencyMs > 30000) {
			return fmt.Errorf("fault.latencyMs must be between 1 and 30000 for latency fault (got %.1f)", req.Fault.LatencyMs)
		}
		if req.Fault.Type == model.FaultErrorRate && (req.Fault.FailureRate <= 0 || req.Fault.FailureRate > 1.0) {
			return fmt.Errorf("fault.failureRate must be in (0.0, 1.0] for error_rate fault (got %.2f)", req.Fault.FailureRate)
		}
	}

	defaults := topology.DefaultServices()
	if req.Services == nil {
		req.Services = defaults
	} else {
		for id, cfg := range req.Services {
			if !topology.IsValidFaultTarget(id) {
				return fmt.Errorf("unknown service in services map: %q", id)
			}
			if cfg.BaseLatencyMs < 0 || cfg.BaseLatencyMs > 30000 {
				return fmt.Errorf("service %q baseLatencyMs must be between 0 and 30000", id)
			}
			if cfg.FailureRate < 0 || cfg.FailureRate > 1.0 {
				return fmt.Errorf("service %q failureRate must be between 0.0 and 1.0", id)
			}
			if cfg.MaxConcurrent <= 0 || cfg.MaxConcurrent > 10000 {
				return fmt.Errorf("service %q maxConcurrent must be between 1 and 10000", id)
			}
			if cfg.QueueLimit < 0 || cfg.QueueLimit > 100000 {
				return fmt.Errorf("service %q queueLimit must be between 0 and 100000", id)
			}
			if cfg.TimeoutMs <= 0 {
				cfg.TimeoutMs = defaults[id].TimeoutMs
				req.Services[id] = cfg
			}
		}
		for id, defCfg := range defaults {
			if _, exists := req.Services[id]; !exists {
				req.Services[id] = defCfg
			}
		}
	}

	if req.Resilience != nil {
		if req.Resilience.Retry.MaxRetries < 0 || req.Resilience.Retry.MaxRetries > MaxAllowedRetries {
			return fmt.Errorf("resilience.retry.maxRetries must be between 0 and %d", MaxAllowedRetries)
		}
		if req.Resilience.CircuitBreaker.ConsecutiveFailuresThreshold < 0 || req.Resilience.CircuitBreaker.ConsecutiveFailuresThreshold > 1000 {
			return fmt.Errorf("resilience.circuitBreaker.consecutiveFailuresThreshold must be between 1 and 1000")
		}
		if req.Resilience.FailFastTimeoutMs < 0 || req.Resilience.FailFastTimeoutMs > 30000 {
			return fmt.Errorf("resilience.failFastTimeoutMs must be between 1 and 30000")
		}
	}
	normRes := resilience.NormalizeResilienceConfig(req.Resilience)
	req.Resilience = &normRes

	if len(req.Strategies) > 10 {
		return fmt.Errorf("too many strategies requested (max 10)")
	}

	return nil
}
