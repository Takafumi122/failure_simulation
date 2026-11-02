import React from 'react';
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import {
  AlertTriangle,
  ArrowDownRight,
  ArrowUpRight,
  CheckCircle2,
  GitCommit,
  Info,
  Sparkles,
} from 'lucide-react';
import { BreakerTransition, StrategyResult } from '../types/simulation';

interface StrategyComparisonProps {
  results: StrategyResult[];
  activeStrategyId: string;
  onSelectStrategy: (strategyId: string) => void;
  durationSec: number;
}

const STRATEGY_COLORS: Record<string, string> = {
  baseline: '#94a3b8',
  retry: '#38bdf8',
  circuit_breaker: '#a78bfa',
  retry_circuit_breaker: '#34d399',
  fail_fast: '#fbbf24',
};

const STRATEGY_SHORT_NAMES: Record<string, string> = {
  baseline: 'Baseline',
  retry: 'Retry',
  circuit_breaker: 'Circuit Breaker',
  retry_circuit_breaker: 'Retry + CB',
  fail_fast: 'Fail-Fast',
};

function formatDeltaPct(val: number, baseVal: number, invertGood = false) {
  const diff = (val - baseVal) * 100;
  if (Math.abs(diff) < 0.05) {
    return <span className="text-[10px] text-slate-500 font-mono">0.0%</span>;
  }
  const isPositive = diff > 0;
  const isGood = invertGood ? !isPositive : isPositive;
  return (
    <span
      className={`inline-flex items-center text-[10px] font-mono font-semibold ${
        isGood ? 'text-emerald-400' : 'text-rose-400'
      }`}
    >
      {isPositive ? (
        <ArrowUpRight className="w-3 h-3" />
      ) : (
        <ArrowDownRight className="w-3 h-3" />
      )}
      {isPositive ? '+' : ''}
      {diff.toFixed(1)}%
    </span>
  );
}

function formatDeltaNum(val: number, baseVal: number, unit = '', invertGood = true) {
  const diff = val - baseVal;
  if (Math.abs(diff) < 0.1) {
    return <span className="text-[10px] text-slate-500 font-mono">0{unit}</span>;
  }
  const isPositive = diff > 0;
  const isGood = invertGood ? !isPositive : isPositive;
  return (
    <span
      className={`inline-flex items-center text-[10px] font-mono font-semibold ${
        isGood ? 'text-emerald-400' : 'text-rose-400'
      }`}
    >
      {isPositive ? '+' : ''}
      {Number.isInteger(diff) ? diff : diff.toFixed(1)}
      {unit}
    </span>
  );
}

