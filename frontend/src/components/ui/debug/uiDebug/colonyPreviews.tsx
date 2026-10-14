import { useLayoutMode } from "@/hooks/useLayoutMode";
import MobileScreen from "@/components/mobile/MobileScreen";
import { ProjectsIcon, LogIcon } from "@/components/mobile/screenIcons";
import ActionsList from "../../popover/content/ActionsList";
import EffectsList from "../../popover/content/EffectsList";
import TagsSummary from "../../popover/content/TagsSummary";
import StoragesList from "../../popover/content/StoragesList";
import VictoryPointsBreakdown from "../../popover/content/VictoryPointsBreakdown";
import GameLogList from "../../popover/content/GameLogList";
import StandardProjectList from "../../popover/content/StandardProjectList";
import { AwardListView } from "../../popover/content/AwardList";
import { MilestoneListView } from "../../popover/content/MilestoneList";
import ProjectFundingList from "../../popover/content/ProjectFundingList";
import { useMemo, useRef, useState, type ReactNode, type RefObject } from "react";
import type {
  CardDto,
  GameDto,
  PendingEffectSelectionDto,
  PlayerActionDto,
  PlayerEffectDto,
  StateDiffDto,
  ChatMessageDto,
  ProjectFundingDto,
  StateErrorDto,
  AwardRewardDto,
  ResourceCondition,
} from "@/types/generated/api-types";
import type { PreviewDefinition, PreviewProps } from "./types";
import { createPreviewGame } from "./fixtures";
import GameButton from "../../buttons/GameButton";
import ColonySelectionOverlay from "../../overlay/ColonySelectionOverlay";
import FreeTradeSelectionOverlay from "../../overlay/FreeTradeSelectionOverlay";
import EffectSelectionOverlay from "../../overlay/EffectSelectionOverlay";
import { ColonyPopoverView } from "../../popover/ColonyPopover";
import ColonyList, { useColonyControlState } from "../../popover/content/ColonyList";
import { ProjectFundingPopoverView } from "../../popover/ProjectFundingPopover";
import { useProjectSeatPurchaseState } from "../../popover/content/ProjectFundingList";
import { AwardFundSelectionPopoverView } from "../../popover/AwardFundSelectionPopover";
import { AwardPopoverView } from "../../popover/AwardPopover";
import { MilestonePopoverView } from "../../popover/MilestonePopover";
import ActionsPopover from "../../popover/ActionsPopover";
import EffectsPopover from "../../popover/EffectsPopover";
import TagsPopover from "../../popover/TagsPopover";
import { StoragesPopoverView } from "../../popover/StoragesPopover";
import VictoryPointsPopover from "../../popover/VictoryPointsPopover";
import { LogPopoverView } from "../../popover/LogPopover";
import StandardProjectPopover from "../../popover/StandardProjectPopover";
import { TravelPopoverView } from "../../popover/TravelPopover";
import type { PlanetTarget } from "@/contexts/PlanetFocusContext";
import { ChatOverlayView } from "../../overlay/ChatOverlay";
import { CardBrowserContent } from "../../CardBrowser";
import { CardBrowserOverlayView } from "../../overlay/CardBrowserOverlay";
import { cardFamily } from "../../cardBrowser/cardCatalog";
import CardStorageSelectionPopover from "../../popover/CardStorageSelectionPopover";
import { listGamePlayers } from "../../popover/content/gamePlayers";

const basic = [
  { id: "default", name: "Default" },
  { id: "empty", name: "Empty" },
  { id: "long", name: "Long content" },
];
const choicePresets = [
  { id: "default", name: "Default" },
  { id: "unavailable", name: "Unavailable" },
  { id: "long", name: "Long content" },
];
// Award reward payloads include vp, outside the generated behavior-output union.
const victoryPointReward: AwardRewardDto = {
  place: 1,
  outputs: [{ type: "vp", amount: 5 } as unknown as ResourceCondition],
};
const unavailableError: StateErrorDto = {
  code: "insufficient-resources",
  message: "Not enough credits for this action.",
  category: "resource",
};

function CompactPanel({
  title,
  onClose,
  children,
}: {
  title: string;
  onClose: () => void;
  children: ReactNode;
}) {
  return (
    <MobileScreen
      title={title}
      icon={ProjectsIcon}
      tabs={[{ id: title, label: title }]}
      activeTab={title}
      onClose={onClose}
    >
      <div className="p-3 h-full">{children}</div>
    </MobileScreen>
  );
}

