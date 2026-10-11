import StateOperationVisual from "../../cards/BehaviorSection/components/StateOperationVisual.tsx";
import React, { useEffect, useMemo } from "react";
import type {
  CalculatedOutputDto,
  CardBehaviorDto,
  GameDto,
  StateDiffDto,
} from "@/types/generated/api-types.ts";
import GameIcon from "@/components/ui/display/GameIcon.tsx";
import VictoryPointIcon from "@/components/ui/display/VictoryPointIcon.tsx";
import CardIcon from "@/components/ui/cards/BehaviorSection/components/CardIcon.tsx";
import BehaviorSection from "@/components/ui/cards/BehaviorSection";
import { getZIndex } from "@/constants/zIndex.ts";
import type { ContentDensity } from "./density.ts";
import { serverLink } from "@/utils/gateway.ts";

interface DensityStyle {
  dividerLabel: string;
  dividerCount: string;
  groupBody: string;
  turnHeader: string;
  turnName: string;
  turnCount: string;
  entry: string;
  entryPlayer: string;
  entrySource: string;
  actionBadge: string;
  gainedLabel: string;
  detail: string;
}

const DENSITY_STYLES: Record<ContentDensity, DensityStyle> = {
  popover: {
    dividerLabel: "text-[10px]",
    dividerCount: "text-[9px]",
    groupBody: "p-2",
    turnHeader: "py-1.5 px-3",
    turnName: "text-[10px]",
    turnCount: "text-[9px]",
    entry: "py-2 px-3",
    entryPlayer: "text-xs",
    entrySource: "text-sm",
    actionBadge: "text-[8px]",
    gainedLabel: "text-[10px]",
    detail: "text-xs",
  },
  screen: {
    dividerLabel: "text-[12px]",
    dividerCount: "text-[11px]",
    groupBody: "px-3 py-2",
    turnHeader: "py-2 px-3",
    turnName: "text-[12px]",
    turnCount: "text-[11px]",
    entry: "py-2.5 px-3",
    entryPlayer: "text-[13px]",
    entrySource: "text-[15px]",
    actionBadge: "text-[11px]",
    gainedLabel: "text-[11px]",
    detail: "text-[13px]",
  },
};

const resourceTypeToIconType: Record<string, string> = {
  credits: "credit",
  steel: "steel",
  titanium: "titanium",
  plants: "plant",
  energy: "energy",
  heat: "heat",
  "credits-production": "credit-production",
  "steel-production": "steel-production",
  "titanium-production": "titanium-production",
  "plants-production": "plant-production",
  "energy-production": "energy-production",
  "heat-production": "heat-production",
  tr: "tr",
  oxygen: "oxygen",
  temperature: "temperature",
  "ocean-placement": "ocean-placement",
  "greenery-placement": "greenery-placement",
  "city-placement": "city-placement",
};

const cardResourceTypes: Record<string, "peek" | "take" | "buy" | "discard" | "none"> = {
  "card-draw": "none",
  "card-peek": "peek",
  "card-take": "take",
  "card-buy": "buy",
  "card-discard": "discard",
};

const TILE_PLACEMENT_TYPES = ["ocean-placement", "greenery-placement", "city-placement"];
const GLOBAL_PARAMETER_TYPES = ["temperature", "oxygen"];

const BEHAVIOR_OUTPUT_TYPES = new Set([...TILE_PLACEMENT_TYPES, ...GLOBAL_PARAMETER_TYPES]);

const BEHAVIOR_WRAPPER_CLASS =
  "[&>div]:!relative [&>div]:!bottom-auto [&>div]:!left-auto [&>div]:!right-auto [&>div]:w-full [&>div:hover]:!transform-none [&>div:hover]:!shadow-none [&>div:hover]:!filter-none scale-90 origin-left";

