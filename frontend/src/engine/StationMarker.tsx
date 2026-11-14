import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { useHudInsets } from "@/hooks/useHudInsets";
import { useLayoutMode } from "@/hooks/useLayoutMode";
import { audioService } from "@/services/audioService";
import { useEngineStore } from "@/stores/engineStore";
import { Z_INDEX } from "@/constants/zIndex";
import type { StationMarker as Marker } from "./scene.generated";

function Station({ marker, compact }: { marker: Marker; compact: boolean }) {
  const [revealed, setRevealed] = useState(false);
  const [hovered, setHovered] = useState(false);
  const start = useRef<[number, number] | null>(null);
  const button = useRef<HTMLButtonElement>(null);
  const insets = useHudInsets();
  const tooltip = useRef<HTMLDivElement>(null);
  const [height, setHeight] = useState(100);
  useLayoutEffect(() => {
    if (!tooltip.current) {
      return;
    }
    const update = () => setHeight(tooltip.current!.getBoundingClientRect().height);
    update();
    const observer = new ResizeObserver(update);
    observer.observe(tooltip.current);
    return () => observer.disconnect();
  }, [hovered, revealed]);
  useEffect(() => {
    const dismiss = (event: PointerEvent) => {
      if (!button.current?.contains(event.target as Node)) {
        start.current = [event.clientX, event.clientY];
      }
    };
    const end = (event: PointerEvent) => {
      const origin = start.current;
      if (
        origin &&
        !button.current?.contains(event.target as Node) &&
        Math.hypot(event.clientX - origin[0], event.clientY - origin[1]) <= 8
      ) {
        setRevealed(false);
      }
    };
    document.addEventListener("pointerdown", dismiss);
    document.addEventListener("pointerup", end);
    return () => {
      document.removeEventListener("pointerdown", dismiss);
      document.removeEventListener("pointerup", end);
    };
  }, []);
  const visible = hovered || revealed;
  const width = Math.min(208, window.innerWidth - insets.left - insets.right - 24);
  const left = Math.max(
    insets.left + 12,
    Math.min(window.innerWidth - insets.right - width - 12, marker.point[0] + 20),
  );
  const top = Math.max(
    insets.top + 12,
    Math.min(window.innerHeight - insets.bottom - height - 12, marker.point[1] - 44),
  );
  return (
    <div
      className="pointer-events-none fixed inset-0"
      style={{ zIndex: Z_INDEX.GAME_BOARD_OVERLAY }}
    >
      <button
        ref={button}
        type="button"
        aria-label={`Travel to ${marker.details.name}`}
        aria-describedby={visible ? "orbital-station-details" : undefined}
        aria-pressed={compact ? revealed : undefined}
        data-station-marker="true"
        onPointerEnter={() => {
          if (!compact) {
            setHovered(true);
          }
        }}
        onPointerLeave={() => setHovered(false)}
        onFocus={() => setHovered(true)}
        onBlur={() => setHovered(false)}
        onKeyDown={(event) => {
          if (event.key === "Escape") {
            setHovered(false);
            setRevealed(false);
          }
        }}
        onPointerDown={(event) => {
          start.current = [event.clientX, event.clientY];
        }}
        onClick={(event) => {
          const origin = start.current;
          start.current = null;
          if (
            event.detail &&
            origin &&
            Math.hypot(event.clientX - origin[0], event.clientY - origin[1]) > 8
          ) {
            return;
          }
          if (compact && !revealed) {
            setRevealed(true);
            return;
          }
          const state = useEngineStore.getState();
          if (state.focus.type !== "orbital-station") {
            void audioService.playTravelSound();
            state.setFocus({ type: "orbital-station" });
          }
        }}
        className="pointer-events-auto absolute min-h-11 min-w-11 cursor-pointer focus-visible:outline focus-visible:outline-white"
        style={{
          left: marker.point[0],
          top: marker.point[1],
          width: marker.hitSize,
          height: marker.hitSize,
          transform: "translate(-50%, -50%)",
        }}
      />
      {visible && (
        <div
          ref={tooltip}
          id="orbital-station-details"
          role="tooltip"
          className="absolute border border-[rgba(60,60,70,0.7)] bg-[rgba(10,10,15,0.98)] px-3 py-2 text-[11px] leading-tight text-white/90"
          style={{
            left,
            top,
            width,
            clipPath:
              "polygon(0 0, calc(100% - 14px) 0, 100% 14px, 100% 100%, 14px 100%, 0 calc(100% - 14px))",
          }}
        >
          <div className="font-orbitron mb-1 text-xs font-bold text-white wrap-break-word">
            {marker.details.name}
          </div>
          <div className="text-emerald-400">Completed</div>
          <div className="mt-1 text-white/80">
            {compact ? "Tap again to travel" : "Click to travel"}
          </div>
        </div>
      )}
    </div>
  );
}

export default function StationMarker({ gameId }: { gameId: string }) {
  const overlay = useEngineStore((s) => s.overlay);
  const focus = useEngineStore((s) => s.focus);
  const { isCompact } = useLayoutMode();
  if (overlay?.gameId !== gameId || !overlay.station) {
    return null;
  }
  return (
    <Station
      key={`${focus.type}-${focus.type === "body" ? focus.body : ""}-${isCompact}`}
      marker={overlay.station}
      compact={isCompact}
    />
  );
}