function Anchor({
  children,
  label = "Open panel",
}: {
  children: (anchor: RefObject<HTMLButtonElement | null>) => ReactNode;
  label?: string;
}) {
  const anchor = useRef<HTMLButtonElement>(null);
  return (
    <div className="h-full min-h-0 flex justify-center items-start pt-24">
      <GameButton ref={anchor} emphasis="secondary">
        {label}
      </GameButton>
      {children(anchor)}
    </div>
  );
}

function colonyGame(cards: CardDto[], preset: string) {
  const game = createPreviewGame(cards);
  if (preset === "long") {
    game.colonies = Array.from({ length: 9 }, (_, index) => ({
      ...game.colonies![index % 3],
      id: `colony-${index}`,
      name: `${game.colonies![index % 3].name} Research Outpost ${index + 1}`,
    }));
  }
  if (preset === "inactive" || preset === "default") {
    const miranda = game.colonies!.find((colony) => colony.id === "miranda");
    if (miranda)
      Object.assign(miranda, {
        active: false,
        markerPosition: -1,
        tradeAvailable: false,
        tradeOptions: [],
      });
  }
  if (preset === "unavailable") {
    game.currentPlayer.actionCosts.forEach((action) =>
      action.costs.forEach((cost) => {
        cost.paymentCapacity = 0;
      }),
    );
    game.colonies!.forEach((colony) => {
      colony.buildAvailable = false;
    });
  }
  return game;
}

function ColonyTradePreview({ cards, presetId, onComplete }: PreviewProps) {
  const { isCompact } = useLayoutMode();
  const [game, setGame] = useState(() => colonyGame(cards, presetId));
  const controls = useColonyControlState(
    game,
    {
      trade: (id, payment, steps) => {
        setGame((previous) => ({
          ...previous,
          colonies: previous.colonies?.map((colony) =>
            colony.id === id
              ? {
                  ...colony,
                  tradedThisGen: true,
                  tradeAvailable: false,
                  traderId: previous.currentPlayer.id,
                }
              : colony,
          ),
        }));
        onComplete(`Traded with ${id} using ${payment}, track adjustment ${steps}.`);
      },
      build: (id) => {
        setGame((previous) => ({
          ...previous,
          colonies: previous.colonies?.map((colony) =>
            colony.id === id
              ? {
                  ...colony,
                  playerColonies: [...colony.playerColonies, previous.currentPlayer.id],
                  buildAvailable: false,
                }
              : colony,
          ),
        }));
        onComplete(`Built a colony on ${id}.`);
      },
    },
    undefined,
    presetId === "build" ? "build" : "trade",
  );
  if (isCompact || presetId === "screen") {
    return (
      <CompactPanel title="Colonies" onClose={() => onComplete("Closed colonies.")}>
        <ColonyList gameState={game} controls={controls} density="screen" />
      </CompactPanel>
    );
  }
  return (
    <Anchor label="Colonies">
      {(anchor) => (
        <ColonyPopoverView
          isVisible
          onClose={() => onComplete("Closed colonies.")}
          anchorRef={anchor}
          gameState={game}
          controls={controls}
        />
      )}
    </Anchor>
  );
}

function ColonyPlacementPreview({ cards, presetId, onComplete }: PreviewProps) {
  const game = useMemo(() => colonyGame(cards, presetId), [cards, presetId]);
  const colonies = game.colonies!;
  return (
    <ColonySelectionOverlay
      onContinue={() => onComplete("Continued without a selection.")}
      isOpen
      colonies={colonies}
      allPlayers={listGamePlayers(game)}
      pendingSelection={{
        canContinueWithoutSelection: presetId === "unavailable" || colonies.length === 0,
        addTile: presetId === "add-tile",
        availableColonyIds: presetId === "unavailable" ? [] : colonies.map((colony) => colony.id),
        allowDuplicatePlayerColony: false,
        source: "Space Port Colony",
        sourceCardId: "C38",
      }}
      onConfirm={(id) => onComplete(`Selected ${id}.`)}
    />
  );
}

