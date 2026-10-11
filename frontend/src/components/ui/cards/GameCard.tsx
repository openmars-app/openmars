import { CardStatePresentation } from "./CardStateVisuals.tsx";
import { cardImage } from "@/assets";
import { useLayoutEffect, useRef, useState, type ReactNode } from "react";
import GameIcon from "../display/GameIcon.tsx";
import CardDecorBar from "../display/CardDecorBar.tsx";
import BehaviorSection, { hasBehaviorVisuals } from "./BehaviorSection";
import RequirementsBox from "./RequirementsBox.tsx";
import CardTagList from "./CardTagList.tsx";
import { CardDto, PlayerCardDto, ResourceTypeCredit } from "@/types/generated/api-types.ts";
import { CardDescriptionSections } from "../display/CardDescriptionSections.tsx";
import { CARD_TYPE_COLORS } from "@/utils/cardTypeColors.ts";
import { Z_INDEX } from "@/constants/zIndex.ts";

export interface GameCardProps {
  card: CardDto | PlayerCardDto;
  isSelected?: boolean;
  showCheckbox?: boolean;
  presentation?: "compact" | "inspection";
  dimUnavailable?: boolean;
  moduleState?: "idle" | "armed" | "releasing";
  description?: ReactNode;
}

export default function GameCard({
  card,
  isSelected = false,
  showCheckbox = false,
  presentation = "compact",
  dimUnavailable = presentation === "compact",
  moduleState = showCheckbox && isSelected ? "armed" : "idle",
  description,
}: GameCardProps) {
  const hasState = "available" in card && "effectiveCost" in card;
  const effectiveCost = hasState ? card.effectiveCost : card.cost;
  const discounted = effectiveCost < card.cost;
  const accent = CARD_TYPE_COLORS[card.type as keyof typeof CARD_TYPE_COLORS] ?? "#4a90e2";
  const hasTags = !!card.tags?.length || card.type === "event";
  const artwork = cardImage(card.id);

  return (
    <CardStatePresentation card={card}>
      <div
        className="game-card select-none [&_*]:select-none"
        data-presentation={presentation}
        data-module-state={moduleState}
        data-card-name={card.name}
        onDragStart={(event) => event.preventDefault()}
      >
        <div className="game-card-requirements">
          <RequirementsBox requirements={card.requirements} inFlow />
        </div>
        <div
          className={`game-card-body group ${dimUnavailable && hasState && !card.available ? "grayscale-[0.6] brightness-[0.65] saturate-[0.2]" : ""}`}
          style={
            {
              "--card-accent": accent,
              "--module-decoration-layer": Z_INDEX.CONTROL_DECORATION,
            } as React.CSSProperties
          }
        >
          <CardChassis showConnector={!showCheckbox} />
          <div className="game-card-stripe" aria-hidden="true">
            <span />
          </div>
          <div className="game-card-heading">
            <CircuitTrace position="art" />
            <div className="game-card-artwork">
              <CardArtwork key={card.id} artwork={artwork} name={card.name} />
            </div>
            {card.type !== "prelude" && (
              <div
                className="game-card-cost absolute top-2 left-2 group/cost"
                style={{ zIndex: Z_INDEX.GAME_BOARD_EFFECTS }}
              >
                <div
                  className={
                    discounted
                      ? "animate-[goldenPulse_2.5s_ease-in-out_infinite] transition-opacity duration-400 pointer-fine:group-hover/cost:opacity-0"
                      : ""
                  }
                >
                  <GameIcon iconType={ResourceTypeCredit} amount={effectiveCost} size="medium" />
                </div>
                {discounted && (
                  <div className="absolute inset-0 opacity-0 transition-opacity duration-400 pointer-fine:group-hover/cost:opacity-100 pointer-coarse:hidden">
                    <GameIcon iconType={ResourceTypeCredit} amount={card.cost} size="medium" />
                  </div>
                )}
                {discounted && (
                  <span className="hidden pointer-coarse:block absolute left-1/2 top-full -translate-x-1/2 font-orbitron text-[12px] font-bold leading-none text-white/70 line-through [text-shadow:1px_1px_2px_rgba(0,0,0,0.8)]">
                    {card.cost}
                  </span>
                )}
              </div>
            )}
            <h3 className="game-card-title font-orbitron" data-card-type={card.type}>
              <span>{card.name}</span>
            </h3>
            <CircuitTrace position="title" />
            {hasTags && (
              <CardTagList
                card={card}
                size="card"
                className="game-card-tags"
                style={{ zIndex: Z_INDEX.GAME_BOARD_EFFECTS }}
              />
            )}
          </div>
          <div className="game-card-labels">
            <CardDecorBar vpConditions={card.vpConditions} resourceStorage={card.resourceStorage} />
          </div>
          {hasBehaviorVisuals(card.behaviors, presentation) && (
            <div className="game-card-behaviors">
              <CardPanelCircuit />
              <BehaviorSection
                cardId={card.id}
                behaviors={card.behaviors}
                computedValues={hasState ? card.computedValues : undefined}
                presentation={presentation}
              />
            </div>
          )}
          {presentation === "inspection" && card.description.length > 0 && (
            <div className="game-card-description">
              {description ?? <CardDescriptionSections sections={card.description} />}
            </div>
          )}
          {showCheckbox && (
            <div
              className="absolute -bottom-3 left-1/2 -translate-x-1/2"
              style={{ zIndex: Z_INDEX.GAME_BOARD_EFFECTS }}
            >
              <div
                className="w-6 h-6 rounded-full border-2 flex items-center justify-center transition-colors duration-200"
                style={{
                  backgroundColor: isSelected
                    ? `color-mix(in srgb, ${accent} 25%, #15202f)`
                    : "#2a3142",
                  borderColor: isSelected ? accent : "rgba(100,150,200,0.3)",
                }}
              >
                {isSelected && <span className="text-white text-sm font-bold">✓</span>}
              </div>
            </div>
          )}
        </div>
      </div>
    </CardStatePresentation>
  );
}

