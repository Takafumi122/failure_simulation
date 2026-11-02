import React, { useRef, useState } from 'react';
import {
  ChevronDown,
  ChevronUp,
  Download,
  Flame,
  Play,
  RefreshCw,
  Repeat,
  Settings2,
  ShieldAlert,
  Sliders,
  Upload,
  Zap,
} from 'lucide-react';
import {
  CircuitBreakerConfig,
  FaultType,
  ResilienceConfig,
  RetryConfig,
  ScenarioPreset,
  ServiceConfig,
  SimulationRequest,
} from '../types/simulation';

interface ControlPanelProps {
  scenarios: ScenarioPreset[];
  selectedScenarioId: string;
  onSelectScenario: (scenario: ScenarioPreset) => void;
  config: SimulationRequest;
  onChangeConfig: (cfg: SimulationRequest) => void;
  onRunSimulation: () => void;
  onImportConfig: (cfg: SimulationRequest) => void;
  isLoading: boolean;
}

const SERVICE_LABELS: Record<string, string> = {
  'api-gateway': 'API Gateway',
  'product-api': 'Product API',
  recommendation: 'Recommendation',
  'feature-db': 'Feature DB',
};

const SERVICE_ORDER = ['api-gateway', 'product-api', 'recommendation', 'feature-db'];

const DEFAULT_RESILIENCE: ResilienceConfig = {
  targetCaller: 'recommendation',
  targetCallee: 'feature-db',
  retry: {
    maxRetries: 3,
    initialBackoffMs: 25,
    maxBackoffMs: 250,
    backoffMultiplier: 2.0,
    jitter: true,
  },
  circuitBreaker: {
    consecutiveFailuresThreshold: 5,
    openTimeoutMs: 3000,
    halfOpenMaxCalls: 2,
    halfOpenSuccessThreshold: 2,
  },
  failFastTimeoutMs: 120,
};

export function validateImportedConfig(raw: unknown): SimulationRequest {
  if (!raw || typeof raw !== 'object') {
    throw new Error('JSON root must be an object.');
  }
  const obj = raw as Record<string, unknown>;
  const durationSec = Number(obj.durationSec);
  const requestsPerSecond = Number(obj.requestsPerSecond);
  const seed = Number(obj.seed ?? 42);

  if (!Number.isFinite(durationSec) || durationSec < 1 || durationSec > 300) {
    throw new Error('durationSec must be between 1 and 300.');
  }
  if (!Number.isFinite(requestsPerSecond) || requestsPerSecond < 1 || requestsPerSecond > 500) {
    throw new Error('requestsPerSecond must be between 1 and 500.');
  }
  if (!obj.fault || typeof obj.fault !== 'object') {
    throw new Error('Missing fault configuration object.');
  }
  const fault = obj.fault as Record<string, unknown>;
  const faultType = String(fault.type || 'none') as FaultType;
  if (!['none', 'outage', 'latency', 'error_rate'].includes(faultType)) {
    throw new Error(`Invalid fault.type: ${faultType}`);
  }
  const targetService = String(fault.targetService || 'feature-db');
  if (!SERVICE_ORDER.includes(targetService)) {
    throw new Error(`Invalid fault.targetService: ${targetService}`);
  }
  const startSec = Number(fault.startSec ?? 0);
  const endSec = Number(fault.endSec ?? durationSec);
  if (faultType !== 'none' && (startSec < 0 || endSec <= startSec || endSec > durationSec)) {
    throw new Error('Fault window must satisfy 0 <= startSec < endSec <= durationSec.');
  }

  return {
    scenarioId: typeof obj.scenarioId === 'string' ? obj.scenarioId : 'custom-imported',
    durationSec,
    requestsPerSecond,
    seed,
    fault: {
      targetService,
      type: faultType,
      startSec,
      endSec,
      latencyMs: Number(fault.latencyMs ?? 450),
      failureRate: Number(fault.failureRate ?? 0.35),
    },
    services: (obj.services as Record<string, ServiceConfig>) || {},
    resilience: (obj.resilience as ResilienceConfig) || DEFAULT_RESILIENCE,
    strategies: Array.isArray(obj.strategies)
      ? (obj.strategies as string[])
      : ['baseline', 'retry', 'circuit_breaker', 'retry_circuit_breaker', 'fail_fast'],
  };
}