function FreeTradePreview({ cards, presetId, onComplete }: PreviewProps) {
  const game = useMemo(() => colonyGame(cards, presetId), [cards, presetId]);
  return (
    <FreeTradeSelectionOverlay
      onContinue={() => onComplete("Continued without a selection.")}
      isOpen
      pendingSelection={{
        canContinueWithoutSelection:
          presetId === "unavailable" || !game.colonies!.some((colony) => colony.active),
        availableColonyIds: game
          .colonies!.filter((colony) => colony.active)
          .map((colony) => colony.id),
        source: "Trading Colony",
        sourceCardId: "C45",
      }}
      colonies={game.colonies!}
      viewingPlayerId={game.currentPlayer.id}
      tradeFleets={
        presetId === "unavailable"
          ? { [game.currentPlayer.id]: { total: 1, available: 0 } }
          : game.tradeFleets!
      }
      allPlayers={listGamePlayers(game)}
      playedCards={game.currentPlayer.playedCards}
      corporation={game.currentPlayer.corporation}
      onConfirm={(id, steps) => onComplete(`Free trade with ${id}, track adjustment ${steps}.`)}
    />
  );
}

function EffectPreview({
  cards,
  presetId,
  onComplete,
  generic = false,
}: PreviewProps & { generic?: boolean }) {
  const game = useMemo(
    () => colonyGame(cards, presetId === "default" ? "active" : presetId),
    [cards, presetId],
  );
  const [attempts, setAttempts] = useState(0);
  const colonies = game.colonies!;
  const selection: PendingEffectSelectionDto = generic
    ? {
        source: "Robotic Workforce",
        canContinueWithoutSelection: presetId === "unavailable",
        sourceCardId: cards.find((card) => card.name === "Robotic Workforce")!.id,
        outputs: [],
        options:
          presetId === "unavailable"
            ? []
            : cards
                .filter(
                  (card) =>
                    card.tags?.includes("building") &&
                    card.behaviors?.some((behavior) => behavior.productionBox === "evaluate"),
                )
                .slice(0, presetId === "long" ? 12 : 3)
                .map((card) => ({
                  cardId: card.id,
                  outputs: card
                    .behaviors!.filter((behavior) => behavior.productionBox === "evaluate")
                    .flatMap((behavior) => behavior.outputs ?? []),
                })),
      }
    : {
        source: "Market Manipulation",
        canContinueWithoutSelection: presetId === "unavailable" || colonies.length === 0,
        sourceCardId: "C22",
        outputs: [
          { type: "colony-track-step", amount: 1, target: "self-player" },
          { type: "colony-track-step", amount: -1, target: "self-player" },
        ],
        options:
          presetId === "unavailable"
            ? []
            : colonies.flatMap((increase, index) =>
                colonies
                  .filter(
                    (_, other) =>
                      other !== index &&
                      (presetId !== "restricted" || other === (index + 1) % colonies.length),
                  )
                  .map((decrease) => ({ colonyIds: [increase.id, decrease.id] })),
              ),
      };
  return (
    <EffectSelectionOverlay
      onContinue={() => onComplete("Continued without a selection.")}
      selection={selection}
      cards={cards}
      colonies={colonies}
      players={listGamePlayers(game)}
      onConfirm={async (index) => {
        if (presetId === "error" && attempts === 0) {
          setAttempts(1);
          throw new Error("Preview request failed. Your selection is preserved; try again.");
        }
        onComplete(`Submitted option ${index + 1}.`);
      }}
    />
  );
}

function ResourcePreview({ cards, presetId, onComplete }: PreviewProps) {
  const eligible = cards
    .filter((card) => card.resourceStorage?.type === "animal")
    .slice(0, presetId === "long" ? 10 : 3);
  return (
    <CardStorageSelectionPopover
      isVisible
      resourceType="animal"
      amount={2}
      reason="trade"
      playedCards={presetId === "unavailable" ? [] : eligible}
      resourceStorage={Object.fromEntries(eligible.map((card, index) => [card.id, index + 2]))}
      onCardSelect={(id) => onComplete(`Stored 2 animals on ${id}.`)}
      onCancel={() => onComplete("Cancelled storage selection.")}
    />
  );
}

