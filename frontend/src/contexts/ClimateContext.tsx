import { createContext, useContext, useEffect, useMemo, useRef, type ReactNode } from "react";
import { useFrame, useThree } from "@react-three/fiber";
import { performanceStore } from "../services/performanceStore";
import {
  climateFromParameters,
  climateMilestones,
  easeClimate,
  TERRAFORMED_PARAMETERS,
  type ClimateMilestone,
  type ClimateParameters,
  type ClimateState,
} from "../components/game/board/climate";

const PULSE_SECONDS: Record<ClimateMilestone, number> = {
  temperature: 1.5,
  oceans: 3,
  terraformed: 2.5,
};

export interface ClimatePulses {
  // Envelopes in 0..1 that rise quickly and fade out.
  warm: number;
  shimmer: number;
  // Progress of the terraformed sweep in 0..1; 0 when idle.
  sweep: number;
}

export interface ClimateRuntime {
  current: ClimateState;
  pulses: ClimatePulses;
}

interface ClimateContextValue {
  runtime: React.RefObject<ClimateRuntime>;
  target: ClimateState;
}

const TERRAFORMED = climateFromParameters(TERRAFORMED_PARAMETERS);

const ClimateContext = createContext<ClimateContextValue>({
  runtime: {
    current: { current: TERRAFORMED, pulses: { warm: 0, shimmer: 0, sweep: 0 } },
  },
  target: TERRAFORMED,
});

function envelope(elapsed: number, duration: number): number {
  if (elapsed < 0 || elapsed >= duration) {
    return 0;
  }
  const t = elapsed / duration;
  return Math.min(1, t * 8) * (1 - t) * (1 - t);
}

export function ClimateProvider({
  parameters,
  children,
}: {
  parameters: ClimateParameters;
  children: ReactNode;
}) {
  const clock = useThree((s) => s.clock);
  const { temperature, oxygen, oceans, maxOceans } = parameters;
  const target = useMemo(
    () => climateFromParameters({ temperature, oxygen, oceans, maxOceans }),
    [temperature, oxygen, oceans, maxOceans],
  );
  const runtime = useRef<ClimateRuntime>({
    current: { ...target },
    pulses: { warm: 0, shimmer: 0, sweep: 0 },
  });
  const previous = useRef<ClimateParameters | null>(null);
  const milestones = useRef(new Map<ClimateMilestone, number>());

  useEffect(() => {
    const next = { temperature, oxygen, oceans, maxOceans };
    // The first value snaps so loading or reconnecting mid-game never replays milestones.
    if (previous.current) {
      const now = clock.elapsedTime;
      for (const milestone of climateMilestones(previous.current, next)) {
        milestones.current.set(milestone, now);
      }
    }
    previous.current = next;
  }, [temperature, oxygen, oceans, maxOceans, clock]);

  useFrame((_, delta) => {
    const state = runtime.current;
    easeClimate(state.current, target, Math.min(delta, 0.1));
    const now = clock.elapsedTime;
    const pulse = (milestone: ClimateMilestone) => {
      const start = milestones.current.get(milestone);
      return start === undefined ? 0 : envelope(now - start, PULSE_SECONDS[milestone]);
    };
    state.pulses.warm = pulse("temperature");
    state.pulses.shimmer = pulse("oceans");
    const sweepStart = milestones.current.get("terraformed");
    const sweep = sweepStart === undefined ? 1 : (now - sweepStart) / PULSE_SECONDS.terraformed;
    state.pulses.sweep = sweep > 0 && sweep < 1 ? sweep : 0;
    if (performanceStore.sectionDue("Climate")) {
      const c = state.current;
      performanceStore.setSection("Climate", {
        parameters: `${temperature}°C · ${oxygen}% O₂ · ${oceans} oceans`,
        "frost / ice": `${c.frost.toFixed(2)} / ${c.ice.toFixed(2)}`,
        "greening / grass": `${c.greening.toFixed(2)} / ${c.grass.toFixed(2)}`,
        "bush / tree gate": `${c.bush.toFixed(2)} / ${c.tree.toFixed(2)}`,
      });
    }
  }, -1);

  const value = useMemo(() => ({ runtime, target }), [target]);
  return <ClimateContext.Provider value={value}>{children}</ClimateContext.Provider>;
}

export function useClimate() {
  return useContext(ClimateContext);
}