// Compute factual, deterministic observations strictly from the calculated StrategyResult metrics.
function computeMeasuredFacts(results: StrategyResult[]): string[] {
  const byId = Object.fromEntries(results.map((r) => [r.strategyId, r]));
  const base = byId['baseline'];
  const retry = byId['retry'];
  const cb = byId['circuit_breaker'];
  const retryCb = byId['retry_circuit_breaker'];
  const failFast = byId['fail_fast'];

  const facts: string[] = [];
  if (!base) return facts;
  const bOv = base.metrics.overall;

  if (retry) {
    const rOv = retry.metrics.overall;
    const deltaSuccPct = (rOv.successRate - bOv.successRate) * 100;
    if (deltaSuccPct > 1.0) {
      facts.push(
        `Retry improved logical request success rate from ${(bOv.successRate * 100).toFixed(1)}% to ${(rOv.successRate * 100).toFixed(1)}% (+${deltaSuccPct.toFixed(1)}%) by executing ${rOv.totalRetries.toLocaleString()} retries (${rOv.retryAmplificationFactor.toFixed(2)}x call amplification).`
      );
    } else if (rOv.totalRetries > 0 && Math.abs(deltaSuccPct) <= 1.0) {
      facts.push(
        `Retry executed ${rOv.totalRetries.toLocaleString()} extra attempts (${rOv.retryAmplificationFactor.toFixed(2)}x call amplification) without improving logical success rate (${(rOv.successRate * 100).toFixed(1)}% vs. ${(bOv.successRate * 100).toFixed(1)}% Baseline).`
      );
    }
    if (rOv.maxQueueLength > bOv.maxQueueLength || rOv.timeoutCount > bOv.timeoutCount * 1.2) {
      facts.push(
        `Retry increased peak queue length from ${bOv.maxQueueLength} to ${rOv.maxQueueLength} and total timeouts from ${bOv.timeoutCount} to ${rOv.timeoutCount}.`
      );
    }
  }

  if (cb) {
    const cbOv = cb.metrics.overall;
    if (cbOv.circuitBreakerRejections > 0) {
      const savedCalls = bOv.targetEdgeCalls - cbOv.targetEdgeCalls;
      facts.push(
        `Circuit Breaker fast-rejected ${cbOv.circuitBreakerRejections.toLocaleString()} calls while OPEN, reducing target downstream calls by ${savedCalls.toLocaleString()} (${bOv.targetEdgeCalls.toLocaleString()} → ${cbOv.targetEdgeCalls.toLocaleString()}).`
      );
    }
  }

  if (retry && retryCb) {
    const rOv = retry.metrics.overall;
    const rcOv = retryCb.metrics.overall;
    if (rOv.totalRetries > rcOv.totalRetries && rcOv.circuitBreakerRejections > 0) {
      facts.push(
        `Combining Retry + Circuit Breaker suppressed ${rOv.totalRetries - rcOv.totalRetries} futile retries while the breaker was OPEN (retries reduced from ${rOv.totalRetries} to ${rcOv.totalRetries}).`
      );
    }
  }

  if (failFast) {
    const ffOv = failFast.metrics.overall;
    if (ffOv.failureP95LatencyMs > 0 && bOv.failureP95LatencyMs > 0 && ffOv.failureP95LatencyMs < bOv.failureP95LatencyMs - 20) {
      facts.push(
        `Fail-Fast reduced failed-request p95 latency from ${bOv.failureP95LatencyMs.toFixed(1)}ms to ${ffOv.failureP95LatencyMs.toFixed(1)}ms (overall p95: ${bOv.p95LatencyMs.toFixed(1)}ms → ${ffOv.p95LatencyMs.toFixed(1)}ms).`
      );
    }
  }

  if (facts.length === 0) {
    facts.push(
      `All ${results.length} strategies completed ${bOv.completedRequests.toLocaleString()} logical requests with ${(bOv.successRate * 100).toFixed(1)}% success rate and ${bOv.p95LatencyMs.toFixed(1)}ms p95 latency.`
    );
  }

  return facts;
}

