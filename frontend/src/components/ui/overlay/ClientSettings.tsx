import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useOverlayPortalRoot } from "@/contexts/OverlayEnvironment";
import { useSound } from "@/contexts/SoundContext";
import { useBackDismiss } from "@/hooks/useBackDismiss";
import { useRenderPause } from "@/stores/renderPauseStore";
import { useCameraSettingsStore } from "@/stores/cameraSettingsStore";
import { useUIOverlayStore } from "@/stores/uiOverlayStore";
import { useBotThoughtsPreferenceStore } from "@/stores/botPresenceStore";
import { defaultGraphicsSettings, useGraphicsSettingsStore } from "@/stores/graphicsSettingsStore";
import { useEngineStore } from "@/stores/engineStore";
import {
  GRAPHICS_PRESETS,
  FRAME_LIMITS,
  graphicsPreset,
  residentGraphicsKey,
} from "@/engine/graphics";
import type { GraphicsSettings } from "@/engine/scene.generated";
import { Z_INDEX } from "@/constants/zIndex";
import GameButton from "../buttons/GameButton";
import CloseButton from "../buttons/CloseButton";
import GameSelect from "../GameSelect";
import {
  OVERLAY_CONTAINER_CLASS,
  OVERLAY_HEADER_CLASS,
  OVERLAY_TITLE_CLASS,
  OVERLAY_FOOTER_CLASS,
  OVERLAY_ROOT_SAFE_AREA_CLASS,
} from "./overlayStyles";

const categories = [
  { id: "controls", label: "Controls" },
  { id: "graphics", label: "Graphics" },
  { id: "audio", label: "Audio" },
  { id: "ai", label: "AI" },
] as const;
type Category = (typeof categories)[number]["id"];
const sliderClass =
  "w-full h-11 bg-transparent appearance-none cursor-pointer [&::-webkit-slider-runnable-track]:h-1 [&::-webkit-slider-runnable-track]:bg-white/20 [&::-webkit-slider-runnable-track]:rounded-full [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:w-4 [&::-webkit-slider-thumb]:h-4 [&::-webkit-slider-thumb]:-mt-1.5 [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-white [&::-moz-range-track]:h-1 [&::-moz-range-track]:bg-white/20 [&::-moz-range-thumb]:w-4 [&::-moz-range-thumb]:h-4 [&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:border-0 [&::-moz-range-thumb]:bg-white disabled:opacity-40";

function Slider({
  id,
  label,
  value,
  display,
  min,
  max,
  step,
  disabled,
  onChange,
}: {
  id: string;
  label: string;
  value: number;
  display: string;
  min: number;
  max: number;
  step: number;
  disabled?: boolean;
  onChange: (value: number) => void;
}) {
  return (
    <div>
      <label htmlFor={id} className="flex items-center justify-between gap-4 text-sm font-orbitron">
        <span>{label}</span>
        <output className="text-white/70 tabular-nums">{display}</output>
      </label>
      <input
        id={id}
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        disabled={disabled}
        aria-valuetext={display}
        className={sliderClass}
        onChange={(event) => onChange(Number(event.target.value))}
      />
    </div>
  );
}