function projectFixture(index: number, unavailable: boolean): ProjectFundingDto {
  return {
    id: `project-${index}`,
    name: ["Orbital Mirror", "Ocean Research", "Mars Transport Network"][index % 3],
    description:
      "Fund seats to develop shared infrastructure. More seats improve your completion reward.",
    seats: [5, 7, 9, 12].map((cost, position) => ({
      cost,
      paymentSubstitutes: [],
      ownerId: position === 0 ? "preview-other-1" : "",
      ownerName: position === 0 ? "Morgan" : "",
      ownerColor: "#4fa3df",
      isFilled: position === 0,
    })),
    seatOwners: [],
    isCompleted: false,
    nextSeatIndex: 1,
    nextSeatCost: 7,
    canBuySeat: !unavailable,
    buyErrors: unavailable ? [unavailableError] : [],
    currentPlayerSeats: 0,
    paymentSubstitutes: [{ resourceType: "steel", conversionRate: 2 }],
    rewardTiers: [
      { seatsOwned: 1, rewards: [{ type: "credit-production", amount: 1 }] },
      { seatsOwned: 2, rewards: [{ type: "credit-production", amount: 3 }] },
    ],
    completionEffect: {
      description: "Raise temperature one step.",
      rewards: [{ type: "credit", amount: 5 }],
      globalEffects: [{ type: "temperature", amount: 1 }],
    },
    style: { color: "#69bdda", icon: "space" },
  };
}

function FundingPreview({ cards, presetId, onComplete }: PreviewProps) {
  const { isCompact } = useLayoutMode();
  const [game, setGame] = useState(() => ({
    ...createPreviewGame(cards),
    projectFunding: Array.from({ length: presetId === "long" ? 8 : 3 }, (_, index) =>
      projectFixture(index, presetId === "unavailable"),
    ),
  }));
  const purchase = useProjectSeatPurchaseState(game, (id, credit, steel, titanium) => {
    setGame((previous) => ({
      ...previous,
      projectFunding: previous.projectFunding?.map((project) =>
        project.id === id
          ? {
              ...project,
              seats: project.seats.map((seat, index) =>
                index === project.nextSeatIndex
                  ? {
                      ...seat,
                      isFilled: true,
                      ownerId: previous.currentPlayer.id,
                      ownerName: previous.currentPlayer.name,
                      ownerColor: previous.currentPlayer.color,
                    }
                  : seat,
              ),
              currentPlayerSeats: project.currentPlayerSeats + 1,
              nextSeatIndex: project.nextSeatIndex + 1,
              nextSeatCost: project.seats[project.nextSeatIndex + 1]?.cost ?? 0,
              canBuySeat: false,
            }
          : project,
      ),
    }));
    onComplete(`Seat purchased: ${credit} credits, ${steel} steel, ${titanium} titanium.`);
  });
  if (isCompact) {
    return (
      <CompactPanel title="Funding" onClose={() => onComplete()}>
        <ProjectFundingList gameState={game} purchase={purchase} density="screen" />
      </CompactPanel>
    );
  }
  return (
    <Anchor label="Projects">
      {(anchor) => (
        <ProjectFundingPopoverView
          isVisible
          onClose={() => onComplete("Closed projects.")}
          anchorRef={anchor}
          gameState={game}
          purchase={purchase}
        />
      )}
    </Anchor>
  );
}

function awardGame(cards: CardDto[], preset: string): GameDto {
  const game = createPreviewGame(cards);
  const count = preset === "long" ? 8 : 3;
  game.awards = Array.from({ length: count }, (_, index) => ({
    type: `award-${index}`,
    name: ["Banker", "Scientist", "Landlord"][index % 3],
    description: ["Most credit production.", "Most science tags in play.", "Most tiles on Mars."][
      index % 3
    ],
    isFunded: index === 1,
    fundedBy: index === 1 ? "preview-other-1" : undefined,
    fundingCost: 14,
    playerProgress: {
      "preview-player": 6 + index,
      "preview-other-1": 4 + index,
      "preview-other-2": 2,
    },
    rewards: [victoryPointReward],
    style: { color: "#d8ae56", icon: `award-${["banker", "scientist", "landlord"][index % 3]}` },
  }));
  game.currentPlayer.awards = game.awards.map((award) => ({
    ...award,
    available: preset !== "unavailable" && !award.isFunded,
    errors: preset === "unavailable" ? [unavailableError] : [],
  }));
  game.milestones = Array.from({ length: count }, (_, index) => ({
    type: `milestone-${index}`,
    name: ["Terraformer", "Builder", "Planner"][index % 3],
    description: [
      "Reach 35 terraform rating.",
      "Have 8 building tags in play.",
      "Have 16 cards in hand.",
    ][index % 3],
    isClaimed: index === 1,
    claimedBy: index === 1 ? "preview-other-1" : undefined,
    claimCost: 8,
    required: [35, 8, 16][index % 3],
    playerProgress: {
      "preview-player": [35, 6, 12][index % 3],
      "preview-other-1": [30, 8, 9][index % 3],
    },
    rewards: [victoryPointReward],
    style: {
      color: "#dc8865",
      icon: `milestone-${["terraformer", "builder", "planner"][index % 3]}`,
    },
  }));
  game.currentPlayer.milestones = game.milestones.map((milestone, index) => ({
    ...milestone,
    progress: milestone.playerProgress[game.currentPlayer.id],
    available: preset !== "unavailable" && index === 0,
    errors:
      preset === "unavailable"
        ? [unavailableError]
        : index === 2
          ? [
              {
                code: "requirement",
                category: "requirement",
                message: "You need 16 cards in hand.",
              },
            ]
          : [],
  }));
  return game;
}