export const StrategyComparison: React.FC<StrategyComparisonProps> = ({
  results,
  activeStrategyId,
  onSelectStrategy,
  durationSec,
}) => {
  if (!results || results.length === 0) return null;

  const baseline = results.find((r) => r.strategyId === 'baseline') || results[0];
  const baseOv = baseline.metrics.overall;
  const anyTruncated = results.some((r) => r.executionMeta.truncated);
  const measuredFacts = computeMeasuredFacts(results);

  const chartData = results.map((r) => {
    const ov = r.metrics.overall;
    return {
      strategyId: r.strategyId,
      name: STRATEGY_SHORT_NAMES[r.strategyId] || r.strategyId,
      successRatePct: Number((ov.successRate * 100).toFixed(1)),
      p95LatencyMs: ov.p95LatencyMs,
      downstreamCalls: ov.totalDownstreamCalls,
      targetEdgeCalls: ov.targetEdgeCalls,
      retries: ov.totalRetries,
      cbRejections: ov.circuitBreakerRejections,
      color: STRATEGY_COLORS[r.strategyId] || '#818cf8',
    };
  });

  const activeResult =
    results.find((r) => r.strategyId === activeStrategyId) || results[0];

  const cbStrategies = results.filter(
    (r) =>
      r.strategyId === 'circuit_breaker' ||
      r.strategyId === 'retry_circuit_breaker' ||
      (r.breakerTransitions && r.breakerTransitions.length > 0)
  );

  return (
    <div className="space-y-4">
      {/* Explicit Truncated Warning Banner */}
      {anyTruncated && (
        <div
          data-testid="truncated-warning-banner"
          className="rounded-xl bg-amber-950/80 border-2 border-amber-500 p-3.5 flex items-start gap-3 text-xs text-amber-100"
        >
          <AlertTriangle className="w-5 h-5 text-amber-400 shrink-0 mt-0.5" />
          <div>
            <div className="font-bold uppercase tracking-wider text-amber-300">
              Simulation Truncated by Safety Event Limit
            </div>
            <p className="mt-0.5 text-amber-200/90">
              One or more strategies reached the maximum discrete event limit before all logical requests completed. Unfinished requests are counted as failures so truncated runs are never mistaken for healthy completions.
            </p>
          </div>
        </div>
      )}

      {/* 1. Strategy Selector Tabs & Comparison Table */}
      <div className="rounded-xl bg-slate-900/90 border border-slate-800 p-4">
        <div className="flex flex-wrap items-center justify-between gap-3 mb-3">
          <div>
            <h2 className="text-xs font-bold uppercase tracking-wider text-indigo-300 flex items-center gap-1.5">
              <Sparkles className="w-4 h-4 text-indigo-400" />
              Resilience Strategy Comparison (Same Traffic, Fault & Coordinate RNG Seed)
            </h2>
            <p className="text-[11px] text-slate-400 mt-0.5">
              Click any strategy row or pill to inspect its topology replay, time-series charts, and discrete event stream below.
            </p>
          </div>

          {/* Strategy Quick Pills */}
          <div className="flex flex-wrap items-center gap-1.5" data-testid="strategy-selector-pills">
            {results.map((r) => {
              const isSelected = r.strategyId === activeStrategyId;
              return (
                <button
                  key={r.strategyId}
                  type="button"
                  data-testid={`strategy-pill-${r.strategyId}`}
                  onClick={() => onSelectStrategy(r.strategyId)}
                  className={`px-3 py-1.5 rounded-lg text-xs font-bold transition cursor-pointer flex items-center gap-1.5 border ${
                    isSelected
                      ? 'bg-indigo-600 border-indigo-400 text-white shadow-md shadow-indigo-950/60'
                      : 'bg-slate-950 border-slate-800 text-slate-300 hover:bg-slate-800'
                  }`}
                >
                  <span
                    className="w-2 h-2 rounded-full"
                    style={{
                      backgroundColor: STRATEGY_COLORS[r.strategyId] || '#818cf8',
                    }}
                  />
                  {STRATEGY_SHORT_NAMES[r.strategyId] || r.strategyId}
                </button>
              );
            })}
          </div>
        </div>

        {/* Comparison Table with Deltas vs Baseline */}
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse text-xs" data-testid="strategy-comparison-table">
            <thead>
              <tr className="border-b border-slate-800 text-[11px] text-slate-400">
                <th className="py-2 pr-3">Strategy</th>
                <th className="py-2 px-2 text-right">Logical Success</th>
                <th className="py-2 px-2 text-right">Error Rate</th>
                <th className="py-2 px-2 text-right">p50 / p95 / p99 (ms)</th>
                <th className="py-2 px-2 text-right">Succ / Fail p95</th>
                <th className="py-2 px-2 text-right">Total Calls</th>
                <th className="py-2 px-2 text-right">Retries (Amp)</th>
                <th className="py-2 px-2 text-right">CB Rejects</th>
                <th className="py-2 px-2 text-right">Timeouts</th>
                <th className="py-2 pl-2 text-right">Max Worker / Queue</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/70 font-mono">
              {results.map((r) => {
                const ov = r.metrics.overall;
                const isSelected = r.strategyId === activeStrategyId;
                const isBase = r.strategyId === 'baseline';
                return (
                  <tr
                    key={r.strategyId}
                    data-testid={`strategy-row-${r.strategyId}`}
                    onClick={() => onSelectStrategy(r.strategyId)}
                    className={`cursor-pointer transition ${
                      isSelected
                        ? 'bg-indigo-950/40 hover:bg-indigo-950/60'
                        : 'hover:bg-slate-800/40'
                    }`}
                  >
                    <td className="py-2.5 pr-3 font-sans">
                      <div className="flex items-center gap-2">
                        <span
                          className="w-2.5 h-2.5 rounded-full shrink-0"
                          style={{
                            backgroundColor:
                              STRATEGY_COLORS[r.strategyId] || '#818cf8',
                          }}
                        />
                        <div>
                          <div className="font-bold text-slate-100 flex items-center gap-1.5">
                            {STRATEGY_SHORT_NAMES[r.strategyId] || r.strategyId}
                            {isSelected && (
                              <span className="text-[9px] bg-indigo-500/30 border border-indigo-400/50 text-indigo-200 px-1.5 py-0.2 rounded uppercase">
                                Active
                              </span>
                            )}
                            {r.executionMeta.truncated && (
                              <span className="text-[9px] bg-amber-500/30 border border-amber-400 text-amber-200 px-1.5 py-0.2 rounded">
                                TRUNCATED
                              </span>
                            )}
                          </div>
                          <div className="text-[10px] text-slate-400">
                            {r.strategyName}
                          </div>
                        </div>
                      </div>
                    </td>

                    <td className="py-2.5 px-2 text-right">
                      <div
                        className={`font-bold ${
                          ov.successRate >= 0.95
                            ? 'text-emerald-400'
                            : ov.successRate >= 0.7
                              ? 'text-amber-300'
                              : 'text-rose-400'
                        }`}
                      >
                        {(ov.successRate * 100).toFixed(1)}%
                      </div>
                      {!isBase && (
                        <div>{formatDeltaPct(ov.successRate, baseOv.successRate, false)}</div>
                      )}
                    </td>

                    <td className="py-2.5 px-2 text-right">
                      <div className="text-slate-200">
                        {(ov.errorRate * 100).toFixed(1)}%
                      </div>
                      {!isBase && (
                        <div>{formatDeltaPct(ov.errorRate, baseOv.errorRate, true)}</div>
                      )}
                    </td>

                    <td className="py-2.5 px-2 text-right">
                      <div className="text-slate-200">
                        <span className="text-slate-400">{ov.p50LatencyMs.toFixed(0)}</span> /{' '}
                        <strong className="text-indigo-300">
                          {ov.p95LatencyMs.toFixed(1)}
                        </strong>{' '}
                        / <span className="text-slate-400">{ov.p99LatencyMs.toFixed(0)}</span>
                      </div>
                      {!isBase && (
                        <div>
                          {formatDeltaNum(ov.p95LatencyMs, baseOv.p95LatencyMs, 'ms', true)}
                        </div>
                      )}
                    </td>

                    <td className="py-2.5 px-2 text-right">
                      <div className="text-[11px]">
                        <span className="text-emerald-400" title="Successful requests p95">
                          {ov.successCount > 0 ? `${ov.successP95LatencyMs.toFixed(0)}ms` : '—'}
                        </span>
                        {' / '}
                        <span className="text-rose-300" title="Failed requests p95">
                          {ov.failureCount > 0 ? `${ov.failureP95LatencyMs.toFixed(0)}ms` : '—'}
                        </span>
                      </div>
                    </td>

                    <td className="py-2.5 px-2 text-right">
                      <div className="text-slate-200">
                        {ov.totalDownstreamCalls.toLocaleString()}
                      </div>
                      {!isBase && (
                        <div>
                          {formatDeltaNum(
                            ov.totalDownstreamCalls,
                            baseOv.totalDownstreamCalls,
                            '',
                            true
                          )}
                        </div>
                      )}
                    </td>

                    <td className="py-2.5 px-2 text-right">
                      <div className="text-sky-300 font-semibold">
                        {ov.totalRetries.toLocaleString()}{' '}
                        <span className="text-[10px] text-slate-400">
                          ({ov.retryAmplificationFactor.toFixed(2)}x)
                        </span>
                      </div>
                    </td>

                    <td className="py-2.5 px-2 text-right">
                      <span
                        className={
                          ov.circuitBreakerRejections > 0
                            ? 'text-violet-300 font-bold'
                            : 'text-slate-500'
                        }
                      >
                        {ov.circuitBreakerRejections.toLocaleString()}
                      </span>
                    </td>

                    <td className="py-2.5 px-2 text-right">
                      <div className="text-amber-300">
                        {ov.timeoutCount.toLocaleString()}
                      </div>
                      {!isBase && (
                        <div>
                          {formatDeltaNum(ov.timeoutCount, baseOv.timeoutCount, '', true)}
                        </div>
                      )}
                    </td>

                    <td className="py-2.5 pl-2 text-right">
                      <div className="text-slate-200">
                        {(ov.maxWorkerUtilization * 100).toFixed(0)}% /{' '}
                        <span
                          className={
                            ov.maxQueueLength > 10 ? 'text-rose-300 font-bold' : 'text-slate-300'
                          }
                        >
                          q={ov.maxQueueLength}
                        </span>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>

        {/* Factual Observations Derived Strictly from Measured Metrics */}
        <div
          className="mt-3.5 rounded-lg bg-slate-950/90 border border-slate-800/90 p-3"
          data-testid="measured-observations"
        >
          <div className="flex items-center gap-1.5 text-[11px] font-bold uppercase tracking-wider text-indigo-300 mb-1.5">
            <Info className="w-3.5 h-3.5 text-indigo-400" />
            Key Observations from Measured Simulation Data
          </div>
          <ul className="space-y-1 text-xs text-slate-300 list-disc list-inside">
            {measuredFacts.map((fact, idx) => (
              <li key={idx} className="leading-relaxed">
                {fact}
              </li>
            ))}
          </ul>
        </div>
      </div>

      {/* 2. Side-by-Side Comparison Charts (Success/p95 + Downstream Calls/Retries/CB Rejects) */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        {/* Chart A: Success Rate (%) vs Strategy */}
        <div className="rounded-xl bg-slate-900/90 border border-slate-800 p-4">
          <h3 className="text-xs font-bold uppercase tracking-wider text-slate-200 mb-1">
            Logical Success Rate & p95 Latency
          </h3>
          <p className="text-[11px] text-slate-400 mb-3">
            Comparison across all 5 resilience strategies
          </p>
          <div className="h-48 w-full">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" />
                <XAxis dataKey="name" stroke="#94a3b8" fontSize={10} />
                <YAxis
                  yAxisId="left"
                  domain={[0, 100]}
                  stroke="#10b981"
                  fontSize={10}
                  tickFormatter={(v) => `${v}%`}
                />
                <YAxis
                  yAxisId="right"
                  orientation="right"
                  stroke="#818cf8"
                  fontSize={10}
                  tickFormatter={(v) => `${v}ms`}
                />
                <Tooltip
                  contentStyle={{
                    backgroundColor: '#0f172a',
                    borderColor: '#334155',
                    fontSize: 12,
                  }}
                />
                <Legend wrapperStyle={{ fontSize: 11 }} />
                <Bar
                  yAxisId="left"
                  dataKey="successRatePct"
                  name="Success Rate (%)"
                  fill="#10b981"
                  radius={[4, 4, 0, 0]}
                />
                <Bar
                  yAxisId="right"
                  dataKey="p95LatencyMs"
                  name="p95 Latency (ms)"
                  fill="#818cf8"
                  radius={[4, 4, 0, 0]}
                />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>

        {/* Chart B: Total Downstream Calls & Retries & CB Rejections */}
        <div className="rounded-xl bg-slate-900/90 border border-slate-800 p-4">
          <h3 className="text-xs font-bold uppercase tracking-wider text-slate-200 mb-1">
            Downstream Load, Retries & CB Shedding
          </h3>
          <p className="text-[11px] text-slate-400 mb-3">
            Target edge calls vs. retry amplification vs. CB fast-rejects
          </p>
          <div className="h-48 w-full">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" />
                <XAxis dataKey="name" stroke="#94a3b8" fontSize={10} />
                <YAxis stroke="#94a3b8" fontSize={10} />
                <Tooltip
                  contentStyle={{
                    backgroundColor: '#0f172a',
                    borderColor: '#334155',
                    fontSize: 12,
                  }}
                />
                <Legend wrapperStyle={{ fontSize: 11 }} />
                <Bar
                  dataKey="targetEdgeCalls"
                  name="Target Callee Calls"
                  fill="#38bdf8"
                  radius={[4, 4, 0, 0]}
                >
                  {chartData.map((entry, idx) => (
                    <Cell key={idx} fill={entry.color} />
                  ))}
                </Bar>
                <Bar
                  dataKey="retries"
                  name="Retries"
                  fill="#f43f5e"
                  radius={[4, 4, 0, 0]}
                />
                <Bar
                  dataKey="cbRejections"
                  name="CB Blocked"
                  fill="#a78bfa"
                  radius={[4, 4, 0, 0]}
                />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>

        {/* Panel C: Circuit Breaker State Transition Timeline */}
        <div className="rounded-xl bg-slate-900/90 border border-slate-800 p-4 flex flex-col">
          <div className="flex items-center justify-between mb-2">
            <div>
              <h3 className="text-xs font-bold uppercase tracking-wider text-slate-200 flex items-center gap-1.5">
                <GitCommit className="w-4 h-4 text-violet-400" />
                Circuit Breaker State Timeline
              </h3>
              <p className="text-[11px] text-slate-400">
                CLOSED → OPEN → HALF_OPEN → CLOSED transitions
              </p>
            </div>
            <span className="text-[10px] font-mono text-violet-300 bg-violet-500/15 border border-violet-500/30 px-2 py-0.5 rounded">
              {activeResult.breakerTransitions?.length || 0} transitions (active)
            </span>
          </div>

          {/* Per-second visual state bar for CB strategies */}
          <div className="space-y-2 mb-2.5">
            {cbStrategies.map((cbs) => {
              const targetCallee = 'feature-db';
              return (
                <div key={cbs.strategyId} className="space-y-1">
                  <div className="flex justify-between text-[10px] text-slate-300 font-semibold">
                    <span>{STRATEGY_SHORT_NAMES[cbs.strategyId] || cbs.strategyId}</span>
                    <span className="font-mono text-slate-400">
                      0s – {durationSec}s
                    </span>
                  </div>
                  <div className="flex w-full h-3 rounded overflow-hidden border border-slate-800 bg-slate-950">
                    {cbs.timeSeries.map((pt) => {
                      const st =
                        pt.services[targetCallee]?.circuitState || 'CLOSED';
                      const bg =
                        st === 'OPEN'
                          ? 'bg-rose-500'
                          : st === 'HALF_OPEN'
                            ? 'bg-amber-400'
                            : 'bg-emerald-500/70';
                      return (
                        <div
                          key={pt.second}
                          title={`t=${pt.second}s: ${st}`}
                          className={`flex-1 ${bg} border-r border-slate-900/40`}
                        />
                      );
                    })}
                  </div>
                </div>
              );
            })}
            <div className="flex items-center gap-3 text-[10px] text-slate-400 pt-0.5">
              <span className="inline-flex items-center gap-1">
                <span className="w-2 h-2 rounded-sm bg-emerald-500/70" /> CLOSED
              </span>
              <span className="inline-flex items-center gap-1">
                <span className="w-2 h-2 rounded-sm bg-rose-500" /> OPEN (Fast-Reject)
              </span>
              <span className="inline-flex items-center gap-1">
                <span className="w-2 h-2 rounded-sm bg-amber-400" /> HALF_OPEN (Probing)
              </span>
            </div>
          </div>

          {/* Transition Log List */}
          <div className="flex-1 max-h-28 overflow-y-auto rounded-lg bg-slate-950 border border-slate-800/80 p-2 font-mono text-[10px] space-y-1">
            {cbStrategies.every(
              (s) => !s.breakerTransitions || s.breakerTransitions.length === 0
            ) ? (
              <div className="h-full flex items-center justify-center text-slate-500 py-4">
                <CheckCircle2 className="w-3.5 h-3.5 mr-1.5 text-emerald-500" />
                Circuit Breaker remained CLOSED throughout simulation.
              </div>
            ) : (
              cbStrategies.map((s) =>
                (s.breakerTransitions || []).map((tr: BreakerTransition) => (
                  <div
                    key={`${s.strategyId}-${tr.sequenceId}`}
                    className="flex items-start gap-1.5 py-0.5 border-b border-slate-900"
                  >
                    <span className="text-indigo-300 font-bold min-w-[58px]">
                      [{STRATEGY_SHORT_NAMES[s.strategyId]}]
                    </span>
                    <span className="text-slate-400 min-w-[46px]">
                      {(tr.timestampMs / 1000).toFixed(2)}s
                    </span>
                    <span
                      className={`font-bold ${
                        tr.toState === 'OPEN'
                          ? 'text-rose-400'
                          : tr.toState === 'HALF_OPEN'
                            ? 'text-amber-300'
                            : 'text-emerald-400'
                      }`}
                    >
                      {tr.fromState}→{tr.toState}
                    </span>
                    <span className="text-slate-400 truncate" title={tr.reason}>
                      {tr.reason}
                    </span>
                  </div>
                ))
              )
            )}
          </div>
        </div>
      </div>
    </div>
  );
};
