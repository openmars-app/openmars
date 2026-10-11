import { assetUrl } from "@/assets";
import { getSoundSettings } from "../utils/soundStorage.ts";

interface AudioFileEntry {
  key: string;
  path: string;
  volumeMultiplier: number;
}

const CONSTRUCTION_SOUNDS = ["construction-1", "construction-2"] as const;

class AudioService {
  private activePlayback = new Map<HTMLAudioElement, string>();
  private audioCache: Map<string, HTMLAudioElement> = new Map();
  private ambientAudio: HTMLAudioElement | null = null;
  private isEnabled: boolean = true;
  private isMusicEnabled: boolean = true;
  private volume: number = 0.5;
  private musicVolume: number = 0.5;
  private volumeMultipliers: Map<string, number> = new Map();
  private ambientVolumeMultiplier: number = 0.3;
  private ambientFadeInterval: ReturnType<typeof setInterval> | null = null;
  private ambientRequested = false;
  private ambientPausedByUser = false;
  private ambientGain = 1;
  private ambientTracks = [
    { title: "Stars", path: assetUrl("audio/music/stars") },
    { title: "Sands", path: assetUrl("audio/music/sands") },
    { title: "Ethereum", path: assetUrl("audio/music/ethereum") },
    { title: "Dreams", path: assetUrl("audio/music/dreams") },
    { title: "Settlers", path: assetUrl("audio/music/settlers") },
  ];
  private musicState: { title: string | null; playing: boolean } = { title: null, playing: false };
  private musicListeners = new Set<() => void>();
  private playOrder: number[] = [];
  private playPosition: number = 0;

  constructor() {
    const settings = getSoundSettings();
    this.isEnabled = settings.enabled;
    this.isMusicEnabled = settings.musicEnabled;
    this.volume = settings.volume;
    this.musicVolume = settings.musicVolume;

    // Empty functions (not null) intercept media keys without triggering playback
    if (navigator.mediaSession) {
      const noop = () => {};
      navigator.mediaSession.setActionHandler("play", noop);
      navigator.mediaSession.setActionHandler("pause", noop);
      navigator.mediaSession.setActionHandler("stop", noop);
      navigator.mediaSession.setActionHandler("seekbackward", noop);
      navigator.mediaSession.setActionHandler("seekforward", noop);
      navigator.mediaSession.setActionHandler("previoustrack", noop);
      navigator.mediaSession.setActionHandler("nexttrack", noop);
    }

    const unlockAudio = () => {
      const ctx = new AudioContext();
      void ctx.resume().then(() => ctx.close());
      document.removeEventListener("click", unlockAudio);
      document.removeEventListener("touchstart", unlockAudio);
      document.removeEventListener("keydown", unlockAudio);
    };
    document.addEventListener("click", unlockAudio);
    document.addEventListener("touchstart", unlockAudio);
    document.addEventListener("keydown", unlockAudio);

    this.preloadAudioFiles();
  }