function SettingsPanel({ onClose }: { onClose: () => void }) {
  const [category, setCategory] = useState<Category>("controls");
  const camera = useCameraSettingsStore();
  const sound = useSound();
  const graphics = useGraphicsSettingsStore((s) => s.settings);
  const applyGraphics = useGraphicsSettingsStore((s) => s.apply);
  const [draftGraphics, setDraftGraphics] = useState(graphics);
  const graphicsChanged = (Object.keys(graphics) as Array<keyof GraphicsSettings>).some(
    (key) => draftGraphics[key] !== graphics[key],
  );
  const showBotThoughts = useBotThoughtsPreferenceStore((state) => state.showBotThoughts);
  const setShowBotThoughts = useBotThoughtsPreferenceStore((state) => state.setShowBotThoughts);
  const panel = useRef<HTMLDivElement>(null);
  useRenderPause("settings", true);
  useBackDismiss(onClose);
  useEffect(() => {
    const previous = document.activeElement;
    panel.current?.querySelector<HTMLButtonElement>("[role=tab][aria-selected=true]")?.focus();
    return () => {
      if (previous instanceof HTMLElement && previous.isConnected) {
        previous.focus();
      }
    };
  }, []);
  const reset = () => {
    if (category === "controls") {
      camera.reset();
    } else if (category === "graphics") {
      setDraftGraphics(defaultGraphicsSettings());
    } else if (category === "audio") {
      sound.setVolume(0.5);
      sound.setMusicVolume(0.5);
    } else {
      setShowBotThoughts(true);
    }
  };
  return (
    <div
      className={`fixed inset-0 flex items-center justify-center bg-black/70 backdrop-blur-sm ${OVERLAY_ROOT_SAFE_AREA_CLASS}`}
      style={{ zIndex: Z_INDEX.LOBBY_SETTINGS_MODAL }}
      onClick={(event) => {
        if (event.target === event.currentTarget) {
          onClose();
        }
      }}
    >
      <div
        ref={panel}
        role="dialog"
        aria-modal="true"
        aria-labelledby="client-settings-title"
        className={`${OVERLAY_CONTAINER_CLASS} text-white !max-w-[900px] h-[min(640px,85dvh)] compact:!max-w-none compact:!h-full`}
        onKeyDown={(event) => {
          if (event.key !== "Tab") {
            return;
          }
          const controls = [
            ...panel.current!.querySelectorAll<HTMLElement>(
              "button:not(:disabled),input:not(:disabled),select:not(:disabled)",
            ),
          ];
          const first = controls[0],
            last = controls.at(-1);
          if (event.shiftKey && document.activeElement === first) {
            event.preventDefault();
            last?.focus();
          } else if (!event.shiftKey && document.activeElement === last) {
            event.preventDefault();
            first?.focus();
          }
        }}
      >
        <div
          className={`${OVERLAY_HEADER_CLASS} !flex !flex-row items-center justify-between gap-4 !py-3 compact:!py-1`}
        >
          <h2 id="client-settings-title" className={OVERLAY_TITLE_CLASS}>
            Settings
          </h2>
          <CloseButton onClick={onClose} label="Close settings" />
        </div>
        <div className="flex flex-1 min-h-0">
          <div
            role="tablist"
            aria-label="Settings categories"
            aria-orientation="vertical"
            className="w-40 compact:w-28 shrink-0 bg-black/30 border-r border-white/15 flex flex-col py-2"
          >
            {categories.map(({ id: tab, label }) => (
              <GameButton
                key={tab}
                role="tab"
                id={`settings-tab-${tab}`}
                aria-selected={category === tab}
                aria-controls="settings-panel"
                tabIndex={category === tab ? 0 : -1}
                emphasis="quiet"
                className={`!justify-start !min-h-11 !rounded-none !border-0 !px-4 ${category === tab ? "!bg-white/10 !text-white" : "!text-white/60"}`}
                onClick={() => setCategory(tab)}
                onKeyDown={(event) => {
                  if (!["ArrowUp", "ArrowDown", "Home", "End"].includes(event.key)) {
                    return;
                  }
                  event.preventDefault();
                  const index = categories.findIndex((item) => item.id === tab);
                  const direction = event.key === "ArrowUp" ? -1 : 1;
                  let next =
                    categories[(index + direction + categories.length) % categories.length].id;
                  if (event.key === "Home") {
                    next = "controls";
                  }
                  if (event.key === "End") {
                    next = "ai";
                  }
                  setCategory(next);
                  panel.current?.querySelector<HTMLButtonElement>(`#settings-tab-${next}`)?.focus();
                }}
              >
                {label}
              </GameButton>
            ))}
          </div>
          <section
            role="tabpanel"
            id="settings-panel"
            aria-labelledby={`settings-tab-${category}`}
            className="flex-1 min-w-0 min-h-0 overflow-y-auto overscroll-contain p-6 compact:p-4"
          >
            {category === "controls" && (
              <div className="max-w-xl mx-auto space-y-3">
                <Slider
                  id="orbit-sensitivity"
                  label="Drag sensitivity"
                  value={camera.orbitSensitivity}
                  display={`${Math.round(camera.orbitSensitivity * 100)}%`}
                  min={0.1}
                  max={2}
                  step={0.05}
                  onChange={(orbitSensitivity) => camera.update({ orbitSensitivity })}
                />
                <Slider
                  id="zoom-sensitivity"
                  label="Zoom sensitivity"
                  value={camera.zoomSensitivity}
                  display={`${Math.round(camera.zoomSensitivity * 100)}%`}
                  min={0.1}
                  max={2}
                  step={0.05}
                  onChange={(zoomSensitivity) => camera.update({ zoomSensitivity })}
                />
                <Slider
                  id="camera-smoothing"
                  label="Camera smoothing"
                  value={camera.smoothing}
                  display={
                    camera.smoothing === 0 ? "Off" : `${Math.round(camera.smoothing * 1000)} ms`
                  }
                  min={0}
                  max={0.4}
                  step={0.02}
                  onChange={(smoothing) => camera.update({ smoothing })}
                />
                <label className="flex items-center gap-3 min-h-11 text-sm">
                  <input
                    type="checkbox"
                    className="game-checkbox"
                    checked={camera.reverseZoom}
                    onChange={(event) => camera.update({ reverseZoom: event.target.checked })}
                  />
                  Reverse scroll direction
                </label>
              </div>
            )}
            {category === "audio" && (
              <div className="max-w-xl mx-auto space-y-5">
                <div>
                  <label className="flex items-center gap-3 min-h-11 text-sm">
                    <input
                      type="checkbox"
                      className="game-checkbox"
                      checked={sound.enabled}
                      onChange={sound.toggleMute}
                    />
                    Sound effects
                  </label>
                  <Slider
                    id="effects-volume"
                    label="Sound effects volume"
                    value={sound.volume}
                    display={`${Math.round(sound.volume * 100)}%`}
                    min={0}
                    max={1}
                    step={0.05}
                    disabled={!sound.enabled}
                    onChange={sound.setVolume}
                  />
                </div>
                <div>
                  <label className="flex items-center gap-3 min-h-11 text-sm">
                    <input
                      type="checkbox"
                      className="game-checkbox"
                      checked={sound.musicEnabled}
                      onChange={sound.toggleMusicMute}
                    />
                    Music
                  </label>
                  <Slider
                    id="music-volume"
                    label="Music volume"
                    value={sound.musicVolume}
                    display={`${Math.round(sound.musicVolume * 100)}%`}
                    min={0}
                    max={1}
                    step={0.05}
                    disabled={!sound.musicEnabled}
                    onChange={sound.setMusicVolume}
                  />
                </div>
              </div>
            )}
            {category === "graphics" && (
              <GraphicsPanel
                value={draftGraphics}
                onChange={setDraftGraphics}
                reloads={residentGraphicsKey(draftGraphics) !== residentGraphicsKey(graphics)}
              />
            )}
            {category === "ai" && (
              <div className="max-w-xl mx-auto">
                <label className="flex items-center gap-3 min-h-11 text-sm">
                  <input
                    type="checkbox"
                    className="game-checkbox"
                    checked={showBotThoughts}
                    onChange={(event) => setShowBotThoughts(event.target.checked)}
                  />
                  Show bot thoughts
                </label>
              </div>
            )}
          </section>
        </div>
        <div className={`${OVERLAY_FOOTER_CLASS} !py-3 compact:!py-1`}>
          <GameButton emphasis="quiet" size="sm" className="!min-h-11" onClick={reset}>
            Reset {categories.find((tab) => tab.id === category)!.label}
          </GameButton>
          <GameButton
            size="sm"
            className="!min-h-11"
            onClick={() => {
              if (graphicsChanged) {
                applyGraphics(draftGraphics);
              }
              onClose();
            }}
          >
            {graphicsChanged ? "Apply graphics" : "Done"}
          </GameButton>
        </div>
      </div>
    </div>
  );
}

