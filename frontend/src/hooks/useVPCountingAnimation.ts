import { useCallback, useEffect, useRef, useState } from "react";
import type { FinalScoreDto } from "../types/generated/api-types";
import type { VPCountingState, VPPhase } from "../contexts/VPCountingContext";

interface VPStep {
  vpIncrement: number;
  highlightTiles?: string[];
  secondaryTiles?: string[];
  delayMs?: number;
  keepHighlights?: boolean;
}

interface VPPhaseConfig extends VPPhase {
  extractSteps: (score: FinalScoreDto) => VPStep[];
}

const VP_PHASES: VPPhaseConfig[] = [
  {
    id: "tr",
    label: "TR",
    color: "#f59e0b",
    extractSteps: (score) => [
      {
        vpIncrement: score.vpBreakdown.terraformRating,
      },
    ],
  },
  {
    id: "greeneries",
    label: "Greeneries",
    color: "#22c55e",
    extractSteps: (score) =>
      (score.vpBreakdown.greeneryVPDetails ?? []).map((detail) => ({
        vpIncrement: detail.vp,
        highlightTiles: [detail.coordinate],
        delayMs: 1100,
      })),
  },
  {
    id: "cities",
    label: "Cities",
    color: "#6366f1",
    extractSteps: (score) => {
      const steps: VPStep[] = [];
      const details = (score.vpBreakdown.cityVPDetails ?? []).filter(
        (d) => (d.adjacentGreeneries ?? []).length > 0,
      );
      for (let c = 0; c < details.length; c++) {
        const detail = details[c];
        const greeneries = detail.adjacentGreeneries ?? [];
        steps.push({
          vpIncrement: 0,
          highlightTiles: [detail.cityCoordinate],
          delayMs: 1200,
          keepHighlights: true,
        });
        for (let i = 0; i < greeneries.length; i++) {
          const isLast = i === greeneries.length - 1;
          steps.push({
            vpIncrement: 1,
            highlightTiles: [detail.cityCoordinate],
            secondaryTiles: greeneries.slice(0, i + 1),
            delayMs: isLast ? 400 + 500 : 400,
            keepHighlights: !isLast,
          });
        }
        if (c < details.length - 1) {
          steps.push({
            vpIncrement: 0,
            delayMs: 500,
          });
        }
      }
      return steps;
    },
  },
  {
    id: "cards",
    label: "Cards",
    color: "#ec4899",
    extractSteps: (score) => {
      if (score.vpBreakdown.cardVP === 0) {
        return [];
      }
      return [{ vpIncrement: score.vpBreakdown.cardVP }];
    },
  },
  {
    id: "milestones",
    label: "Milestones",
    color: "#14b8a6",
    extractSteps: (score) => {
      if (score.vpBreakdown.milestoneVP === 0) {
        return [];
      }
      return [{ vpIncrement: score.vpBreakdown.milestoneVP }];
    },
  },
  {
    id: "awards",
    label: "Awards",
    color: "#a855f7",
    extractSteps: (score) => {
      if (score.vpBreakdown.awardVP === 0) {
        return [];
      }
      return [{ vpIncrement: score.vpBreakdown.awardVP }];
    },
  },
];

const BETWEEN_PLAYERS_MS = 600;
const BEFORE_PLAYER_MS = 600;
const AFTER_PLAYER_MS = 500;
const BETWEEN_PHASES_MS = 1200;
const PHASE_START_PAUSE_MS = 700;
const INITIAL_PAUSE_MS = 800;
const FINISHING_FADE_MS = 1000;
const BAR_BASE_MS = 800;
const BAR_MS_PER_PERCENT = 30;

