import React, { useMemo } from 'react';
import {
  Area,
  AreaChart,
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ReferenceArea,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { FaultConfig, StrategyResult } from '../types/simulation';

interface ChartsPanelProps {
  result: StrategyResult | null;
  fault: FaultConfig;
  mode: 'summary' | 'playback';
  currentSecond: number;
  onSelectSecond: (sec: number) => void;
}

export const ChartsPanel: React.FC<ChartsPanelProps> = ({
  result,
  fault,
  mode,
  currentSecond,
  onSelectSecond,
}) => {
  const chartData = useMemo(() => {
    if (!result) return [];
    return result.timeSeries.map((pt) => ({
      second: pt.second,
      label: `${pt.second}s`,
      successRatePct: Number((pt.successRate * 100).toFixed(1)),
      errorRatePct: Number((pt.errorRate * 100).toFixed(1)),
      p95LatencyMs: pt.p95LatencyMs,
      avgLatencyMs: pt.avgLatencyMs,
      gwQueue: pt.services['api-gateway']?.peakQueueLength || 0,
      prodQueue: pt.services['product-api']?.peakQueueLength || 0,
      recQueue: pt.services['recommendation']?.peakQueueLength || 0,
      dbQueue: pt.services['feature-db']?.peakQueueLength || 0,
      gwP95: pt.services['api-gateway']?.p95LatencyMs || 0,
      prodP95: pt.services['product-api']?.p95LatencyMs || 0,
      recP95: pt.services['recommendation']?.p95LatencyMs || 0,
      dbP95: pt.services['feature-db']?.p95LatencyMs || 0,
    }));
  }, [result]);

  if (!result) return null;

  const hasFaultWindow = fault.type !== 'none';

  return (
    <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
      {/* 1. Success & Error Rate Over Time */}
      <div className="rounded-xl bg-slate-900/90 border border-slate-800 p-4">
        <div className="flex items-center justify-between mb-3">
          <div>
            <h3 className="text-xs font-bold uppercase tracking-wider text-slate-200">
              Success & Error Rate (%)
            </h3>
            <p className="text-[11px] text-slate-400">
              End-to-end client outcome per second
            </p>
          </div>
          {hasFaultWindow && (
            <span className="text-[10px] rounded bg-rose-500/15 border border-rose-500/30 px-2 py-0.5 text-rose-300">
              Fault: {fault.startSec}s–{fault.endSec}s
            </span>
          )}
        </div>

        <div className="h-52 w-full">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart
              data={chartData}
              onClick={(e) => {
                if (e && typeof e.activeTooltipIndex === 'number') {
                  onSelectSecond(e.activeTooltipIndex);
                }
              }}
            >
              <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" />
              <XAxis
                dataKey="second"
                tickFormatter={(v) => `${v}s`}
                stroke="#64748b"
                fontSize={11}
              />
              <YAxis domain={[0, 100]} stroke="#64748b" fontSize={11} />
              <Tooltip
                contentStyle={{
                  backgroundColor: '#0f172a',
                  borderColor: '#334155',
                  fontSize: 12,
                }}
              />
              <Legend wrapperStyle={{ fontSize: 11 }} />
              {hasFaultWindow && (
                <ReferenceArea
                  x1={fault.startSec}
                  x2={fault.endSec}
                  fill="#f43f5e"
                  fillOpacity={0.12}
                />
              )}
              {mode === 'playback' && (
                <ReferenceLine
                  x={currentSecond}
                  stroke="#818cf8"
                  strokeWidth={2}
                  strokeDasharray="3 3"
                />
              )}
              <Area
                type="monotone"
                dataKey="successRatePct"
                name="Success %"
                stroke="#10b981"
                fill="#10b981"
                fillOpacity={0.2}
                strokeWidth={2}
              />
              <Area
                type="monotone"
                dataKey="errorRatePct"
                name="Error %"
                stroke="#f43f5e"
                fill="#f43f5e"
                fillOpacity={0.25}
                strokeWidth={2}
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* 2. End-to-End & Service p95 Latency */}
      <div className="rounded-xl bg-slate-900/90 border border-slate-800 p-4">
        <div className="flex items-center justify-between mb-3">
          <div>
            <h3 className="text-xs font-bold uppercase tracking-wider text-slate-200">
              p95 Latency Propagation (ms)
            </h3>
            <p className="text-[11px] text-slate-400">
              How downstream delay amplifies upstream response times
            </p>
          </div>
        </div>

        <div className="h-52 w-full">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart
              data={chartData}
              onClick={(e) => {
                if (e && typeof e.activeTooltipIndex === 'number') {
                  onSelectSecond(e.activeTooltipIndex);
                }
              }}
            >
              <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" />
              <XAxis
                dataKey="second"
                tickFormatter={(v) => `${v}s`}
                stroke="#64748b"
                fontSize={11}
              />
              <YAxis stroke="#64748b" fontSize={11} />
              <Tooltip
                contentStyle={{
                  backgroundColor: '#0f172a',
                  borderColor: '#334155',
                  fontSize: 12,
                }}
              />
              <Legend wrapperStyle={{ fontSize: 11 }} />
              {hasFaultWindow && (
                <ReferenceArea
                  x1={fault.startSec}
                  x2={fault.endSec}
                  fill="#f43f5e"
                  fillOpacity={0.12}
                />
              )}
              {mode === 'playback' && (
                <ReferenceLine
                  x={currentSecond}
                  stroke="#818cf8"
                  strokeWidth={2}
                  strokeDasharray="3 3"
                />
              )}
              <Line
                type="monotone"
                dataKey="p95LatencyMs"
                name="Client E2E p95"
                stroke="#818cf8"
                strokeWidth={2.5}
                dot={false}
              />
              <Line
                type="monotone"
                dataKey="recP95"
                name="Recommendation p95"
                stroke="#e879f9"
                strokeWidth={1.8}
                dot={false}
              />
              <Line
                type="monotone"
                dataKey="dbP95"
                name="Feature DB p95"
                stroke="#34d399"
                strokeWidth={1.8}
                dot={false}
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* 3. Per-Service Peak Queue Length */}
      <div className="rounded-xl bg-slate-900/90 border border-slate-800 p-4">
        <div className="flex items-center justify-between mb-3">
          <div>
            <h3 className="text-xs font-bold uppercase tracking-wider text-slate-200">
              Service Queue Backpressure
            </h3>
            <p className="text-[11px] text-slate-400">
              Peak queued requests waiting for busy workers
            </p>
          </div>
        </div>

        <div className="h-52 w-full">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart
              data={chartData}
              onClick={(e) => {
                if (e && typeof e.activeTooltipIndex === 'number') {
                  onSelectSecond(e.activeTooltipIndex);
                }
              }}
            >
              <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" />
              <XAxis
                dataKey="second"
                tickFormatter={(v) => `${v}s`}
                stroke="#64748b"
                fontSize={11}
              />
              <YAxis stroke="#64748b" fontSize={11} />
              <Tooltip
                contentStyle={{
                  backgroundColor: '#0f172a',
                  borderColor: '#334155',
                  fontSize: 12,
                }}
              />
              <Legend wrapperStyle={{ fontSize: 11 }} />
              {hasFaultWindow && (
                <ReferenceArea
                  x1={fault.startSec}
                  x2={fault.endSec}
                  fill="#f43f5e"
                  fillOpacity={0.12}
                />
              )}
              {mode === 'playback' && (
                <ReferenceLine
                  x={currentSecond}
                  stroke="#818cf8"
                  strokeWidth={2}
                  strokeDasharray="3 3"
                />
              )}
              <Line
                type="monotone"
                dataKey="gwQueue"
                name="API Gateway"
                stroke="#60a5fa"
                strokeWidth={2}
                dot={false}
              />
              <Line
                type="monotone"
                dataKey="prodQueue"
                name="Product API"
                stroke="#a78bfa"
                strokeWidth={2}
                dot={false}
              />
              <Line
                type="monotone"
                dataKey="recQueue"
                name="Recommendation"
                stroke="#f472b6"
                strokeWidth={2}
                dot={false}
              />
              <Line
                type="monotone"
                dataKey="dbQueue"
                name="Feature DB"
                stroke="#fbbf24"
                strokeWidth={2}
                dot={false}
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
      </div>
    </div>
  );
};