export function CardChassis({ showConnector }: { showConnector: boolean }) {
  return (
    <div
      className="game-card-chassis"
      aria-hidden="true"
      style={{ zIndex: Z_INDEX.CONTROL_DECORATION }}
    >
      <div className="game-card-frame" />
      <CircuitTrace position="feed" />
      <CircuitTrace position="spine" />
      <span className="game-card-catch game-card-catch--left" />
      <span className="game-card-catch game-card-catch--right" />
      {showConnector && <span className="game-card-connector" />}
    </div>
  );
}

export function CardPanelCircuit() {
  const panelRef = useRef<HTMLSpanElement>(null);
  const [size, setSize] = useState({ width: 0, height: 0 });
  useLayoutEffect(() => {
    const panel = panelRef.current;
    if (!panel) {
      return;
    }
    const observer = new ResizeObserver(([entry]) => {
      const { width, height } = entry.contentRect;
      setSize((previous) =>
        previous.width === width && previous.height === height ? previous : { width, height },
      );
    });
    observer.observe(panel);
    return () => observer.disconnect();
  }, []);
  const right = size.width - 0.5;
  const bottom = size.height - 0.5;
  const paths = [`M 0.5 ${bottom} V 0.5 H ${right}`, `M 0.5 ${bottom} H ${right} V 0.5`];
  return (
    <span ref={panelRef} className="card-panel-circuit" aria-hidden="true">
      {size.width > 0 && size.height > 0 && (
        <svg
          viewBox={`0 0 ${size.width} ${size.height}`}
          preserveAspectRatio="none"
          focusable="false"
        >
          <g className="card-panel-trails">
            {paths.map((d, index) => (
              <path key={index} d={d} pathLength="1" />
            ))}
          </g>
          <g className="card-panel-tips">
            {paths.map((d, index) => (
              <path key={index} d={d} pathLength="1" />
            ))}
          </g>
        </svg>
      )}
      <span className="card-panel-junction" />
    </span>
  );
}

function CircuitTrace({ position }: { position: "feed" | "spine" | "art" | "title" }) {
  return (
    <span className={`game-card-circuit game-card-circuit--${position}`} aria-hidden="true">
      <span />
    </span>
  );
}

function CardArtwork({ artwork, name }: { artwork: ReturnType<typeof cardImage>; name: string }) {
  const [failed, setFailed] = useState(false);
  const [loaded, setLoaded] = useState(false);
  if (!artwork || failed) {
    return null;
  }
  return (
    <img
      {...artwork}
      decoding="async"
      alt={name}
      className={`w-full h-full object-cover transition-opacity duration-300 ${loaded ? "opacity-100" : "opacity-0"}`}
      onLoad={() => setLoaded(true)}
      onError={() => setFailed(true)}
    />
  );
}
