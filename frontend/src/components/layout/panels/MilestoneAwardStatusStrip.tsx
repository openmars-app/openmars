import React, { useMemo, useRef, useState } from "react";
import { useGameStore } from "@/stores/gameStore.ts";
import { AwardDto, MilestoneDto } from "@/types/generated/api-types.ts";
import EarthSupportProduction, {
  earthSupportLabel,
} from "../../ui/display/EarthSupportProduction.tsx";
import GameIcon from "../../ui/display/GameIcon.tsx";
import RevealTrigger from "../../ui/display/RevealTrigger.tsx";
import AwardScoreboard from "../../ui/display/AwardScoreboard.tsx";
import { FormattedDescription } from "../../ui/display/FormattedDescription.tsx";
import { Z_INDEX } from "@/constants/zIndex.ts";
import { GamePopover } from "../../ui/GamePopover";
import GameButton, { ANGLE_INDENT, BUTTON_SPACING } from "../../ui/buttons/GameButton.tsx";

const SLOTS_PER_SIDE = 3;
const CHIP_WIDTH = 84;
const INNER_CHIP_WIDTH = CHIP_WIDTH - 12;
const CHIP_HEIGHT = 40;
const PLACEHOLDER_COLOR = "#374151";
const MILESTONE_FALLBACK_COLOR = "#ff6b35";
const AWARD_FALLBACK_COLOR = "#f39c12";

interface PlayerInfo {
  id: string;
  name: string;
  color: string;
}

type ChipKind = "milestone" | "award";

