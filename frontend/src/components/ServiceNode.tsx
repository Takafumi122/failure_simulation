import React from 'react';
import { Handle, Position, NodeProps } from '@xyflow/react';
import {
  Activity,
  AlertTriangle,
  CheckCircle2,
  Database,
  Globe,
  Layers,
  Server,
  ShieldAlert,
  Sparkles,
  XCircle,
  Zap,
} from 'lucide-react';
import { CircuitState, ServiceStatus } from '../types/simulation';

export interface ServiceNodeData extends Record<string, unknown> {
  id: string;
  label: string;
  role: string;
  isClient?: boolean;
  status: ServiceStatus;
  faultTarget: boolean;
  faultActiveNow: boolean;
  faultType?: string;
  callCount: number;
  retryCount: number;
  cbRejections: number;
  circuitState?: CircuitState;
  successRate: number;
  errorRate: number;
  p95LatencyMs: number;
  peakConcurrent: number;
  maxConcurrent: number;
  peakQueueLength: number;
  queueLimit: number;
  mode: 'summary' | 'playback';
  currentSecond: number;
}

const statusStyles: Record<
  ServiceStatus,
  { border: string; badgeBg: string; badgeText: string; label: string; glow: string }
> = {
  healthy: {
    border: 'border-emerald-500/50',
    badgeBg: 'bg-emerald-500/15',
    badgeText: 'text-emerald-300',
    label: 'HEALTHY',
    glow: 'shadow-emerald-950/40',
  },
  degraded: {
    border: 'border-amber-500/70',
    badgeBg: 'bg-amber-500/20',
    badgeText: 'text-amber-300',
    label: 'DEGRADED',
    glow: 'shadow-amber-900/40',
  },
  critical: {
    border: 'border-orange-500/80',
    badgeBg: 'bg-orange-500/25',
    badgeText: 'text-orange-300',
    label: 'CRITICAL',
    glow: 'shadow-orange-900/50',
  },
  down: {
    border: 'border-rose-500',
    badgeBg: 'bg-rose-500/30',
    badgeText: 'text-rose-200',
    label: 'OUTAGE / DOWN',
    glow: 'shadow-rose-900/60',
  },
};

function getNodeIcon(id: string) {
  switch (id) {
    case 'client':
      return <Globe className="w-4 h-4 text-sky-400" />;
    case 'api-gateway':
      return <Layers className="w-4 h-4 text-indigo-400" />;
    case 'product-api':
      return <Server className="w-4 h-4 text-violet-400" />;
    case 'recommendation':
      return <Sparkles className="w-4 h-4 text-fuchsia-400" />;
    case 'feature-db':
      return <Database className="w-4 h-4 text-emerald-400" />;
    default:
      return <Activity className="w-4 h-4 text-slate-400" />;
  }
}

