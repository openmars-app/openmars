import { useState } from "react";
import { useLayoutMode } from "@/hooks/useLayoutMode";
import MobileScreen from "@/components/mobile/MobileScreen";
import { PlayedCardsTab } from "@/components/mobile/screens/MobileTableauScreen";
import { TableauIcon } from "@/components/mobile/screenIcons";
import type {
  CardDto,
  ChoiceDto,
  PendingBehaviorResolutionDto,
  PlayerActionDto,
  PlayerCardDto,
  StateErrorDto,
} from "@/types/generated/api-types";
import type { PreviewDefinition, PreviewProps } from "./types";
import { previewResources, previewProduction } from "./fixtures";
import { BehaviorResolutionView } from "../../overlay/BehaviorResolutionOverlay";
import CardDrawSelectionOverlay from "../../overlay/CardDrawSelectionOverlay";
import CardRevealOverlay from "../../overlay/CardRevealOverlay";
import PendingCardSelectionOverlay from "../../overlay/PendingCardSelectionOverlay";
import CardDetailPortal from "../../overlay/CardDetailPortal";
import CardsPlayedModal from "../../modals/CardsPlayedModal";
import PlayedCardNotification from "../../overlay/PlayedCardNotification";
import ChoiceSelectionPopover from "../../popover/ChoiceSelectionPopover";
import AmountSelectionPopover from "../../popover/AmountSelectionPopover";
import TargetPlayerSelectionPopover from "../../popover/TargetPlayerSelectionPopover";
import CardStorageSelectionPopover from "../../popover/CardStorageSelectionPopover";
import CardResourceSelectionPopover from "../../popover/CardResourceSelectionPopover";
import ActionReusePopover from "../../popover/ActionReusePopover";
import { PaymentPicker } from "../../popover/PaymentSelectionPopover";

function named(cards: CardDto[], name: string): CardDto {
  const card = cards.find((item) => item.name === name);
  if (!card) {
    throw new Error(`Preview requires the card ${name}`);
  }
  return card;
}
function hand(cards: CardDto[], count = 4): PlayerCardDto[] {
  return cards
    .filter((card) => card.type !== "corporation" && card.type !== "prelude")
    .slice(0, count)
    .map((card) => ({ ...card, available: true, errors: [], effectiveCost: card.cost }));
}
function playabilityHand(cards: CardDto[], count: number): PlayerCardDto[] {
  return hand(cards, count).map((card, index) => ({
    ...card,
    available: index > 1,
    errors:
      index === 0
        ? [
            {
              code: "insufficient-resources",
              category: "cost",
              message: "Not enough credits to play this card",
            },
          ]
        : index === 1
          ? [
              {
                code: "invalid-requirement",
                category: "requirement",
                message: "Card requirements are not met",
              },
            ]
          : [],
    effectiveCost: index === 2 ? Math.max(0, card.cost - 2) : card.cost,
  }));
}
const scienceError: StateErrorDto = {
  code: "insufficient-resources",
  category: "input",
  message: "Not enough resources on Olympus Conference",
};

