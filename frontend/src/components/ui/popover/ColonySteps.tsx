import React from "react";
import { ColonyStepDto, ColonyOutputDto } from "@/types/generated/api-types.ts";
import { getZIndex } from "@/constants/zIndex.ts";
import GameIcon from "../display/GameIcon.tsx";
import RevealTrigger from "../display/RevealTrigger.tsx";

interface ColonyStepsProps {
  steps: ColonyStepDto[];
  markerPosition: number;
  previewPosition?: number;
  playerColonies: string[];
  maxSlots: number;
  getPlayerColor: (id: string) => string;
  getPlayerName: (id: string) => string;
}

type StepPattern =
  | "same-resource-varying"
  | "same-resource-all-one"
  | "mixed-all-one"
  | "mixed-varying";

function analyzeSteps(steps: ColonyStepDto[]): {
  pattern: StepPattern;
  sameType: string | null;
} {
  const types = new Set<string>();
  const amounts = new Set<number>();

  for (const step of steps) {
    for (const output of step.outputs) {
      types.add(output.type);
      amounts.add(output.amount);
    }
  }

  const sameType = types.size === 1 ? [...types][0] : null;
  const allOne = amounts.size === 1 && amounts.has(1);

  if (sameType && allOne) {
    return { pattern: "same-resource-all-one", sameType };
  }
  if (sameType) {
    return { pattern: "same-resource-varying", sameType };
  }
  if (allOne) {
    return { pattern: "mixed-all-one", sameType: null };
  }
  return { pattern: "mixed-varying", sameType: null };
}

function mapOutputTypeToIcon(outputType: string): string {
  const mapping: Record<string, string> = {
    credit: "credit",
    steel: "steel",
    titanium: "titanium",
    plant: "plant",
    energy: "energy",
    heat: "heat",
    "credit-production": "credit-production",
    "steel-production": "steel-production",
    "titanium-production": "titanium-production",
    "plant-production": "plant-production",
    "energy-production": "energy-production",
    "heat-production": "heat-production",
    "card-draw": "card-draw",
    microbe: "microbe",
    animal: "animal",
    floater: "floater",
    "ocean-placement": "ocean",
  };
  return mapping[outputType] ?? outputType;
}

function getStepAmount(outputs: ColonyOutputDto[]): number {
  return outputs.reduce((sum, o) => sum + o.amount, 0);
}

function isCreditType(type: string): boolean {
  return type === "credit" || type === "credit-production";
}

