import { useLayoutEffect, useRef, useState } from "react";
import { useLayoutMode } from "@/hooks/useLayoutMode";
import { useEngineStore } from "@/stores/engineStore";
import { audioService } from "@/services/audioService";
import { Z_INDEX, getZIndex } from "@/constants/zIndex";
import type { BodyId, BodyMarker } from "./scene.generated";
import { useHudInsets } from "@/hooks/useHudInsets";
import { placeSolarLabels, type OverlayObstacle } from "./solarLabelLayout";

const NAMES: Partial<Record<BodyId, string>> = {
  mars: "Mars",
  mercury: "Mercury",
  venus: "Venus",
  earth: "Earth",
  ceres: "Ceres",
  jupiter: "Jupiter",
  saturn: "Saturn",
  uranus: "Uranus",
  neptune: "Neptune",
};

function Marker({
  marker,
  compact,
  selected,
  select,
  x,
  y,
  width,
  height,
}: {
  marker: BodyMarker;
  compact: boolean;
  selected: boolean;
  select: () => void;
  x: number;
  y: number;
  width: number;
  height: number;
}) {
  const [hovered, setHovered] = useState(false);
  const emphasized = compact ? selected : hovered;
  const start = useRef<[number, number] | null>(null);
  const pointerDown = (event: React.PointerEvent) => {
    start.current = [event.clientX, event.clientY];
  };
  const click = (event: React.MouseEvent) => {
    const previous = start.current;
    start.current = null;
    if (
      event.detail &&
      previous &&
      Math.hypot(event.clientX - previous[0], event.clientY - previous[1]) > 8
    ) {
      return;
    }
    select();
  };
  return (
    <>
      {Math.hypot(x - marker.point[0], y - marker.point[1] + (compact ? 24 : 14)) > 2 && (
        <svg className="absolute inset-0 h-full w-full overflow-visible" aria-hidden="true">
          <line
            x1={marker.point[0]}
            y1={marker.point[1]}
            x2={Math.max(x - width / 2, Math.min(x + width / 2, marker.point[0]))}
            y2={Math.max(y - height / 2, Math.min(y + height / 2, marker.point[1]))}
            stroke={emphasized ? "#ffffff80" : "#ffffff40"}
            strokeWidth={1}
          />
        </svg>
      )}
      <button
        type="button"
        aria-label={`Travel to ${NAMES[marker.body]}`}
        aria-pressed={compact ? selected : undefined}
        data-solar-body={marker.body}
        onPointerEnter={() => setHovered(true)}
        onPointerLeave={() => setHovered(false)}
        onFocus={() => setHovered(true)}
        onBlur={() => setHovered(false)}
        onPointerDown={pointerDown}
        onClick={click}
        className={`pointer-events-auto absolute flex min-h-11 min-w-11 cursor-pointer flex-col items-center justify-center whitespace-nowrap px-3 py-2 focus-visible:outline focus-visible:outline-white ${compact && selected ? "bg-black/60 outline outline-1 outline-white/50" : ""}`}
        style={{
          left: x,
          top: y,
          width,
          height,
          zIndex: getZIndex("LOCAL", 1),
          transform: "translate(-50%, -50%)",
          opacity: compact || emphasized ? 1 : 0.7,
          transition: "opacity 0.2s",
          textShadow: "0 0 8px rgba(0,0,0,0.9)",
        }}
      >
        <span
          className={`font-orbitron font-bold tracking-wider ${compact ? "text-[13px]" : "text-[11px]"}`}
          style={{ color: marker.body === "mars" ? "#ff6b4a" : "white" }}
        >
          {NAMES[marker.body]}
        </span>
        <span className="mt-0.5 text-[11px] text-white/80" style={{ opacity: emphasized ? 1 : 0 }}>
          {compact ? "Tap again to travel" : "Click to travel"}
        </span>
      </button>
      <button
        type="button"
        tabIndex={-1}
        aria-hidden="true"
        data-solar-marker={marker.body}
        onPointerEnter={() => setHovered(true)}
        onPointerLeave={() => setHovered(false)}
        onPointerDown={pointerDown}
        onClick={click}
        className="pointer-events-auto absolute cursor-pointer"
        style={{
          left: marker.point[0],
          top: marker.point[1],
          width: marker.hitSize,
          height: marker.hitSize,
          transform: "translate(-50%, -50%)",
        }}
      />
    </>
  );
}

function Labels({ markers, compact }: { markers: BodyMarker[]; compact: boolean }) {
  const [selected, setSelected] = useState<BodyId | null>(null);
  const insets = useHudInsets();
  const [obstacles, setObstacles] = useState<OverlayObstacle[]>([]);
  useLayoutEffect(() => {
    const controls = document.querySelector('[aria-label="Board camera"]');
    if (!controls) {
      return;
    }
    const update = () => {
      const { x, y, width, height } = controls.getBoundingClientRect();
      setObstacles([{ x, y, width, height }]);
    };
    update();
    const observer = new ResizeObserver(update);
    observer.observe(controls);
    window.addEventListener("resize", update);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", update);
    };
  }, []);
  const labels = placeSolarLabels(
    markers,
    compact,
    window.innerWidth,
    window.innerHeight,
    insets,
    obstacles,
  );
  return (
    <div
      className="pointer-events-none fixed inset-0"
      style={{ zIndex: Z_INDEX.GAME_BOARD_OVERLAY }}
      aria-label="Solar system destinations"
    >
      {labels.map(({ marker, x, y, width, height }) => (
        <Marker
          key={marker.body}
          marker={marker}
          x={x}
          y={y}
          width={width}
          height={height}
          compact={compact}
          selected={selected === marker.body}
          select={() => {
            if (compact && selected !== marker.body) {
              setSelected(marker.body);
              return;
            }
            void audioService.playTravelSound();
            useEngineStore.getState().setFocus({ type: "body", body: marker.body });
          }}
        />
      ))}
    </div>
  );
}

export default function SolarLabels({ gameId }: { gameId: string }) {
  const overlay = useEngineStore((state) => state.overlay);
  const focus = useEngineStore((state) => state.focus);
  const { isCompact } = useLayoutMode();
  if (focus.type !== "solar-system" || overlay?.gameId !== gameId || !overlay.bodies.length) {
    return null;
  }
  return <Labels key={String(isCompact)} compact={isCompact} markers={overlay.bodies} />;
}
