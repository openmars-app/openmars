import { useEffect, useRef, useState, type ReactNode } from "react";
import type {
  CardDto,
  ErrorPayload,
  PendingBehaviorResolutionDto,
  PlayerCardDto,
} from "@/types/generated/api-types";
import { globalWebSocketManager } from "@/services/globalWebSocketManager";
import { webSocketService } from "@/services/webSocketService";
import { useLayoutMode } from "@/hooks/useLayoutMode";
import { useElementSize } from "@/hooks/useElementSize";
import BehaviorSection from "../cards/BehaviorSection";
import CardChoice from "../cards/CardChoice";
import { CardFitHeightContext } from "../cards/CardFitHeightContext";
import GameButton from "../buttons/GameButton";
import {
  GameFlowPopover,
  GameFlowTitle,
  GameFlowBody,
  GameFlowFooter,
} from "../popover/GameFlowPopover";

const DISCARD_ROW_BOTTOM_PADDING = 16;

function DiscardRow({ children }: { children: ReactNode }) {
  const { isCompact } = useLayoutMode();
  if (!isCompact) {
    return <div className="flex flex-wrap gap-3 justify-center">{children}</div>;
  }
  return <CompactDiscardRow>{children}</CompactDiscardRow>;
}

function CompactDiscardRow({ children }: { children: ReactNode }) {
  const scrollerRef = useRef<HTMLDivElement>(null);
  const { height } = useElementSize(scrollerRef);
  let fitHeight: number | null = null;
  if (height > 0) {
    fitHeight = Math.max(0, height - DISCARD_ROW_BOTTOM_PADDING);
  }
  return (
    <div className="relative flex-1 min-h-[192px]">
      <div
        ref={scrollerRef}
        className="absolute inset-0 flex items-start justify-center-safe gap-3 pb-4 overflow-x-auto overflow-y-hidden overscroll-contain snap-x snap-proximity"
      >
        <CardFitHeightContext.Provider value={fitHeight}>{children}</CardFitHeightContext.Provider>
      </div>
    </div>
  );
}

interface Props {
  resolutions: PendingBehaviorResolutionDto[];
  handCards: PlayerCardDto[];
  playedCards: CardDto[];
  corporation?: CardDto;
}

