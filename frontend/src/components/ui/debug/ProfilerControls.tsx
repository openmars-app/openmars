import { useEngineProfilerStore } from "@/stores/engineProfilerStore";
import GameButton from "../buttons/GameButton";

const milliseconds = (value: number | null) =>
  value === null ? "—" : `${value.toLocaleString(undefined, { maximumFractionDigits: 1 })} ms`;

export default function ProfilerControls() {
  const profiler = useEngineProfilerStore();
  const recording = profiler.phase === "recording";
  const hasSummary =
    profiler.summary.worstCpuFrameMs !== null ||
    profiler.summary.longestFrameGapMs !== null ||
    profiler.summary.hottestStage !== null;
  return (
    <section
      aria-labelledby="engine-profiler-title"
      className="flex flex-col gap-3 border-b border-white/15 pb-4"
    >
      <div className="flex items-center justify-between gap-3">
        <h4 id="engine-profiler-title" className="font-orbitron text-xs text-white m-0">
          Profiler
        </h4>
        {profiler.phase !== "idle" && (
          <output
            aria-label="Profiler recording progress"
            className="font-mono text-[11px] text-white/70 tabular-nums"
          >
            {recording
              ? `${(profiler.elapsedMs / 1000).toFixed(1)} / ${Math.round(profiler.durationMs / 1000)} s`
              : "Recorded"}
            {` · ${profiler.eventCount.toLocaleString()} events`}
            {profiler.dropped > 0 && ` · ${profiler.dropped.toLocaleString()} dropped`}
          </output>
        )}
      </div>
      <div className="grid grid-cols-3 gap-2">
        <GameButton
          size="sm"
          className="!min-h-11 !px-2"
          onClick={() => {
            if (recording) {
              profiler.stop();
            } else {
              profiler.start();
            }
          }}
        >
          {recording ? "Stop" : "Record 10s"}
        </GameButton>
        <GameButton
          size="sm"
          emphasis="secondary"
          className="!min-h-11 !px-2"
          disabled={profiler.phase !== "ready"}
          onClick={() => profiler.download()}
        >
          Download
        </GameButton>
        <GameButton
          size="sm"
          emphasis="secondary"
          className="!min-h-11 !px-2"
          aria-pressed={profiler.armed}
          selected={profiler.armed}
          onClick={() => profiler.toggleReload()}
        >
          Next reload
        </GameButton>
      </div>
      {hasSummary && (
        <dl className="grid grid-cols-3 gap-3 m-0">
          <div className="min-w-0">
            <dt className="text-[11px] text-white/60">Worst CPU frame</dt>
            <dd className="m-0 mt-1 font-mono text-sm text-white tabular-nums">
              {milliseconds(profiler.summary.worstCpuFrameMs)}
            </dd>
          </div>
          <div className="min-w-0">
            <dt className="text-[11px] text-white/60">Longest frame gap</dt>
            <dd className="m-0 mt-1 font-mono text-sm text-white tabular-nums">
              {milliseconds(profiler.summary.longestFrameGapMs)}
            </dd>
          </div>
          <div className="min-w-0">
            <dt className="text-[11px] text-white/60">Main stage</dt>
            <dd className="m-0 mt-1 font-mono text-xs text-white break-words">
              {profiler.summary.hottestStage ?? "—"}
              {profiler.summary.hottestStageMs !== null && (
                <span className="block tabular-nums text-white/70">
                  {milliseconds(profiler.summary.hottestStageMs)}
                </span>
              )}
            </dd>
          </div>
        </dl>
      )}
      {profiler.error && (
        <p role="alert" className="m-0 text-xs text-red-300">
          {profiler.error}
        </p>
      )}
    </section>
  );
}