const MilestoneAwardStatusStrip: React.FC = () => {
  const game = useGameStore((state) => state.game);
  const [productionOpen, setProductionOpen] = useState(false);
  const supportRef = useRef<HTMLButtonElement>(null);

  const allPlayers: PlayerInfo[] = useMemo(() => {
    if (!game) {
      return [];
    }
    const players: PlayerInfo[] = [];
    if (game.currentPlayer?.id) {
      players.push({
        id: game.currentPlayer.id,
        name: game.currentPlayer.name,
        color: game.currentPlayer.color,
      });
    }
    for (const p of game.otherPlayers ?? []) {
      players.push({ id: p.id, name: p.name, color: p.color });
    }
    return players;
  }, [game]);

  const claimedMilestones = useMemo(
    () => (game?.milestones ?? []).filter((m) => m.isClaimed),
    [game?.milestones],
  );
  const fundedAwards = useMemo(
    () => (game?.awards ?? []).filter((a) => a.isFunded),
    [game?.awards],
  );

  if (!game) {
    return null;
  }

  const renderTooltip = (kind: ChipKind, item: MilestoneDto | AwardDto) => {
    const playerId =
      kind === "milestone" ? (item as MilestoneDto).claimedBy : (item as AwardDto).fundedBy;
    const player = allPlayers.find((p) => p.id === playerId);
    return (
      <div className="flex flex-col gap-2">
        <div className="flex flex-col gap-2">
          <div className="flex items-baseline justify-between gap-3">
            <div className="font-orbitron font-bold text-white text-[12px] leading-tight">
              {item.name}
            </div>
            <div className="text-white/70 text-[10px] leading-tight text-right shrink-0">
              {kind === "milestone" ? "Claimed by " : "Funded by "}
              <span style={{ color: player?.color ?? "#ffffff" }}>{player?.name ?? "Unknown"}</span>
            </div>
          </div>
          {item.description && (
            <div className="text-white/70 text-[10px] leading-tight">
              <FormattedDescription text={item.description} />
            </div>
          )}
        </div>
        {kind === "award" && (
          <AwardScoreboard
            players={allPlayers}
            playerProgress={(item as AwardDto).playerProgress ?? {}}
          />
        )}
      </div>
    );
  };

  // Slot index meaning: 0 = innermost (next to where the two groups meet), SLOTS_PER_SIDE-1 = outermost.
  // Render order for milestones: outermost first (left-to-right places slot 2 → slot 0).
  // Render order for awards: innermost first (left-to-right places slot 0 → slot 2).
  type Connect = "none" | "overlap" | "gap";
  const renderSlot = (kind: ChipKind, slot: number, connect: Connect) => {
    const isMilestone = kind === "milestone";
    const filled = isMilestone ? (claimedMilestones[slot] ?? null) : (fundedAwards[slot] ?? null);
    const isFilled = filled !== null;
    const isInnermost = slot === 0;

    const fallbackColor = isMilestone ? MILESTONE_FALLBACK_COLOR : AWARD_FALLBACK_COLOR;
    const ownerId = isMilestone
      ? (filled as MilestoneDto | null)?.claimedBy
      : (filled as AwardDto | null)?.fundedBy;
    const ownerColor = allPlayers.find((p) => p.id === ownerId)?.color;
    const chipColor = isFilled ? (ownerColor ?? fallbackColor) : PLACEHOLDER_COLOR;

    const slopedLeft = isMilestone ? "slope-right" : "slope-left";
    const slopedRight = isMilestone ? "slope-left" : "slope-right";
    const leftEdge = isMilestone ? slopedLeft : isInnermost ? "flat" : slopedLeft;
    const rightEdge = isMilestone ? (isInnermost ? "flat" : slopedRight) : slopedRight;
    const overlapMargin = -ANGLE_INDENT + BUTTON_SPACING;
    const marginLeft = connect === "none" ? 0 : connect === "gap" ? BUTTON_SPACING : overlapMargin;

    const wrapperClass = isFilled ? "" : "pointer-events-none";
    const iconType = filled?.style?.icon;
    const slotLayer = isMilestone ? slot + 1 : SLOTS_PER_SIDE - slot;

    return (
      <RevealTrigger
        key={`${kind}-${slot}`}
        as="div"
        className={`relative flex items-center ${wrapperClass}`}
        style={{
          marginLeft,
          zIndex: Z_INDEX.UI_BASE + slotLayer,
        }}
        content={filled ? renderTooltip(kind, filled) : null}
        placement="below"
        maxWidth={isMilestone ? 260 : 320}
      >
        <GameButton
          shape="toolbar"
          surface="console"
          className="hud-status-slot !cursor-default"
          clickSound={false}
          data-occupied={isFilled || undefined}
          tabIndex={isFilled ? 0 : -1}
          aria-label={filled?.name}
          emphasis="secondary"
          width={isInnermost ? INNER_CHIP_WIDTH : CHIP_WIDTH}
          height={CHIP_HEIGHT}
          accent={chipColor}
          leftEdge={leftEdge}
          rightEdge={rightEdge}
        >
          {iconType ? <GameIcon iconType={iconType} size="small" /> : null}
        </GameButton>
      </RevealTrigger>
    );
  };

  const supportLabel = earthSupportLabel(game);
  const milestoneOrder = [2, 1, 0];
  const awardOrder = [0, 1, 2];

  return (
    <>
      <div className="relative flex items-center pointer-events-auto">
        {milestoneOrder.map((slot, i) =>
          renderSlot("milestone", slot, i === 0 ? "none" : "overlap"),
        )}
        {awardOrder.map((slot, i) => renderSlot("award", slot, i === 0 ? "gap" : "overlap"))}
        {supportLabel && (
          <GameButton
            ref={supportRef}
            shape="toolbar"
            surface="console"
            emphasis="secondary"
            leftEdge="slope-right"
            rightEdge="slope-right"
            height={32}
            className="absolute top-full mt-0.5 !text-[11px] !font-normal"
            style={
              {
                left: 21,
                width: "calc(100% - 42px)",
                "--button-angle": "16px",
              } as React.CSSProperties
            }
            selected={productionOpen}
            aria-expanded={productionOpen}
            aria-label="Earth support and your production"
            onClick={() => setProductionOpen((open) => !open)}
          >
            {supportLabel}
          </GameButton>
        )}
      </div>
      <GamePopover
        isVisible={productionOpen}
        onClose={() => setProductionOpen(false)}
        position={{ type: "anchor", anchorRef: supportRef, placement: "below", align: "center" }}
        theme="menu"
        width={320}
        header={{ title: "Your production", showCloseButton: true }}
      >
        <EarthSupportProduction production={game.currentPlayer?.modeProduction} />
      </GamePopover>
    </>
  );
};

export default MilestoneAwardStatusStrip;
