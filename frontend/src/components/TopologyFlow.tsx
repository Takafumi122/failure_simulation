import React, { useMemo } from 'react';
import {
  Background,
  Controls,
  Edge,
  MarkerType,
  Node,
  ReactFlow,
} from '@xyflow/react';
import {
  CircuitState,
  ServiceStatus,
  SimulationRequest,
  StrategyResult,
  Topology,
} from '../types/simulation';
import { ServiceNode, ServiceNodeData } from './ServiceNode';

interface TopologyFlowProps {
  topology: Topology;
  config: SimulationRequest;
  result: StrategyResult | null;
  mode: 'summary' | 'playback';
  currentSecond: number;
}

const nodeTypes = {
  serviceNode: ServiceNode,
};

export const TopologyFlow: React.FC<TopologyFlowProps> = ({
  topology,
  config,
  result,
  mode,
  currentSecond,
}) => {
  const { nodes, edges } = useMemo(() => {
    const secPoint =
      mode === 'playback' && result && result.timeSeries[currentSecond]
        ? result.timeSeries[currentSecond]
        : null;

    const flowNodes: Node[] = topology.nodes.map((tn, idx) => {
      const isClient = tn.id === 'client';
      const srvCfg = config.services[tn.id] || tn.defaultConfig;
      const isFaultTarget =
        config.fault.type !== 'none' && config.fault.targetService === tn.id;

      let status: ServiceStatus = 'healthy';
      let callCount = 0;
      let retryCount = 0;
      let cbRejections = 0;
      let circuitState: CircuitState | undefined = undefined;
      let successRate = 1.0;
      let errorRate = 0.0;
      let p95LatencyMs = 0;
      let peakConcurrent = 0;
      let peakQueueLength = 0;
      let faultActiveNow = false;

      if (result) {
        if (mode === 'playback' && secPoint) {
          if (isClient) {
            callCount = secPoint.totalRequests;
            successRate = secPoint.successRate;
            errorRate = secPoint.errorRate;
            p95LatencyMs = secPoint.p95LatencyMs;
            if (errorRate >= 0.9) status = 'down';
            else if (errorRate >= 0.25) status = 'critical';
            else if (errorRate > 0.01) status = 'degraded';
          } else {
            const sm = secPoint.services[tn.id];
            if (sm) {
              status = sm.status;
              callCount = sm.callCount;
              retryCount = sm.retryCount || 0;
              cbRejections = sm.circuitBreakerRejections || 0;
              circuitState = sm.circuitState;
              successRate = sm.successRate;
              errorRate = sm.errorRate;
              p95LatencyMs = sm.p95LatencyMs;
              peakConcurrent = sm.peakConcurrent;
              peakQueueLength = sm.peakQueueLength;
              faultActiveNow = sm.faultActive;
            }
          }
        } else {
          // Summary mode
          if (isClient) {
            const ov = result.metrics.overall;
            callCount = ov.completedRequests;
            successRate = ov.successRate;
            errorRate = ov.errorRate;
            p95LatencyMs = ov.p95LatencyMs;
            if (errorRate >= 0.9) status = 'down';
            else if (errorRate >= 0.25) status = 'critical';
            else if (errorRate > 0.01) status = 'degraded';
          } else {
            const sm = result.metrics.services[tn.id];
            if (sm) {
              status = sm.status;
              callCount = sm.callCount;
              retryCount = sm.retryCallCount || 0;
              cbRejections = sm.circuitBreakerRejections || 0;
              successRate = sm.successRate;
              errorRate = sm.errorRate;
              p95LatencyMs = sm.p95LatencyMs;
              peakConcurrent = sm.peakConcurrent;
              peakQueueLength = sm.peakQueueLength;
              faultActiveNow = isFaultTarget;
            }
          }
        }
      }

      const data: ServiceNodeData = {
        id: tn.id,
        label: tn.label,
        role: tn.role,
        isClient,
        status,
        faultTarget: isFaultTarget,
        faultActiveNow,
        faultType: isFaultTarget ? config.fault.type : undefined,
        callCount,
        retryCount,
        cbRejections,
        circuitState,
        successRate,
        errorRate,
        p95LatencyMs,
        peakConcurrent,
        maxConcurrent: srvCfg?.maxConcurrent || 0,
        peakQueueLength,
        queueLimit: srvCfg?.queueLimit || 0,
        mode,
        currentSecond,
      };

      return {
        id: tn.id,
        type: 'serviceNode',
        position: { x: idx * 275 + 20, y: 45 },
        data,
        draggable: false,
      };
    });

    const flowEdges: Edge[] = topology.edges.map((te) => {
      const targetNode = flowNodes.find((n) => n.id === te.target);
      const tData = targetNode?.data as unknown as ServiceNodeData | undefined;
      const errRate = tData?.errorRate || 0;
      const status = tData?.status || 'healthy';
      const cbState = tData?.circuitState;
      const cbRej = tData?.cbRejections || 0;

      let stroke = '#34d399'; // emerald-400
      if (cbState === 'OPEN') {
        stroke = '#a855f7'; // purple-500 for open breaker
      } else if (status === 'down' || errRate >= 0.5) {
        stroke = '#f43f5e'; // rose-500
      } else if (status === 'critical' || status === 'degraded' || errRate > 0.02) {
        stroke = '#fbbf24'; // amber-400
      }

      let edgeLabel: string | undefined;
      if (cbState === 'OPEN') {
        edgeLabel = `CB OPEN (${cbRej} blocked)`;
      } else if (tData && tData.callCount > 0) {
        edgeLabel = `${(tData.errorRate * 100).toFixed(0)}% err`;
      }

      return {
        id: te.id,
        source: te.source,
        target: te.target,
        animated: cbState !== 'OPEN',
        label: edgeLabel,
        labelStyle: {
          fill:
            cbState === 'OPEN'
              ? '#d8b4fe'
              : errRate > 0.01
                ? '#fda4af'
                : '#94a3b8',
          fontSize: 10,
          fontWeight: 600,
        },
        labelBgStyle: {
          fill: '#0f172a',
          fillOpacity: 0.85,
        },
        style: {
          stroke,
          strokeWidth: 2.5,
          strokeDasharray: cbState === 'OPEN' ? '6 4' : undefined,
        },
        markerEnd: {
          type: MarkerType.ArrowClosed,
          color: stroke,
        },
      };
    });

    return { nodes: flowNodes, edges: flowEdges };
  }, [topology, config, result, mode, currentSecond]);

  return (
    <div className="w-full h-[320px] rounded-xl bg-slate-900/60 border border-slate-800 overflow-hidden relative">
      <div className="absolute top-3 left-4 z-10 flex flex-wrap items-center gap-2.5 pointer-events-none">
        <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">
          Service Dependency Topology
        </span>
        {result && (
          <span className="inline-flex items-center gap-1.5 rounded-full bg-indigo-500/20 border border-indigo-500/40 px-2.5 py-0.5 text-[11px] font-bold text-indigo-300">
            Active Strategy: {result.strategyName}
          </span>
        )}
        <span className="inline-flex items-center gap-1.5 rounded-full bg-slate-800/90 border border-slate-700 px-2.5 py-0.5 text-[11px] font-medium text-slate-300">
          <span
            className={`w-2 h-2 rounded-full ${
              mode === 'playback' ? 'bg-indigo-400 animate-ping' : 'bg-emerald-400'
            }`}
          />
          {mode === 'playback'
            ? `Time-Series Playback (t = ${currentSecond}s)`
            : 'Full Simulation Summary'}
        </span>
      </div>

      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.14 }}
        minZoom={0.4}
        maxZoom={1.5}
        proOptions={{ hideAttribution: true }}
      >
        <Background color="#334155" gap={20} size={1} />
        <Controls
          showInteractive={false}
          className="!bg-slate-800 !border-slate-700 !text-slate-200 [&>button]:!bg-slate-800 [&>button]:!border-slate-700 [&>button]:!fill-slate-200 hover:[&>button]:!bg-slate-700"
        />
      </ReactFlow>
    </div>
  );
};
