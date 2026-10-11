import { useCallback } from "react";
import { audioService } from "../services/audioService.ts";

/**
 * Hook for playing sound effects in components
 * Thin wrapper around audioService methods
 */
export function useSoundEffects() {
  const playSound = useCallback((soundKey: string) => {
    return audioService.playSound(soundKey);
  }, []);

  const playProductionSound = useCallback(() => {
    return audioService.playProductionSound();
  }, []);

  const playTemperatureSound = useCallback(() => {
    return audioService.playTemperatureSound();
  }, []);

  const playWaterPlacementSound = useCallback(() => {
    return audioService.playWaterPlacementSound();
  }, []);

  const playOxygenSound = useCallback(() => {
    return audioService.playOxygenSound();
  }, []);

  const playVenusSound = useCallback(() => {
    return audioService.playVenusSound();
  }, []);

  const playButtonHoverSound = useCallback(() => {
    return audioService.playButtonHoverSound();
  }, []);

  const playButtonClickSound = useCallback(() => {
    return audioService.playButtonClickSound();
  }, []);

  const playCardHoverSound = useCallback(() => {
    return audioService.playCardHoverSound();
  }, []);

  const playCardPlayedSound = useCallback(() => {
    return audioService.playCardPlayedSound();
  }, []);

  const playConstructionSound = useCallback(() => {
    return audioService.playConstructionSound();
  }, []);

  const playGameStartSound = useCallback(() => {
    return audioService.playGameStartSound();
  }, []);

  const playPlayerJoinedSound = useCallback(() => {
    return audioService.playPlayerJoinedSound();
  }, []);

  const playProductionScoreSound = useCallback(() => {
    return audioService.playProductionScoreSound();
  }, []);

  return {
    playSound,
    playProductionSound,
    playTemperatureSound,
    playWaterPlacementSound,
    playOxygenSound,
    playVenusSound,
    playButtonHoverSound,
    playButtonClickSound,
    playCardHoverSound,
    playCardPlayedSound,
    playConstructionSound,
    playGameStartSound,
    playPlayerJoinedSound,
    playProductionScoreSound,
  };
}