function AwardsPreview({
  cards,
  presetId,
  onComplete,
  milestone = false,
}: PreviewProps & { milestone?: boolean }) {
  const { isCompact } = useLayoutMode();
  const [game, setGame] = useState(() => awardGame(cards, presetId));
  const choose = (id: string) => {
    setGame((previous) => ({
      ...previous,
      currentPlayer: {
        ...previous.currentPlayer,
        awards: previous.currentPlayer.awards.map((award) =>
          award.type === id
            ? { ...award, isFunded: true, fundedBy: previous.currentPlayer.id }
            : award,
        ),
        milestones: previous.currentPlayer.milestones.map((entry) =>
          entry.type === id
            ? { ...entry, isClaimed: true, claimedBy: previous.currentPlayer.id }
            : entry,
        ),
      },
    }));
    onComplete(`${milestone ? "Claimed" : "Funded"} ${id}.`);
  };
  if (isCompact) {
    return (
      <CompactPanel title={milestone ? "Milestones" : "Awards"} onClose={() => onComplete()}>
        {milestone ? (
          <MilestoneListView gameState={game} density="screen" onClaimMilestone={choose} />
        ) : (
          <AwardListView gameState={game} density="screen" onFundAward={choose} />
        )}
      </CompactPanel>
    );
  }
  return (
    <Anchor label={milestone ? "Milestones" : "Awards"}>
      {(anchor) =>
        milestone ? (
          <MilestonePopoverView
            isVisible
            onClose={() => onComplete()}
            anchorRef={anchor}
            gameState={game}
            onClaimMilestone={choose}
          />
        ) : (
          <AwardPopoverView
            isVisible
            onClose={() => onComplete()}
            anchorRef={anchor}
            gameState={game}
            onFundAward={choose}
          />
        )
      }
    </Anchor>
  );
}

function actionFixtures(cards: CardDto[], preset: string): PlayerActionDto[] {
  if (preset === "empty") {
    return [];
  }
  return cards
    .flatMap((card) =>
      (card.behaviors ?? []).flatMap((behavior, behaviorIndex) =>
        behavior.triggers?.some((trigger) => trigger.type === "manual")
          ? [
              {
                cardId: card.id,
                cardName: card.name,
                behaviorIndex,
                behavior,
                available: preset !== "unavailable",
                errors: preset === "unavailable" ? [unavailableError] : [],
                timesUsedThisTurn: 0,
                timesUsedThisGeneration: 0,
              },
            ]
          : [],
      ),
    )
    .slice(0, preset === "long" ? 15 : 4);
}

