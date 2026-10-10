import { useState } from "react";
import type { GameDto, ProductionDto } from "@/types/generated/api-types.ts";
import GameModeRules from "../lobby/GameModeRules.tsx";

interface Props {
  game: GameDto;
  compact?: boolean;
}

const resources: [keyof ProductionDto, string][] = [
  ["credits", "M€"],
  ["steel", "Steel"],
  ["titanium", "Titanium"],
  ["plants", "Plants"],
  ["energy", "Energy"],
  ["heat", "Heat"],
];

export default function GameModeStatus({ game, compact = false }: Props) {
  const [open, setOpen] = useState(false);
  const mode = game.settings.availableGameModes.find(
    (item) => item.id === game.settings.gameModeId,
  );
  if (!mode) return null;
  const runtime = game.modeRuntime;
  const windows =
    runtime?.windows.filter((window) => ["pending", "active"].includes(window.status)) ?? [];

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        aria-label={`${mode.name}: status and rules`}
        className={`w-full border-y border-white/15 bg-black/90 text-white text-left ${compact ? "px-1.5 py-2" : "px-2 py-2.5"}`}
      >
        <span className="block font-orbitron text-[9px] leading-tight text-white/60">
          {mode.name}
        </span>
        {mode.statuses.map((status) => {
          const value = runtime?.effectiveValues[status.state];
          if (value === undefined) return null;
          return (
            <span key={status.state} className="block mt-1 text-[10px] leading-tight">
              <span className="block text-white/50">{status.label}</span>
              <span className="text-sky-200">{status.labels?.[String(value)] ?? value}</span>
            </span>
          );
        })}
        {windows.length > 0 && (
          <span className="block mt-1 text-[9px] text-amber-200">
            {windows.length} temporary {windows.length === 1 ? "window" : "windows"}
          </span>
        )}
        <span className="block mt-1 text-[9px] text-white/45">Rules & details</span>
      </button>
      <GameModeRules
        mode={mode}
        parameters={game.settings.gameModeParameters}
        open={open}
        onClose={() => setOpen(false)}
      >
        {runtime && (
          <section className="mb-6 border border-white/15 bg-white/5 p-4">
            <h3 className="font-orbitron text-sm mb-3">
              Current status · Generation {game.generation}
            </h3>
            <dl className="space-y-2">
              {mode.statuses.map((status) => {
                const value = runtime.effectiveValues[status.state];
                if (value === undefined) return null;
                const initial = runtime.initialValues[status.state];
                return (
                  <div key={status.state} className="flex justify-between gap-4 text-sm">
                    <dt className="text-white/60">{status.label}</dt>
                    <dd>
                      {status.labels?.[String(value)] ?? value}
                      {status.kind === "counter" && initial !== undefined && initial !== value && (
                        <span className="text-white/40"> (started at {initial})</span>
                      )}
                    </dd>
                  </div>
                );
              })}
            </dl>
            {game.currentPlayer && (
              <div className="mt-4 border-t border-white/10 pt-3">
                <h4 className="text-sm mb-2">Your production</h4>
                <table className="w-full text-xs text-right">
                  <thead>
                    <tr className="text-white/50">
                      <th className="text-left font-normal">Resource</th>
                      <th className="font-normal">Permanent</th>
                      <th className="font-normal">Available now</th>
                    </tr>
                  </thead>
                  <tbody>
                    {resources.map(([key, label]) => (
                      <tr key={key}>
                        <th className="text-left font-normal py-1">{label}</th>
                        <td>{game.currentPlayer.permanentProduction[key]}</td>
                        <td>{game.currentPlayer.production[key]}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                <p className="text-xs text-white/50 mt-2">
                  Only permanent production can cover permanent production costs. Dependent
                  operations are marked on your played cards.
                </p>
              </div>
            )}
            {windows.length > 0 && (
              <div className="mt-4 space-y-2 text-sm text-sky-200">
                {windows.map((window) => (
                  <p key={window.id}>
                    {window.status === "active" ? "Active" : "Scheduled"}: generation{" "}
                    {window.startGeneration} {window.startBoundary.replaceAll("-", " ")} through
                    generation {window.endGeneration} {window.endBoundary.replaceAll("-", " ")}.
                    Source: {window.source}.
                  </p>
                ))}
              </div>
            )}
          </section>
        )}
      </GameModeRules>
    </>
  );
}
