import FloatingWindow from "./FloatingWindow.tsx";
import React, { useMemo } from "react";
import Sparkline from "./Sparkline.tsx";
import { usePerformanceMetrics } from "@/hooks/usePerformanceMetrics.ts";
import { useWindowDrag, useWindowManager } from "./WindowManager.tsx";
import ProfilerControls from "./ProfilerControls";

interface PerformanceWindowProps {
  isVisible: boolean;
  onClose: () => void;
}

const WINDOW_ID = "performance";
const WINDOW_WIDTH = 420;
const EXCLUDE_SELECTORS = [".perf-content-area"];
const value = (number: number | null | undefined, digits = 0) =>
  number == null ? "—" : number.toLocaleString(undefined, { maximumFractionDigits: digits });

function Counter({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex min-w-0 flex-col gap-1">
      <span className="text-white/60 text-[11px]">{label}</span>
      <span className="font-mono text-sm text-white tabular-nums">{children}</span>
    </div>
  );
}

const PerformanceWindow: React.FC<PerformanceWindowProps> = ({ isVisible, onClose }) => {
  const { position, handleMouseDown } = useWindowDrag({
    windowId: WINDOW_ID,
    width: WINDOW_WIDTH,
    height: () => window.innerHeight * 0.7,
    defaultPosition:
      typeof window !== "undefined"
        ? { x: window.innerWidth - WINDOW_WIDTH - 20, y: 60 }
        : undefined,
    excludeSelectors: EXCLUDE_SELECTORS,
    isVisible,
  });
  const { getZIndex } = useWindowManager();
  const { snapshots, latest } = usePerformanceMetrics(isVisible);
  const browserFps = useMemo(() => snapshots.map((sample) => sample.fps), [snapshots]);
  const browserMemory = useMemo(() => snapshots.map((sample) => sample.jsHeapUsedMB), [snapshots]);
  const engineFps = useMemo(
    () => snapshots.map((sample) => sample.engine?.fps ?? null),
    [snapshots],
  );
  const engineCpu = useMemo(
    () => snapshots.map((sample) => sample.engine?.cpuMs ?? null),
    [snapshots],
  );
  const engine = latest?.engine;
  if (!isVisible) return null;
  const chartWidth = WINDOW_WIDTH - 32;
  return (
    <FloatingWindow
      title="Performance"
      onClose={onClose}
      onMouseDown={handleMouseDown}
      style={{
        top: position.y,
        left: position.x,
        width: WINDOW_WIDTH,
        maxHeight: `calc(100dvh - clamp(16px, ${position.y}px, calc(100dvh - 80px)) - 16px)`,
        zIndex: getZIndex(WINDOW_ID),
      }}
    >
      <div className="perf-content-area flex flex-col gap-4">
        <ProfilerControls />
        <section aria-label="Browser performance" className="flex flex-col gap-3">
          <h4 className="font-orbitron text-xs text-white m-0">Browser / UI</h4>
          <div className="grid grid-cols-2 gap-3">
            <Counter label="Animation-frame cadence">{value(latest?.fps)} FPS</Counter>
            <Counter label="Mean frame interval">{value(latest?.frameTimeMs, 1)} ms</Counter>
          </div>
          <Sparkline
            data={browserFps}
            width={chartWidth}
            height={48}
            color="#00ff88"
            fillColor="rgba(0,255,136,0.1)"
            min={0}
            label="Browser FPS"
            currentValue={value(latest?.fps)}
          />
          <Counter label="JavaScript heap (used / allocated)">
            {latest?.jsHeapUsedMB == null
              ? "Unavailable"
              : `${value(latest.jsHeapUsedMB)} / ${value(latest.jsHeapTotalMB)} MiB`}
          </Counter>
          {latest?.jsHeapUsedMB != null && (
            <Sparkline
              data={browserMemory}
              width={chartWidth}
              height={40}
              color="#00ccff"
              fillColor="rgba(0,204,255,0.1)"
              label="JS heap"
              currentValue={`${value(latest.jsHeapUsedMB)} MiB`}
            />
          )}
        </section>
        <section
          aria-label="Engine performance"
          className="flex flex-col gap-3 border-t border-white/15 pt-4"
        >
          <div className="flex items-center justify-between gap-3">
            <h4 className="font-orbitron text-xs text-white m-0">Bevy engine</h4>
            <span className="text-[11px] text-white/60">
              {engine
                ? `${engine.backend === "webgpu" ? "WebGPU" : "WebGL2"} · ${engine.state}`
                : "Waiting for engine"}
            </span>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <Counter label="Submitted frames">
              {engine?.state === "paused" ? "Paused" : `${value(engine?.fps)} FPS`}
            </Counter>
            <Counter label="Native frame CPU">{value(engine?.cpuMs, 2)} ms</Counter>
          </div>
          <Sparkline
            data={engineFps}
            width={chartWidth}
            height={48}
            color="#00ff88"
            fillColor="rgba(0,255,136,0.1)"
            min={0}
            label="Engine submitted FPS"
            currentValue={engine?.state === "paused" ? "Paused" : value(engine?.fps)}
          />
          <Sparkline
            data={engineCpu}
            width={chartWidth}
            height={48}
            color="#ffd700"
            fillColor="rgba(255,215,0,0.1)"
            min={0}
            label="Native frame CPU"
            currentValue={engine?.cpuMs == null ? "—" : `${value(engine.cpuMs, 2)} ms`}
          />
          <div className="grid grid-cols-2 gap-x-5 gap-y-3">
            <Counter label="Draw calls / last frame">{value(engine?.drawCalls)}</Counter>
            <Counter label="Visible meshes (all views)">{value(engine?.visibleMeshes)}</Counter>
            <Counter label="Submitted triangles">{value(engine?.submittedTriangles)}</Counter>
            <Counter label="Indirect draw calls">{value(engine?.indirectDraws)}</Counter>
            <Counter label="Render passes">{value(engine?.renderPasses)}</Counter>
            <Counter label="Submitted frames (total)">{value(engine?.submittedFrames)}</Counter>
            <Counter label="Resident meshes">{value(engine?.residentMeshes)}</Counter>
            <Counter label="Resident triangles">{value(engine?.residentTriangles)}</Counter>
            <Counter label="Resident vertices">{value(engine?.residentVertices)}</Counter>
            <Counter label="Bevy GPU images / materials">
              {value(engine?.residentImages)} / {value(engine?.residentMaterials)}
            </Counter>
            <Counter label="ECS entities">{value(engine?.entities)}</Counter>
            <Counter label="WASM memory capacity">
              {engine ? `${value(engine.wasmBytes / 1048576, 1)} MiB` : "—"}
            </Counter>
          </div>
        </section>
      </div>
    </FloatingWindow>
  );
};

export default PerformanceWindow;
