export type Locale = 'en' | 'ja';

export interface Translations {
  nav: {
    title: string;
    subtitle: string;
    engineBadge: string;
    engineTech: string;
    strategiesCount: string;
    langSwitch: string;
    langEn: string;
    langJa: string;
    errorPrefix: string;
    loadingApp: string;
  };
  scenarioGallery: {
    header: string;
    description: string;
    activeBadge: string;
    scenarios: Record<
      string,
      {
        shortTitle: string;
        category: string;
        description: string;
        highlight: string;
      }
    >;
  };
  controlPanel: {
    title: string;
    customParams: string;
    selectPreset: string;
    workloadHeader: string;
    durationSec: string;
    durationSecHint: string;
    rps: string;
    rpsHint: string;
    seed: string;
    seedHint: string;
    faultHeader: string;
    targetService: string;
    faultType: string;
    faultTypeNone: string;
    faultTypeOutage: string;
    faultTypeLatency: string;
    faultTypeErrorRate: string;
    faultWindow: string;
    startSec: string;
    endSec: string;
    addedLatencyMs: string;
    errorRatePct: string;
    resilienceHeader: string;
    targetEdgeLabel: string;
    retryGroup: string;
    maxRetries: string;
    initialBackoffMs: string;
    maxBackoffMs: string;
    backoffMultiplier: string;
    jitterLabel: string;
    cbGroup: string;
    cbFailureThreshold: string;
    cbOpenTimeoutMs: string;
    cbHalfOpenMaxCalls: string;
    cbHalfOpenSuccessThreshold: string;
    failFastGroup: string;
    failFastTimeoutMs: string;
    serviceLimitsHeader: string;
    serviceLimitWorker: string;
    serviceLimitQueue: string;
    serviceLimitTimeout: string;
    exportJson: string;
    importJson: string;
    runSimulation: string;
    simulating: string;
  };
  strategyComparison: {
    header: string;
    subtitle: string;
    activeView: string;
    truncatedWarning: string;
    colStrategy: string;
    colLogicalSuccess: string;
    colP95Latency: string;
    colP95Split: string;
    colDownstreamCalls: string;
    colCbRejections: string;
    colPeakQueue: string;
    retriesLabel: string;
    amplificationLabel: string;
    observedFactsHeader: string;
    observedFactsSubtext: string;
    cbTimelineHeader: string;
    cbTimelineSubtext: string;
    cbStateClosed: string;
    cbStateOpen: string;
    cbStateHalfOpen: string;
    chartTitle: string;
    chartSuccessRate: string;
    chartP95: string;
    chartCalls: string;
    facts: {
      retryImprove: (
        fromPct: string,
        toPct: string,
        deltaPct: string,
        retries: string,
        amp: string
      ) => string;
      retryFutility: (
        retries: string,
        amp: string,
        rPct: string,
        bPct: string
      ) => string;
      retryOverload: (
        bQueue: number,
        rQueue: number,
        bTimeout: number,
        rTimeout: number
      ) => string;
      cbShedding: (
        rejections: string,
        saved: string,
        bCalls: string,
        cbCalls: string
      ) => string;
      combinedSuppression: (
        suppressed: number,
        rRetries: number,
        rcRetries: number
      ) => string;
      failFastSpeedup: (
        bFailP95: string,
        ffFailP95: string,
        bOverallP95: string,
        ffOverallP95: string
      ) => string;
      allHealthy: (
        count: number,
        reqs: string,
        successPct: string,
        p95: string
      ) => string;
    };
    strategies: Record<
      string,
      {
        shortName: string;
        fullName: string;
        description: string;
      }
    >;
  };
  topology: {
    summaryView: string;
    timelineReplay: string;
    statusHealthy: string;
    statusDegraded: string;
    statusCritical: string;
    statusDown: string;
    clientLabel: string;
    clientRole: string;
    legendTitle: string;
    legendWorkers: string;
    legendQueue: string;
    legendCircuit: string;
    faultActiveBadge: string;
    playbackSecond: (sec: number, total: number) => string;
  };
  playbackBar: {
    summaryView: string;
    timelineReplay: string;
    play: string;
    pause: string;
    reset: string;
    stepBack: string;
    stepForward: string;
    speed: string;
    faultWindow: (start: number, end: number) => string;
    faultActiveNow: string;
  };
  metricsCards: {
    logicalSuccessRate: string;
    successCountRatio: (success: string, total: string) => string;
    errorRateBreakdown: string;
    timeouts: string;
    queueOverflows: string;
    p95AvgLatency: string;
    splitLatency: (succP95: string, failP95: string) => string;
    downstreamCallsResilience: string;
    retriesExecuted: string;
    callAmp: string;
    cbFastRejections: string;
  };
  charts: {
    successErrorTitle: string;
    successErrorSub: string;
    latencyTitle: string;
    latencySub: string;
    queueTitle: string;
    queueSub: string;
    successRateSeries: string;
    errorRateSeries: string;
    p95LatencySeries: string;
    avgLatencySeries: string;
    faultBadge: (start: number, end: number) => string;
  };
  eventLog: {
    serviceBreakdownTitle: string;
    serviceBreakdownSub: string;
    colService: string;
    colCalls: string;
    colCbReject: string;
    colSuccessRate: string;
    colTimeouts: string;
    colP95: string;
    colWorkerQueue: string;
    eventStreamTitle: string;
    eventStreamSub: string;
    filterAll: string;
    colSec: string;
    colTime: string;
    colEvent: string;
    colRequest: string;
    colFromTo: string;
    colDetails: string;
    events: Record<string, string>;
  };
}