  private preloadAudioFiles() {
    const audioFiles: AudioFileEntry[] = [
      { key: "production", path: assetUrl("audio/effects/production"), volumeMultiplier: 0.7 },
      {
        key: "temperature-increase",
        path: assetUrl("audio/effects/temperature-increase"),
        volumeMultiplier: 0.56,
      },
      {
        key: "water-placement",
        path: assetUrl("audio/effects/water-placement"),
        volumeMultiplier: 0.7,
      },
      {
        key: "oxygen-increase",
        path: assetUrl("audio/effects/oxygen-increase"),
        volumeMultiplier: 0.7,
      },
      {
        key: "venus-increase",
        path: assetUrl("audio/effects/venus-increase"),
        volumeMultiplier: 1.0,
      },
      { key: "button-hover", path: assetUrl("audio/effects/button-hover"), volumeMultiplier: 0.4 },
      {
        key: "button-click",
        path: assetUrl("audio/effects/button-click"),
        volumeMultiplier: 0.224,
      },
      { key: "card-hover", path: assetUrl("audio/effects/card-hover"), volumeMultiplier: 0.2 },
      { key: "card-played", path: assetUrl("audio/effects/card-played"), volumeMultiplier: 0.4 },
      ...CONSTRUCTION_SOUNDS.map((key) => ({
        key,
        path: assetUrl(`audio/effects/${key}`),
        volumeMultiplier: 1.0,
      })),
      {
        key: "nuclear-blast",
        path: assetUrl("audio/effects/nuclear-blast"),
        volumeMultiplier: 0.8,
      },
      {
        key: "earth-withdrawal-warning",
        path: assetUrl("audio/effects/earth-withdrawal-warning"),
        volumeMultiplier: 0.7,
      },
      {
        key: "earth-contact-lost",
        path: assetUrl("audio/effects/earth-contact-lost"),
        volumeMultiplier: 1.0,
      },
      { key: "your-turn", path: assetUrl("audio/effects/your-turn"), volumeMultiplier: 1.0 },
      { key: "award-funded", path: assetUrl("audio/effects/award-funded"), volumeMultiplier: 1.0 },
      { key: "game-start", path: assetUrl("audio/effects/game-start"), volumeMultiplier: 1.0 },
      {
        key: "player-joined",
        path: assetUrl("audio/effects/player-joined"),
        volumeMultiplier: 1.0,
      },
      { key: "travel", path: assetUrl("audio/effects/travel"), volumeMultiplier: 0.8 },
      {
        key: "production-score",
        path: assetUrl("audio/effects/production-score"),
        volumeMultiplier: 1.0,
      },
    ];

    audioFiles.forEach(({ key, path, volumeMultiplier }) => {
      try {
        const audio = new Audio(path);
        audio.preload = "auto";
        audio.volume = this.volume * volumeMultiplier;

        audio.addEventListener("error", (e) => {
          console.warn(`Failed to preload audio: ${key}`, e);
        });

        this.audioCache.set(key, audio);
        this.volumeMultipliers.set(key, volumeMultiplier);
      } catch (error) {
        console.warn(`Error creating audio element for ${key}:`, error);
      }
    });
  }

  public async playSound(soundKey: string): Promise<void> {
    if (!this.isEnabled) {
      return;
    }

    const audio = this.audioCache.get(soundKey);
    if (!audio) {
      console.warn(`Sound not found: ${soundKey}`);
      return;
    }

    try {
      const audioClone = audio.cloneNode() as HTMLAudioElement;
      const multiplier = this.volumeMultipliers.get(soundKey) ?? 1.0;
      audioClone.volume = this.volume * multiplier;

      await audioClone.play();
    } catch (error) {
      console.warn(`Failed to play sound ${soundKey}:`, error);
    }
  }

  public playSoundWithHandle(soundKey: string): HTMLAudioElement | null {
    if (!this.isEnabled) {
      return null;
    }

    const audio = this.audioCache.get(soundKey);
    if (!audio) {
      return null;
    }

    try {
      const audioClone = audio.cloneNode() as HTMLAudioElement;
      const multiplier = this.volumeMultipliers.get(soundKey) ?? 1.0;
      audioClone.volume = this.volume * multiplier;
      this.activePlayback.set(audioClone, soundKey);
      const release = () => this.activePlayback.delete(audioClone);
      audioClone.addEventListener("ended", release, { once: true });
      audioClone.addEventListener("error", release, { once: true });
      void audioClone.play().catch(release);
      return audioClone;
    } catch {
      return null;
    }
  }

  public async playProductionSound(): Promise<void> {
    return this.playSound("production");
  }

  public async playTemperatureSound(): Promise<void> {
    return this.playSound("temperature-increase");
  }

  public async playWaterPlacementSound(): Promise<void> {
    return this.playSound("water-placement");
  }

  public async playOxygenSound(): Promise<void> {
    return this.playSound("oxygen-increase");
  }

  public async playVenusSound(): Promise<void> {
    return this.playSound("venus-increase");
  }

