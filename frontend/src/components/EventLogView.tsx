import React, { useMemo, useState } from 'react';
import { Filter, Terminal } from 'lucide-react';
import { EventType, SimulationEventLog, StrategyResult } from '../types/simulation';

interface EventLogViewProps {
  result: StrategyResult | null;
  mode: 'summary' | 'playback';
  currentSecond: number;
}

const EVENT_BADGE: Record<EventType, { bg: string; text: string }> = {
  REQUEST_ARRIVAL: { bg: 'bg-sky-500/20', text: 'text-sky-300' },
  QUEUE_ENQUEUED: { bg: 'bg-amber-500/20', text: 'text-amber-300' },
  PROCESSING_START: { bg: 'bg-indigo-500/20', text: 'text-indigo-300' },
  DOWNSTREAM_CALL: { bg: 'bg-violet-500/20', text: 'text-violet-300' },
  RETRY_BACKOFF_COMPLETE: { bg: 'bg-cyan-500/25', text: 'text-cyan-200' },
  PROCESSING_COMPLETE: { bg: 'bg-emerald-500/20', text: 'text-emerald-300' },
  REQUEST_FAILURE: { bg: 'bg-rose-500/25', text: 'text-rose-300' },
  REQUEST_TIMEOUT: { bg: 'bg-orange-500/25', text: 'text-orange-300' },
  CIRCUIT_STATE_CHANGE: { bg: 'bg-fuchsia-500/30', text: 'text-fuchsia-200' },
};

const SERVICE_ORDER = ['api-gateway', 'product-api', 'recommendation', 'feature-db'];

