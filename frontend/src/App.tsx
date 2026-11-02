import React, { useCallback, useEffect, useState } from 'react';
import { Activity, AlertCircle, Cpu, GitBranch, Hash } from 'lucide-react';
import { fetchScenarios, runSimulationCompare } from './api/client';
import {
  CompareResponse,
  ScenarioPreset,
  SimulationRequest,
  Topology,
} from './types/simulation';
import { ScenarioGallery } from './components/ScenarioGallery';
import { ControlPanel } from './components/ControlPanel';
import { StrategyComparison } from './components/StrategyComparison';
import { TopologyFlow } from './components/TopologyFlow';
import { PlaybackBar } from './components/PlaybackBar';
import { MetricsCards } from './components/MetricsCards';
import { ChartsPanel } from './components/ChartsPanel';
import { EventLogView } from './components/EventLogView';

const ALL_STRATEGIES = [
  'baseline',
  'retry',
  'circuit_breaker',
  'retry_circuit_breaker',
  'fail_fast',
];

export const App: React.FC = () => {
  const [topology, setTopology] = useState<Topology | null>(null);
  const [scenarios, setScenarios] = useState<ScenarioPreset[]>([]);
  const [selectedScenarioId, setSelectedScenarioId] = useState<string>('');
  const [config, setConfig] = useState<SimulationRequest | null>(null);
  const [compareResp, setCompareResp] = useState<CompareResponse | null>(null);
  const [activeStrategyId, setActiveStrategyId] = useState<string>('baseline');

  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  // Playback state
  const [mode, setMode] = useState<'summary' | 'playback'>('summary');
  const [currentSecond, setCurrentSecond] = useState<number>(0);
  const [isPlaying, setIsPlaying] = useState<boolean>(false);
  const [speed, setSpeed] = useState<1 | 2 | 4>(1);

  const executeSimulation = useCallback(async (reqToRun: SimulationRequest) => {
    setIsLoading(true);
    setErrorMsg(null);
    setIsPlaying(false);
    try {
      const payload: SimulationRequest = {
        ...reqToRun,
        strategies: ALL_STRATEGIES,
      };
      const resp = await runSimulationCompare(payload);
      setCompareResp(resp);
      setCurrentSecond((prev) =>
        prev >= reqToRun.durationSec ? 0 : prev
      );
    } catch (err) {
      setErrorMsg(err instanceof Error ? err.message : String(err));
    } finally {
      setIsLoading(false);
    }
  }, []);

  // Load initial scenarios and run the first preset automatically
  useEffect(() => {
    let mounted = true;
    (async () => {
      try {
        const data = await fetchScenarios();
        if (!mounted) return;
        setTopology(data.topology);
        setScenarios(data.scenarios);
        if (data.scenarios.length > 0) {
          const first = data.scenarios[0];
          setSelectedScenarioId(first.id);
          setConfig(first.config);
          await executeSimulation(first.config);
        }
      } catch (err) {
        if (!mounted) return;
        setErrorMsg(
          err instanceof Error
            ? `Backend API connection failed: ${err.message}`
            : 'Failed to connect to Go simulation server.'
        );
      }
    })();
    return () => {
      mounted = false;
    };
  }, [executeSimulation]);

  // Playback interval timer
  useEffect(() => {
    if (!isPlaying || !compareResp) return;
    const duration = compareResp.request.durationSec;
    const intervalMs = Math.round(700 / speed);

    const timer = window.setInterval(() => {
      setCurrentSecond((prev) => {
        if (prev + 1 >= duration) {
          setIsPlaying(false);
          return duration - 1;
        }
        return prev + 1;
      });
    }, intervalMs);

    return () => window.clearInterval(timer);
  }, [isPlaying, speed, compareResp]);

  const handleSelectScenario = (preset: ScenarioPreset) => {
    setSelectedScenarioId(preset.id);
    const cloned: SimulationRequest = JSON.parse(JSON.stringify(preset.config));
    setConfig(cloned);
    setMode('summary');
    setCurrentSecond(0);
    executeSimulation(cloned);
  };

  const handleImportConfig = (imported: SimulationRequest) => {
    setSelectedScenarioId(imported.scenarioId || '');
    setConfig(imported);
    setMode('summary');
    setCurrentSecond(0);
    executeSimulation(imported);
  };

  const results = compareResp?.results || [];
  const activeResult =
    results.find((r) => r.strategyId === activeStrategyId) ||
    results[0] ||
    null;
  const engineVer =
    compareResp?.engineVersion ||
    activeResult?.executionMeta.engineVersion ||
    'v3.0.0-des';

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col">
      {/* Top Navigation Bar */}
      <header className="border-b border-slate-800/90 bg-slate-900/80 backdrop-blur sticky top-0 z-30">
        <div className="max-w-[1600px] mx-auto px-4 sm:px-6 py-3 flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-xl bg-indigo-600/20 border border-indigo-500/40 text-indigo-400">
              <Activity className="w-5 h-5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h1 className="text-base font-extrabold tracking-tight text-white">
                  FailureLab
                </h1>
                <span
                  data-testid="engine-version-badge"
                  className="rounded-md bg-indigo-500/20 border border-indigo-500/30 px-2 py-0.5 text-[10px] font-mono font-bold text-indigo-300"
                >
                  Engine {engineVer}
                </span>
              </div>
              <p className="text-xs text-slate-400">
                Distributed Systems Cascading Failure & Resilience Strategy Simulator (Retry / Circuit Breaker / Fail-Fast)
              </p>
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-2 text-xs text-slate-400">
            <span className="hidden md:inline-flex items-center gap-1.5 rounded-lg bg-slate-900 border border-slate-800 px-2.5 py-1">
              <Cpu className="w-3.5 h-3.5 text-emerald-400" />
              Go <code className="text-slate-200 font-mono">container/heap</code> + Coordinate RNG
            </span>
            {compareResp && (
              <span
                data-testid="reproducibility-meta-badge"
                className="inline-flex items-center gap-1.5 rounded-lg bg-slate-900 border border-slate-800 px-2.5 py-1 font-mono text-[11px] text-slate-300"
              >
                <Hash className="w-3 h-3 text-indigo-400" />
                seed={compareResp.request.seed} | {compareResp.request.requestsPerSecond} RPS × {compareResp.request.durationSec}s
              </span>
            )}
            <span className="hidden sm:inline-flex items-center gap-1.5 rounded-lg bg-slate-900 border border-slate-800 px-2.5 py-1">
              <GitBranch className="w-3.5 h-3.5 text-indigo-400" />
              <strong className="text-indigo-300">5 Strategies</strong>
            </span>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="flex-1 max-w-[1600px] w-full mx-auto px-4 sm:px-6 py-5">
        {errorMsg && (
          <div
            data-testid="simulation-error-banner"
            className="mb-4 rounded-xl bg-rose-950/70 border border-rose-500/60 p-3.5 flex items-center gap-3 text-xs text-rose-200"
          >
            <AlertCircle className="w-5 h-5 text-rose-400 shrink-0" />
            <div>
              <span className="font-bold">Simulation Error: </span>
              {errorMsg}
            </div>
          </div>
        )}

        {config && topology ? (
          <>
            {/* Interactive 5-Scenario Gallery */}
            <ScenarioGallery
              scenarios={scenarios}
              selectedScenarioId={selectedScenarioId}
              onSelectScenario={handleSelectScenario}
              isLoading={isLoading}
            />

            <div className="grid grid-cols-1 lg:grid-cols-12 gap-5 items-start">
              {/* Left Sidebar: Control Panel (3 cols on XL screens) */}
              <div className="lg:col-span-4 xl:col-span-3">
                <ControlPanel
                  scenarios={scenarios}
                  selectedScenarioId={selectedScenarioId}
                  onSelectScenario={handleSelectScenario}
                  config={config}
                  onChangeConfig={setConfig}
                  onRunSimulation={() => executeSimulation(config)}
                  onImportConfig={handleImportConfig}
                  isLoading={isLoading}
                />
              </div>

              {/* Right Workspace: 5-Strategy Comparison, Topology, Playback, KPI Cards, Charts, Event Logs */}
              <div className="lg:col-span-8 xl:col-span-9 space-y-4">
                <StrategyComparison
                  results={results}
                  activeStrategyId={activeResult?.strategyId || activeStrategyId}
                  onSelectStrategy={(id) => setActiveStrategyId(id)}
                  durationSec={compareResp?.request.durationSec || config.durationSec}
                />

                <TopologyFlow
                  topology={topology}
                  config={config}
                  result={activeResult}
                  mode={mode}
                  currentSecond={currentSecond}
                />

                <PlaybackBar
                  mode={mode}
                  setMode={setMode}
                  currentSecond={currentSecond}
                  setCurrentSecond={setCurrentSecond}
                  durationSec={compareResp?.request.durationSec || config.durationSec}
                  isPlaying={isPlaying}
                  setIsPlaying={setIsPlaying}
                  speed={speed}
                  setSpeed={setSpeed}
                  fault={compareResp?.request.fault || config.fault}
                />

                <MetricsCards
                  result={activeResult}
                  mode={mode}
                  currentSecond={currentSecond}
                />

                <ChartsPanel
                  result={activeResult}
                  fault={compareResp?.request.fault || config.fault}
                  mode={mode}
                  currentSecond={currentSecond}
                  onSelectSecond={(sec) => {
                    setIsPlaying(false);
                    setMode('playback');
                    setCurrentSecond(sec);
                  }}
                />

                <EventLogView
                  result={activeResult}
                  mode={mode}
                  currentSecond={currentSecond}
                />
              </div>
            </div>
          </>
        ) : (
          <div className="h-96 flex items-center justify-center text-sm text-slate-400">
            Loading FailureLab topology and scenarios...
          </div>
        )}
      </main>
    </div>
  );
};

export default App;
