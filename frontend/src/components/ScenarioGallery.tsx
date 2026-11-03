import React from 'react';
import {
  Activity,
  CheckCircle2,
  Flame,
  RefreshCw,
  ShieldAlert,
  Sparkles,
  Zap,
} from 'lucide-react';
import { ScenarioPreset } from '../types/simulation';
import { useI18n } from '../i18n/I18nContext';

interface ScenarioGalleryProps {
  scenarios: ScenarioPreset[];
  selectedScenarioId: string;
  onSelectScenario: (scenario: ScenarioPreset) => void;
  isLoading: boolean;
}

function getScenarioIcon(id: string) {
  switch (id) {
    case 'healthy-system':
      return <CheckCircle2 className="w-4 h-4 text-emerald-400" />;
    case 'transient-failure':
      return <RefreshCw className="w-4 h-4 text-sky-400" />;
    case 'prolonged-outage':
      return <Flame className="w-4 h-4 text-rose-400" />;
    case 'retry-storm':
      return <Zap className="w-4 h-4 text-amber-400" />;
    case 'circuit-breaker-recovery':
      return <ShieldAlert className="w-4 h-4 text-violet-400" />;
    default:
      return <Activity className="w-4 h-4 text-indigo-400" />;
  }
}

export const ScenarioGallery: React.FC<ScenarioGalleryProps> = ({
  scenarios,
  selectedScenarioId,
  onSelectScenario,
  isLoading,
}) => {
  const { t } = useI18n();

  return (
    <div className="rounded-xl bg-slate-900/90 border border-slate-800 p-4 mb-5">
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-2 mb-3">
        <div>
          <div className="flex items-center gap-2">
            <Sparkles className="w-4 h-4 text-indigo-400" />
            <h2 className="text-xs font-bold uppercase tracking-wider text-slate-200">
              {t.scenarioGallery.header}
            </h2>
          </div>
          <p className="text-xs text-slate-400 mt-0.5">
            {t.scenarioGallery.description}
          </p>
        </div>
      </div>

      <div
        className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-2.5"
        data-testid="scenario-gallery"
      >
        {scenarios.map((sc) => {
          const isSelected = sc.id === selectedScenarioId;
          const loc = t.scenarioGallery.scenarios[sc.id];
          const title = loc?.shortTitle || sc.shortTitle || sc.name;
          const desc = loc?.description || sc.description;
          const highlight = loc?.highlight || sc.highlight;

          return (
            <button
              key={sc.id}
              type="button"
              disabled={isLoading}
              data-testid={`scenario-card-${sc.id}`}
              onClick={() => onSelectScenario(sc)}
              className={`text-left rounded-xl p-3 border transition cursor-pointer flex flex-col justify-between ${
                isSelected
                  ? 'bg-indigo-950/60 border-indigo-400 shadow-lg shadow-indigo-950/50'
                  : 'bg-slate-950/80 border-slate-800/90 hover:border-slate-700 hover:bg-slate-900'
              }`}
            >
              <div>
                <div className="flex items-center justify-between gap-1.5 mb-1.5">
                  <span className="inline-flex items-center gap-1.5 text-xs font-bold text-slate-100">
                    {getScenarioIcon(sc.id)}
                    {title}
                  </span>
                  {isSelected && (
                    <span className="rounded bg-indigo-500/30 border border-indigo-400/50 px-1.5 py-0.5 text-[9px] font-bold uppercase text-indigo-200">
                      {t.scenarioGallery.activeBadge}
                    </span>
                  )}
                </div>
                <p className="text-[11px] text-slate-400 line-clamp-2 leading-snug">
                  {desc}
                </p>
              </div>

              <div className="mt-2.5 pt-2 border-t border-slate-800/80 flex items-center justify-between text-[10px] font-mono">
                <span className="text-indigo-300 truncate">{highlight}</span>
              </div>
            </button>
          );
        })}
      </div>
    </div>
  );
};
