import { CardStatePresentation } from "./CardStateVisuals.tsx";
import CardTagList from "./CardTagList.tsx";
import React, { useRef, useEffect, useCallback } from "react";
import GameIcon from "../display/GameIcon.tsx";
import CardDecorBar from "../display/CardDecorBar.tsx";
import BehaviorSection from "./BehaviorSection";

import {
  CardDto,
  CardBehaviorDto,
  ResourceTypeCredit,
  ResourceTypeSteel,
  ResourceTypeTitanium,
  ResourceTypePlant,
  ResourceTypeEnergy,
  ResourceTypeHeat,
} from "../../../types/generated/api-types.ts";
import { getCorporationLogo } from "@/utils/corporationLogos.tsx";
import { getCorporationBorderColor } from "@/utils/corporationColors.ts";
import { CardDescriptionSections } from "../display/CardDescriptionSections.tsx";
import { useSoundEffects } from "@/hooks/useSoundEffects.ts";
import { Z_INDEX } from "@/constants/zIndex.ts";
import { CardChassis, CardPanelCircuit } from "./GameCard.tsx";

interface CorporationCardProps {
  card: CardDto;
  isSelected: boolean;
  onSelect: (cardId: string) => void;
  showCheckbox?: boolean;
  borderColor?: string;
  disableInteraction?: boolean;
  catalog?: boolean;
}

const behaviorCache = new WeakMap<CardBehaviorDto[], CardBehaviorDto[]>();

function filterBehaviors(behaviors: CardBehaviorDto[] | undefined) {
  if (!behaviors || behaviors.length === 0) {
    return [];
  }
  const cached = behaviorCache.get(behaviors);
  if (cached) {
    return cached;
  }
  const filtered = behaviors.filter((behavior) => {
    const isAutoCorporationStart = behavior.triggers?.some(
      (t) => t.type === "auto-corporation-start",
    );
    const isAutoCorporationFirstAction = behavior.triggers?.some(
      (t) => t.type === "auto-corporation-first-action",
    );
    if (isAutoCorporationStart) {
      return behavior.triggers?.some((t) => t.condition !== undefined) ?? false;
    }
    if (isAutoCorporationFirstAction) {
      return false;
    }
    return true;
  });
  behaviorCache.set(behaviors, filtered);
  return filtered;
}