const CalculatedOutputsDisplay: React.FC<{
  outputs: CalculatedOutputDto[];
  showAll?: boolean;
  excludeBehaviors?: boolean;
  styles: DensityStyle;
}> = ({ outputs, showAll = false, excludeBehaviors = false, styles }) => {
  const outputsToShow = outputs.filter(
    (o) =>
      (showAll || o.isScaled) &&
      o.amount !== 0 &&
      (!excludeBehaviors || !BEHAVIOR_OUTPUT_TYPES.has(o.resourceType)),
  );

  if (outputsToShow.length === 0) {
    return null;
  }

  return (
    <div className="mt-1 flex flex-wrap items-center gap-2 px-1">
      <span className={`${styles.gainedLabel} text-gray-400 uppercase tracking-wider`}>
        Gained:
      </span>
      {outputsToShow.map((output, index) => {
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
    </div>
  );
};

interface PlayerTurnGroup {
  playerId: string;
  entries: StateDiffDto[];
}

interface LogGroup {
  generation: number;
  playerTurns: PlayerTurnGroup[];
}

function hexToPlayerStyle(hex: string): { border: string; bg: string; text: string } {
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  return {
    border: `rgba(${r}, ${g}, ${b}, 0.4)`,
    bg: `rgba(${r}, ${g}, ${b}, 0.05)`,
    text: hex,
  };
}

const DEFAULT_PLAYER_STYLE = {
  border: "rgba(100, 200, 255, 0.4)",
  bg: "rgba(100, 200, 255, 0.05)",
  text: "#64c8ff",
};

function groupLogsByGeneration(logs: StateDiffDto[]): LogGroup[] {
  const groups: LogGroup[] = [];
  let currentGeneration = 1;

  for (const log of logs) {
    if (log.changes?.generation) {
      currentGeneration = log.changes.generation.new;
    }

    let genGroup = groups.find((g) => g.generation === currentGeneration);
    if (!genGroup) {
      genGroup = { generation: currentGeneration, playerTurns: [] };
      groups.push(genGroup);
    }

    const lastTurn = genGroup.playerTurns[genGroup.playerTurns.length - 1];
    if (lastTurn && lastTurn.playerId === log.playerId) {
      lastTurn.entries.push(log);
    } else {
      genGroup.playerTurns.push({ playerId: log.playerId, entries: [log] });
    }
  }

  return groups;
}

const GenerationDivider: React.FC<{
  generation: number;
  entryCount: number;
  styles: DensityStyle;
}> = ({ generation, entryCount, styles }) => (
  <div
    className="sticky top-0 flex items-center gap-2 py-2 px-3 bg-[#1a1a2e] border-b border-[rgba(100,200,255,0.3)]"
    style={{ zIndex: getZIndex("LOCAL", 10) }}
  >
    <div className="flex items-center gap-1.5">
      <span className={`${styles.dividerLabel} font-bold uppercase tracking-wider text-[#64c8ff]`}>
        Generation {generation}
      </span>
    </div>
    <div className="flex-1 h-px bg-gradient-to-r from-[rgba(100,200,255,0.3)] to-transparent" />
    <span className={`${styles.dividerCount} text-gray-500`}>{entryCount} actions</span>
  </div>
);

interface PlayerTurnSectionProps {
  playerName: string;
  entries: StateDiffDto[];
  playerColor: string | undefined;
  playerNames: Map<string, string>;
  styles: DensityStyle;
}

const PlayerTurnSection: React.FC<PlayerTurnSectionProps> = ({
  playerName,
  entries,
  playerColor,
  playerNames,
  styles,
}) => {
  const color = playerColor ? hexToPlayerStyle(playerColor) : DEFAULT_PLAYER_STYLE;

  return (
    <div
      className="rounded-lg mb-2 overflow-hidden"
      style={{
        borderLeft: `3px solid ${color.border}`,
        backgroundColor: color.bg,
      }}
    >
      <div
        className={`flex items-center gap-2 ${styles.turnHeader}`}
        style={{ borderBottom: `1px solid ${color.border}` }}
      >
        <div className="w-1.5 h-1.5 rounded-full" style={{ backgroundColor: color.text }} />
        <span
          className={`${styles.turnName} font-semibold uppercase tracking-wider`}
          style={{ color: color.text }}
        >
          {playerName}
        </span>
        <span className={`${styles.turnCount} text-gray-500`}>
          {entries.length} action{entries.length !== 1 ? "s" : ""}
        </span>
      </div>
      <div className="flex flex-col">
        {entries.map((diff) => (
          <LogEntry
            key={diff.sequenceNumber}
            diff={diff}
            playerNames={playerNames}
            styles={styles}
          />
        ))}
      </div>
    </div>
  );
};

type ChoiceDisplay =
  | { type: "none" }
  | { type: "within-behavior"; behavior: CardBehaviorDto }
  | { type: "between-behaviors" };

function getChoiceDisplay(
  diff: StateDiffDto,
  behaviors: CardBehaviorDto[],
  isCardPlay: boolean,
): ChoiceDisplay {
  if (diff.choiceIndex === undefined || diff.choiceIndex === null) {
    return { type: "none" };
  }
  if (behaviors.length === 1 && behaviors[0].choices && behaviors[0].choices.length > 0) {
    return { type: "within-behavior", behavior: behaviors[0] };
  }
  if (isCardPlay && behaviors.length > 1) {
    return { type: "between-behaviors" };
  }
  return { type: "none" };
}

function BehaviorChoices({
  diff,
  behaviors,
  choiceDisplay,
}: {
  diff: StateDiffDto;
  behaviors: CardBehaviorDto[];
  choiceDisplay: ChoiceDisplay;
}) {
  if (choiceDisplay.type === "within-behavior") {
    const base = choiceDisplay.behavior;
    return (
      <div className="mt-1 flex flex-col gap-1">
        {(base.choices ?? []).map((choice, choiceIndex) => {
          const isChosen = choiceIndex === diff.choiceIndex;
          const syntheticBehavior = {
            ...base,
            choices: undefined,
            inputs: choice.inputs,
            outputs: choice.outputs,
          };
          return (
            <div
              key={choiceIndex}
              className={`${BEHAVIOR_WRAPPER_CLASS} ${!isChosen ? "opacity-40 grayscale" : ""}`}
            >
              <BehaviorSection behaviors={[syntheticBehavior]} greyOutAll={!isChosen} />
            </div>
          );
        })}
      </div>
    );
  }
  if (choiceDisplay.type === "between-behaviors") {
    return (
      <div className="mt-1 flex flex-col gap-1">
        {behaviors.map((behavior, index) => {
          const isChosen = index === diff.choiceIndex;
          return (
            <div
              key={index}
              className={`${BEHAVIOR_WRAPPER_CLASS} ${!isChosen ? "opacity-40 grayscale" : ""}`}
            >
              <BehaviorSection behaviors={[behavior]} greyOutAll={!isChosen} />
            </div>
          );
        })}
      </div>
    );
  }
  if (behaviors.length === 0) {
    return null;
  }
  return (
    <div className={`mt-1 ${BEHAVIOR_WRAPPER_CLASS}`}>
      <BehaviorSection behaviors={behaviors} />
    </div>
  );
}

const LogEntry: React.FC<{
  diff: StateDiffDto;
  playerNames: Map<string, string>;
  styles: DensityStyle;
}> = ({ diff, playerNames, styles }) => {
  const isCardPlay = diff.sourceType === "card_play";
  const isCardAction = diff.sourceType === "card_action";
  const isResourceConvert = diff.sourceType === "resource_convert";
  const isGameEvent = diff.sourceType === "game_event";
  const isCardSource = isCardPlay || isCardAction;
  const isBehaviorSource = isResourceConvert || isGameEvent;
  const displayData = diff.displayData;

  const playerName = playerNames.get(diff.playerId) || "Unknown";

  const cardTags = displayData?.tags || [];
  const vpConditions = displayData?.vpConditions || [];
  const behaviorsToShow = useMemo(() => displayData?.behaviors || [], [displayData?.behaviors]);

  const choiceDisplay = useMemo(
    () => getChoiceDisplay(diff, behaviorsToShow, isCardPlay),
    [diff, behaviorsToShow, isCardPlay],
  );

  return (
    <div
      className={`relative flex flex-col gap-1 ${styles.entry} hover:bg-white/5 rounded transition-colors border-b border-[rgba(100,200,255,0.2)] last:border-b-0`}
    >
      <div className="flex items-center gap-2">
        <span className={`${styles.entryPlayer} text-[#64c8ff] font-medium shrink-0`}>
          {playerName}
        </span>
        <span className={`${styles.entrySource} text-white truncate font-medium`}>
          {diff.source}
        </span>
        {isCardAction && (
          <span
            className={`bg-[linear-gradient(135deg,rgba(100,200,255,0.3)_0%,rgba(80,160,220,0.4)_100%)] text-[#64c8ff] ${styles.actionBadge} font-semibold uppercase tracking-[0.3px] py-0.5 px-1.5 rounded-lg border border-[rgba(100,200,255,0.4)] shrink-0`}
          >
            action
          </span>
        )}
        {isCardPlay && cardTags.length > 0 && (
          <div className="flex items-center gap-1 shrink-0">
            {cardTags.map((tag, i) => (
              <GameIcon key={i} iconType={tag} size="small" />
            ))}
          </div>
        )}
        {isCardPlay && vpConditions.length > 0 && (
          <div className="shrink-0">
            <VictoryPointIcon vpConditions={vpConditions} />
          </div>
        )}
      </div>

      <BehaviorChoices diff={diff} behaviors={behaviorsToShow} choiceDisplay={choiceDisplay} />

      {displayData?.revealedCards?.map((card, index) => (
        <div key={`${index}-${card.cardId}`} className={`${styles.entrySource} text-white/80`}>
          Revealed and discarded:{" "}
          <a
            className="text-[#64c8ff] underline cursor-pointer"
            href={serverLink(`/cards?cId=${encodeURIComponent(card.cardId)}`)}
            target="_blank"
            rel="noreferrer"
          >
            {card.name}
          </a>
          {card.matched ? " — matched" : " — no match"}
        </div>
      ))}
      {diff.calculatedOutputs && diff.calculatedOutputs.length > 0 && (
        <CalculatedOutputsDisplay
          outputs={diff.calculatedOutputs}
          showAll={!isCardSource || isCardAction}
          excludeBehaviors={isBehaviorSource}
          styles={styles}
        />
      )}

      {!displayData && !isBehaviorSource && (
        <div className={`${styles.detail} text-gray-400`}>{diff.description}</div>
      )}
    </div>
  );
};

interface GameLogListProps {
  logs: StateDiffDto[];
  gameState?: GameDto;
  density: ContentDensity;
  scrollContainerRef: React.RefObject<HTMLElement | null>;
  active: boolean;
}

export default function GameLogList({
  logs,
  gameState,
  density,
  scrollContainerRef,
  active,
}: GameLogListProps) {
  const styles = DENSITY_STYLES[density];

  const playerNames = useMemo(() => {
    const names = new Map<string, string>();
    if (gameState?.currentPlayer) {
      names.set(gameState.currentPlayer.id, gameState.currentPlayer.name);
    }
    gameState?.otherPlayers?.forEach((p) => {
      names.set(p.id, p.name);
    });
    return names;
  }, [gameState?.currentPlayer, gameState?.otherPlayers]);

  const playerColorMap = useMemo(() => {
    const map = new Map<string, string>();
    if (gameState?.currentPlayer?.color) {
      map.set(gameState.currentPlayer.id, gameState.currentPlayer.color);
    }
    gameState?.otherPlayers?.forEach((p) => {
      if (p.color) {
        map.set(p.id, p.color);
      }
    });
    return map;
  }, [gameState?.currentPlayer, gameState?.otherPlayers]);

  useEffect(() => {
    if (!active || logs.length === 0) {
      return undefined;
    }
    const frame = requestAnimationFrame(() => {
      const el = scrollContainerRef.current;
      if (el) {
        el.scrollTop = el.scrollHeight;
      }
    });
    return () => cancelAnimationFrame(frame);
  }, [active, logs, scrollContainerRef]);

  const groupedLogs = useMemo(() => groupLogsByGeneration(logs), [logs]);

  if (logs.length === 0) {
    return (
      <div className="flex items-center justify-center py-10 px-5">
        <span className="font-orbitron text-sm text-white/50">No logs</span>
      </div>
    );
  }

  return (
    <div className="flex flex-col">
      {groupedLogs.map((group) => {
        const totalEntries = group.playerTurns.reduce((sum, t) => sum + t.entries.length, 0);
        return (
          <div key={group.generation} className="flex flex-col">
            <GenerationDivider
              generation={group.generation}
              entryCount={totalEntries}
              styles={styles}
            />
            <div className={`${styles.groupBody} flex flex-col`}>
              {group.playerTurns.map((turn, turnIndex) => (
                <PlayerTurnSection
                  key={`${turn.playerId}-${turnIndex}`}
                  playerName={playerNames.get(turn.playerId) || "Unknown"}
                  entries={turn.entries}
                  playerColor={playerColorMap.get(turn.playerId)}
                  playerNames={playerNames}
                  styles={styles}
                />
              ))}
            </div>
          </div>
        );
      })}
    </div>
  );
}