function BehaviorPreview({ cards, presetId, onComplete }: PreviewProps) {
  const olympus = named(cards, "Olympus Conference");
  const index = olympus.behaviors?.findIndex((item) => item.choices?.length) ?? 0;
  const choices: ChoiceDto[] = (olympus.behaviors?.[index].choices ?? []).map((choice, i) => ({
    ...choice,
    originalIndex: i,
    available: presetId !== "unavailable" || i === 0,
    errors: presetId === "unavailable" && i !== 0 ? [scienceError] : [],
  }));
  const resolution: PendingBehaviorResolutionDto = {
    id: "preview-olympus",
    kind: "choice",
    source: olympus.name,
    sourceCardId: olympus.id,
    sourceBehaviorIndex: index,
    triggeringCardId: olympus.id,
    triggeringCardName: olympus.name,
    minCards: 0,
    maxCards: 0,
    choices,
  };
  const mars = named(cards, "Mars University");
  const discard: PendingBehaviorResolutionDto = {
    id: "preview-discard",
    kind: "card-discard",
    source: mars.name,
    sourceCardId: mars.id,
    sourceBehaviorIndex: 0,
    triggeringCardId: olympus.id,
    triggeringCardName: olympus.name,
    minCards: 0,
    maxCards: 1,
    outputs: [{ type: "card-draw", amount: 1, target: "self-player" }],
  };
  const [resolutions, setResolutions] = useState(
    presetId === "multiple"
      ? [resolution, discard]
      : presetId.includes("discard")
        ? [discard]
        : [resolution],
  );
  const finish = async (id: string) => {
    if (presetId === "error") {
      throw new Error("Preview: connection lost. Your choice was not submitted.");
    }
    const remaining = resolutions.filter((item) => item.id !== id);
    setResolutions(remaining);
    if (!remaining.length) {
      onComplete("Sample effects resolved locally");
    }
  };
  return (
    <BehaviorResolutionView
      resolutions={resolutions}
      handCards={presetId === "empty-discard" ? [] : hand(cards)}
      playedCards={[olympus, mars]}
      onChoice={finish}
      onDiscard={finish}
      onOptional={finish}
    />
  );
}
function OptionalPreview({ cards, presetId, onComplete }: PreviewProps) {
  const card = named(cards, "Diplomatic Extension");
  const index = card.behaviors!.findIndex((behavior) => behavior.optional);
  const available = presetId !== "unavailable";
  const resolution: PendingBehaviorResolutionDto = {
    id: "preview-optional",
    kind: "optional",
    source: card.name,
    sourceCardId: card.id,
    sourceBehaviorIndex: index,
    minCards: 0,
    maxCards: 0,
    optionalEffect: {
      behavior: card.behaviors![index],
      available,
      errors: available
        ? []
        : [
            {
              code: "insufficient-credits",
              category: "input",
              message: "You need 3 M€ to apply this effect",
            },
          ],
    },
  };
  return (
    <BehaviorResolutionView
      resolutions={[resolution]}
      handCards={[]}
      playedCards={[card]}
      onOptional={async (_id, accepted) =>
        onComplete(accepted ? "Sample optional effect accepted" : "Sample optional effect skipped")
      }
      onChoice={async () => {}}
      onDiscard={async () => {}}
    />
  );
}
function PlayedCardsPreview({ cards, presetId, onComplete }: PreviewProps) {
  const { isCompact } = useLayoutMode();
  const [detail, setDetail] = useState<CardDto | null>(null);
  const sample = presetId === "empty" ? [] : hand(cards, presetId === "many" ? 60 : 8);
  const close = () => onComplete("Played cards closed");
  if (isCompact) {
    return (
      <>
        <MobileScreen
          icon={TableauIcon}
          title="Played Cards"
          tabs={[{ id: "played", label: "Played Cards" }]}
          activeTab="played"
          onClose={close}
          keyboardAware
        >
          <PlayedCardsTab cards={sample} onCardTap={setDetail} />
        </MobileScreen>
        {detail && <CardDetailPortal card={detail} onClose={() => setDetail(null)} />}
      </>
    );
  }
  return <CardsPlayedModal isVisible cards={sample} onClose={close} />;
}

