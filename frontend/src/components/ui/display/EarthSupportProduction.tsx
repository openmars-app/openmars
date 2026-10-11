import type { GameDto, ModeProductionDto, ProductionDto } from "@/types/generated/api-types.ts";
import GameIcon from "./GameIcon.tsx";

const resources: { key: keyof ProductionDto; icon: string; label: string }[] = [
  { key: "credits", icon: "credit", label: "M€" },
  { key: "steel", icon: "steel", label: "Steel" },
  { key: "titanium", icon: "titanium", label: "Titanium" },
  { key: "plants", icon: "plant", label: "Plants" },
  { key: "energy", icon: "energy", label: "Energy" },
  { key: "heat", icon: "heat", label: "Heat" },
];

export function earthSupportLabel(game: GameDto): string | undefined {
  const values = game.modeRuntime?.effectiveValues;
  const withdrawal = values?.["withdrawal-generation"];
  if (withdrawal === undefined) {
    return undefined;
  }
  if (values?.["withdrawal-occurred"] !== 0) {
    return values?.["earth-contact"] === 1 ? "Earth connected" : "Earth disconnected";
  }
  const remaining = Math.max(0, withdrawal - game.generation);
  if (remaining === 0) {
    return "Earth withdraws this generation";
  }
  return `Earth withdraws in ${remaining} generation${remaining === 1 ? "" : "s"}`;
}

export default function EarthSupportProduction({ production }: { production?: ModeProductionDto }) {
  if (!production) {
    return <p className="p-3 text-sm text-white/60">No player selected.</p>;
  }
  return (
    <table
      className="w-full table-fixed text-sm text-white"
      aria-label="Your production with and without Earth"
    >
      <thead className="text-xs text-white/60">
        <tr>
          <th scope="col" className="px-2 py-2 font-normal">
            Connected
          </th>
          <th scope="col" className="px-2 py-2 font-normal">
            Disconnected
          </th>
        </tr>
      </thead>
      <tbody>
        {resources.map(({ key, icon, label }) => (
          <tr key={key} className="border-t border-white/5">
            {(["connected", "disconnected"] as const).map((state) => {
              const value = production[state][key];
              const reduced = state === "disconnected" && value < production.connected[key];
              return (
                <td key={state} className="px-3 py-1 text-center" aria-label={`${value} ${label}`}>
                  <span
                    aria-hidden="true"
                    className={`inline-flex items-center justify-center gap-2 align-middle font-orbitron tabular-nums ${reduced ? "text-red-300" : ""}`}
                  >
                    {key === "credits" ? (
                      <GameIcon iconType="credit" amount={value} size="small" />
                    ) : (
                      <>
                        <span>{value}</span>
                        <GameIcon iconType={icon} size="small" />
                      </>
                    )}
                  </span>
                </td>
              );
            })}
          </tr>
        ))}
      </tbody>
    </table>
  );
}