function InformationPreview({
  cards,
  presetId,
  onComplete,
  kind,
}: PreviewProps & { kind: string }) {
  const { isCompact } = useLayoutMode();
  const game = useMemo(() => createPreviewGame(cards), [cards]);
  const [planet, setPlanet] = useState<PlanetTarget>("mars");
  const scroll = useRef<HTMLDivElement>(null);
  const screenAnchor = useRef<HTMLButtonElement>(null);
  const closed = () => onComplete(`Closed ${kind}.`);
  const effects: PlayerEffectDto[] =
    presetId === "empty"
      ? []
      : cards
          .flatMap((card) =>
            (card.behaviors ?? []).flatMap((behavior, behaviorIndex) =>
              behavior.triggers?.some((trigger) => trigger.type !== "manual" && trigger.condition)
                ? [{ cardId: card.id, cardName: card.name, behaviorIndex, behavior }]
                : [],
            ),
          )
          .slice(0, presetId === "long" ? 16 : 4);
  const logs: StateDiffDto[] =
    presetId === "empty"
      ? []
      : Array.from({ length: presetId === "long" ? 32 : 4 }, (_, index) => ({
          sequenceNumber: index + 1,
          timestamp: "2026-01-01T12:00:00Z",
          gameId: game.id,
          source: cards[index]?.name ?? "Standard project",
          sourceType: "card",
          playerId: index % 2 ? "preview-other-1" : game.currentPlayer.id,
          description: "Project played",
          calculatedOutputs: [{ resourceType: "credit", amount: index + 1, isScaled: false }],
        }));
  const standardOutputs: ResourceCondition[][] = [
    [{ type: "energy-production", amount: 1, target: "self-player" }],
    [{ type: "temperature", amount: 1, target: "none" }],
    [{ type: "ocean-placement", amount: 1, target: "none" }],
    [{ type: "greenery-placement", amount: 1, target: "none" }],
    [
      { type: "city-placement", amount: 1, target: "none" },
      { type: "credit-production", amount: 1, target: "self-player" },
    ],
  ];
  game.currentPlayer.standardProjects =
    presetId === "empty"
      ? []
      : ["Power Plant", "Asteroid", "Aquifer", "Greenery", "City"].map((name, index) => ({
          projectType: ["power-plant", "asteroid", "aquifer", "greenery", "city"][index],
          name,
          description: "Pay the indicated cost to perform this standard project.",
          behaviors: [{ outputs: standardOutputs[index] }],
          baseCost: { credit: [11, 14, 18, 23, 25][index] },
          effectiveCost: { credit: [11, 14, 18, 23, 25][index] },
          available: presetId !== "unavailable",
          errors: presetId === "unavailable" ? [unavailableError] : [],
          style: { color: "#629ec5", icon: "building" },
        }));
  const tags =
    presetId === "empty"
      ? []
      : ["building", "space", "science", "earth", "plant", "animal", "wild"].map((tag, index) => ({
          tag,
          count: presetId === "long" ? index * 7 + 2 : index + 1,
        }));
  const storages =
    presetId === "empty"
      ? []
      : cards
          .filter((card) => card.resourceStorage)
          .slice(0, presetId === "long" ? 15 : 4)
          .map((card, index) => ({
            cardId: card.id,
            cardName: card.name,
            resourceType: card.resourceStorage!.type,
            count: index + 2,
            tags: card.tags ?? [],
          }));
  const granters =
    presetId === "empty"
      ? []
      : cards.slice(0, presetId === "long" ? 15 : 4).map((card, index) => ({
          cardId: card.id,
          cardName: card.name,
          description: "1 VP per 2 animals on this card.",
          computedValue: index + 1,
          conditions: [
            {
              conditionType: "per",
              amount: 1,
              perType: "animal",
              perAmount: 2,
              adjacentToSelfTile: false,
              count: (index + 1) * 2,
              computedVP: index + 1,
              explanation: "1 VP per 2 animals on this card.",
            },
          ],
        }));
  const renderContent = (anchor: RefObject<HTMLButtonElement | null>) => {
    const props = { isVisible: true, onClose: closed, anchorRef: anchor };
    switch (kind) {
      case "actions": {
        const actions = actionFixtures(cards, presetId);
        const select = (action: PlayerActionDto) => onComplete(`Selected ${action.cardName}.`);
        return isCompact ? (
          <ActionsList
            actions={actions}
            gameState={game}
            onActionSelect={select}
            density="screen"
          />
        ) : (
          <ActionsPopover {...props} actions={actions} gameState={game} onActionSelect={select} />
        );
      }
      case "effects":
        return isCompact ? (
          <EffectsList effects={effects} density="screen" />
        ) : (
          <EffectsPopover {...props} effects={effects} />
        );
      case "tags":
        return isCompact ? (
          <TagsSummary tagCounts={tags} density="screen" />
        ) : (
          <TagsPopover {...props} tagCounts={tags} />
        );
      case "storages":
        return isCompact ? (
          <StoragesList items={storages} density="screen" />
        ) : (
          <StoragesPopoverView {...props} storageItems={storages} />
        );
      case "victory-points":
        return isCompact ? (
          <VictoryPointsBreakdown vpGranters={granters} density="screen" />
        ) : (
          <VictoryPointsPopover
            {...props}
            totalVP={granters.reduce((sum, granter) => sum + granter.computedValue, 0)}
            vpGranters={granters}
          />
        );
      case "game-log":
        return isCompact ? (
          <div ref={scroll} className="h-full overflow-y-auto">
            <GameLogList
              logs={logs}
              gameState={game}
              density="screen"
              scrollContainerRef={scroll}
              active
            />
          </div>
        ) : (
          <LogPopoverView {...props} gameState={game} logs={logs} />
        );
      case "standard-projects": {
        const select = (project: string) => onComplete(`Selected ${project}.`);
        return isCompact ? (
          <StandardProjectList gameState={game} onProjectSelect={select} density="screen" />
        ) : (
          <StandardProjectPopover {...props} gameState={game} onProjectSelect={select} />
        );
      }
      default:
        return (
          <TravelPopoverView
            {...props}
            activePlanet={planet}
            onTravel={(destination) => {
              setPlanet(destination);
              onComplete(`Travelled to ${destination}.`);
            }}
          />
        );
    }
  };
  if (isCompact && kind !== "travel") {
    return (
      <CompactPanel title={kind.replaceAll("-", " ")} onClose={closed}>
        {renderContent(screenAnchor)}
      </CompactPanel>
    );
  }
  return <Anchor label={kind}>{renderContent}</Anchor>;
}