const qualityOptions = [
  { id: "low", name: "Low" },
  { id: "medium", name: "Medium" },
  { id: "high", name: "High" },
];

function GraphicsPanel({
  value,
  onChange,
  reloads,
}: {
  value: GraphicsSettings;
  onChange: (settings: GraphicsSettings) => void;
  reloads: boolean;
}) {
  const status = useEngineStore((s) => s.status);
  const canvas = document.querySelector<HTMLCanvasElement>(
    'canvas[aria-label="Open Mars game world"]',
  );
  const update = (patch: Partial<GraphicsSettings>) => onChange({ ...value, ...patch });
  return (
    <div className="max-w-xl mx-auto space-y-4">
      <div className="space-y-2 text-sm">
        <label htmlFor="graphics-preset" className="font-orbitron">
          Quality preset
        </label>
        <GameSelect
          id="graphics-preset"
          className="w-full"
          label="Graphics quality preset"
          value={graphicsPreset(value)}
          placeholder="Custom"
          options={qualityOptions}
          onChange={(next) => {
            const preset = next as keyof typeof GRAPHICS_PRESETS;
            onChange({
              ...GRAPHICS_PRESETS[preset],
              backgroundBrightness: value.backgroundBrightness,
            });
          }}
        />
      </div>
      <Slider
        id="render-resolution"
        label="Render resolution"
        min={0.5}
        max={1.5}
        step={0.05}
        value={value.renderScale}
        display={`${Math.round(value.renderScale * 100)}%`}
        onChange={(renderScale) => update({ renderScale })}
      />
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2 text-sm">
          <label htmlFor="graphics-texture" className="font-orbitron">
            Texture quality
          </label>
          <GameSelect
            id="graphics-texture"
            className="w-full"
            label="Texture quality"
            value={value.textureQuality}
            options={qualityOptions}
            onChange={(next) =>
              update({ textureQuality: next as GraphicsSettings["textureQuality"] })
            }
          />
        </div>
        <div className="space-y-2 text-sm">
          <label htmlFor="graphics-effects" className="font-orbitron">
            Effect detail
          </label>
          <GameSelect
            id="graphics-effects"
            className="w-full"
            label="Effect detail"
            value={value.effects}
            options={qualityOptions}
            onChange={(next) => update({ effects: next as GraphicsSettings["effects"] })}
          />
        </div>
        <div className="space-y-2 text-sm">
          <label htmlFor="graphics-antialiasing" className="font-orbitron">
            Anti-aliasing
          </label>
          <GameSelect
            id="graphics-antialiasing"
            className="w-full"
            label="Anti-aliasing"
            value={String(value.antialiasing)}
            options={[
              { id: "false", name: "Off" },
              { id: "true", name: "FXAA" },
            ]}
            onChange={(next) => update({ antialiasing: next === "true" })}
          />
        </div>
        <div className="space-y-2 text-sm">
          <label htmlFor="graphics-frame-limit" className="font-orbitron">
            FPS limit
          </label>
          <GameSelect
            id="graphics-frame-limit"
            className="w-full"
            label="FPS limit"
            value={String(value.frameLimit)}
            options={FRAME_LIMITS.map((limit) => ({
              id: String(limit),
              name: limit === 0 ? "Display refresh rate" : `${limit} FPS`,
            }))}
            onChange={(next) => update({ frameLimit: Number(next) })}
          />
        </div>
      </div>
      <Slider
        id="space-brightness"
        label="Space brightness"
        min={0.1}
        max={1}
        step={0.05}
        value={value.backgroundBrightness}
        display={`${Math.round(value.backgroundBrightness * 100)}%`}
        onChange={(backgroundBrightness) => update({ backgroundBrightness })}
      />
      {reloads && (
        <p className="text-sm text-white/75">
          Texture and effect changes reload the world when applied.
        </p>
      )}
      <div className="flex flex-wrap items-center justify-between gap-3 border-t border-white/15 pt-4 text-sm">
        <div className="flex gap-4">
          <span>
            {status.backend === "webgpu"
              ? "WebGPU"
              : status.backend === "webgl2"
                ? "WebGL2"
                : "Starting renderer"}
          </span>
          {canvas && (
            <span className="font-mono tabular-nums">
              {canvas.width} × {canvas.height}
            </span>
          )}
        </div>
        <GameButton
          size="sm"
          className="!min-h-11"
          emphasis="quiet"
          onClick={() => {
            useUIOverlayStore.getState().setShowSettings(false);
            useUIOverlayStore.getState().setShowPerformanceWindow(true);
          }}
        >
          Performance
        </GameButton>
      </div>
    </div>
  );
}

export default function ClientSettings() {
  const open = useUIOverlayStore((state) => state.showSettings);
  const close = useUIOverlayStore((state) => state.setShowSettings);
  const portalRoot = useOverlayPortalRoot();
  return open ? createPortal(<SettingsPanel onClose={() => close(false)} />, portalRoot) : null;
}