export const ServiceNode: React.FC<NodeProps> = ({ data }) => {
  const d = data as unknown as ServiceNodeData;
  const st = statusStyles[d.status] || statusStyles.healthy;

  const queueRatio =
    d.queueLimit > 0
      ? Math.min(100, Math.round((d.peakQueueLength / d.queueLimit) * 100))
      : 0;
  const workerRatio =
    d.maxConcurrent > 0
      ? Math.min(100, Math.round((d.peakConcurrent / d.maxConcurrent) * 100))
      : 0;

  return (
    <div
      className={`relative w-56 rounded-xl bg-slate-900/95 backdrop-blur border-2 ${st.border} shadow-xl ${st.glow} p-3 transition-all duration-200`}
    >
      {!d.isClient && (
        <Handle
          type="target"
          position={Position.Left}
          className="!w-3 !h-3 !bg-slate-400 !border-2 !border-slate-900"
        />
      )}

      {/* Fault Active Banner */}
      {d.faultActiveNow && (
        <div className="-mt-1 mb-1.5 flex items-center justify-center gap-1 rounded-md bg-rose-500/25 border border-rose-500/50 px-2 py-0.5 text-[10px] font-semibold tracking-wider text-rose-200 animate-pulse">
          <Zap className="w-3 h-3 text-rose-400" />
          FAULT ({d.faultType?.toUpperCase()})
        </div>
      )}

      {/* Circuit Breaker State Banner (if OPEN or HALF_OPEN) */}
      {d.circuitState && d.circuitState !== 'CLOSED' && (
        <div
          className={`mb-1.5 flex items-center justify-center gap-1 rounded-md px-2 py-0.5 text-[10px] font-bold tracking-wider ${
            d.circuitState === 'OPEN'
              ? 'bg-violet-500/30 border border-violet-400 text-violet-200'
              : 'bg-amber-500/25 border border-amber-400 text-amber-200'
          }`}
        >
          <ShieldAlert className="w-3 h-3" />
          CB: {d.circuitState}
        </div>
      )}

      {/* Header */}
      <div className="flex items-start justify-between gap-2">
        <div className="flex items-center gap-2">
          <div className="p-1.5 rounded-lg bg-slate-800 border border-slate-700/70">
            {getNodeIcon(d.id)}
          </div>
          <div>
            <div className="text-xs font-bold text-slate-100 leading-tight">
              {d.label}
            </div>
            <div className="text-[10px] text-slate-400 leading-tight mt-0.5">
              {d.role}
            </div>
          </div>
        </div>
      </div>

      {/* Status Badge & Calls */}
      <div className="mt-2 flex items-center justify-between">
        <span
          className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-bold ${st.badgeBg} ${st.badgeText}`}
        >
          {d.status === 'healthy' && <CheckCircle2 className="w-3 h-3" />}
          {(d.status === 'degraded' || d.status === 'critical') && (
            <AlertTriangle className="w-3 h-3" />
          )}
          {d.status === 'down' && <XCircle className="w-3 h-3" />}
          {st.label}
        </span>
        <span className="text-[10px] font-mono text-slate-300">
          {d.callCount} {d.mode === 'playback' ? 'req/s' : 'calls'}
          {d.retryCount > 0 && (
            <span className="text-sky-300 ml-1">(+{d.retryCount}r)</span>
          )}
        </span>
      </div>

      {/* Metrics Grid */}
      <div className="mt-2 grid grid-cols-2 gap-1.5 rounded-lg bg-slate-950/80 p-2 border border-slate-800/80">
        <div>
          <div className="text-[10px] text-slate-400">Success Rate</div>
          <div
            className={`text-xs font-mono font-bold ${
              d.successRate >= 0.98
                ? 'text-emerald-400'
                : d.successRate >= 0.75
                  ? 'text-amber-400'
                  : 'text-rose-400'
            }`}
          >
            {(d.successRate * 100).toFixed(1)}%
          </div>
        </div>
        <div>
          <div className="text-[10px] text-slate-400">p95 Latency</div>
          <div
            className={`text-xs font-mono font-bold ${
              d.p95LatencyMs > 400
                ? 'text-rose-400'
                : d.p95LatencyMs > 150
                  ? 'text-amber-300'
                  : 'text-slate-100'
            }`}
          >
            {d.p95LatencyMs.toFixed(1)} ms
          </div>
        </div>
      </div>

      {/* Resource Saturation Bars (Backend services only) */}
      {!d.isClient && (
        <div className="mt-2 space-y-1.5">
          <div>
            <div className="flex justify-between text-[10px] text-slate-400 mb-0.5">
              <span>Workers</span>
              <span className="font-mono text-slate-300">
                {d.peakConcurrent}/{d.maxConcurrent}
              </span>
            </div>
            <div className="w-full h-1.5 bg-slate-800 rounded-full overflow-hidden">
              <div
                className={`h-full transition-all duration-200 ${
                  workerRatio >= 90 ? 'bg-amber-400' : 'bg-indigo-500'
                }`}
                style={{ width: `${workerRatio}%` }}
              />
            </div>
          </div>

          <div>
            <div className="flex justify-between text-[10px] text-slate-400 mb-0.5">
              <span>Queue</span>
              <span className="font-mono text-slate-300">
                {d.peakQueueLength}/{d.queueLimit}
              </span>
            </div>
            <div className="w-full h-1.5 bg-slate-800 rounded-full overflow-hidden">
              <div
                className={`h-full transition-all duration-200 ${
                  queueRatio >= 85
                    ? 'bg-rose-500'
                    : queueRatio >= 30
                      ? 'bg-amber-400'
                      : 'bg-emerald-500'
                }`}
                style={{ width: `${queueRatio}%` }}
              />
            </div>
          </div>
        </div>
      )}

      {d.id !== 'feature-db' && (
        <Handle
          type="source"
          position={Position.Right}
          className="!w-3 !h-3 !bg-slate-400 !border-2 !border-slate-900"
        />
      )}
    </div>
  );
};
