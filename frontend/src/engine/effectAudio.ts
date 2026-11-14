import { audioService } from "../services/audioService";
import type { SceneSound } from "./scene.generated";

/** One owned audio handle per actual native effect; settings stay in audioService. */
export function effectAudio() {
  const active = new Map<string, HTMLAudioElement | null>();
  let paused = false;
  return {
    dispatch(event: SceneSound) {
      if (event.action === "stop") {
        if (!active.has(event.id)) {
          return;
        }
        audioService.stopNuclearBlastSound(active.get(event.id) ?? null);
        active.delete(event.id);
      } else if (!active.has(event.id)) {
        const audio = audioService.playNuclearBlastSound();
        if (paused) {
          audio?.pause();
        }
        active.set(event.id, audio);
      }
    },
    pause(value: boolean) {
      if (paused === value) {
        return;
      }
      paused = value;
      for (const audio of active.values()) {
        if (paused) {
          audio?.pause();
        } else if (audio && !audio.ended) {
          void audio.play().catch(() => {});
        }
      }
    },
    dispose() {
      for (const audio of active.values()) {
        audioService.stopNuclearBlastSound(audio);
      }
      active.clear();
    },
  };
}
