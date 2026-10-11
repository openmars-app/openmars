import type { ComponentType } from "react";
import type { ModePresentationEffectDto } from "@/types/generated/api-types.ts";
import { useReducedMotion } from "@/hooks/useReducedMotion.ts";

interface EffectProps {
  effect: ModePresentationEffectDto;
}

function LinkTransition({ effect }: EffectProps) {
  const reducedMotion = useReducedMotion();
  const link = effect.link;
  if (!link?.from || !link.to) {
    return null;
  }
  const joined = "M20 20 H80 M80 20 H140";
  const broken = "M20 20 H64 M96 20 H140";
  const destination = link.connected ? joined : broken;
  const origin = link.connected ? broken : joined;
  return (
    <div
      className="flex items-center gap-3 mb-2 font-orbitron text-xs uppercase tracking-widest text-sky-100 compact:gap-2"
      aria-hidden="true"
      data-presentation-effect="link-transition"
    >
      <span>{link.from}</span>
      <svg width="160" height="40" viewBox="0 0 160 40" className="max-w-[28vw]">
        <circle cx="12" cy="20" r="7" fill="none" stroke="currentColor" strokeWidth="2" />
        <circle cx="148" cy="20" r="7" fill="none" stroke="currentColor" strokeWidth="2" />
        <path d={destination} stroke="currentColor" strokeWidth="2" fill="none">
          {!reducedMotion && (
            <animate attributeName="d" from={origin} to={destination} dur="0.9s" fill="freeze" />
          )}
        </path>
        {!link.connected && (
          <path d="M74 14 L86 26 M86 14 L74 26" stroke="#fbbf24" strokeWidth="2">
            {!reducedMotion && (
              <animate attributeName="opacity" from="0" to="1" dur="0.9s" fill="freeze" />
            )}
          </path>
        )}
        {link.connected && !reducedMotion && (
          <circle cx="140" cy="20" r="3" fill="currentColor" opacity="0">
            <animate attributeName="cx" from="20" to="140" begin="0.9s" dur="0.8s" />
            <animate attributeName="opacity" values="0;1;1;0" begin="0.9s" dur="0.8s" />
          </circle>
        )}
      </svg>
      <span>{link.to}</span>
    </div>
  );
}

const effectRenderers: Record<string, ComponentType<EffectProps>> = {
  "link-transition": LinkTransition,
};

export default function ModeEventEffects({ effects }: { effects?: ModePresentationEffectDto[] }) {
  return effects?.map((effect, index) => {
    const Renderer = effectRenderers[effect.kind];
    if (!Renderer) {
      return null;
    }
    return <Renderer key={`${effect.kind}-${index}`} effect={effect} />;
  });
}