function ReceiptPreview({ cards, presetId, onComplete }: PreviewProps) {
  const olympus = named(cards, "Olympus Conference");
  return (
    <CardDrawSelectionOverlay
      mode="received"
      isOpen
      receipt={{
        id: "preview-receipt",
        source: presetId === "olympus" ? olympus.name : "Pluto",
        sourceCardId: presetId === "olympus" ? olympus.id : "",
        cards: hand(cards, presetId === "many" ? 8 : presetId === "olympus" ? 1 : 2),
      }}
      onClose={() => onComplete("Sample receipt acknowledged")}
    />
  );
}
function DrawPreview({ cards, presetId, onComplete }: PreviewProps) {
  return (
    <CardDrawSelectionOverlay
      mode="selection"
      isOpen
      playerCredits={presetId === "poor" ? 0 : 9}
      selection={{
        availableCards: playabilityHand(cards, presetId === "many" ? 12 : 4),
        source: "Inventors' Guild",
        minFreeTakeCount: presetId === "required" ? 1 : 0,
        freeTakeCount: 1,
        maxBuyCount: 2,
        cardBuyCost: 3,
        playAsPrelude: false,
      }}
      onConfirm={(take, buy) =>
        onComplete(`Selected ${take.length} free and ${buy.length} bought cards locally`)
      }
    />
  );
}
function RevealPreview({ cards, presetId, onComplete }: PreviewProps) {
  const matched = presetId !== "no-match";
  const otherCards = cards.filter(
    (card) =>
      card.type !== "corporation" && card.type !== "prelude" && !card.tags?.includes("microbe"),
  );
  const revealed = [...(matched ? [named(cards, "Tardigrades")] : []), ...otherCards].slice(
    0,
    presetId === "many" ? 8 : 2,
  );
  return (
    <CardRevealOverlay
      reveal={{
        source: "Search For Life",
        sourceCardId: named(cards, "Search For Life").id,
        cards: revealed,
        results: revealed.map((card, index) => ({
          cardId: card.id,
          name: card.name,
          matched: matched && index === 0,
        })),
        rewards: matched ? [{ resourceType: "science", amount: 1, isScaled: false }] : [],
      }}
      onConfirm={async () => {
        if (presetId === "error") {
          throw new Error("Preview: could not confirm reveal");
        }
        onComplete("Sample reveal acknowledged");
      }}
    />
  );
}
function PendingPreview({ cards, presetId, onComplete }: PreviewProps) {
  const sample = playabilityHand(cards, presetId === "many" ? 12 : 4);
  const sell = presetId === "sell";
  return (
    <PendingCardSelectionOverlay
      isOpen
      selection={{
        availableCards: sample,
        source: sell ? "sell-patents" : "research-phase",
        minCards: 0,
        maxCards: sample.length,
        cardCosts: Object.fromEntries(sample.map((card) => [card.id, sell ? 0 : 3])),
        cardRewards: Object.fromEntries(sample.map((card) => [card.id, sell ? 1 : 0])),
      }}
      playerCredits={presetId === "poor" ? 0 : 6}
      onSelectCards={(selected) => onComplete(`Selected ${selected.length} sample cards`)}
      onCancel={() => onComplete("Cancelled")}
    />
  );
}
function EcologyPreview({ cards, presetId, onComplete }: PreviewProps) {
  const ants = named(cards, "Ants");
  const [step, setStep] = useState(presetId === "microbes" ? 1 : 0);
  return (
    <CardStorageSelectionPopover
      key={step}
      isVisible
      resourceType={step === 0 ? "animal" : "microbe"}
      amount={step === 0 ? 1 : 2}
      playedCards={[ants]}
      resourceStorage={{ [ants.id]: 2 }}
      onCardSelect={() => {
        if (step === 0) {
          setStep(1);
        } else {
          onComplete("Placed 2 sample microbes on Ants");
        }
      }}
      onCancel={() => onComplete("Cancelled")}
    />
  );
}
function StoragePreview({ cards, presetId, onComplete }: PreviewProps) {
  const type = presetId === "science" ? "science" : "microbe";
  return (
    <CardStorageSelectionPopover
      isVisible
      resourceType={type}
      amount={3}
      playedCards={
        presetId === "empty"
          ? []
          : cards.filter((card) => card.resourceStorage?.type === type).slice(0, 8)
      }
      onCardSelect={(id) => onComplete(id ? "Sample storage selected" : "Sample gain skipped")}
      onCancel={() => onComplete("Cancelled")}
    />
  );
}
function ChoicePreview({ cards, presetId, onComplete }: PreviewProps) {
  const card = named(cards, "Atmo Collectors");
  const index = card.behaviors?.findIndex((behavior) => behavior.choices?.length) ?? 0;
  const behaviors =
    card.behaviors?.map((behavior) => ({
      ...behavior,
      choices: behavior.choices?.map((choice, i) => ({
        ...choice,
        originalIndex: i,
        available: true,
        errors: [],
      })),
    })) ?? [];
  return (
    <ChoiceSelectionPopover
      isVisible
      isAction
      cardId={card.id}
      cardName={card.name}
      behaviors={behaviors}
      behaviorIndex={index}
      playerResources={previewResources}
      resourceStorage={{ [card.id]: presetId === "empty" ? 0 : 2 }}
      onChoiceSelect={(i) => onComplete(`Selected sample action ${i + 1}`)}
      onCancel={() => onComplete("Cancelled")}
    />
  );
}
function AmountPreview({ presetId, onComplete }: PreviewProps) {
  return (
    <AmountSelectionPopover
      isVisible
      resourceType="energy"
      maxAmount={presetId === "zero" ? 0 : presetId === "large" ? 100 : 5}
      onAmountSelect={(amount) => onComplete(`Selected ${amount} sample energy`)}
      onCancel={() => onComplete("Cancelled")}
    />
  );
}
function TargetPreview({ presetId, onComplete }: PreviewProps) {
  const players = [
    {
      id: "preview-player",
      name: "You",
      resources: previewResources,
      production: previewProduction,
    },
    {
      id: "preview-other-1",
      name: "A very long opponent corporation name",
      resources: { ...previewResources, plants: 3 },
      production: previewProduction,
    },
  ];
  return (
    <TargetPlayerSelectionPopover
      isVisible
      resourceType="plant"
      amount={3}
      isSteal={presetId === "steal"}
      players={players}
      currentPlayerId="preview-player"
      removalTargets={
        presetId === "none"
          ? []
          : players.map((player) => ({ playerId: player.id, resourceType: "plant", amount: 3 }))
      }
      onPlayerSelect={(id) => onComplete(id ? "Sample player selected" : "Sample removal skipped")}
      onCancel={() => onComplete("Cancelled")}
    />
  );
}
function RemovalPreview({ presetId, onComplete }: PreviewProps) {
  const players = [
    {
      id: "preview-player",
      name: "You",
      resources: previewResources,
      production: previewProduction,
    },
    {
      id: "preview-other-1",
      name: "Helion",
      resources: { ...previewResources, plants: 2 },
      production: previewProduction,
    },
  ];
  return (
    <TargetPlayerSelectionPopover
      isVisible
      mandatory
      resourceType="plant"
      amount={5}
      removalTargets={[]}
      players={players}
      currentPlayerId="preview-player"
      pendingRemoval={{
        selection: {
          id: "preview-removal",
          maxAmounts: { "preview-player": 5, "preview-other-1": 2 },
          eligiblePlayerIds: presetId === "none" ? [] : players.map((p) => p.id),
          resourceType: "plant",
          amount: 5,
          source: "Asteroid",
          sourceCardId: "009",
        },
        onConfirm: async (_id, target, amount) => {
          if (presetId === "error") {
            throw new Error("Preview: removal failed");
          }
          onComplete(target ? `Removed ${amount} sample plants` : "Sample removal skipped");
        },
      }}
    />
  );
}
function CardResourcePreview({ cards, presetId, onComplete }: PreviewProps) {
  const ants = named(cards, "Ants");
  return (
    <CardResourceSelectionPopover
      isVisible
      resourceType="microbe"
      amount={1}
      optional={presetId !== "spend"}
      selection={
        presetId === "spend"
          ? { kind: "spend", playerId: "preview-player", eligibleCardIds: [ants.id] }
          : { kind: "remove" }
      }
      players={[
        {
          id: "preview-player",
          name: "You",
          playedCards: [ants],
          resourceStorage: { [ants.id]: 3 },
        },
        {
          id: "preview-other-1",
          name: "Helion",
          playedCards: [ants],
          resourceStorage: { [ants.id]: 1 },
        },
      ]}
      removalTargets={
        presetId === "none"
          ? []
          : [
              { playerId: "preview-player", cardId: ants.id, resourceType: "microbe", amount: 3 },
              { playerId: "preview-other-1", cardId: ants.id, resourceType: "microbe", amount: 1 },
            ]
      }
      onCardSelect={() => onComplete("Sample resource target selected")}
      onCancel={() => onComplete("Cancelled")}
    />
  );
}
function PaymentPreview({ cards, presetId, onComplete }: PreviewProps) {
  return (
    <PaymentPicker
      sourceCards={cards}
      loading={presetId === "loading"}
      error={presetId === "error" ? "Payment quote expired. Try again." : ""}
      quote={{
        costs: { credit: 18 },
        options: [
          {
            source: { target: "self-player", resource: "credit" },
            targetResource: "credit",
            conversionRate: 1,
            available: presetId === "poor" ? 2 : 24,
          },
          {
            source: { target: "self-player", resource: "steel" },
            targetResource: "credit",
            conversionRate: 2,
            available: 4,
          },
          {
            source: { target: "self-player", resource: "heat" },
            targetResource: "credit",
            conversionRate: 1,
            available: 3,
          },
        ],
      }}
      onConfirm={(payment) =>
        onComplete(`Sample payment: ${payment.allocations.length} allocations`)
      }
      onCancel={() => onComplete("Cancelled")}
    />
  );
}
function ReusePreview({ cards, presetId, onComplete }: PreviewProps) {
  const candidates = cards
    .filter((card) =>
      card.behaviors?.some((behavior) =>
        behavior.triggers?.some((trigger) => trigger.type === "manual"),
      ),
    )
    .slice(0, 3);
  const actions: PlayerActionDto[] = candidates.map((card) => {
    const index = card.behaviors!.findIndex((behavior) =>
      behavior.triggers?.some((trigger) => trigger.type === "manual"),
    );
    return {
      cardId: card.id,
      cardName: card.name,
      behaviorIndex: index,
      behavior: card.behaviors![index],
      timesUsedThisTurn: 1,
      timesUsedThisGeneration: 1,
      available: false,
      errors: [],
    };
  });
  const source: PlayerActionDto = {
    cardId: "preview-reuse",
    cardName: "Virgo",
    behaviorIndex: 0,
    behavior: {},
    timesUsedThisTurn: 0,
    timesUsedThisGeneration: 0,
    available: true,
    errors: [],
    reuseOptions:
      presetId === "empty"
        ? []
        : actions.map((action, index) => ({
            cardId: action.cardId,
            behaviorIndex: action.behaviorIndex,
            available: index !== 1,
            errors: index === 1 ? [scienceError] : [],
          })),
  };
  return (
    <ActionReusePopover
      isVisible
      actions={[source, ...actions]}
      reuseSourceCardId={source.cardId}
      onClose={() => onComplete("Cancelled")}
      onActionSelect={(action) => onComplete(`Reused ${action.cardName} locally`)}
    />
  );
}
function DetailPreview({ cards, presetId, onComplete }: PreviewProps) {
  return (
    <CardDetailPortal
      card={named(
        cards,
        presetId === "atmo"
          ? "Atmo Collectors"
          : presetId === "ecology"
            ? "Ecology Research"
            : "Olympus Conference",
      )}
      onClose={() => onComplete("Sample card closed")}
    />
  );
}
function PlayedPreview({ cards, onComplete }: PreviewProps) {
  const [pinned, setPinned] = useState(true);
  return (
    <PlayedCardNotification
      notification={{
        id: 1,
        card: named(cards, "Olympus Conference"),
        playerName: "Helion",
        playerColor: "#ff9f43",
      }}
      isPinned={pinned}
      onTogglePin={() => setPinned((value) => !value)}
      onAdvance={() => onComplete("Sample notification closed")}
    />
  );
}
const presets = (...items: [string, string][]) => items.map(([id, name]) => ({ id, name }));
const entry = (
  id: string,
  name: string,
  path: string,
  Component: PreviewDefinition["Component"],
  variants: [string, string][],
  relatedPaths: string[] = [],
): PreviewDefinition => ({
  id,
  name,
  category: "Cards and resources",
  componentPaths: [`components/ui/${path}.tsx`, ...relatedPaths],
  presets: presets(...variants),
  Component,
});
export const cardPreviews: PreviewDefinition[] = [
  entry(
    "behavior-resolution",
    "Resolve card effects · Olympus / Mars University",
    "overlay/BehaviorResolutionOverlay",
    BehaviorPreview,
    [
      ["choices", "Olympus: two legal choices"],
      ["unavailable", "Olympus: unavailable spend"],
      ["multiple", "Multiple pending effects"],
      ["discard", "Discard cards"],
      ["empty-discard", "Empty hand"],
      ["error", "Submission error"],
    ],
  ),
  entry(
    "optional-effect",
    "Optional card effect",
    "overlay/BehaviorResolutionOverlay",
    OptionalPreview,
    [
      ["available", "Pay to apply"],
      ["unavailable", "Cannot afford; skip available"],
    ],
  ),
  entry(
    "played-cards",
    "Played cards collection",
    "modals/CardsPlayedModal",
    PlayedCardsPreview,
    [
      ["normal", "Search played cards"],
      ["many", "Long collection"],
      ["empty", "No played cards"],
    ],
    ["components/mobile/screens/MobileTableauScreen.tsx"],
  ),
  entry(
    "card-receipt",
    "Cards received · Pluto / Olympus",
    "overlay/CardDrawSelectionOverlay",
    ReceiptPreview,
    [
      ["pluto", "Pluto: two cards"],
      ["olympus", "Olympus: one card"],
      ["many", "Many cards"],
    ],
  ),
  entry("card-draw", "Draw / peek / buy cards", "overlay/CardDrawSelectionOverlay", DrawPreview, [
    ["normal", "Take one, buy two"],
    ["required", "Required free selection"],
    ["poor", "Cannot afford purchases"],
    ["many", "Long card row"],
  ]),
  entry("card-reveal", "Reveal cards", "overlay/CardRevealOverlay", RevealPreview, [
    ["match", "Match and reward"],
    ["no-match", "No match"],
    ["many", "Long reveal"],
    ["error", "Confirmation error"],
  ]),
  entry(
    "pending-card-selection",
    "Buy / sell cards",
    "overlay/PendingCardSelectionOverlay",
    PendingPreview,
    [
      ["buy", "Research purchase"],
      ["sell", "Sell patents"],
      ["poor", "No credits"],
      ["many", "Long selection"],
    ],
  ),
  entry(
    "ecology-storage",
    "Ecology Research storage sequence",
    "popover/CardStorageSelectionPopover",
    EcologyPreview,
    [
      ["sequence", "No animal → two microbes"],
      ["microbes", "Place two microbes"],
    ],
  ),
  entry(
    "card-storage",
    "Choose card storage",
    "popover/CardStorageSelectionPopover",
    StoragePreview,
    [
      ["microbes", "Microbe destinations"],
      ["science", "Science destinations"],
      ["empty", "No storage warning"],
    ],
  ),
  entry(
    "card-choice",
    "Choose card action · Atmo Collectors",
    "popover/ChoiceSelectionPopover",
    ChoicePreview,
    [
      ["normal", "Stored floaters"],
      ["empty", "No floaters"],
    ],
  ),
  entry(
    "resource-amount",
    "Choose resource amount",
    "popover/AmountSelectionPopover",
    AmountPreview,
    [
      ["normal", "Five energy"],
      ["zero", "Zero available"],
      ["large", "Large amount"],
    ],
  ),
  entry(
    "target-player",
    "Choose target player",
    "popover/TargetPlayerSelectionPopover",
    TargetPreview,
    [
      ["normal", "Remove plants"],
      ["steal", "Steal plants"],
      ["none", "No eligible targets"],
    ],
  ),
  entry(
    "resource-removal",
    "Remove player resources",
    "popover/TargetPlayerSelectionPopover",
    RemovalPreview,
    [
      ["normal", "Select player and amount"],
      ["none", "No eligible players"],
      ["error", "Submission error"],
    ],
  ),
  entry(
    "card-resource",
    "Remove / spend card resources",
    "popover/CardResourceSelectionPopover",
    CardResourcePreview,
    [
      ["remove", "Remove microbes"],
      ["spend", "Spend own microbes"],
      ["none", "No eligible cards"],
    ],
  ),
  entry("payment", "Select payment", "popover/PaymentSelectionPopover", PaymentPreview, [
    ["normal", "Credits, steel and heat"],
    ["poor", "Insufficient funds"],
    ["loading", "Refreshing quote"],
    ["error", "Quote error"],
  ]),
  entry("action-reuse", "Reuse card action", "popover/ActionReusePopover", ReusePreview, [
    ["normal", "Available and unavailable"],
    ["empty", "No used actions"],
  ]),
  entry("card-detail", "Card detail", "overlay/CardDetailPortal", DetailPreview, [
    ["olympus", "Olympus Conference"],
    ["atmo", "Atmo Collectors"],
    ["ecology", "Ecology Research"],
  ]),
  entry(
    "played-card",
    "Played card notification",
    "overlay/PlayedCardNotification",
    PlayedPreview,
    [["pinned", "Pinned (click to dismiss)"]],
  ),
];
