import GameIcon from "@/components/ui/display/GameIcon";
import { useState } from "react";
import type { ColonyDto, PendingEffectSelectionDto } from "@/types/generated/api-types.ts";
import GameButton from "../buttons/GameButton.tsx";
import ColonyOutputDisplay from "../display/ColonyOutputDisplay.tsx";

export function isColonyTrackSelection(selection: PendingEffectSelectionDto) {
  return (
    selection.outputs.length > 0 &&
    selection.outputs.every((output) => output.type === "colony-track-step") &&
    selection.options.every(
      (option) =>
        option.colonyIds?.length === selection.outputs.length &&
        !option.cardId &&
        !option.targetPlayerId &&
        !option.outputs?.length,
    )
  );
}

export default function ColonyTrackSelection({
  selection,
  colonies,
  disabled,
  onSelect,
}: {
  selection: PendingEffectSelectionDto;
  colonies: ColonyDto[];
  disabled: boolean;
  onSelect: (index: number | null) => void;
}) {
  const [chosen, setChosen] = useState<(string | undefined)[]>([]);
  // Keep the server's output positions when displaying decreases before increases.
  const columns = selection.outputs
    .map((output, position) => ({ amount: output.amount, position }))
    .sort((a, b) => a.amount - b.amount);

  const matches = (ids: (string | undefined)[], optionIds: string[]) =>
    ids.every((id, position) => id === undefined || optionIds[position] === id);

  const choose = (position: number, id: string) => {
    const value = chosen[position] === id ? undefined : id;
    const next = selection.outputs.map((_, index) => (index === position ? value : chosen[index]));
    setChosen(next);
    const index = next.every(Boolean)
      ? selection.options.findIndex((option) => matches(next, option.colonyIds!))
      : -1;
    onSelect(index < 0 ? null : index);
  };

  return (
    <div className="flex flex-col gap-4">
      <p className="m-0 text-sm text-white/65">
        Choose a colony for each track change. Changes apply together.
      </p>
      <div
        className={`grid ${columns.length > 1 ? "grid-cols-2" : "grid-cols-1"} gap-3 compact:gap-2`}
      >
        {columns.map(({ amount, position }) => {
          const decreasing = amount < 0;
          const label = decreasing ? "Decrease" : "Increase";
          const ids = new Set(selection.options.map((option) => option.colonyIds![position]));
          const candidates = colonies.filter((colony) => ids.has(colony.id));
          return (
            <section
              key={position}
              aria-label={`${label} colony track`}
              className="min-w-0 border border-white/15 bg-white/[0.02]"
            >
              <h3 className="m-0 flex items-center justify-between gap-2 border-b border-white/10 px-3 py-3 compact:py-2 font-orbitron text-sm text-white/90">
                {label}
                <span>
                  {amount > 0 ? "+" : "−"}
                  {Math.abs(amount)}
                </span>
              </h3>
              <div className="p-2 space-y-2" role="group" aria-label={label}>
                {candidates.map((colony) => {
                  const next = selection.outputs.map((_, index) =>
                    index === position ? colony.id : chosen[index],
                  );
                  const available = selection.options.some((option) =>
                    matches(next, option.colonyIds!),
                  );
                  const selected = chosen[position] === colony.id;
                  const current = colony.steps[colony.markerPosition]?.outputs ?? [];
                  const result = colony.steps[colony.markerPosition + amount]?.outputs ?? [];
                  return (
                    <GameButton
                      key={colony.id}
                      emphasis="secondary"
                      selected={selected}
                      aria-pressed={selected}
                      aria-label={colony.name}
                      disabled={disabled || !available}
                      className="w-full min-h-16 px-3 py-2 text-left compact:min-h-14"
                      onClick={() => choose(position, colony.id)}
                    >
                      <span className="flex w-full flex-col gap-1.5">
                        <span className="flex justify-between gap-2 font-orbitron text-xs">
                          <GameIcon iconType={colony.style.icon} size="small" /> {colony.name}
                        </span>
                        <span
                          className="flex items-center gap-2 text-white/60"
                          aria-label="Trade gain before and after"
                        >
                          <ColonyOutputDisplay outputs={current} />
                          <span aria-hidden="true">→</span>
                          <ColonyOutputDisplay outputs={result} />
                        </span>
                      </span>
                    </GameButton>
                  );
                })}
              </div>
            </section>
          );
        })}
      </div>
    </div>
  );
}