export const EventLogView: React.FC<EventLogViewProps> = ({
  result,
  mode,
  currentSecond,
}) => {
  const [filterType, setFilterType] = useState<string>('ALL');

  const filteredEvents = useMemo(() => {
    if (!result) return [];
    return result.events.filter((ev) => {
      if (mode === 'playback' && ev.second !== currentSecond) {
        return false;
      }
      if (filterType !== 'ALL' && ev.eventType !== filterType) {
        return false;
      }
      return true;
    });
  }, [result, mode, currentSecond, filterType]);

  if (!result) return null;

  const serviceBreakdown = SERVICE_ORDER.map((id) => result.metrics.services[id]).filter(
    Boolean
  );

  return (
    <div className="grid grid-cols-1 xl:grid-cols-12 gap-4">
      {/* Left: Per-Service Summary Table */}
      <div className="xl:col-span-6 rounded-xl bg-slate-900/90 border border-slate-800 p-4">
        <div className="flex flex-wrap items-center justify-between gap-2 mb-3">
          <div>
            <h3 className="text-xs font-bold uppercase tracking-wider text-slate-200">
              Service-by-Service Breakdown
            </h3>
            <p className="text-[11px] text-slate-400">
              Primary calls, retry attempts, CB rejections, and worker/queue saturation
            </p>
          </div>
          <span className="text-[11px] font-mono text-indigo-300 bg-indigo-500/10 border border-indigo-500/30 rounded px-2 py-0.5">
            {result.strategyName}
          </span>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse text-xs">
            <thead>
              <tr className="border-b border-slate-800 text-[11px] text-slate-400">
                <th className="py-2 pr-2">Service</th>
                <th className="py-2 px-1.5 text-right">Calls (+Retry)</th>
                <th className="py-2 px-1.5 text-right">CB Reject</th>
                <th className="py-2 px-1.5 text-right">Success</th>
                <th className="py-2 px-1.5 text-right">Timeout</th>
                <th className="py-2 px-1.5 text-right">p95 (ms)</th>
                <th className="py-2 pl-1.5 text-right">Worker / Queue</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/70 font-mono">
              {serviceBreakdown.map((sm) => (
                <tr key={sm.serviceId} className="hover:bg-slate-800/40">
                  <td className="py-2.5 pr-2 font-sans font-semibold text-slate-200">
                    {sm.serviceName}
                  </td>
                  <td className="py-2.5 px-1.5 text-right text-slate-300">
                    {sm.callCount.toLocaleString()}
                    {sm.retryCallCount > 0 && (
                      <span className="text-sky-300 text-[10px] ml-1">
                        (+{sm.retryCallCount})
                      </span>
                    )}
                  </td>
                  <td className="py-2.5 px-1.5 text-right text-violet-300">
                    {sm.circuitBreakerRejections}
                  </td>
                  <td
                    className={`py-2.5 px-1.5 text-right font-bold ${
                      sm.successRate >= 0.95
                        ? 'text-emerald-400'
                        : sm.successRate >= 0.75
                          ? 'text-amber-300'
                          : 'text-rose-400'
                    }`}
                  >
                    {(sm.successRate * 100).toFixed(1)}%
                  </td>
                  <td className="py-2.5 px-1.5 text-right text-amber-300">
                    {sm.timeoutCount}
                  </td>
                  <td className="py-2.5 px-1.5 text-right text-indigo-300">
                    {sm.p95LatencyMs.toFixed(1)}
                  </td>
                  <td className="py-2.5 pl-1.5 text-right text-slate-300">
                    {(sm.workerUtilization * 100).toFixed(0)}% / q={sm.peakQueueLength}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Right: Discrete Event Stream Log */}
      <div className="xl:col-span-6 rounded-xl bg-slate-900/90 border border-slate-800 p-4 flex flex-col">
        <div className="flex flex-wrap items-center justify-between gap-2 mb-3">
          <div className="flex items-center gap-2">
            <Terminal className="w-4 h-4 text-indigo-400" />
            <div>
              <h3 className="text-xs font-bold uppercase tracking-wider text-slate-200">
                Discrete Event Stream{' '}
                {mode === 'playback' ? `(t = ${currentSecond}s)` : '(Sampled Log)'}
              </h3>
              <p className="text-[11px] text-slate-400">
                Showing {filteredEvents.length} of{' '}
                {result.executionMeta.recordedEventCount} recorded events (
                {result.executionMeta.totalEventsProcessed.toLocaleString()} processed in
                heap, status={result.executionMeta.status})
              </p>
            </div>
          </div>

          <div className="flex items-center gap-1.5">
            <Filter className="w-3.5 h-3.5 text-slate-400" />
            <select
              value={filterType}
              onChange={(e) => setFilterType(e.target.value)}
              className="rounded bg-slate-950 border border-slate-700 px-2 py-1 text-[11px] text-slate-200"
            >
              <option value="ALL">All Event Types</option>
              <option value="CIRCUIT_STATE_CHANGE">CIRCUIT_STATE_CHANGE</option>
              <option value="RETRY_BACKOFF_COMPLETE">RETRY_BACKOFF_COMPLETE</option>
              <option value="REQUEST_FAILURE">REQUEST_FAILURE</option>
              <option value="REQUEST_TIMEOUT">REQUEST_TIMEOUT</option>
              <option value="QUEUE_ENQUEUED">QUEUE_ENQUEUED</option>
              <option value="PROCESSING_COMPLETE">PROCESSING_COMPLETE</option>
            </select>
          </div>
        </div>

        <div className="h-52 overflow-y-auto rounded-lg bg-slate-950 border border-slate-800/90 p-2 font-mono text-[11px] space-y-1.5">
          {filteredEvents.length === 0 ? (
            <div className="h-full flex items-center justify-center text-slate-500">
              No matching events in this time window.
            </div>
          ) : (
            filteredEvents.slice(0, 250).map((ev: SimulationEventLog) => {
              const badge = EVENT_BADGE[ev.eventType] || {
                bg: 'bg-slate-800',
                text: 'text-slate-300',
              };
              return (
                <div
                  key={ev.sequenceId}
                  className="flex items-start gap-2 py-1 px-2 rounded hover:bg-slate-900/80 border-b border-slate-900"
                >
                  <span className="text-slate-500 min-w-[68px]">
                    {(ev.timestampMs / 1000).toFixed(3)}s
                  </span>
                  <span
                    className={`px-1.5 py-0.5 rounded text-[10px] font-bold shrink-0 ${badge.bg} ${badge.text}`}
                  >
                    {ev.eventType}
                  </span>
                  <span className="text-slate-300 flex-1 break-all">{ev.message}</span>
                </div>
              );
            })
          )}
        </div>
      </div>
    </div>
  );
};