const CorporationCard: React.FC<CorporationCardProps> = ({
  card,
  isSelected,
  onSelect,
  showCheckbox = false,
  borderColor,
  disableInteraction = false,
  catalog = false,
}) => {
  const { playCardHoverSound } = useSoundEffects();
  const pendingSoundRef = useRef(false);

  useEffect(() => {
    if (pendingSoundRef.current) {
      pendingSoundRef.current = false;
      void playCardHoverSound();
    }
  }, [isSelected, playCardHoverSound]);

  const effectiveBorderColor = borderColor || getCorporationBorderColor(card.name);

  const handleClick = useCallback(() => {
    pendingSoundRef.current = true;
    onSelect(card.id);
  }, [onSelect, card.id]);

  const renderResource = (type: string, amount: number) => {
    const resourceTypeMap: { [key: string]: string } = {
      credits: ResourceTypeCredit,
      steel: ResourceTypeSteel,
      titanium: ResourceTypeTitanium,
      plants: ResourceTypePlant,
      energy: ResourceTypeEnergy,
      heat: ResourceTypeHeat,
    };
    const resourceType = resourceTypeMap[type];
    if (!resourceType) {
      return null;
    }
    return (
      <GameIcon amountPlacement="badge" iconType={resourceType} amount={amount} size="large" />
    );
  };

  const renderProduction = (type: string, amount: number) => {
    const resourceTypeMap: { [key: string]: string } = {
      credits: ResourceTypeCredit,
      steel: ResourceTypeSteel,
      titanium: ResourceTypeTitanium,
      plants: ResourceTypePlant,
      energy: ResourceTypeEnergy,
      heat: ResourceTypeHeat,
    };
    const resourceType = resourceTypeMap[type];
    if (!resourceType) {
      return null;
    }
    return (
      <GameIcon
        amountPlacement="badge"
        iconType={`${resourceType}-production`}
        amount={amount}
        size="medium"
      />
    );
  };

  const getAutoCorporationFirstAction = (behaviors: CardBehaviorDto[] | undefined) => {
    if (!behaviors) {
      return null;
    }
    return behaviors.find((behavior) =>
      behavior.triggers?.some((t) => t.type === "auto-corporation-first-action"),
    );
  };

  const getAutoCorporationStart = (behaviors: CardBehaviorDto[] | undefined) => {
    if (!behaviors) {
      return null;
    }
    return behaviors.find((behavior) =>
      behavior.triggers?.some((t) => t.type === "auto-corporation-start" && !t.condition),
    );
  };

  const getStartingResourcesFromBehaviors = (behaviors: CardBehaviorDto[] | undefined) => {
    if (!behaviors) {
      return null;
    }
    const startBehavior = getAutoCorporationStart(behaviors);
    if (!startBehavior?.outputs) {
      return null;
    }
    const resources: Record<string, number> = {};
    for (const output of startBehavior.outputs) {
      if (output.type === "credit") {
        resources.credits = output.amount;
      } else if (output.type === "steel") {
        resources.steel = output.amount;
      } else if (output.type === "titanium") {
        resources.titanium = output.amount;
      } else if (output.type === "plant") {
        resources.plants = output.amount;
      } else if (output.type === "energy") {
        resources.energy = output.amount;
      } else if (output.type === "heat") {
        resources.heat = output.amount;
      }
    }
    return Object.keys(resources).length > 0 ? resources : null;
  };

  const getStartingProductionFromBehaviors = (behaviors: CardBehaviorDto[] | undefined) => {
    if (!behaviors) {
      return null;
    }
    const startBehavior = getAutoCorporationStart(behaviors);
    if (!startBehavior?.outputs) {
      return null;
    }
    const production: Record<string, number> = {};
    for (const output of startBehavior.outputs) {
      if (output.type === "credit-production") {
        production.credits = output.amount;
      } else if (output.type === "steel-production") {
        production.steel = output.amount;
      } else if (output.type === "titanium-production") {
        production.titanium = output.amount;
      } else if (output.type === "plant-production") {
        production.plants = output.amount;
      } else if (output.type === "energy-production") {
        production.energy = output.amount;
      } else if (output.type === "heat-production") {
        production.heat = output.amount;
      }
    }
    return Object.keys(production).length > 0 ? production : null;
  };

  const renderAutoCorporationFirstAction = (behavior: CardBehaviorDto) => {
    if (!behavior.outputs || behavior.outputs.length === 0) {
      return null;
    }
    return (
      <BehaviorSection
        behaviors={[behavior]}
        showTooltips
        hideActionChip
        noContainer
        layout="corporation"
      />
    );
  };

  const startingResources =
    card.startingResources || getStartingResourcesFromBehaviors(card.behaviors);
  const startingProduction =
    card.startingProduction || getStartingProductionFromBehaviors(card.behaviors);
  const firstAction = getAutoCorporationFirstAction(card.behaviors);
  const filteredBehaviors = filterBehaviors(card.behaviors);
  const hasStartingSection = startingResources || startingProduction || firstAction;
  const hasBehaviors = filteredBehaviors.length > 0;
  const hasTags = card.tags && card.tags.length > 0;
  const hasVpOrStorage =
    (card.vpConditions && card.vpConditions.length > 0) || card.resourceStorage;

  return (
    <CardStatePresentation card={card}>
      <div
        data-card-name={card.name}
        className={`corporation-card relative isolate ${catalog ? "w-full" : "w-[400px]"} min-h-[380px] p-4 transition-colors duration-200 group select-none [&_*]:select-none ${disableInteraction ? "" : "cursor-pointer"}`}
        data-module-state={isSelected ? "armed" : "idle"}
        onDragStart={(event) => event.preventDefault()}
        style={
          {
            "--card-accent": effectiveBorderColor,
            "--module-decoration-layer": Z_INDEX.CONTROL_DECORATION,
            zIndex: Z_INDEX.GAME_BOARD_BASE,
          } as React.CSSProperties
        }
        onClick={disableInteraction ? undefined : handleClick}
      >
        <CardChassis showConnector={!showCheckbox} />

        {/* Left accent stripe */}
        <div className="game-card-stripe" aria-hidden="true">
          <span />
        </div>

        {/* VP + Resource Storage - bottom right */}
        {hasVpOrStorage && (
          <div className="absolute bottom-0 right-0" style={{ zIndex: Z_INDEX.GAME_BOARD_EFFECTS }}>
            <CardDecorBar
              vpConditions={card.vpConditions}
              resourceStorage={card.resourceStorage}
              corner="top-left"
            />
          </div>
        )}

        {/* Corporation logo area */}
        <div className="corporation-card-logo relative mb-1 px-[34px] py-[22px] flex justify-center items-center h-[152px]">
          {getCorporationLogo(card.name, "w-full max-w-[264px] h-[108px]")}
        </div>

        {/* Tags on right side */}
        {hasTags && (
          <CardTagList
            card={card}
            size="card"
            className="corporation-card-tags absolute top-[38%] right-3 flex flex-col gap-1 items-center pointer-events-auto"
            style={{ zIndex: Z_INDEX.GAME_BOARD_EFFECTS }}
          />
        )}

        {/* Content sections */}
        <div className="relative mt-2">
          {/* Section 1: Starting resources/production */}
          {hasStartingSection && (
            <div className="corporation-card-resources flex flex-wrap gap-2 justify-center items-center py-2">
              <CardPanelCircuit />
              {startingResources &&
                Object.entries(startingResources).map(([type, amount]) =>
                  amount && amount > 0 ? (
                    <div key={type} className="flex items-center">
                      {renderResource(type, amount)}
                    </div>
                  ) : null,
                )}
              {startingProduction &&
                Object.entries(startingProduction).map(([type, amount]) =>
                  amount && amount > 0 ? (
                    <div key={type} className="flex items-center">
                      {renderProduction(type, amount)}
                    </div>
                  ) : null,
                )}
              {firstAction && (
                <div className="flex items-center">
                  {renderAutoCorporationFirstAction(firstAction)}
                </div>
              )}
            </div>
          )}

          {/* Faded divider */}
          {hasStartingSection && hasBehaviors && (
            <div
              className="h-px mx-4 my-1"
              style={{
                background:
                  "linear-gradient(to right, transparent, rgba(70,80,89,0.3), transparent)",
              }}
            />
          )}

          {/* Section 2: Behaviors */}
          {hasBehaviors && (
            <div className="corporation-card-behaviors py-2">
              <CardPanelCircuit />
              <div className="relative [&>div]:static [&>div]:!bottom-auto [&>div]:!left-auto [&>div]:!right-auto">
                <BehaviorSection behaviors={filteredBehaviors} showTooltips layout="corporation" />
              </div>
            </div>
          )}

          {/* Description */}
          {card.description.length > 0 && (
            <div className="corporation-card-description text-xs text-white/80 leading-[1.4] text-center mt-1 px-2">
              <CardDescriptionSections sections={card.description} />
            </div>
          )}
        </div>

        {/* Selection checkbox */}
        {showCheckbox && (
          <div
            className="absolute -bottom-3 left-1/2 -translate-x-1/2"
            style={{ zIndex: Z_INDEX.GAME_BOARD_EFFECTS }}
          >
            <div
              className="w-6 h-6 rounded-full border-2 flex items-center justify-center transition-all duration-300"
              style={{
                backgroundColor: isSelected
                  ? `color-mix(in srgb, ${effectiveBorderColor} 20%, #1a1508)`
                  : "#1a1508",
                borderColor: isSelected ? effectiveBorderColor : `${effectiveBorderColor}4d`,
              }}
            >
              {isSelected && <span className="text-white text-sm font-bold">✓</span>}
            </div>
          </div>
        )}
      </div>
    </CardStatePresentation>
  );
};

export default CorporationCard;
