import GameButton from "@/components/ui/buttons/GameButton.tsx";
import React, { useSyncExternalStore } from "react";
import { useSound } from "../../../contexts/SoundContext.tsx";
import { audioService } from "../../../services/audioService.ts";

const SpeakerIcon: React.FC<{ enabled: boolean; volume: number }> = ({ enabled, volume }) => {
  if (!enabled || volume === 0) {
    return (
      <svg
        className="w-[18px] h-[18px] shrink-0"
        width="18"
        height="18"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5" />
        <line x1="23" y1="9" x2="17" y2="15" />
        <line x1="17" y1="9" x2="23" y2="15" />
      </svg>
    );
  } else if (volume < 0.5) {
    return (
      <svg
        className="w-[18px] h-[18px] shrink-0"
        width="18"
        height="18"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5" />
        <path d="M15.54 8.46a5 5 0 0 1 0 7.07" />
      </svg>
    );
  }
  return (
    <svg
      className="w-[18px] h-[18px] shrink-0"
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5" />
      <path d="M15.54 8.46a5 5 0 0 1 0 7.07" />
      <path d="M19.07 4.93a10 10 0 0 1 0 14.14" />
    </svg>
  );
};

const MusicNoteIcon: React.FC<{ enabled: boolean }> = ({ enabled }) => {
  if (!enabled) {
    return (
      <svg
        className="w-[18px] h-[18px] shrink-0"
        width="18"
        height="18"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M9 18V5l12-2v13" />
        <circle cx="6" cy="18" r="3" />
        <circle cx="18" cy="16" r="3" />
        <line x1="2" y1="2" x2="22" y2="22" />
      </svg>
    );
  }
  return (
    <svg
      className="w-[18px] h-[18px] shrink-0"
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M9 18V5l12-2v13" />
      <circle cx="6" cy="18" r="3" />
      <circle cx="18" cy="16" r="3" />
    </svg>
  );
};

const SoundToggleButton: React.FC = () => {
  const { enabled, musicEnabled, volume, toggleMute, toggleMusicMute } = useSound();
  const music = useSyncExternalStore(audioService.subscribeMusic, audioService.getMusicState);

  return (
    <div className="flex flex-col gap-2 px-4 py-3 text-white">
      <div className="flex items-center gap-2">
        <GameButton
          emphasis="quiet"
          onClick={() => {
            toggleMute();
          }}
          className="!min-h-11 flex-1 !px-2 gap-2 hover:text-white transition-colors"
          aria-label={enabled ? "Mute sound" : "Unmute sound"}
        >
          <SpeakerIcon enabled={enabled} volume={volume} />
          <span className="text-xs">Sound</span>
        </GameButton>
        <GameButton
          emphasis="quiet"
          onClick={() => {
            toggleMusicMute();
          }}
          className="!min-h-11 flex-1 !px-2 gap-2 hover:text-white transition-colors"
          aria-label={musicEnabled ? "Mute music" : "Unmute music"}
        >
          <MusicNoteIcon enabled={musicEnabled} />
          <span className="text-xs">Music</span>
        </GameButton>
      </div>
      <div className="flex flex-col items-center gap-1">
        <div className="flex items-center justify-center gap-1">
          <GameButton
            emphasis="quiet"
            className="!p-0 !min-h-11 w-11 h-11"
            aria-label="Restart or previous song"
            disabled={!music.title}
            onClick={() => audioService.skipMusicTrack(-1)}
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
              <path d="M5 5h2v14H5zM19 5v14L8 12z" />
            </svg>
          </GameButton>
          <GameButton
            emphasis="quiet"
            className="!p-0 !min-h-11 w-11 h-11"
            aria-label={music.playing ? "Pause music" : "Play music"}
            onClick={() => {
              if (!music.playing && !musicEnabled) {
                toggleMusicMute();
              }
              audioService.toggleMusicPlayback();
            }}
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
              <path d={music.playing ? "M6 4h4v16H6zM14 4h4v16h-4z" : "M7 4v16l13-8z"} />
            </svg>
          </GameButton>
          <GameButton
            emphasis="quiet"
            className="!p-0 !min-h-11 w-11 h-11"
            aria-label="Next song"
            disabled={!music.title}
            onClick={() => audioService.skipMusicTrack(1)}
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
              <path d="M17 5h2v14h-2zM5 5l11 7-11 7z" />
            </svg>
          </GameButton>
        </div>
        <div className="font-orbitron text-[11px] text-white/70 text-center" aria-live="polite">
          {music.title ?? "No track selected"}
        </div>
      </div>
    </div>
  );
};

export default SoundToggleButton;