export const ControlPanel: React.FC<ControlPanelProps> = ({
  scenarios,
  selectedScenarioId,
  onSelectScenario,
  config,
  onChangeConfig,
  onRunSimulation,
  onImportConfig,
  isLoading,
}) => {
  const [showResilienceControls, setShowResilienceControls] = useState(true);
  const [showAdvancedServices, setShowAdvancedServices] = useState(false);
  const [importError, setImportError] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  const currentScenario = scenarios.find((s) => s.id === selectedScenarioId);
  const resCfg: ResilienceConfig = config.resilience || DEFAULT_RESILIENCE;

  const updateFault = (patch: Partial<SimulationRequest['fault']>) => {
    onChangeConfig({
      ...config,
      fault: {
        ...config.fault,
        ...patch,
      },
    });
  };

  const updateRetry = (patch: Partial<RetryConfig>) => {
    onChangeConfig({
      ...config,
      resilience: {
        ...resCfg,
        retry: {
          ...resCfg.retry,
          ...patch,
        },
      },
    });
  };

  const updateCB = (patch: Partial<CircuitBreakerConfig>) => {
    onChangeConfig({
      ...config,
      resilience: {
        ...resCfg,
        circuitBreaker: {
          ...resCfg.circuitBreaker,
          ...patch,
        },
      },
    });
  };

  const updateFailFast = (timeoutMs: number) => {
    onChangeConfig({
      ...config,
      resilience: {
        ...resCfg,
        failFastTimeoutMs: timeoutMs,
      },
    });
  };

  const updateService = (serviceId: string, patch: Partial<ServiceConfig>) => {
    onChangeConfig({
      ...config,
      services: {
        ...config.services,
        [serviceId]: {
          ...config.services[serviceId],
          ...patch,
        },
      },
    });
  };

  const handleExportJSON = () => {
    const jsonStr = JSON.stringify(config, null, 2);
    const blob = new Blob([jsonStr], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `failurelab-config-${config.scenarioId || 'custom'}-seed${config.seed}.json`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const handleImportFile = (e: React.ChangeEvent<HTMLInputElement>) => {
    setImportError(null);
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
      try {
        const parsed = JSON.parse(String(reader.result));
        const validated = validateImportedConfig(parsed);
        onImportConfig(validated);
      } catch (err) {
        setImportError(err instanceof Error ? err.message : 'Invalid JSON configuration file');
      } finally {
        if (fileInputRef.current) fileInputRef.current.value = '';
      }
    };
    reader.readAsText(file);
  };

  return (
    <div className="rounded-xl bg-slate-900/90 border border-slate-800 p-4 space-y-4">
      {/* 1. Scenario Selector */}
      <div>
        <div className="flex items-center justify-between mb-2">
          <label className="text-xs font-bold uppercase tracking-wider text-slate-300 flex items-center gap-1.5">
            <Sliders className="w-3.5 h-3.5 text-indigo-400" />
            Scenario Presets
          </label>
          <span className="text-[10px] text-indigo-400 font-medium">
            {currentScenario?.category || 'Custom Parameters'}
          </span>
        </div>

        <select
          data-testid="scenario-select"
          value={selectedScenarioId}
          onChange={(e) => {
            const found = scenarios.find((s) => s.id === e.target.value);
            if (found) onSelectScenario(found);
          }}
          className="w-full rounded-lg bg-slate-950 border border-slate-700 px-3 py-2 text-xs font-medium text-slate-100 focus:outline-none focus:border-indigo-500"
        >
          {scenarios.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </select>

        {currentScenario && (
          <p className="mt-2 text-xs text-slate-400 leading-relaxed bg-slate-950/60 rounded-lg p-2.5 border border-slate-800/80">
            {currentScenario.description}
          </p>
        )}
      </div>

      {/* 2. Traffic & Global Settings */}
      <div className="border-t border-slate-800 pt-3.5">
        <div className="text-xs font-bold uppercase tracking-wider text-slate-300 mb-2.5">
          Traffic & Deterministic Seed
        </div>
        <div className="grid grid-cols-3 gap-2">
          <div>
            <label className="block text-[10px] text-slate-400 mb-1">
              RPS (Req/sec)
            </label>
            <input
              data-testid="input-rps"
              type="number"
              min={1}
              max={500}
              value={config.requestsPerSecond}
              onChange={(e) =>
                onChangeConfig({
                  ...config,
                  requestsPerSecond: Math.max(1, Number(e.target.value)),
                })
              }
              className="w-full rounded-lg bg-slate-950 border border-slate-700 px-2 py-1.5 text-xs font-mono text-slate-100"
            />
          </div>

          <div>
            <label className="block text-[10px] text-slate-400 mb-1">
              Duration (sec)
            </label>
            <input
              data-testid="input-duration"
              type="number"
              min={5}
              max={300}
              value={config.durationSec}
              onChange={(e) =>
                onChangeConfig({
                  ...config,
                  durationSec: Math.max(5, Number(e.target.value)),
                })
              }
              className="w-full rounded-lg bg-slate-950 border border-slate-700 px-2 py-1.5 text-xs font-mono text-slate-100"
            />
          </div>

          <div>
            <label className="block text-[10px] text-slate-400 mb-1">
              Random Seed
            </label>
            <input
              data-testid="input-seed"
              type="number"
              value={config.seed}
              onChange={(e) =>
                onChangeConfig({
                  ...config,
                  seed: Number(e.target.value),
                })
              }
              className="w-full rounded-lg bg-slate-950 border border-slate-700 px-2 py-1.5 text-xs font-mono text-slate-100"
            />
          </div>
        </div>
      </div>

      {/* 3. Fault Injection Parameters */}
      <div className="border-t border-slate-800 pt-3.5">
        <div className="text-xs font-bold uppercase tracking-wider text-rose-300 flex items-center gap-1.5 mb-2.5">
          <Flame className="w-3.5 h-3.5 text-rose-400" />
          Fault Injection Settings
        </div>

        <div className="space-y-2.5">
          <div className="grid grid-cols-2 gap-2">
            <div>
              <label className="block text-[10px] text-slate-400 mb-1">
                Target Service
              </label>
              <select
                value={config.fault.targetService}
                onChange={(e) => updateFault({ targetService: e.target.value })}
                className="w-full rounded-lg bg-slate-950 border border-slate-700 px-2 py-1.5 text-xs text-slate-100"
              >
                {SERVICE_ORDER.map((id) => (
                  <option key={id} value={id}>
                    {SERVICE_LABELS[id]}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="block text-[10px] text-slate-400 mb-1">
                Fault Type
              </label>
              <select
                value={config.fault.type}
                onChange={(e) => {
                  const newType = e.target.value as FaultType;
                  updateFault({
                    type: newType,
                    latencyMs:
                      newType === 'latency' &&
                      (!config.fault.latencyMs || config.fault.latencyMs <= 0)
                        ? 450
                        : config.fault.latencyMs,
                    failureRate:
                      newType === 'error_rate' &&
                      (!config.fault.failureRate || config.fault.failureRate <= 0)
                        ? 0.35
                        : config.fault.failureRate,
                  });
                }}
                className="w-full rounded-lg bg-slate-950 border border-slate-700 px-2 py-1.5 text-xs text-slate-100"
              >
                <option value="latency">Latency Spike (遅延)</option>
                <option value="outage">Total Outage (100% 停止)</option>
                <option value="error_rate">Probabilistic Error (確率的失敗)</option>
                <option value="none">None (障害なし)</option>
              </select>
            </div>
          </div>

          {config.fault.type !== 'none' && (
            <>
              <div className="grid grid-cols-2 gap-2">
                <div>
                  <label className="block text-[10px] text-slate-400 mb-1">
                    Fault Start (sec)
                  </label>
                  <input
                    type="number"
                    min={0}
                    max={config.durationSec - 1}
                    value={config.fault.startSec}
                    onChange={(e) =>
                      updateFault({ startSec: Math.max(0, Number(e.target.value)) })
                    }
                    className="w-full rounded-lg bg-slate-950 border border-slate-700 px-2 py-1.5 text-xs font-mono text-slate-100"
                  />
                </div>
                <div>
                  <label className="block text-[10px] text-slate-400 mb-1">
                    Fault End (sec)
                  </label>
                  <input
                    type="number"
                    min={1}
                    max={config.durationSec}
                    value={config.fault.endSec}
                    onChange={(e) =>
                      updateFault({ endSec: Math.max(1, Number(e.target.value)) })
                    }
                    className="w-full rounded-lg bg-slate-950 border border-slate-700 px-2 py-1.5 text-xs font-mono text-slate-100"
                  />
                </div>
              </div>

              {config.fault.type === 'latency' && (
                <div>
                  <label className="block text-[10px] text-amber-300 mb-1">
                    Injected Extra Latency (ms)
                  </label>
                  <input
                    type="number"
                    min={10}
                    max={10000}
                    step={20}
                    value={config.fault.latencyMs || 450}
                    onChange={(e) =>
                      updateFault({ latencyMs: Math.max(1, Number(e.target.value)) })
                    }
                    className="w-full rounded-lg bg-slate-950 border border-amber-500/50 px-2.5 py-1.5 text-xs font-mono text-amber-200"
                  />
                </div>
              )}

              {config.fault.type === 'error_rate' && (
                <div>
                  <label className="block text-[10px] text-rose-300 mb-1">
                    Injected Failure Probability (0.05 – 1.0)
                  </label>
                  <input
                    type="number"
                    min={0.05}
                    max={1.0}
                    step={0.05}
                    value={config.fault.failureRate || 0.35}
                    onChange={(e) =>
                      updateFault({
                        failureRate: Math.min(1, Math.max(0.01, Number(e.target.value))),
                      })
                    }
                    className="w-full rounded-lg bg-slate-950 border border-rose-500/50 px-2.5 py-1.5 text-xs font-mono text-rose-200"
                  />
                </div>
              )}
            </>
          )}
        </div>
      </div>

      {/* 4. Phase 2/3: Retry, Circuit Breaker & Fail-Fast Controls with Descriptions */}
      <div className="border-t border-slate-800 pt-3">
        <button
          type="button"
          onClick={() => setShowResilienceControls(!showResilienceControls)}
          className="w-full flex items-center justify-between text-xs font-bold uppercase tracking-wider text-sky-300 hover:text-white py-1 cursor-pointer"
        >
          <span className="flex items-center gap-1.5">
            <ShieldAlert className="w-3.5 h-3.5 text-sky-400" />
            Retry, Circuit Breaker & Fail-Fast
          </span>
          {showResilienceControls ? (
            <ChevronUp className="w-4 h-4 text-slate-400" />
          ) : (
            <ChevronDown className="w-4 h-4 text-slate-400" />
          )}
        </button>

        {showResilienceControls && (
          <div className="mt-2.5 space-y-3">
            <div className="text-[10px] text-slate-400 bg-slate-950/70 border border-slate-800 rounded px-2.5 py-1.5 font-mono">
              Target Call Edge:{' '}
              <span className="text-indigo-300 font-semibold">
                {SERVICE_LABELS[resCfg.targetCaller] || resCfg.targetCaller} →{' '}
                {SERVICE_LABELS[resCfg.targetCallee] || resCfg.targetCallee}
              </span>
            </div>

            {/* Retry Policy Box */}
            <div className="rounded-lg bg-slate-950/90 border border-slate-800 p-2.5 space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-[11px] font-bold text-sky-300 flex items-center gap-1">
                  <Repeat className="w-3 h-3" /> Retry (Exp Backoff + Jitter)
                </span>
                <label className="flex items-center gap-1 text-[10px] text-slate-300 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={resCfg.retry.jitter}
                    onChange={(e) => updateRetry({ jitter: e.target.checked })}
                    className="accent-indigo-500"
                  />
                  Jitter
                </label>
              </div>
              <p className="text-[10px] text-slate-400 leading-snug">
                Re-attempts failed calls after exponential backoff. Recovers transient errors, but can trigger a Retry Storm during prolonged outages.
              </p>
              <div className="grid grid-cols-3 gap-2">
                <div>
                  <label className="block text-[10px] text-slate-400">
                    Max Retries
                  </label>
                  <input
                    type="number"
                    min={0}
                    max={10}
                    value={resCfg.retry.maxRetries}
                    onChange={(e) =>
                      updateRetry({ maxRetries: Math.max(0, Number(e.target.value)) })
                    }
                    className="w-full rounded bg-slate-900 border border-slate-700 px-2 py-1 text-xs font-mono"
                  />
                </div>
                <div>
                  <label className="block text-[10px] text-slate-400">
                    Base Backoff (ms)
                  </label>
                  <input
                    type="number"
                    min={5}
                    max={2000}
                    value={resCfg.retry.initialBackoffMs}
                    onChange={(e) =>
                      updateRetry({
                        initialBackoffMs: Math.max(1, Number(e.target.value)),
                      })
                    }
                    className="w-full rounded bg-slate-900 border border-slate-700 px-2 py-1 text-xs font-mono"
                  />
                </div>
                <div>
                  <label className="block text-[10px] text-slate-400">
                    Max Backoff (ms)
                  </label>
                  <input
                    type="number"
                    min={10}
                    max={5000}
                    value={resCfg.retry.maxBackoffMs}
                    onChange={(e) =>
                      updateRetry({
                        maxBackoffMs: Math.max(10, Number(e.target.value)),
                      })
                    }
                    className="w-full rounded bg-slate-900 border border-slate-700 px-2 py-1 text-xs font-mono"
                  />
                </div>
              </div>
            </div>

            {/* Circuit Breaker Box */}
            <div className="rounded-lg bg-slate-950/90 border border-slate-800 p-2.5 space-y-2">
              <div className="text-[11px] font-bold text-violet-300 flex items-center gap-1">
                <ShieldAlert className="w-3 h-3" /> Circuit Breaker (Closed/Open/Half-Open)
              </div>
              <p className="text-[10px] text-slate-400 leading-snug">
                Trips OPEN after N consecutive failures to fast-reject calls and protect downstream workers. Probes in HALF_OPEN after cooldown.
              </p>
              <div className="grid grid-cols-2 gap-2">
                <div>
                  <label className="block text-[10px] text-slate-400">
                    Failure Threshold
                  </label>
                  <input
                    type="number"
                    min={1}
                    max={100}
                    value={resCfg.circuitBreaker.consecutiveFailuresThreshold}
                    onChange={(e) =>
                      updateCB({
                        consecutiveFailuresThreshold: Math.max(1, Number(e.target.value)),
                      })
                    }
                    className="w-full rounded bg-slate-900 border border-slate-700 px-2 py-1 text-xs font-mono"
                  />
                </div>
                <div>
                  <label className="block text-[10px] text-slate-400">
                    Open Cooldown (ms)
                  </label>
                  <input
                    type="number"
                    min={200}
                    max={30000}
                    step={250}
                    value={resCfg.circuitBreaker.openTimeoutMs}
                    onChange={(e) =>
                      updateCB({
                        openTimeoutMs: Math.max(100, Number(e.target.value)),
                      })
                    }
                    className="w-full rounded bg-slate-900 border border-slate-700 px-2 py-1 text-xs font-mono"
                  />
                </div>
                <div>
                  <label className="block text-[10px] text-slate-400">
                    Half-Open Max Calls
                  </label>
                  <input
                    type="number"
                    min={1}
                    max={20}
                    value={resCfg.circuitBreaker.halfOpenMaxCalls}
                    onChange={(e) =>
                      updateCB({
                        halfOpenMaxCalls: Math.max(1, Number(e.target.value)),
                      })
                    }
                    className="w-full rounded bg-slate-900 border border-slate-700 px-2 py-1 text-xs font-mono"
                  />
                </div>
                <div>
                  <label className="block text-[10px] text-slate-400">
                    Successes to Close
                  </label>
                  <input
                    type="number"
                    min={1}
                    max={20}
                    value={resCfg.circuitBreaker.halfOpenSuccessThreshold}
                    onChange={(e) =>
                      updateCB({
                        halfOpenSuccessThreshold: Math.max(1, Number(e.target.value)),
                      })
                    }
                    className="w-full rounded bg-slate-900 border border-slate-700 px-2 py-1 text-xs font-mono"
                  />
                </div>
              </div>
            </div>

            {/* Fail-Fast Timeout Box */}
            <div className="rounded-lg bg-slate-950/90 border border-slate-800 p-2.5 space-y-1.5">
              <div className="flex items-center justify-between gap-2">
                <div className="text-[11px] font-bold text-amber-300 flex items-center gap-1">
                  <Zap className="w-3 h-3" /> Fail-Fast Attempt Timeout (ms)
                </div>
                <input
                  type="number"
                  min={20}
                  max={5000}
                  step={10}
                  value={resCfg.failFastTimeoutMs}
                  onChange={(e) =>
                    updateFailFast(Math.max(10, Number(e.target.value)))
                  }
                  className="w-24 rounded bg-slate-900 border border-amber-500/40 px-2 py-1 text-xs font-mono text-right text-amber-200"
                />
              </div>
              <p className="text-[10px] text-slate-400 leading-snug">
                Enforces a shorter per-attempt deadline so slow downstream calls abort early before exhausting upstream worker pools.
              </p>
            </div>
          </div>
        )}
      </div>

      {/* 5. Per-Service Capacity & Latency Tuning (Collapsible) */}
      <div className="border-t border-slate-800 pt-3">
        <button
          type="button"
          onClick={() => setShowAdvancedServices(!showAdvancedServices)}
          className="w-full flex items-center justify-between text-xs font-semibold text-slate-300 hover:text-white py-1 cursor-pointer"
        >
          <span className="flex items-center gap-1.5">
            <Settings2 className="w-3.5 h-3.5 text-slate-400" />
            Per-Service Capacity & Timeouts
          </span>
          {showAdvancedServices ? (
            <ChevronUp className="w-4 h-4 text-slate-400" />
          ) : (
            <ChevronDown className="w-4 h-4 text-slate-400" />
          )}
        </button>

        {showAdvancedServices && (
          <div className="mt-3 space-y-3 max-h-64 overflow-y-auto pr-1">
            {SERVICE_ORDER.map((srvId) => {
              const sc = config.services[srvId];
              if (!sc) return null;
              return (
                <div
                  key={srvId}
                  className="rounded-lg bg-slate-950/90 border border-slate-800 p-2.5 space-y-2"
                >
                  <div className="text-xs font-bold text-indigo-300">
                    {SERVICE_LABELS[srvId]}
                  </div>
                  <div className="grid grid-cols-2 gap-2">
                    <div>
                      <label className="block text-[10px] text-slate-400">
                        Base Latency (ms)
                      </label>
                      <input
                        type="number"
                        min={1}
                        value={sc.baseLatencyMs}
                        onChange={(e) =>
                          updateService(srvId, { baseLatencyMs: Number(e.target.value) })
                        }
                        className="w-full rounded bg-slate-900 border border-slate-700 px-2 py-1 text-xs font-mono"
                      />
                    </div>
                    <div>
                      <label className="block text-[10px] text-slate-400">
                        Timeout (ms)
                      </label>
                      <input
                        type="number"
                        min={50}
                        value={sc.timeoutMs}
                        onChange={(e) =>
                          updateService(srvId, { timeoutMs: Number(e.target.value) })
                        }
                        className="w-full rounded bg-slate-900 border border-slate-700 px-2 py-1 text-xs font-mono"
                      />
                    </div>
                    <div>
                      <label className="block text-[10px] text-slate-400">
                        Max Concurrent
                      </label>
                      <input
                        type="number"
                        min={1}
                        value={sc.maxConcurrent}
                        onChange={(e) =>
                          updateService(srvId, { maxConcurrent: Number(e.target.value) })
                        }
                        className="w-full rounded bg-slate-900 border border-slate-700 px-2 py-1 text-xs font-mono"
                      />
                    </div>
                    <div>
                      <label className="block text-[10px] text-slate-400">
                        Queue Limit
                      </label>
                      <input
                        type="number"
                        min={0}
                        value={sc.queueLimit}
                        onChange={(e) =>
                          updateService(srvId, { queueLimit: Number(e.target.value) })
                        }
                        className="w-full rounded bg-slate-900 border border-slate-700 px-2 py-1 text-xs font-mono"
                      />
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* 6. Reproducibility: Export / Import JSON Configuration */}
      <div className="border-t border-slate-800 pt-3">
        <div className="text-[10px] font-bold uppercase tracking-wider text-slate-400 mb-2">
          Reproducibility (JSON Config)
        </div>
        <div className="grid grid-cols-2 gap-2">
          <button
            type="button"
            data-testid="export-json-btn"
            onClick={handleExportJSON}
            className="flex items-center justify-center gap-1.5 rounded-lg bg-slate-950 hover:bg-slate-800 border border-slate-700 py-1.5 px-2.5 text-xs font-medium text-slate-200 transition cursor-pointer"
          >
            <Download className="w-3.5 h-3.5 text-indigo-400" />
            Export JSON
          </button>

          <button
            type="button"
            data-testid="import-json-btn"
            onClick={() => fileInputRef.current?.click()}
            className="flex items-center justify-center gap-1.5 rounded-lg bg-slate-950 hover:bg-slate-800 border border-slate-700 py-1.5 px-2.5 text-xs font-medium text-slate-200 transition cursor-pointer"
          >
            <Upload className="w-3.5 h-3.5 text-emerald-400" />
            Import JSON
          </button>
          <input
            ref={fileInputRef}
            data-testid="import-json-input"
            type="file"
            accept="application/json,.json"
            onChange={handleImportFile}
            className="hidden"
          />
        </div>
        {importError && (
          <div
            data-testid="import-error-msg"
            className="mt-2 rounded bg-rose-950/80 border border-rose-500/60 px-2.5 py-1.5 text-[11px] text-rose-200"
          >
            {importError}
          </div>
        )}
      </div>

      {/* 7. Run All 5 Strategies Comparison Action */}
      <button
        type="button"
        data-testid="run-simulation-btn"
        disabled={isLoading}
        onClick={onRunSimulation}
        className="w-full flex items-center justify-center gap-2 rounded-xl bg-indigo-600 hover:bg-indigo-500 disabled:bg-indigo-800 text-white font-bold py-2.5 px-4 text-sm shadow-lg shadow-indigo-950/60 transition cursor-pointer"
      >
        {isLoading ? (
          <>
            <RefreshCw className="w-4 h-4 animate-spin" />
            Comparing 5 Strategies...
          </>
        ) : (
          <>
            <Play className="w-4 h-4 fill-current" />
            Run & Compare All 5 Strategies
          </>
        )}
      </button>
    </div>
  );
};
