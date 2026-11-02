import React from 'react';
import {
  Activity,
  AlertOctagon,
  CheckCircle2,
  Clock,
  Layers,
  Repeat,
  ShieldAlert,
} from 'lucide-react';
import { StrategyResult } from '../types/simulation';

interface MetricsCardsProps {
  result: StrategyResult | null;
  mode: 'summary' | 'playback';
  currentSecond: number;
}

export const MetricsCards: React.FC<MetricsCardsProps> = ({
  result,
  mode,
  currentSecond,
}) => {
  if (!result) return null;

  const ov = result.metrics.overall;
  const secPt =
    mode === 'playback' && result.timeSeries[currentSecond]
      ? result.timeSeries[currentSecond]
      : null;

  const successRate = secPt ? secPt.successRate : ov.successRate;
  const errorRate = secPt ? secPt.errorRate : ov.errorRate;
  const p95Latency = secPt ? secPt.p95LatencyMs : ov.p95LatencyMs;
  const avgLatency = secPt ? secPt.avgLatencyMs : ov.avgLatencyMs;
  const totalRequests = secPt ? secPt.totalRequests : ov.completedRequests;
  const successCount = secPt ? secPt.successCount : ov.successCount;
  const failureCount = secPt ? secPt.failureCount : ov.failureCount;
  const timeoutCount = secPt ? secPt.timeoutCount : ov.timeoutCount;
  const queueOverflowCount = secPt ? secPt.queueOverflowCount : ov.queueOverflowCount;
  const retryCount = secPt ? secPt.retryCount : ov.totalRetries;
  const cbRejects = secPt
    ? secPt.circuitBreakerRejections
    : ov.circuitBreakerRejections;

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-3.5">
      {/* 1. Logical Success Rate Card */}
      <div className="rounded-xl bg-slate-900/90 border border-slate-800 p-4 flex flex-col justify-between">
        <div className="flex items-center justify-between">
          <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">
            Logical Success Rate {mode === 'playback' && `(t=${currentSecond}s)`}
          </span>
          <CheckCircle2
            className={`w-4 h-4 ${
              successRate >= 0.95
                ? 'text-emerald-400'
                : successRate >= 0.7
                  ? 'text-amber-400'
                  : 'text-rose-400'
            }`}
          />
        </div>
        <div className="mt-2 flex items-baseline gap-2">
          <span
            className={`text-2xl font-mono font-extrabold ${
              successRate >= 0.95
                ? 'text-emerald-400'
                : successRate >= 0.7
                  ? 'text-amber-400'
                  : 'text-rose-400'
            }`}
          >
            {(successRate * 100).toFixed(1)}%
          </span>
          <span className="text-xs text-slate-400 font-mono">
            ({successCount.toLocaleString()} / {totalRequests.toLocaleString()} req)
          </span>
        </div>
        <div className="mt-2 w-full h-1.5 bg-slate-800 rounded-full overflow-hidden">
          <div
            className={`h-full transition-all duration-200 ${
              successRate >= 0.95
                ? 'bg-emerald-500'
                : successRate >= 0.7
                  ? 'bg-amber-500'
                  : 'bg-rose-500'
            }`}
            style={{ width: `${Math.min(100, successRate * 100)}%` }}
          />
        </div>
      </div>

      {/* 2. Error Rate, Timeouts & Queue Drops Card */}
      <div className="rounded-xl bg-slate-900/90 border border-slate-800 p-4 flex flex-col justify-between">
        <div className="flex items-center justify-between">
          <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">
            Error Rate & Breakdown
          </span>
          <AlertOctagon
            className={`w-4 h-4 ${
              errorRate > 0.05 ? 'text-rose-400' : 'text-slate-500'
            }`}
          />
        </div>
        <div className="mt-2 flex items-baseline gap-2">
          <span
            className={`text-2xl font-mono font-extrabold ${
              errorRate > 0.1
                ? 'text-rose-400'
                : errorRate > 0
                  ? 'text-amber-300'
                  : 'text-slate-200'
            }`}
          >
            {(errorRate * 100).toFixed(1)}%
          </span>
          <span className="text-xs text-slate-400 font-mono">
            ({failureCount.toLocaleString()} failed)
          </span>
        </div>
        <div className="mt-2 flex items-center gap-3 text-[11px] font-mono text-slate-400">
          <span className="text-amber-300/90">
            Timeouts: <strong>{timeoutCount}</strong>
          </span>
          <span className="text-rose-300/90">
            Queue Drops: <strong>{queueOverflowCount}</strong>
          </span>
        </div>
      </div>

      {/* 3. p95 Latency Card */}
      <div className="rounded-xl bg-slate-900/90 border border-slate-800 p-4 flex flex-col justify-between">
        <div className="flex items-center justify-between">
          <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">
            p95 End-to-End Latency
          </span>
          <Clock className="w-4 h-4 text-indigo-400" />
        </div>
        <div className="mt-2 flex items-baseline gap-2">
          <span
            className={`text-2xl font-mono font-extrabold ${
              p95Latency > 450
                ? 'text-rose-400'
                : p95Latency > 160
                  ? 'text-amber-300'
                  : 'text-indigo-300'
            }`}
          >
            {p95Latency.toFixed(1)} ms
          </span>
          <span className="text-xs text-slate-400 font-mono">
            avg {avgLatency.toFixed(1)} ms
          </span>
        </div>
        <div className="mt-2 flex items-center gap-3 text-[11px] font-mono text-slate-400">
          <span>p50: {ov.p50LatencyMs.toFixed(1)} ms</span>
          <span>p99: {ov.p99LatencyMs.toFixed(1)} ms</span>
        </div>
      </div>

      {/* 4. Retries, Amplification & CB Shedding Card */}
      <div className="rounded-xl bg-slate-900/90 border border-slate-800 p-4 flex flex-col justify-between">
        <div className="flex items-center justify-between">
          <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">
            Retries & CB Shedding
          </span>
          <Repeat className="w-4 h-4 text-sky-400" />
        </div>
        <div className="mt-2 flex items-baseline gap-2">
          <span className="text-2xl font-mono font-extrabold text-sky-300">
            {retryCount.toLocaleString()}
          </span>
          <span className="text-xs text-slate-400 font-mono">
            retries (Amp: <strong>{ov.retryAmplificationFactor.toFixed(2)}x</strong>)
          </span>
        </div>
        <div className="mt-2 flex items-center justify-between text-[11px] font-mono text-slate-400">
          <span className="flex items-center gap-1 text-violet-300">
            <ShieldAlert className="w-3 h-3" />
            CB Blocked: <strong>{cbRejects.toLocaleString()}</strong>
          </span>
          <span className="flex items-center gap-1">
            <Layers className="w-3 h-3 text-slate-500" />
            {ov.totalDownstreamCalls.toLocaleString()} total calls
          </span>
        </div>
      </div>
    </div>
  );
};
