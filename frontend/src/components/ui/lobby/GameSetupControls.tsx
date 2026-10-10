import { useId, useState } from "react";
import type {
  GameModeDto,
  MapInfoDto,
  UpdateGameSettingsRequest,
} from "@/types/generated/api-types.ts";
import GameSelect from "../GameSelect.tsx";
import GameButton from "../buttons/GameButton.tsx";
import MapPreview from "./MapPreview.tsx";
import GameModeRules, { modeText } from "./GameModeRules.tsx";

interface Props {
  maps: MapInfoDto[];
  modes: GameModeDto[];
  mapId: string;
  gameModeId: string;
  gameModeParameters: Record<string, number>;
  features: string[];
  maxPlayers: number;
  onChange?: (patch: UpdateGameSettingsRequest) => void;
  disabled?: boolean;
  layout?: "stacked" | "split";
}

export default function GameSetupControls({
  maps,
  modes,
  mapId,
  gameModeId,
  gameModeParameters,
  features,
  maxPlayers,
  onChange,
  disabled,
  layout = "stacked",
}: Props) {
  const id = useId();
  const [rulesOpen, setRulesOpen] = useState(false);
  const activeMap = maps.find((map) => map.id === mapId);
  const mode = modes.find((item) => item.id === gameModeId);
  const chooseMode = (modeId: string) => {
    const next = modes.find((item) => item.id === modeId);
    if (!next) {
      return;
    }
    onChange?.({
      gameModeId: modeId,
      gameModeParameters: Object.fromEntries(
        next.parameters.map((parameter) => [parameter.id, parameter.default]),
      ),
      features: features.filter((feature) => next.features.some((item) => item.id === feature)),
      maxPlayers: Math.min(maxPlayers, next.maxPlayers),
    });
  };
  const toggleFeature = (featureId: string) => {
    onChange?.({
      features: features.includes(featureId)
        ? features.filter((item) => item !== featureId)
        : [...features, featureId],
    });
  };
  const settings = (
    <div className="flex flex-col gap-5 min-w-0">
      <div className="flex items-center justify-between gap-4">
        <label htmlFor={`${id}-mode`} className="text-sm text-white/60">
          Game mode
        </label>
        {onChange ? (
          <GameSelect
            id={`${id}-mode`}
            label="Game mode"
            value={gameModeId}
            options={modes}
            disabled={disabled}
            onChange={chooseMode}
            className="w-full max-w-64"
          />
        ) : (
          <span className="font-orbitron text-sm">{mode?.name ?? gameModeId}</span>
        )}
      </div>
      {mode && (
        <div className="border-l-2 border-space-blue-400/60 pl-4">
          <p className="text-sm leading-relaxed text-white/75">
            {modeText(mode.summary, gameModeParameters)}
          </p>
          <GameButton
            emphasis="quiet"
            size="sm"
            onClick={() => setRulesOpen(true)}
            className="mt-2 min-w-11 !px-1"
          >
            Rules
          </GameButton>
        </div>
      )}
      {mode?.parameters.map((parameter) => (
        <div key={parameter.id} className="flex items-center justify-between gap-4">
          <label htmlFor={`${id}-${parameter.id}`} className="text-sm text-white/70">
            {parameter.label}
          </label>
          {onChange ? (
            <GameSelect
              id={`${id}-${parameter.id}`}
              label={parameter.label}
              value={String(gameModeParameters[parameter.id] ?? parameter.default)}
              options={parameter.values.map((value) => ({
                id: String(value),
                name: String(value),
              }))}
              disabled={disabled}
              onChange={(value) =>
                onChange({
                  gameModeParameters: { ...gameModeParameters, [parameter.id]: Number(value) },
                })
              }
              className="w-24"
            />
          ) : (
            <span className="font-orbitron">
              {gameModeParameters[parameter.id] ?? parameter.default}
            </span>
          )}
        </div>
      ))}
      {[false, true].map((experimental) => {
        const options =
          mode?.features.filter((feature) => feature.experimental === experimental) ?? [];
        if (!options.length) {
          return null;
        }
        return (
          <fieldset
            key={String(experimental)}
            className="border-t border-white/10 pt-3"
            disabled={disabled || !onChange}
          >
            <legend className="font-orbitron text-xs text-white/60 pr-3">
              {experimental ? "Experimental features" : "Options"}
            </legend>
            {options.map((feature) => (
              <label
                key={feature.id}
                className="flex items-start gap-3 py-3 min-h-11 cursor-pointer"
              >
                <input
                  type="checkbox"
                  className="game-checkbox mt-0.5 shrink-0"
                  checked={features.includes(feature.id)}
                  onChange={() => toggleFeature(feature.id)}
                />
                <span className="min-w-0">
                  <span className="block font-orbitron text-sm">{feature.name}</span>
                  <span className="block text-xs leading-relaxed text-white/60 mt-1">
                    {feature.description}
                  </span>
                </span>
              </label>
            ))}
          </fieldset>
        );
      })}
      <div className="flex items-center justify-between gap-4 border-t border-white/10 pt-4">
        <label htmlFor={`${id}-map`} className="text-sm text-white/60">
          Map
        </label>
        {onChange ? (
          <GameSelect
            id={`${id}-map`}
            label="Map"
            value={mapId}
            options={maps}
            disabled={disabled}
            onChange={(value) => onChange({ mapId: value })}
            className="w-full max-w-60"
          />
        ) : (
          <span className="font-orbitron text-sm">{activeMap?.name ?? mapId}</span>
        )}
      </div>
    </div>
  );
  return (
    <section
      className={`text-white text-left min-w-0 ${layout === "split" ? "h-full min-h-0 flex gap-4" : "flex flex-col gap-5"}`}
    >
      <div
        className={
          layout === "split" ? "flex-1 min-w-0 overflow-y-auto overscroll-contain pr-2" : "min-w-0"
        }
      >
        {settings}
      </div>
      <div
        key={mapId}
        className={
          layout === "split"
            ? "menu-enter shrink-0 self-center max-w-[40%] w-72 [&_svg]:w-full [&_svg]:h-auto"
            : "menu-enter flex justify-center py-3 [&_svg]:w-full [&_svg]:max-w-[300px] [&_svg]:h-auto compact:hidden"
        }
      >
        <MapPreview tiles={activeMap?.tiles ?? []} />
      </div>
      {mode && (
        <GameModeRules
          mode={mode}
          parameters={gameModeParameters}
          open={rulesOpen}
          onClose={() => setRulesOpen(false)}
        />
      )}
    </section>
  );
}