export default function BehaviorResolutionOverlay({
  resolutions,
  handCards,
  playedCards,
  corporation,
}: Props) {
  const [activeId, setActiveId] = useState(resolutions[0]?.id);
  const active = resolutions.find((item) => item.id === activeId) ?? resolutions[0];
  const [selectedCards, setSelectedCards] = useState<string[]>([]);
  const [choiceIndex, setChoiceIndex] = useState<number | null>(null);
  const [targets, setTargets] = useState<string[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const submission = useRef<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const { isCompact } = useLayoutMode();

  useEffect(() => {
    setActiveId(active?.id);
    setSelectedCards([]);
    setChoiceIndex(null);
    setTargets([]);
    setSubmitting(false);
    submission.current = null;
    setError(null);
  }, [active?.id]);

  useEffect(() => {
    const failed = (payload: unknown) => {
      const result = payload as Partial<ErrorPayload>;
      if (submission.current && result.resolutionId === submission.current) {
        setError(result.message ?? "Could not resolve effect");
        submission.current = null;
        setSubmitting(false);
      }
    };
    const disconnected = () => {
      if (submission.current) {
        setError("Connection lost. Reconnect to check whether this effect was resolved.");
        submission.current = null;
        setSubmitting(false);
      }
    };
    webSocketService.on("error", failed);
    webSocketService.on("disconnect", disconnected);
    return () => {
      webSocketService.off("error", failed);
      webSocketService.off("disconnect", disconnected);
    };
  }, []);

  if (!active) {
    return null;
  }

  const optional = active.optionalEffect;
  const optionalInput = optional?.behavior.inputs?.[0];
  let optionalLabel = "Apply effect";
  if (optionalInput?.type === "credit") {
    optionalLabel = `Pay ${optionalInput.amount} M€`;
  } else if (optionalInput?.type === "energy") {
    optionalLabel = `Pay ${optionalInput.amount} energy`;
  } else if (optionalInput?.type === "energy-production") {
    optionalLabel = "Decrease energy production";
  }

  const choice = active.choices?.find((item) => item.originalIndex === choiceIndex);
  const storageTargets = choice?.storageTargets ?? [];
  const validCards = selectedCards.filter((id) => handCards.some((card) => card.id === id));
  const allCards = corporation ? [...playedCards, corporation] : playedCards;
  const canConfirmChoice =
    !!choice?.available &&
    storageTargets.every((eligible, index) => eligible.includes(targets[index]));
  const canConfirmDiscard =
    validCards.length >= active.minCards && validCards.length <= active.maxCards;
  const groups = new Map<string, PendingBehaviorResolutionDto[]>();
  for (const resolution of resolutions) {
    const key = `${resolution.sourceCardId}:${resolution.sourceBehaviorIndex}:${resolution.triggeringCardId ?? ""}:${resolution.kind}`;
    const entries = groups.get(key) ?? [];
    entries.push(resolution);
    groups.set(key, entries);
  }

  const submit = async (skip = false) => {
    if (
      submission.current ||
      (active.kind === "choice" && !canConfirmChoice) ||
      (active.kind === "optional" && !skip && !optional?.available)
    ) {
      return;
    }
    submission.current = active.id;
    setSubmitting(true);
    setError(null);
    try {
      if (active.kind === "optional") {
        await globalWebSocketManager.confirmOptionalBehavior(active.id, !skip);
      } else if (active.kind === "card-discard") {
        await globalWebSocketManager.confirmCardDiscard(active.id, skip ? [] : validCards);
      } else if (choice && canConfirmChoice) {
        await globalWebSocketManager.confirmBehaviorChoice(
          active.id,
          choice.originalIndex,
          targets,
        );
      }
    } catch (reason) {
      submission.current = null;
      setSubmitting(false);
      setError(reason instanceof Error ? reason.message : "Could not resolve effect");
    }
  };

  let panelClass = "w-[min(920px,95vw)]! max-h-[90dvh]! text-white";
  if (optional) {
    panelClass = "w-[min(480px,95vw)]! max-h-[90dvh]! text-white";
  }
  if (isCompact) {
    panelClass = "text-white";
  }

  return (
    <GameFlowPopover isVisible type="interactive-mandatory" className={panelClass} compactFill>
      <GameFlowTitle className="compact:py-2.5">
        <h3 className="font-orbitron text-base font-bold">
          {active.kind === "optional"
            ? "Optional effect"
            : `Resolve effects · ${resolutions.length} remaining`}
        </h3>
      </GameFlowTitle>
      {groups.size > 1 && (
        <div className="shrink-0 flex flex-wrap gap-2 px-4 pb-3 compact:flex-nowrap compact:overflow-x-auto compact:overscroll-contain compact:pt-2 compact:pb-2">
          {[...groups.entries()].map(([key, entries]) => (
            <GameButton
              key={key}
              size="sm"
              emphasis="secondary"
              aria-pressed={entries.some((item) => item.id === active.id)}
              selected={entries.some((item) => item.id === active.id)}
              disabled={submitting}
              className="shrink-0 compact:min-h-11"
              onClick={() => setActiveId(entries[0].id)}
            >
              {entries[0].source} · {entries.length}
            </GameButton>
          ))}
        </div>
      )}
      <GameFlowBody>
        <div className="flex flex-col gap-4 compact:gap-2 p-1 compact:min-h-full">
          <div>
            <p className="font-orbitron text-sm">{active.source}</p>
            {active.triggeringCardName && (
              <p className="text-sm text-white/70">Triggered by {active.triggeringCardName}</p>
            )}
          </div>
          {optional && (
            <div className="flex flex-col gap-3">
              <p className="text-sm text-white/80">{optional.behavior.description}</p>
              <BehaviorSection
                cardId={active.sourceCardId}
                behaviors={[optional.behavior]}
                noContainer
                showTooltips={false}
                greyOutAll={!optional.available}
              />
              {!optional.available &&
                optional.errors.map((issue, index) => (
                  <p key={index} className="text-sm text-red-300">
                    {issue.message}
                  </p>
                ))}
            </div>
          )}
          {active.kind === "card-discard" ? (
            <>
              <p className="text-sm">
                Select{" "}
                {active.minCards === active.maxCards ? active.minCards : `up to ${active.maxCards}`}{" "}
                card(s) to discard.
              </p>
              <BehaviorSection
                cardId={active.sourceCardId}
                behaviors={[{ outputs: active.outputs }]}
                noContainer
                showTooltips={false}
              />
              {handCards.length === 0 && (
                <p className="text-white/70">
                  No cards in hand. You can skip or resolve another effect first.
                </p>
              )}
              <DiscardRow>
                {handCards.map((card) => (
                  <div key={card.id} className="shrink-0 snap-start">
                    <CardChoice
                      card={card}
                      isSelected={validCards.includes(card.id)}
                      disabled={submitting}
                      showCheckbox
                      onSelect={(id) => {
                        if (submitting) {
                          return;
                        }
                        setSelectedCards((previous) => {
                          const current = previous.filter((item) =>
                            handCards.some((candidate) => candidate.id === item),
                          );
                          if (current.includes(id)) {
                            return current.filter((item) => item !== id);
                          }
                          return current.length < active.maxCards ? [...current, id] : current;
                        });
                      }}
                    />
                  </div>
                ))}
              </DiscardRow>
            </>
          ) : (
            !optional && (
              <div className="flex flex-col gap-2">
                {active.choices?.map((item) => (
                  <div key={item.originalIndex}>
                    <GameButton
                      emphasis="secondary"
                      aria-pressed={choiceIndex === item.originalIndex}
                      className="w-full p-3"
                      selected={choiceIndex === item.originalIndex}
                      disabled={!item.available || submitting}
                      onClick={() => {
                        setChoiceIndex(item.originalIndex);
                        setTargets([]);
                        setError(null);
                      }}
                    >
                      <div className="flex flex-col gap-2">
                        <BehaviorSection
                          cardId={active.sourceCardId}
                          behaviors={[
                            {
                              triggers: [{ type: "manual" }],
                              inputs: item.inputs,
                              outputs: item.outputs,
                            },
                          ]}
                          hideActionChip
                          noContainer
                          showTooltips={false}
                        />
                        {item.outputs
                          ?.filter((output) => output.target === "triggering-card")
                          .map((output, index) => (
                            <span key={index} className="text-sm">
                              Add {output.amount} {output.type} to {active.triggeringCardName}
                            </span>
                          ))}
                      </div>
                    </GameButton>
                    {!item.available &&
                      item.errors?.map((issue, index) => (
                        <p key={index} className="text-sm text-white/60 mt-1">
                          {issue.message}
                        </p>
                      ))}
                  </div>
                ))}
                {storageTargets.map((eligible, index) => (
                  <label key={index} className="flex flex-col gap-1 text-sm">
                    Resource destination {index + 1}
                    <select
                      className="bg-black border border-white/30 rounded p-2 compact:min-h-11"
                      value={targets[index] ?? ""}
                      disabled={submitting}
                      onChange={(event) =>
                        setTargets((previous) => {
                          const next = [...previous];
                          next[index] = event.target.value;
                          return next;
                        })
                      }
                    >
                      <option value="">Choose a card</option>
                      {eligible.map((id) => (
                        <option key={id} value={id}>
                          {allCards.find((card) => card.id === id)?.name ?? id}
                        </option>
                      ))}
                    </select>
                  </label>
                ))}
              </div>
            )
          )}
          {error && (
            <p role="alert" className="text-sm text-red-300">
              {error}
            </p>
          )}
        </div>
      </GameFlowBody>
      <GameFlowFooter>
        <div className="flex justify-end gap-2">
          {(active.kind === "optional" ||
            (active.kind === "card-discard" && active.minCards === 0)) && (
            <GameButton emphasis="quiet" disabled={submitting} onClick={() => void submit(true)}>
              Skip
            </GameButton>
          )}
          <GameButton
            disabled={
              submitting ||
              (active.kind === "optional" && !optional?.available) ||
              (active.kind === "choice" && !canConfirmChoice) ||
              (active.kind === "card-discard" && !canConfirmDiscard)
            }
            onClick={() => void submit()}
          >
            {submitting ? "Resolving…" : optionalLabelForKind(active.kind, optionalLabel)}
          </GameButton>
        </div>
      </GameFlowFooter>
    </GameFlowPopover>
  );
}

function optionalLabelForKind(kind: string, label: string): string {
  return kind === "optional" ? label : "Confirm";
}
