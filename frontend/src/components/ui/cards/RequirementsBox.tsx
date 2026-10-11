import { formatRequirementDescription } from "@/utils/cardDescription.ts";
import { StateSymbol, visibleStateRequirements } from "./CardStateVisuals.tsx";
import React, { useState, useRef, useEffect } from "react";
import { createPortal } from "react-dom";
import GameIcon from "../display/GameIcon.tsx";
import { FormattedDescription } from "../display/FormattedDescription.tsx";
import { CardRequirementsDto, CardTag, ResourceType } from "@/types/generated/api-types.ts";
import { Z_INDEX, getZIndex } from "@/constants/zIndex.ts";
import { useTapReveal } from "@/hooks/useTapReveal.ts";

interface RequirementsBoxProps {
  requirements?: CardRequirementsDto;
  inFlow?: boolean;
}

const RequirementsBox: React.FC<RequirementsBoxProps> = ({ requirements, inFlow = false }) => {
  const [isHovered, setIsHovered] = useState(false);
  const [tooltipPos, setTooltipPos] = useState<{ x: number; y: number } | null>(null);
  const badgeRef = useRef<HTMLDivElement>(null);
  const { open: isRevealed, triggerProps } = useTapReveal<HTMLDivElement>({
    enabled: !!requirements?.description,
  });

  useEffect(() => {
    if (isRevealed && badgeRef.current) {
      const rect = badgeRef.current.getBoundingClientRect();
      setTooltipPos({ x: rect.left, y: rect.bottom });
    }
  }, [isRevealed]);

  if (!requirements || (!requirements.items?.length && !requirements.description)) {
    return null;
  }

  const items = visibleStateRequirements(requirements.items ?? []);

  // Group requirements by type/tag
  const groupRequirements = (requirements: any[]) => {
    const grouped: { [key: string]: any[] } = {};

    requirements.forEach((req) => {
      const key = req.tag || req.affectedTags?.[0] || req.state || req.type;
      if (!grouped[key]) {
        grouped[key] = [];
      }
      grouped[key].push(req);
    });

    return Object.values(grouped);
  };

  const renderRequirementGroup = (group: any[], index: number) => {
    const firstReq = group[0];
    const { type, min, max, amount, affectedTags, tag, resource } = firstReq;
    if (type === "state") {
      return (
        <StateSymbol
          key={index}
          state={firstReq.state}
          value={firstReq.equals}
          className="[--behavior-icon-size:20px]"
        />
      );
    }

    // Determine if it's a tag requirement
    const isTagRequirement = tag || (affectedTags && affectedTags.length > 0);
    // Determine if it's a production requirement
    const isProductionRequirement = type === "production" && resource;
    const permanentProduction = isProductionRequirement && firstReq.permanent;
    // Determine if it's a named resource requirement (e.g., floater, microbe)
    const isNamedResourceRequirement = type === "resource" && resource;
    const key = tag || affectedTags?.[0] || resource || type;

    let resourceType: ResourceType | null = null;
    let cardTag: CardTag | null = null;
    let displayText = "";
    let showMultipleIcons = false;
    let iconCount = 1;

    if (isTagRequirement) {
      cardTag = key;
    } else if (isProductionRequirement) {
      resourceType = !resource.endsWith("-production") ? `${resource}-production` : resource;
    } else if (type === "resource" && resource) {
      // Named resource requirements (e.g., floater, microbe)
      resourceType = resource;
    } else {
      // Regular resource/parameter requirements
      resourceType = type;
    }

    // Determine count and display text
    if (isTagRequirement || isProductionRequirement || isNamedResourceRequirement) {
      if (min !== undefined && min > 0) {
        if (min === 1) {
          iconCount = 1;
          showMultipleIcons = true;
          displayText = "";
        } else {
          iconCount = 1;
          showMultipleIcons = false;
          displayText = `${min}`;
        }
      } else if (max !== undefined) {
        iconCount = 1;
        showMultipleIcons = false;
        displayText = `≤${max}`;
      } else if (amount !== undefined) {
        if (amount === 1) {
          iconCount = 1;
          showMultipleIcons = true;
          displayText = "";
        } else {
          iconCount = 1;
          showMultipleIcons = false;
          displayText = `${amount}`;
        }
      } else {
        iconCount = 1;
        showMultipleIcons = true;
        displayText = "";
      }
    } else {
      // Regular resource requirements - handle parameter-specific formatting

      // Add proper units for global parameters
      if (type === "oxygen") {
        if (min !== undefined && min > 0) {
          displayText = `${min}%+`;
        } else if (max !== undefined) {
          displayText = `≤${max}%`;
        } else if (amount !== undefined) {
          displayText = `${amount}%`;
        }
      } else if (type === "temperature") {
        if (min !== undefined) {
          displayText = `${min}°C+`;
        } else if (max !== undefined) {
          displayText = `≤${max}°C`;
        } else if (amount !== undefined) {
          displayText = `${amount}°C`;
        }
      } else if (type === "venus") {
        if (min !== undefined && min > 0) {
          displayText = `${min}%+`;
        } else if (max !== undefined) {
          displayText = `≤${max}%`;
        } else if (amount !== undefined) {
          displayText = `${amount}%`;
        }
      } else {
        // Regular resources
        if (min !== undefined && min > 0) {
          displayText = `${min}+`;
        } else if (max !== undefined) {
          displayText = `≤${max}`;
        } else if (amount !== undefined) {
          displayText = `${amount}`;
        } else {
          displayText = "";
        }
      }
    }

    return (
      <div
        key={index}
        className="flex items-center gap-px px-0.5 py-px [&:has(span)]:px-1 [&:has(span)]:py-0.5"
      >
        {/* Show amount before icon for tag/production requirements with multiple units */}
        {(isTagRequirement || isProductionRequirement || isNamedResourceRequirement) &&
          displayText &&
          !showMultipleIcons && (
            <span className="text-[11px] font-orbitron font-bold text-white [text-shadow:1px_1px_2px_rgba(0,0,0,0.8)] leading-none">
              {displayText}
            </span>
          )}

        {resourceType || cardTag ? (
          <div
            className={`relative flex items-center gap-px ${permanentProduction ? "pr-1 pb-1" : ""}`}
            data-permanent-production={permanentProduction ? resource : undefined}
          >
            {showMultipleIcons ? (
              // Show multiple icons for single tag/resource requirements
              Array.from({ length: Math.min(iconCount, 4) }, (_, i) => (
                <GameIcon
                  key={i}
                  iconType={cardTag ? `${cardTag}-tag` : (resourceType as string)}
                  size="small"
                />
              ))
            ) : (
              // Show single icon
              <GameIcon
                iconType={cardTag ? `${cardTag}-tag` : (resourceType as string)}
                size="small"
              />
            )}
            {permanentProduction && (
              <span
                className="absolute bottom-0 right-0 inline-flex rounded-sm bg-[#0b1017] [--behavior-icon-size:15px]"
                role="img"
                aria-label="Permanent production"
              >
                <GameIcon iconType="permanent" size="behavior" />
              </span>
            )}
          </div>
        ) : (
          <span className="text-[10px] font-orbitron font-semibold text-white [text-shadow:1px_1px_2px_rgba(0,0,0,0.8)] capitalize">
            {key}
          </span>
        )}

        {/* Show amount after icon for non-tag, non-production requirements */}
        {!isTagRequirement &&
          !isProductionRequirement &&
          !isNamedResourceRequirement &&
          displayText && (
            <span className="text-[11px] font-orbitron font-bold text-white [text-shadow:1px_1px_2px_rgba(0,0,0,0.8)] leading-none">
              {displayText}
            </span>
          )}
      </div>
    );
  };

  let layer = getZIndex("GAME_BOARD_BACKGROUND", -1);
  if (inFlow) {
    layer = Z_INDEX.GAME_BOARD_BASE;
  }
  if (isHovered || isRevealed) {
    layer = Z_INDEX.CARD_HOVER;
  }
  const groupedRequirements = groupRequirements(items);

  return (
    <>
      <div
        {...triggerProps}
        className={`${inFlow ? "relative ml-[10%]" : "absolute bottom-full left-[10%]"} w-fit min-w-[60px] max-w-[80%]`}
        style={{ zIndex: layer }}
        onMouseEnter={() => setIsHovered(true)}
        onMouseLeave={() => setIsHovered(false)}
      >
        <div
          ref={badgeRef}
          className="relative shadow-[0_3px_8px_rgba(0,0,0,0.4)] backdrop-blur-[2px] pl-2 pr-6 py-0.5 border border-b-0 border-[rgba(60,60,70,0.7)] motion-safe:animate-[stripeMove_4s_linear_infinite]"
          style={{
            clipPath: "polygon(0 0, calc(100% - 16px) 0, 100% 16px, 100% 100%, 0 100%)",
            background:
              "linear-gradient(-45deg, #5a2a10 25%, #2d1508 25%, #2d1508 50%, #5a2a10 50%, #5a2a10 75%, #2d1508 75%)",
            backgroundSize: "20px 20px",
          }}
        >
          <div
            className="absolute inset-0 bg-black/40 pointer-events-none"
            style={{ clipPath: "polygon(0 0, calc(100% - 16px) 0, 100% 16px, 100% 100%, 0 100%)" }}
          />
          <svg
            className="absolute top-0 right-0 w-[16px] h-[16px] pointer-events-none"
            viewBox="0 0 16 16"
          >
            <line x1="0" y1="0" x2="16" y2="16" stroke="rgba(60,60,70,0.7)" strokeWidth="1.5" />
          </svg>
          <div className="relative flex items-center justify-start gap-[3px] flex-wrap">
            {groupedRequirements.map((group, index) => renderRequirementGroup(group, index))}
            {items.length === 0 && <GameIcon iconType="effect" size="small" />}
          </div>
        </div>
      </div>
      {isRevealed &&
        requirements.description &&
        tooltipPos &&
        createPortal(
          <div
            className="fixed w-max max-w-44 pt-1 animate-[fadeIn_150ms_ease-in] pointer-events-none"
            style={{ left: tooltipPos.x, top: tooltipPos.y, zIndex: Z_INDEX.FLOATING_TOOLTIP }}
          >
            <div
              className="relative bg-[rgba(10,10,15,0.98)] border border-[rgba(60,60,70,0.7)] text-white/90 text-[11px] leading-tight px-3 py-2 shadow-[0_2px_8px_rgba(0,0,0,0.5)]"
              style={{
                clipPath:
                  "polygon(0 0, calc(100% - 14px) 0, 100% 14px, 100% 100%, 14px 100%, 0 calc(100% - 14px))",
              }}
            >
              <FormattedDescription text={formatRequirementDescription(requirements.description)} />
              <svg
                className="absolute top-0 right-0 w-[14px] h-[14px] pointer-events-none"
                viewBox="0 0 14 14"
              >
                <line x1="0" y1="0" x2="14" y2="14" stroke="rgba(60,60,70,0.7)" strokeWidth="1.5" />
              </svg>
              <svg
                className="absolute bottom-0 left-0 w-[14px] h-[14px] pointer-events-none"
                viewBox="0 0 14 14"
              >
                <line x1="0" y1="0" x2="14" y2="14" stroke="rgba(60,60,70,0.7)" strokeWidth="1.5" />
              </svg>
            </div>
          </div>,
          document.body,
        )}
    </>
  );
};

export default RequirementsBox;
