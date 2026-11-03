import React from 'react';
import {
  FastForward,
  Pause,
  Play,
  RotateCcw,
  SkipBack,
  SkipForward,
  Sparkles,
} from 'lucide-react';
import { FaultConfig } from '../types/simulation';
import { useI18n } from '../i18n/I18nContext';

interface PlaybackBarProps {
  mode: 'summary' | 'playback';
  setMode: (m: 'summary' | 'playback') => void;
  currentSecond: number;
  setCurrentSecond: (sec: number) => void;
  durationSec: number;
  isPlaying: boolean;
  setIsPlaying: (playing: boolean) => void;
  speed: 1 | 2 | 4;
  setSpeed: (s: 1 | 2 | 4) => void;
  fault: FaultConfig;
}

export const PlaybackBar: React.FC<PlaybackBarProps> = ({
  mode,
  setMode,
  currentSecond,
  setCurrentSecond,
  durationSec,
  isPlaying,
  setIsPlaying,
  speed,
  setSpeed,
  fault,
}) => {
  const { t } = useI18n();
  const maxSec = Math.max(0, durationSec - 1);
  const isFaultNow =
    fault.type !== 'none' &&
    currentSecond >= fault.startSec &&
    currentSecond < fault.endSec;

  const handleTogglePlay = () => {
    if (mode === 'summary') {
      setMode('playback');
      setCurrentSecond(0);
      setIsPlaying(true);
    } else {
      if (!isPlaying && currentSecond >= maxSec) {
        setCurrentSecond(0);
      }
      setIsPlaying(!isPlaying);
    }
  };

  return (
    <div className="rounded-xl bg-slate-900/90 border border-slate-800 p-3.5 flex flex-col lg:flex-row items-center justify-between gap-4">
      {/* Mode Switcher & Transport Buttons */}
      <div className="flex flex-wrap items-center gap-2 w-full lg:w-auto">
        <div className="inline-flex rounded-lg bg-slate-950 p-1 border border-slate-800">
          <button
            type="button"
            onClick={() => {
              setIsPlaying(false);
              setMode('summary');
            }}
            className={`px-3 py-1 rounded-md text-xs font-semibold transition ${
              mode === 'summary'
                ? 'bg-indigo-600 text-white shadow'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            {t.playbackBar.summaryView}
          </button>
          <button
            type="button"
            onClick={() => setMode('playback')}
            className={`px-3 py-1 rounded-md text-xs font-semibold transition flex items-center gap-1 ${
              mode === 'playback'
                ? 'bg-indigo-600 text-white shadow'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <Sparkles className="w-3 h-3" />
            {t.playbackBar.timelineReplay}
          </button>
        </div>

        <div className="flex items-center gap-1.5 ml-1">
          <button
            type="button"
            onClick={handleTogglePlay}
            className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white px-3 py-1.5 text-xs font-bold shadow transition cursor-pointer"
          >
            {isPlaying ? (
              <>
                <Pause className="w-3.5 h-3.5" /> {t.playbackBar.pause}
              </>
            ) : (
              <>
                <Play className="w-3.5 h-3.5" /> {t.playbackBar.play}
              </>
            )}
          </button>

          <button
            type="button"
            title={t.playbackBar.stepBack}
            onClick={() => {
              setIsPlaying(false);
              setMode('playback');
              setCurrentSecond(Math.max(0, currentSecond - 1));
            }}
            className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 transition cursor-pointer"
          >
            <SkipBack className="w-3.5 h-3.5" />
          </button>

          <button
            type="button"
            title={t.playbackBar.stepForward}
            onClick={() => {
              setIsPlaying(false);
              setMode('playback');
              setCurrentSecond(Math.min(maxSec, currentSecond + 1));
            }}
            className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 transition cursor-pointer"
          >
            <SkipForward className="w-3.5 h-3.5" />
          </button>

          <button
            type="button"
            title={t.playbackBar.reset}
            onClick={() => {
              setIsPlaying(false);
              setCurrentSecond(0);
            }}
            className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 transition cursor-pointer"
          >
            <RotateCcw className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Scrubber Slider */}
      <div className="flex-1 w-full flex items-center gap-3">
        <span className="text-xs font-mono font-bold text-indigo-300 min-w-[68px]">
          {t.topology.playbackSecond(currentSecond, maxSec)}
        </span>

        <div className="relative flex-1 flex items-center">
          <input
            type="range"
            min={0}
            max={maxSec}
            value={currentSecond}
            onChange={(e) => {
              setIsPlaying(false);
              setMode('playback');
              setCurrentSecond(Number(e.target.value));
            }}
            className="w-full accent-indigo-500 cursor-pointer h-2 bg-slate-800 rounded-lg"
          />
        </div>

        {mode === 'playback' && isFaultNow && (
          <span className="hidden sm:inline-flex items-center rounded-md bg-rose-500/20 border border-rose-500/40 px-2 py-0.5 text-[11px] font-semibold text-rose-300">
            {t.playbackBar.faultWindow(fault.startSec, fault.endSec)}
          </span>
        )}
      </div>

      {/* Speed Selector */}
      <div className="flex items-center gap-1.5 self-end lg:self-auto">
        <FastForward className="w-3.5 h-3.5 text-slate-400 mr-0.5" />
        {([1, 2, 4] as const).map((s) => (
          <button
            key={s}
            type="button"
            onClick={() => setSpeed(s)}
            className={`px-2 py-1 rounded text-xs font-mono font-semibold border transition cursor-pointer ${
              speed === s
                ? 'bg-indigo-600/30 border-indigo-500 text-indigo-200'
                : 'bg-slate-800 border-slate-700 text-slate-400 hover:text-slate-200'
            }`}
          >
            {s}x
          </button>
        ))}
      </div>
    </div>
  );
};