function computeAccumulatedVPThroughPhase(
  scores: FinalScoreDto[],
  throughPhaseIndex: number,
): Record<string, number> {
  const accumulated: Record<string, number> = {};
  for (const score of scores) {
    accumulated[score.playerId] = 0;
  }
  for (let i = 0; i <= throughPhaseIndex && i < VP_PHASES.length; i++) {
    const phase = VP_PHASES[i];
    for (const score of scores) {
      const steps = phase.extractSteps(score);
      for (const step of steps) {
        accumulated[score.playerId] += step.vpIncrement;
      }
    }
  }
  return accumulated;
}

const INITIAL_STATE: VPCountingState = {
  isActive: false,
  currentPhaseIndex: -1,
  currentPhaseId: null,
  activePlayerId: null,
  playerAccumulatedVP: {},
  highlightedTiles: new Set(),
  secondaryHighlightedTiles: new Set(),
  isFinishing: false,
  isComplete: false,
};

export function useVPCountingAnimation(gameId: string | null, scores: FinalScoreDto[]) {
  const [state, setState] = useState<VPCountingState & { gameId: string | null }>({
    ...INITIAL_STATE,
    gameId: null,
  });
  const input = useRef({ gameId, scores });
  const running = useRef<AbortController | null>(null);

  useEffect(() => {
    input.current = { gameId, scores };
  }, [gameId, scores]);

  const cancel = useCallback(() => {
    running.current?.abort();
    running.current = null;
  }, []);

  const skip = useCallback(() => {
    cancel();
    const { gameId, scores } = input.current;

    const finalVP: Record<string, number> = {};
    for (const score of scores) {
      finalVP[score.playerId] = score.vpBreakdown.totalVP;
    }

    setState({
      gameId,
      isActive: false,
      currentPhaseIndex: VP_PHASES.length - 1,
      currentPhaseId: VP_PHASES[VP_PHASES.length - 1].id,
      activePlayerId: null,
      playerAccumulatedVP: finalVP,
      highlightedTiles: new Set(),
      secondaryHighlightedTiles: new Set(),
      isFinishing: false,
      isComplete: true,
    });
  }, [cancel]);

  const goToPhase = useCallback(
    (phaseIndex: number) => {
      cancel();
      const { gameId, scores } = input.current;

      const clamped = Math.max(0, Math.min(phaseIndex, VP_PHASES.length - 1));
      const accumulated = computeAccumulatedVPThroughPhase(scores, clamped);

      setState({
        gameId,
        isActive: false,
        currentPhaseIndex: clamped,
        currentPhaseId: VP_PHASES[clamped].id,
        activePlayerId: null,
        playerAccumulatedVP: accumulated,
        highlightedTiles: new Set(),
        secondaryHighlightedTiles: new Set(),
        isFinishing: false,
        isComplete: true,
      });
    },
    [cancel],
  );

  const start = useCallback(() => {
    cancel();
    const { gameId, scores } = input.current;
    if (!gameId || scores.length === 0) {
      return;
    }
    const controller = new AbortController();
    running.current = controller;
    const { signal } = controller;
    const sortedScores = [...scores].sort((a, b) => b.vpBreakdown.totalVP - a.vpBreakdown.totalVP);

    const initialVP: Record<string, number> = {};
    for (const score of scores) {
      initialVP[score.playerId] = 0;
    }

    const maxVP = Math.max(...scores.map((s) => s.vpBreakdown.totalVP), 1);

    setState({
      gameId,
      isActive: true,
      currentPhaseIndex: -1,
      currentPhaseId: null,
      activePlayerId: null,
      playerAccumulatedVP: initialVP,
      highlightedTiles: new Set(),
      secondaryHighlightedTiles: new Set(),
      isFinishing: false,
      isComplete: false,
    });

    const accumulated = { ...initialVP };

    const stepDelayMs = (vpIncrement: number): number => {
      const pct = (vpIncrement / maxVP) * 100;
      return BAR_BASE_MS + pct * BAR_MS_PER_PERCENT;
    };

    const runSequence = async () => {
      await delay(INITIAL_PAUSE_MS, signal);
      if (signal.aborted) {
        return;
      }

      let isFirstVisiblePhase = true;

      for (let phaseIdx = 0; phaseIdx < VP_PHASES.length; phaseIdx++) {
        if (signal.aborted) {
          return;
        }

        const phase = VP_PHASES[phaseIdx];

        const hasAnySteps = sortedScores.some((s) => phase.extractSteps(s).length > 0);
        if (!hasAnySteps) {
          continue;
        }

        setState((prev) => ({
          ...prev,
          currentPhaseIndex: phaseIdx,
          currentPhaseId: phase.id,
          activePlayerId: null,
          highlightedTiles: new Set(),
          secondaryHighlightedTiles: new Set(),
        }));

        if (!isFirstVisiblePhase) {
          await delay(BETWEEN_PHASES_MS, signal);
          if (signal.aborted) {
            return;
          }
        }
        isFirstVisiblePhase = false;

        await delay(PHASE_START_PAUSE_MS, signal);
        if (signal.aborted) {
          return;
        }

        for (let playerIdx = 0; playerIdx < sortedScores.length; playerIdx++) {
          if (signal.aborted) {
            return;
          }

          const score = sortedScores[playerIdx];
          const steps = phase.extractSteps(score);

          if (steps.length === 0) {
            continue;
          }

          setState((prev) => ({
            ...prev,
            activePlayerId: score.playerId,
          }));

          await delay(BEFORE_PLAYER_MS, signal);
          if (signal.aborted) {
            return;
          }

          for (const step of steps) {
            if (signal.aborted) {
              return;
            }

            accumulated[score.playerId] += step.vpIncrement;

            setState((prev) => ({
              ...prev,
              playerAccumulatedVP: { ...accumulated },
              highlightedTiles: new Set(step.highlightTiles ?? []),
              secondaryHighlightedTiles: new Set(step.secondaryTiles ?? []),
            }));

            const ms = step.delayMs ?? stepDelayMs(step.vpIncrement);
            await delay(ms, signal);
            if (signal.aborted) {
              return;
            }

            if (!step.keepHighlights) {
              setState((prev) => ({
                ...prev,
                highlightedTiles: new Set(),
                secondaryHighlightedTiles: new Set(),
              }));
            }
          }

          await delay(AFTER_PLAYER_MS, signal);
          if (signal.aborted) {
            return;
          }

          if (playerIdx < sortedScores.length - 1) {
            await delay(BETWEEN_PLAYERS_MS, signal);
          }
        }
      }

      if (!signal.aborted) {
        setState((prev) => ({
          ...prev,
          currentPhaseIndex: -1,
          currentPhaseId: null,
          activePlayerId: null,
          highlightedTiles: new Set(),
          secondaryHighlightedTiles: new Set(),
          isFinishing: true,
        }));

        await delay(FINISHING_FADE_MS, signal);

        if (!signal.aborted) {
          setState((prev) => ({
            ...prev,
            isActive: false,
            isFinishing: false,
            isComplete: true,
          }));
        }
      }
    };

    void runSequence();
  }, [cancel]);

  const hasScores = scores.length > 0;
  useEffect(() => {
    // Start only after the results render commits. An interrupted render must not
    // leave a running timer behind with its initial active state discarded.
    if (gameId && hasScores) {
      start();
    } else {
      setState({ ...INITIAL_STATE, gameId: null });
    }
    return cancel;
  }, [gameId, hasScores, start, cancel]);

  const phases: VPPhase[] = VP_PHASES.map(({ id, label, color }) => ({ id, label, color }));

  return {
    state: gameId && hasScores && state.gameId === gameId ? state : INITIAL_STATE,
    start,
    skip,
    goToPhase,
    phases,
  };
}

function delay(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    const finish = () => {
      clearTimeout(timer);
      signal.removeEventListener("abort", finish);
      resolve();
    };
    const timer = setTimeout(finish, ms);
    signal.addEventListener("abort", finish, { once: true });
    if (signal.aborted) {
      finish();
    }
  });
}