function ChatPreview({ presetId, onComplete }: PreviewProps) {
  const { isCompact } = useLayoutMode();
  const [messages, setMessages] = useState<ChatMessageDto[]>(() =>
    presetId === "empty"
      ? []
      : Array.from({ length: presetId === "long" ? 24 : 4 }, (_, index) => ({
          senderId: index % 2 ? "preview-other-1" : "preview-player",
          senderName: index % 2 ? "Morgan" : "Avery",
          senderColor: index % 2 ? "#4fa3df" : "#e76f51",
          message:
            index % 2
              ? "I can trade with Luna after the next generation. Shall we fund Banker first?"
              : "My science engine is finally running. One more card and I can place a colony.",
          timestamp: "2026-01-01T12:00:00Z",
          isSpectator: false,
          kind: "chat",
        })),
  );
  const content = (
    <div className="h-full px-2 pt-1 pb-1">
      <ChatOverlayView
        messages={messages}
        embedded={isCompact}
        fill={isCompact}
        isSpectator={presetId === "spectator"}
        typingNames={presetId === "typing" ? ["Morgan", "Robin"] : []}
        onSendMessage={(message) => {
          setMessages((previous) => [
            ...previous,
            {
              senderId: "preview-player",
              senderName: "Avery",
              senderColor: "#e76f51",
              message,
              timestamp: new Date().toISOString(),
              isSpectator: false,
              kind: "chat",
            },
          ]);
          onComplete("Message added to preview chat.");
        }}
        onSendEmote={(emote) => onComplete(`Sent ${emote} emote in preview.`)}
      />
    </div>
  );
  return isCompact ? (
    <MobileScreen
      title="Log"
      icon={LogIcon}
      tabs={[{ id: "chat", label: "Chat" }]}
      activeTab="chat"
      onClose={() => onComplete("Closed chat.")}
      keyboardAware
    >
      {content}
    </MobileScreen>
  ) : (
    content
  );
}

function BrowserPreview({ cards, presetId, onComplete }: PreviewProps) {
  const [retried, setRetried] = useState(false);
  const catalog = useMemo(
    () => ({
      entries:
        presetId === "empty"
          ? []
          : (presetId === "long" ? cards : cards.slice(0, 24)).map((card) => ({
              card,
              family: cardFamily(card),
              search: `${card.id} ${card.name} ${(card.tags ?? []).join(" ")}`.toLowerCase(),
            })),
      status:
        !retried && presetId === "error"
          ? ("error" as const)
          : presetId === "loading"
            ? ("loading" as const)
            : ("ready" as const),
      error: "Card catalog could not be loaded. Try again.",
    }),
    [cards, presetId, retried],
  );
  return (
    <CardBrowserOverlayView isVisible onClose={() => onComplete("Closed card browser.")}>
      <CardBrowserContent
        onBack={() => onComplete("Closed card browser.")}
        backLabel="Back to UI Debug"
        catalog={catalog}
        modes={[]}
        onRetry={() => setRetried(true)}
      />
    </CardBrowserOverlayView>
  );
}

