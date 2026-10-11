import { createContext, useContext, type ReactNode } from "react";
import type {
  CardDto,
  GameModeDto,
  ResourceStorageDto,
  StateRequirementDto,
} from "@/types/generated/api-types.ts";
import { useGameStore } from "@/stores/gameStore.ts";
import GameIcon from "../display/GameIcon.tsx";

const CatalogContext = createContext<{
  modes: GameModeDto[];
  modeId: string;
} | null>(null);
const StateContext = createContext<GameModeDto["stateVisuals"] | null>(null);
const StorageContext = createContext<ResourceStorageDto | undefined>(undefined);

export function useCardResourceStorage(): ResourceStorageDto | undefined {
  return useContext(StorageContext);
}

export function CardCatalogPresentation({
  modes,
  modeId,
  children,
}: {
  modes: GameModeDto[];
  modeId: string;
  children: ReactNode;
}) {
  return <CatalogContext.Provider value={{ modes, modeId }}>{children}</CatalogContext.Provider>;
}

export function CardStatePresentation({
  card,
  children,
}: {
  card: Pick<CardDto, "gameModes" | "resourceStorage">;
  children: ReactNode;
}) {
  const catalog = useContext(CatalogContext);
  const settings = useGameStore((store) => (store.game ?? store.gameForSelection)?.settings);
  const modes = catalog?.modes ?? settings?.availableGameModes ?? [];
  let modeId = catalog?.modeId ?? settings?.gameModeId;
  if (!modeId && card.gameModes.length === 1) {
    modeId = card.gameModes[0];
  }
  const mode = modes.find((candidate) => candidate.id === modeId);
  return (
    <StateContext.Provider value={mode?.stateVisuals ?? null}>
      <StorageContext.Provider value={card.resourceStorage}>{children}</StorageContext.Provider>
    </StateContext.Provider>
  );
}

export function CardSourcePresentation({
  cardId,
  cardName,
  children,
}: {
  cardId?: string;
  cardName?: string;
  children: ReactNode;
}) {
  const card = useGameStore((store) => {
    const game = store.game ?? store.gameForSelection;
    if (!game || (!cardId && !cardName)) return undefined;
    const matches = (candidate: CardDto) =>
      cardId ? candidate.id === cardId : candidate.name === cardName;
    const hand = game.currentPlayer.cards.find(matches);
    if (hand) return hand;
    for (const player of [game.currentPlayer, ...game.otherPlayers]) {
      if (player.corporation && matches(player.corporation)) return player.corporation;
      const played = player.playedCards.find(matches);
      if (played) return played;
    }
    return undefined;
  });
  if (!card) return <>{children}</>;
  return <CardStatePresentation card={card}>{children}</CardStatePresentation>;
}

function useStateVisual(state: string, value?: number) {
  const descriptors = useContext(StateContext);
  const settings = useGameStore((store) => store.game?.settings);
  const mode = settings?.availableGameModes.find(
    (candidate) => candidate.id === settings.gameModeId,
  );
  const descriptor = (descriptors ?? mode?.stateVisuals)?.[state];
  const variant = value === undefined ? undefined : descriptor?.values?.[String(value)];
  const icon = variant?.icon ?? descriptor?.icon ?? "effect";
  const label = variant?.label ?? descriptor?.label ?? state;
  return { icon, label, hasVariant: Boolean(variant) };
}

export function useStateVisualLabel(state: string, value?: number): string {
  return useStateVisual(state, value).label;
}

export function StateSymbol({
  state,
  value,
  className = "",
}: {
  state: string;
  value?: number;
  className?: string;
}) {
  const { icon, label, hasVariant } = useStateVisual(state, value);
  return (
    <span
      className={`inline-flex items-center gap-0.5 ${className}`}
      role="img"
      aria-label={label}
      data-state-symbol={state}
      data-state-value={value}
    >
      <GameIcon iconType={icon} size="behavior" />
      {value !== undefined && !hasVariant && (
        <span className="font-orbitron text-[11px]">={value}</span>
      )}
    </span>
  );
}

export function visibleStateRequirements<T extends { state?: string; equals?: number }>(
  requirements: T[],
): T[] {
  const disconnected = requirements.some(
    (item) => item.state === "earth-contact" && item.equals === 0,
  );
  return disconnected
    ? requirements.filter((item) => item.state !== "withdrawal-occurred" || item.equals !== 1)
    : requirements;
}

export function StatePredicates({ requirements }: { requirements?: StateRequirementDto[] }) {
  if (!requirements?.length) {
    return null;
  }
  return (
    <span className="inline-flex flex-wrap items-center justify-center gap-1" data-state-predicates>
      {visibleStateRequirements(requirements).map((requirement) => (
        <StateSymbol
          key={`${requirement.state}:${requirement.equals}`}
          state={requirement.state}
          value={requirement.equals}
        />
      ))}
    </span>
  );
}

export function ModeSymbolLegend({ mode }: { mode: GameModeDto }) {
  const descriptors = Object.values(mode.stateVisuals ?? {});
  if (!descriptors.length) {
    return null;
  }
  const symbols = descriptors.flatMap((descriptor) =>
    descriptor.values ? Object.values(descriptor.values) : [descriptor],
  );
  const operations = [
    {
      icon: "technology-import",
      label:
        "Make the Earth-dependent effects of one eligible owned project permanent, including upkeep.",
    },
    { icon: "operation-retired", label: "Support permanently shut down." },
    { icon: "withdrawal-earlier", label: "Earth withdraws one generation earlier." },
    { icon: "withdrawal-later", label: "Delay Earth's withdrawal by one generation." },
    {
      icon: "contact-window-generation",
      label: "Restore Earth contact for the number of generations shown.",
    },
    {
      icon: "contact-window-output",
      label: "Restore Earth contact for production only. No actions can be taken.",
    },
    { icon: "permanent", label: "Permanent effects or production, independent of Earth contact." },
    {
      icon: "reserve",
      label: "Stored reserves. The card explains how much can be stored and gained.",
    },
  ];
  return (
    <section aria-label="Card symbols" className="mb-6 space-y-3">
      <div className="grid gap-2 text-sm text-white/75">
        {[...symbols, ...operations].map((symbol) => (
          <div key={symbol.icon} className="flex items-center gap-3">
            <GameIcon iconType={symbol.icon} size="small" />
            <span>{symbol.label}</span>
          </div>
        ))}
      </div>
    </section>
  );
}
