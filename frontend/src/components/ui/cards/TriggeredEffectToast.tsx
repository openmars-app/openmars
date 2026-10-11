import StateOperationVisual from "./BehaviorSection/components/StateOperationVisual.tsx";
import React from "react";
import type {
  CalculatedOutputDto,
  CardBehaviorDto,
  TriggeredEffectDto,
  VPConditionDto,
} from "@/types/generated/api-types.ts";
import BehaviorSection from "./BehaviorSection";
import GameIcon from "@/components/ui/display/GameIcon.tsx";
import CardIcon from "./BehaviorSection/components/CardIcon.tsx";
import VictoryPointIcon from "@/components/ui/display/VictoryPointIcon.tsx";

const resourceTypeToIconType: Record<string, string> = {
  credit: "credit",
  steel: "steel",
  titanium: "titanium",
  plant: "plant",
  energy: "energy",
  heat: "heat",
  microbe: "microbe",
  animal: "animal",
  floater: "floater",
  science: "science",
  asteroid: "asteroid",
  fighter: "fighter",
  disease: "disease",
  camp: "camp",
  "credit-production": "credit-production",
  "steel-production": "steel-production",
  "titanium-production": "titanium-production",
  "plant-production": "plant-production",
  "energy-production": "energy-production",
  "heat-production": "heat-production",
  tr: "tr",
  oxygen: "oxygen",
  temperature: "temperature",
  "ocean-placement": "ocean-placement",
  "greenery-placement": "greenery-placement",
  "city-placement": "city-placement",
  "card-draw": "card-draw",
  colony: "colony",
};

const cardResourceTypes: Record<string, "peek" | "take" | "buy" | "discard" | "none"> = {
  "card-draw": "none",
  "card-peek": "peek",
  "card-take": "take",
  "card-buy": "buy",
  "card-discard": "discard",
};

export interface TriggeredEffectItem {
  id: string;
  cardName: string;
  sourceType: string;
  calculatedOutputs: CalculatedOutputDto[];
  behaviors: CardBehaviorDto[];
  vpConditions: VPConditionDto[];
}

export function groupTriggeredEffects(
  effects: TriggeredEffectDto[],
  playerId: string,
): TriggeredEffectItem[] {
  const grouped = new Map<string, TriggeredEffectItem>();
  for (const effect of effects) {
    if (effect.playerId !== playerId) {
      continue;
    }
    const key = `${effect.cardName}::${effect.sourceType}`;
    const existing = grouped.get(key);
    if (existing) {
      if (effect.calculatedOutputs) {
        existing.calculatedOutputs.push(...effect.calculatedOutputs);
      }
      if (effect.behaviors) {
        existing.behaviors.push(...effect.behaviors);
      }
      if (effect.vpConditions) {
        existing.vpConditions.push(...effect.vpConditions);
      }
    } else {
      grouped.set(key, {
        id: `${Date.now()}-${Math.random()}`,
        cardName: effect.cardName,
        sourceType: effect.sourceType || "",
        calculatedOutputs: effect.calculatedOutputs ? [...effect.calculatedOutputs] : [],
        behaviors: effect.behaviors ? [...effect.behaviors] : [],
        vpConditions: effect.vpConditions ? [...effect.vpConditions] : [],
      });
    }
  }

  return Array.from(grouped.values()).filter((item) => {
    const hasOutputs = item.calculatedOutputs.some((o) => o.amount !== 0);
    return hasOutputs || item.vpConditions.length > 0 || item.behaviors.length > 0;
  });
}

const GainedDisplay: React.FC<{
  outputs: CalculatedOutputDto[];
  vpConditions?: VPConditionDto[];
}> = ({ outputs, vpConditions }) => {
  const nonZero = outputs.filter((o) => o.amount !== 0);
  const hasVP = vpConditions && vpConditions.length > 0;
  if (nonZero.length === 0 && !hasVP) {
    return null;
  }

  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <span className="text-[9px] text-gray-400 uppercase tracking-wider font-orbitron">
        Gained:
      </span>
      {nonZero.map((output, index) => {
        if (output.operation) {
          return (
            <span key={index} className="inline-flex [--behavior-icon-size:24px]">
              <StateOperationVisual operation={output.operation} />
            </span>
          );
        }
        const badgeType = cardResourceTypes[output.resourceType];
        if (badgeType !== undefined) {
          return (
            <CardIcon
              key={index}
              amount={Math.abs(output.amount)}
              badgeType={badgeType}
              totalCardTypes={1}
            />
          );
        }
        const iconType = resourceTypeToIconType[output.resourceType] || output.resourceType;
        return (
          <div key={index} className="flex items-center gap-0.5">
            <GameIcon iconType={iconType} amount={output.amount} size="small" />
          </div>
        );
      })}
      {hasVP && <VictoryPointIcon vpConditions={vpConditions} />}
    </div>
  );
};

function ToastDetail({ item }: { item: TriggeredEffectItem }) {
  const hasBehaviors = item.behaviors.length > 0;
  let label: string | null = null;
  if (hasBehaviors && item.sourceType === "action_added") {
    label = "New Action:";
  } else if (hasBehaviors && item.sourceType === "effect_added") {
    label = "New Effect:";
  }
  if (label) {
    return (
      <div className="flex flex-col items-center gap-0.5">
        <span className="text-[9px] text-gray-400 uppercase tracking-wider font-orbitron">
          {label}
        </span>
        <BehaviorSection behaviors={item.behaviors} cardName={item.cardName} />
      </div>
    );
  }
  return (
    <GainedDisplay
      outputs={item.calculatedOutputs}
      vpConditions={item.vpConditions.length > 0 ? item.vpConditions : undefined}
    />
  );
}

interface TriggeredEffectToastProps {
  item: TriggeredEffectItem;
  header?: React.ReactNode;
  style?: React.CSSProperties;
}

export default function TriggeredEffectToast({ item, header, style }: TriggeredEffectToastProps) {
  return (
    <div
      className="notification-content relative flex flex-col items-center gap-1 px-3 py-2 bg-[rgba(10,10,15,0.98)] border border-[rgba(60,60,70,0.7)] shadow-[0_2px_8px_rgba(0,0,0,0.5)]"
      style={{
        clipPath:
          "polygon(0 0, calc(100% - 10px) 0, 100% 10px, 100% 100%, 10px 100%, 0 calc(100% - 10px))",
        ...style,
      }}
    >
      <style>{`
        .notification-content div {
          flex-wrap: nowrap !important;
        }
        .notification-content > div > div {
          background: none !important;
          border-color: transparent !important;
          box-shadow: none !important;
        }
      `}</style>
      <svg
        className="absolute top-0 right-0 w-[10px] h-[10px] pointer-events-none"
        viewBox="0 0 10 10"
      >
        <line x1="0" y1="0" x2="10" y2="10" stroke="rgba(60,60,70,0.7)" strokeWidth="1.5" />
      </svg>
      <svg
        className="absolute bottom-0 left-0 w-[10px] h-[10px] pointer-events-none"
        viewBox="0 0 10 10"
      >
        <line x1="0" y1="0" x2="10" y2="10" stroke="rgba(60,60,70,0.7)" strokeWidth="1.5" />
      </svg>
      {header}
      <span className="text-white text-xs font-bold font-orbitron [text-shadow:0_1px_2px_rgba(0,0,0,0.8)]">
        {item.cardName}
      </span>
      <ToastDetail item={item} />
    </div>
  );
}
