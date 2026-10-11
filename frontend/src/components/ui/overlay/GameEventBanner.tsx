import { useEffect, useRef } from "react";
import { createPortal } from "react-dom";
import type { GameEvent } from "../../../hooks/useGameEvent.ts";
import { Z_INDEX } from "@/constants/zIndex.ts";
import { audioService } from "@/services/audioService.ts";
import ModeEventEffects from "./ModeEventEffects.tsx";

function renderColoredTitle(title: string, playerName: string, playerColor: string) {
  const idx = title.indexOf(playerName);
  if (idx === -1) {
    return title;
  }
  const before = title.slice(0, idx);
  const after = title.slice(idx + playerName.length);
  return (
    <>
      {before}
      <span style={{ color: playerColor }}>{playerName}</span>
      {after}
    </>
  );
}

interface GameEventBannerProps {
  event: GameEvent | null;
  onDismiss: () => void;
}

export default function GameEventBanner({ event, onDismiss }: GameEventBannerProps) {
  useEffect(() => {
    if (!event) {
      return;
    }
    const timer = setTimeout(onDismiss, event.duration);
    return () => clearTimeout(timer);
  }, [event, onDismiss]);

  const lastSoundEvent = useRef<number | null>(null);
  const eventId = event?.id;
  const soundId = event?.soundId;
  useEffect(() => {
    if (eventId === undefined || !soundId || lastSoundEvent.current === eventId) {
      return;
    }
    let playback: HTMLAudioElement | null = null;
    // Defer playback so a cancelled render or Strict Mode effect replay stays silent.
    const timer = setTimeout(() => {
      lastSoundEvent.current = eventId;
      playback = audioService.playSoundWithHandle(soundId);
    }, 0);
    return () => {
      clearTimeout(timer);
      audioService.stopSound(playback);
    };
  }, [eventId, soundId]);

  if (!event) {
    return null;
  }

  const hasSubtitle = event.achievementName && event.playerName;
  const isLarge = event.size === "large";
  const outDelay = Math.max(0, (event.duration - 400) / 1000);
  const animateStyle = {
    animation: `yourTurnIn 0.4s ease-out forwards, yourTurnOut 0.4s ease-in ${outDelay}s forwards`,
  };

  const titleParts =
    isLarge && event.playerName && event.playerColor
      ? renderColoredTitle(event.title, event.playerName, event.playerColor)
      : event.title;

  return createPortal(
    <div
      className="fixed inset-0 flex items-center justify-center pointer-events-none"
      style={{ zIndex: Z_INDEX.STANDARD_MODAL }}
    >
      <div
        key={event.id}
        className="flex flex-col items-center gap-2 max-w-[90vw] text-center motion-reduce:!animate-none"
        style={animateStyle}
        role="status"
        aria-live="polite"
      >
        <ModeEventEffects effects={event.effects} />
        <div
          className={`font-orbitron font-black text-white tracking-[0.2em] compact:!text-xl ${isLarge ? "text-6xl [text-shadow:0_2px_4px_rgba(0,0,0,0.9)]" : "text-4xl [text-shadow:0_0_40px_rgba(100,200,255,0.8),0_0_80px_rgba(100,200,255,0.4),0_2px_4px_rgba(0,0,0,0.9)]"}`}
        >
          {titleParts}
        </div>
        {event.description && (
          <p className="max-w-xl text-sm text-white/85 [text-shadow:0_2px_4px_black]">
            {event.description}
          </p>
        )}
        {hasSubtitle && (
          <div className="text-lg tracking-wide [text-shadow:0_2px_4px_rgba(0,0,0,0.9)]">
            <span className="font-orbitron font-bold text-white">{event.achievementName}</span>
            <span className="text-gray-300"> by </span>
            <span className="font-semibold" style={{ color: event.playerColor }}>
              {event.playerName}
            </span>
          </div>
        )}
      </div>
    </div>,
    document.body,
  );
}