  public async playButtonHoverSound(): Promise<void> {
    return this.playSound("button-hover");
  }

  public async playButtonClickSound(): Promise<void> {
    return this.playSound("button-click");
  }

  public async playCardHoverSound(): Promise<void> {
    return this.playSound("card-hover");
  }

  public async playCardPlayedSound(): Promise<void> {
    return this.playSound("card-played");
  }

  public stopSound(audio: HTMLAudioElement | null): void {
    if (audio) {
      audio.pause();
      audio.currentTime = 0;
      this.activePlayback.delete(audio);
    }
  }

  public playNuclearBlastSound(): HTMLAudioElement | null {
    return this.playSoundWithHandle("nuclear-blast");
  }

  public stopNuclearBlastSound(audio: HTMLAudioElement | null): void {
    this.stopSound(audio);
  }

  public async playConstructionSound(): Promise<void> {
    const sound = CONSTRUCTION_SOUNDS[Math.floor(Math.random() * CONSTRUCTION_SOUNDS.length)];
    return this.playSound(sound);
  }

  public async playGameStartSound(): Promise<void> {
    return this.playSound("game-start");
  }

  public async playPlayerJoinedSound(): Promise<void> {
    return this.playSound("player-joined");
  }

  public async playTravelSound(): Promise<void> {
    return this.playSound("travel");
  }

  public async playProductionScoreSound(): Promise<void> {
    return this.playSound("production-score");
  }

  private shufflePlayOrder(): void {
    const lastPlayed = this.playOrder.length > 0 ? this.playOrder[this.playOrder.length - 1] : -1;
    const order = this.ambientTracks.map((_, i) => i);
    for (let i = order.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      [order[i], order[j]] = [order[j], order[i]];
    }
    if (order.length > 1 && order[0] === lastPlayed) {
      [order[0], order[1]] = [order[1], order[0]];
    }
    this.playOrder = order;
    this.playPosition = 0;
  }

  private createAmbientAudio(): HTMLAudioElement {
    if (this.playOrder.length === 0) {
      this.shufflePlayOrder();
    }
    const track = this.ambientTracks[this.playOrder[this.playPosition]];
    const audio = new Audio(track.path);
    audio.loop = false;
    this.updateMusicState(track.title, false);
    audio.addEventListener("playing", () => {
      if (this.ambientAudio === audio) {
        this.updateMusicState(track.title, true);
      }
    });
    audio.addEventListener("pause", () => {
      if (this.ambientAudio === audio) {
        this.updateMusicState(track.title, false);
      }
    });
    audio.addEventListener("error", () => {
      if (this.ambientAudio === audio) {
        this.updateMusicState(track.title, false);
      }
    });
    audio.addEventListener("ended", () => {
      if (this.ambientAudio !== audio || !this.ambientRequested) {
        return;
      }
      this.playNextAmbient();
    });
    return audio;
  }

  public getMusicState = () => this.musicState;

  public subscribeMusic = (listener: () => void) => {
    this.musicListeners.add(listener);
    return () => {
      this.musicListeners.delete(listener);
    };
  };

  private updateMusicState(title: string | null, playing: boolean): void {
    if (this.musicState.title === title && this.musicState.playing === playing) {
      return;
    }
    this.musicState = { title, playing };
    this.musicListeners.forEach((listener) => listener());
  }

  public toggleMusicPlayback(): void {
    if (this.musicState.playing) {
      this.ambientPausedByUser = true;
      this.ambientRequested = false;
      this.clearAmbientFade();
      this.ambientAudio?.pause();
      this.updateMusicState(this.musicState.title, false);
    } else {
      this.ambientPausedByUser = false;
      this.playAmbient();
    }
  }

  public skipMusicTrack(direction: -1 | 1): void {
    if (direction === -1 && this.ambientAudio && this.ambientAudio.currentTime > 3) {
      this.ambientAudio.currentTime = 0;
      return;
    }
    const resume = this.ambientRequested && !this.ambientPausedByUser;
    this.selectAmbientTrack(direction);
    if (resume) {
      this.playAmbient();
    }
  }