const definition = (
  id: string,
  name: string,
  category: string,
  paths: string[],
  Component: PreviewDefinition["Component"],
  presets = basic,
): PreviewDefinition => ({
  id,
  name,
  category,
  componentPaths: paths.map((path) => `components/ui/${path}.tsx`),
  presets,
  Component,
});
export const colonyPreviews: PreviewDefinition[] = [
  definition(
    "colony-trade",
    "Colony trade and build",
    "Colonies",
    ["popover/ColonyPopover", "popover/content/ColonyList", "popover/ColonySteps"],
    ColonyTradePreview,
    [
      ...choicePresets,
      { id: "inactive", name: "Inactive colony" },
      { id: "build", name: "Build colony" },
      { id: "screen", name: "Full screen list" },
    ],
  ),
  definition(
    "colony-placement",
    "Place colony",
    "Colonies",
    ["overlay/ColonySelectionOverlay"],
    ColonyPlacementPreview,
    [...choicePresets, { id: "add-tile", name: "Add colony tile" }],
  ),
  definition(
    "free-trade",
    "Free trade",
    "Colonies",
    ["overlay/FreeTradeSelectionOverlay"],
    FreeTradePreview,
    choicePresets,
  ),
  definition(
    "market-manipulation",
    "Market Manipulation",
    "Colonies",
    ["overlay/EffectSelectionOverlay", "overlay/ColonyTrackSelection"],
    EffectPreview,
    [
      ...choicePresets,
      { id: "restricted", name: "Restricted pairs" },
      { id: "error", name: "Error and retry" },
    ],
  ),
  definition(
    "effect-selection",
    "Copy production · Robotic Workforce",
    "Colonies",
    ["overlay/EffectSelectionOverlay"],
    (props) => <EffectPreview {...props} generic />,
    choicePresets,
  ),
  definition(
    "colony-resources",
    "Colony resource storage",
    "Colonies",
    ["popover/CardStorageSelectionPopover"],
    ResourcePreview,
    choicePresets,
  ),
  definition(
    "project-funding",
    "Project funding",
    "Funding",
    ["popover/ProjectFundingPopover", "popover/content/ProjectFundingList"],
    FundingPreview,
    choicePresets,
  ),
  definition(
    "award-fund-selection",
    "Free award funding",
    "Funding",
    ["popover/AwardFundSelectionPopover"],
    ({ cards, presetId, onComplete }) => {
      const game = awardGame(cards, presetId);
      return (
        <AwardFundSelectionPopoverView
          onContinue={() => onComplete("Continued without a selection.")}
          isOpen
          gameState={game}
          selection={{
            canContinueWithoutSelection: presetId === "unavailable" || game.awards.length === 0,
            availableAwards:
              presetId === "unavailable" ? [] : game.awards.map((award) => award.type),
            source: "Sponsored Award",
          }}
          onSelect={(id) => onComplete(`Funded ${id} for free.`)}
        />
      );
    },
    choicePresets,
  ),
  definition(
    "awards",
    "Fund awards",
    "Funding",
    ["popover/AwardPopover", "popover/content/AwardList"],
    AwardsPreview,
    choicePresets,
  ),
  definition(
    "milestones",
    "Claim milestones",
    "Funding",
    ["popover/MilestonePopover", "popover/content/MilestoneList"],
    (props) => <AwardsPreview {...props} milestone />,
    choicePresets,
  ),
  ...[
    ["actions", "Card actions", "ActionsPopover", "ActionsList"],
    ["effects", "Card effects", "EffectsPopover", "EffectsList"],
    ["tags", "Tags", "TagsPopover", "TagsSummary"],
    ["storages", "Card storages", "StoragesPopover", "StoragesList"],
    ["victory-points", "Victory points", "VictoryPointsPopover", "VictoryPointsBreakdown"],
    ["game-log", "Game log", "LogPopover", "GameLogList"],
    ["standard-projects", "Standard projects", "StandardProjectPopover", "StandardProjectList"],
    ["travel", "Travel", "TravelPopover", ""],
  ].map(([id, name, popover, list]) =>
    definition(
      id,
      name,
      "Information",
      [`popover/${popover}`, ...(list ? [`popover/content/${list}`] : [])],
      (props) => <InformationPreview {...props} kind={id} />,
      (id === "travel" ? basic.slice(0, 1) : basic).concat(
        id === "actions" || id === "standard-projects"
          ? [{ id: "unavailable", name: "Unavailable" }]
          : [],
      ),
    ),
  ),
  definition("chat", "Chat", "Information", ["overlay/ChatOverlay"], ChatPreview, [
    ...basic,
    { id: "typing", name: "Players typing" },
    { id: "spectator", name: "Spectator" },
  ]),
  definition(
    "card-browser",
    "Card browser",
    "Information",
    ["CardBrowser", "overlay/CardBrowserOverlay", "cardBrowser/VirtualCardGrid"],
    BrowserPreview,
    [...basic, { id: "loading", name: "Loading" }, { id: "error", name: "Error and retry" }],
  ),
];