const ColonySteps: React.FC<ColonyStepsProps> = ({
  steps,
  markerPosition,
  previewPosition,
  playerColonies,
  maxSlots,
  getPlayerColor,
  getPlayerName,
}) => {
  const { pattern, sameType } = analyzeSteps(steps);

  const stepCount = steps.length;
  const hasMarker = markerPosition >= 0 && markerPosition < stepCount;
  const markerLeftPercent = (markerPosition / stepCount) * 100;

  return (
    <div className="w-full">
      {/* Colony slots above first N step positions + MAX indicator */}
      <div className="flex w-full mb-1">
        {steps.map((_, i) => (
          <div key={i} className="flex-1 flex justify-center">
            {i < maxSlots && (
              <RevealTrigger
                as="div"
                className={`relative w-4 h-4 rounded-sm cursor-default pointer-coarse:before:absolute pointer-coarse:before:-inset-2 pointer-coarse:before:content-[''] ${
                  playerColonies[i] ? "" : "border border-white/20"
                }`}
                style={{
                  backgroundColor: playerColonies[i]
                    ? getPlayerColor(playerColonies[i])
                    : "transparent",
                }}
                content={
                  <div className="flex items-center gap-1.5">
                    {playerColonies[i] && (
                      <div
                        className="w-2 h-2 rounded-full shrink-0"
                        style={{ backgroundColor: getPlayerColor(playerColonies[i]) }}
                      />
                    )}
                    <span className="font-orbitron font-bold text-[10px]">
                      {playerColonies[i] ? getPlayerName(playerColonies[i]) : "Empty colony slot"}
                    </span>
                  </div>
                }
                cornerSize={8}
                maxWidth={220}
              />
            )}
            {i === steps.length - 1 && markerPosition === steps.length - 1 && (
              <span className="text-[9px] font-orbitron font-bold text-white mt-2">MAX</span>
            )}
          </div>
        ))}
      </div>

      {/* Step boxes with sliding marker overlay */}
      <div className="relative flex w-full">
        {/* Animated light region covering steps up to marker */}
        {hasMarker && (
          <div
            className="absolute top-0 h-full pointer-events-none bg-white/5 rounded-l"
            style={{
              zIndex: getZIndex("LOCAL", 5),
              width: `${((markerPosition + 1) / stepCount) * 100}%`,
              transition: "width 500ms cubic-bezier(0.4, 0, 0.2, 1)",
            }}
          />
        )}

        {/* Sliding marker highlight */}
        {hasMarker && (
          <div
            data-colony-marker
            className="absolute top-0 h-full pointer-events-none ring-1 ring-white bg-white/25 rounded-sm"
            style={{
              zIndex: getZIndex("LOCAL", 10),
              width: `${100 / stepCount}%`,
              left: `${markerLeftPercent}%`,
              transition: "left 500ms cubic-bezier(0.4, 0, 0.2, 1)",
            }}
          />
        )}

        {/* Boosted marker from Trade Envoys */}
        {hasMarker &&
          previewPosition !== undefined &&
          previewPosition >= 0 &&
          previewPosition < stepCount &&
          previewPosition !== markerPosition && (
            <div
              className="absolute top-0 h-full pointer-events-none ring-1 ring-amber-400/60 bg-amber-400/15 rounded-sm"
              style={{
                zIndex: getZIndex("LOCAL", 8),
                width: `${100 / stepCount}%`,
                left: `${(previewPosition / stepCount) * 100}%`,
                transition: "left 500ms cubic-bezier(0.4, 0, 0.2, 1)",
              }}
            />
          )}

        {steps.map((step, i) => {
          const isFirst = i === 0;
          const isLast = i === steps.length - 1;
          const roundingClass = isFirst ? "rounded-l" : isLast ? "rounded-r" : "";

          return (
            <div
              key={i}
              className={`flex-1 flex items-center justify-center py-1 text-[10px] font-orbitron font-bold min-h-[24px] ${roundingClass} bg-white/[0.02] text-white/50 relative`}
              style={{ zIndex: getZIndex("LOCAL", 20) }}
            >
              {pattern === "same-resource-all-one" && sameType && (
                <GameIcon
                  iconType={mapOutputTypeToIcon(sameType)}
                  amount={isCreditType(sameType) ? 1 : undefined}
                  size="small"
                />
              )}
              {pattern === "same-resource-varying" && sameType && isCreditType(sameType) && (
                <GameIcon
                  iconType={mapOutputTypeToIcon(sameType)}
                  amount={getStepAmount(step.outputs)}
                  size="small"
                />
              )}
              {pattern === "mixed-all-one" &&
                step.outputs.map((o, j) => (
                  <GameIcon
                    key={j}
                    iconType={mapOutputTypeToIcon(o.type)}
                    amount={isCreditType(o.type) ? o.amount : undefined}
                    size="small"
                  />
                ))}
              {pattern === "mixed-varying" &&
                step.outputs.map((o, j) => {
                  const useAmountProp = isCreditType(o.type);
                  return (
                    <span key={j} className="inline-flex items-center gap-0.5">
                      {!useAmountProp && <span>{o.amount}</span>}
                      <GameIcon
                        iconType={mapOutputTypeToIcon(o.type)}
                        amount={useAmountProp ? o.amount : undefined}
                        size="small"
                      />
                    </span>
                  );
                })}
            </div>
          );
        })}
      </div>

      {/* Numbers underneath (only for same-resource-varying with non-credit types) */}
      {pattern === "same-resource-varying" && sameType && !isCreditType(sameType) && (
        <div className="flex w-full mt-0.5">
          {steps.map((step, i) => (
            <div
              key={i}
              className={`flex-1 text-center text-[11px] font-orbitron font-bold ${
                i === markerPosition ? "text-white" : "text-white/70"
              }`}
            >
              {getStepAmount(step.outputs)}
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

export function getTradeExpression(
  steps: ColonyStepDto[],
): { type: string; icon: string; isCreditType: boolean } | null {
  const { pattern, sameType } = analyzeSteps(steps);

  if (sameType) {
    const isCreditType = sameType === "credit" || sameType === "credit-production";
    if (pattern === "same-resource-all-one") {
      return { type: "icon-only", icon: mapOutputTypeToIcon(sameType), isCreditType };
    }
    return { type: "x-icon", icon: mapOutputTypeToIcon(sameType), isCreditType };
  }
  return null;
}

export { mapOutputTypeToIcon };
export default ColonySteps;