  private selectAmbientTrack(direction: -1 | 1): void {
    this.clearAmbientFade();
    if (this.ambientAudio) {
      this.ambientAudio.pause();
      this.playPosition += direction;
      if (this.playPosition >= this.playOrder.length) {
        this.shufflePlayOrder();
      } else if (this.playPosition < 0) {
        this.playPosition = this.playOrder.length - 1;
      }
    }
    this.ambientAudio = this.createAmbientAudio();
    this.updateAmbientVolume();
  }

  private clearAmbientFade(): void {
    if (this.ambientFadeInterval !== null) {
      clearInterval(this.ambientFadeInterval);
      this.ambientFadeInterval = null;
    }
  }

  private updateAmbientVolume(): void {
    if (this.ambientAudio) {
      this.ambientAudio.volume = this.musicVolume * this.ambientVolumeMultiplier * this.ambientGain;
    }
  }

  private fadeAmbient(target: number, duration: number, onComplete?: () => void): void {
    this.clearAmbientFade();
    const startGain = this.ambientGain;
    const startedAt = performance.now();
    this.ambientFadeInterval = setInterval(() => {
      const progress = Math.min((performance.now() - startedAt) / duration, 1);
      this.ambientGain = startGain + (target - startGain) * progress;
      this.updateAmbientVolume();
      if (progress === 1) {
        this.clearAmbientFade();
        onComplete?.();
      }
    }, 20);
  }

  public playAmbient(fadeDuration: number = 0): void {
    if (this.ambientPausedByUser) {
      return;
    }
    this.clearAmbientFade();
    this.ambientRequested = true;
    if (!this.ambientAudio) {
      this.ambientAudio = this.createAmbientAudio();
    }
    this.ambientGain = fadeDuration > 0 ? 0 : 1;
    this.updateAmbientVolume();
    if (this.isMusicEnabled) {
      void this.ambientAudio.play().catch(() => {});
    }
    if (fadeDuration > 0) {
      this.fadeAmbient(1, fadeDuration);
    }
  }

  public playNextAmbient(fadeDuration: number = 0): void {
    this.selectAmbientTrack(1);
    this.playAmbient(fadeDuration);
  }

  public stopAmbientWithDuration(duration: number): void {
    this.ambientRequested = false;
    if (this.ambientAudio) {
      this.fadeAmbient(0, duration, () => {
        this.ambientAudio?.pause();
        if (this.ambientAudio) {
          this.ambientAudio.currentTime = 0;
        }
      });
    }
  }

  public setEnabled(enabled: boolean): void {
    this.isEnabled = enabled;
    this.activePlayback.forEach((_, audio) => {
      audio.muted = !enabled;
    });
  }

  public setVolume(volume: number): void {
    this.volume = Math.max(0, Math.min(1, volume));
    this.activePlayback.forEach((soundKey, audio) => {
      audio.volume = this.volume * (this.volumeMultipliers.get(soundKey) ?? 1.0);
    });

    this.audioCache.forEach((audio, key) => {
      const multiplier = this.volumeMultipliers.get(key) ?? 1.0;
      audio.volume = this.volume * multiplier;
    });
  }

  public setMusicVolume(volume: number): void {
    this.musicVolume = Math.max(0, Math.min(1, volume));

    this.updateAmbientVolume();
  }

  public setMusicEnabled(enabled: boolean): void {
    if (this.isMusicEnabled === enabled) {
      return;
    }
    this.isMusicEnabled = enabled;
    if (!enabled) {
      this.ambientAudio?.pause();
    } else if (this.ambientRequested) {
      void this.ambientAudio?.play().catch(() => {});
    }
  }

  public getSettings() {
    return {
      enabled: this.isEnabled,
      musicEnabled: this.isMusicEnabled,
      volume: this.volume,
      musicVolume: this.musicVolume,
    };
  }
}

export const audioService = new AudioService();
export default audioService;
