import { useEffect, useId, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { apiService } from "@/services/apiService";
import type { GameDto, GameOptionsDto, GameSetupDto } from "@/types/generated/api-types";
import { useJoinGame } from "@/hooks/useJoinGame";
import { useLayoutMode } from "@/hooks/useLayoutMode";
import { useVisualViewport } from "@/hooks/useVisualViewport";
import { MAX_PLAYER_NAME_LENGTH } from "@/constants/gameConstants";
import { Z_INDEX } from "@/constants/zIndex";
import BackButton from "../ui/buttons/BackButton";
import GameButton from "../ui/buttons/GameButton";
import GameSetupControls from "../ui/lobby/GameSetupControls";
import MapPreview from "../ui/lobby/MapPreview";
import GameSelect from "../ui/GameSelect";
import CollapsibleSection from "../ui/CollapsibleSection";
import { MainMenuDrawerButton } from "../ui/buttons/MainMenuHamburger";

export default function CreateGamePage() {
  const navigate = useNavigate();
  const [options, setOptions] = useState<GameOptionsDto | null>(null);
  const [setup, setSetup] = useState<GameSetupDto | null>(null);
  const [created, setCreated] = useState<GameDto | null>(null);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");
  const [attempt, setAttempt] = useState(0);
  const submitting = useRef(false);
  const mounted = useRef(true);
  const { playerName, setPlayerName, isLoading, handleJoin } = useJoinGame({
    game: created,
  });
  const { isCompact } = useLayoutMode();
  const viewport = useVisualViewport(isCompact);
  const formId = useId();
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  useEffect(() => {
    const controller = new AbortController();
    setError("");
    void apiService
      .getGameOptions(controller.signal)
      .then((data) => {
        if (!controller.signal.aborted) {
          setOptions(data);
          setSetup(data.defaults);
        }
      })
      .catch((err: Error) => {
        if (!controller.signal.aborted) {
          setError(err.message);
        }
      });
    return () => controller.abort();
  }, [attempt]);
  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!setup || submitting.current || playerName.trim().length < 2) {
      return;
    }
    submitting.current = true;
    setCreating(true);
    setError("");
    try {
      const game = created ?? (await apiService.createGame(setup));
      if (!mounted.current) {
        return;
      }
      setCreated(game);
      await handleJoin(game);
    } catch (err) {
      if (mounted.current) {
        setError(err instanceof Error ? err.message : "Could not create game");
      }
    } finally {
      submitting.current = false;
      if (mounted.current) {
        setCreating(false);
      }
    }
  };
  const busy = creating || isLoading;
  const submitButton = (
    <GameButton
      type="submit"
      form={formId}
      size={isCompact ? "sm" : "lg"}
      loading={busy}
      disabled={!setup || playerName.trim().length < 2}
      className={isCompact ? "shrink-0 min-h-11 whitespace-nowrap" : "self-end"}
    >
      {created && !busy ? "Join lobby" : "Create lobby"}
    </GameButton>
  );
  const content = (
    <>
      {error && (
        <p role="alert" className="text-red-300 mb-5">
          {error}
        </p>
      )}
      {!options || !setup ? (
        <div>
          {error ? (
            <GameButton emphasis="secondary" onClick={() => setAttempt((value) => value + 1)}>
              Retry
            </GameButton>
          ) : (
            <p role="status" className="text-white/60">
              Loading options…
            </p>
          )}
        </div>
      ) : (
        <form
          id={formId}
          onSubmit={(event) => void submit(event)}
          className={isCompact ? "" : "game-panel p-6 sm:p-10 menu-grid"}
        >
          <div className="flex flex-col gap-8 compact:gap-5 min-w-0">
            <input
              className="game-input text-lg w-full"
              placeholder="Your name"
              aria-label="Your name"
              value={playerName}
              onChange={(e) => setPlayerName(e.target.value)}
              disabled={busy || !!created}
              autoFocus={!isCompact}
              required
              minLength={2}
              maxLength={MAX_PLAYER_NAME_LENGTH}
              autoComplete="nickname"
            />
            <GameSetupControls
              maps={options.availableMaps}
              mapId={setup.mapId}
              modes={options.gameModes}
              gameModeId={setup.gameModeId}
              gameModeParameters={setup.gameModeParameters}
              features={setup.features}
              maxPlayers={setup.maxPlayers}
              showMapPreview={false}
              showMapSelect={isCompact}
              collapsibleFeatures
              disabled={busy || !!created}
              onChange={(patch) => setSetup({ ...setup, ...patch })}
            />
            <CollapsibleSection label="Game options">
              <fieldset disabled={busy || !!created} className="flex flex-col gap-5 mt-4">
                <label className="flex items-center justify-between gap-4 min-h-11">
                  Players
                  <input
                    className="game-input w-20"
                    type="number"
                    min={1}
                    max={
                      options.gameModes.find((mode) => mode.id === setup.gameModeId)?.maxPlayers ??
                      5
                    }
                    value={setup.maxPlayers}
                    onChange={(e) => setSetup({ ...setup, maxPlayers: Number(e.target.value) })}
                  />
                </label>
                {(
                  [
                    ["developmentMode", "Development mode"],
                    ["demoGame", "Demo game"],
                    ["allowRandomBuy", "Random buy"],
                  ] as const
                ).map(([key, label]) => (
                  <label key={key} className="flex items-center justify-between gap-4 min-h-11">
                    {label}
                    <input
                      type="checkbox"
                      checked={setup[key]}
                      onChange={(e) => setSetup({ ...setup, [key]: e.target.checked })}
                      className="game-checkbox"
                    />
                  </label>
                ))}
              </fieldset>
            </CollapsibleSection>
            {!isCompact && submitButton}
          </div>
          {!isCompact && (
            <div className="flex flex-col gap-8 min-w-0">
              <div className="flex items-center justify-between gap-4 min-h-14">
                <label htmlFor={`${formId}-map`} className="text-sm text-white/60">
                  Map
                </label>
                <GameSelect
                  id={`${formId}-map`}
                  label="Map"
                  value={setup.mapId}
                  options={options.availableMaps}
                  disabled={busy || !!created}
                  onChange={(mapId) => setSetup({ ...setup, mapId })}
                  className="flex-1"
                />
              </div>
              <div className="flex-1 flex items-center justify-center [&_svg]:w-full [&_svg]:max-w-[420px] [&_svg]:h-auto">
                <MapPreview
                  tiles={options.availableMaps.find((map) => map.id === setup.mapId)?.tiles ?? []}
                />
              </div>
            </div>
          )}
        </form>
      )}
    </>
  );
  if (isCompact) {
    return (
      <main
        data-testid="mobile-create-game"
        className="fixed inset-x-0 flex flex-col text-white text-left bg-[rgba(3,3,4,0.98)]"
        style={{
          zIndex: Z_INDEX.MENU_DROPDOWN,
          top: viewport.offsetTop,
          height: viewport.height,
          paddingTop: "var(--safe-top)",
          paddingRight: "var(--safe-right)",
          paddingBottom: "var(--safe-bottom)",
          paddingLeft: "var(--safe-left)",
        }}
      >
        <header className="shrink-0 h-11 flex items-center gap-1 pr-2 border-b border-white/15">
          <MainMenuDrawerButton />
          <BackButton onClick={() => navigate("/")} />
          <h1 className="flex-1 min-w-0 px-2 font-orbitron text-sm">Create game</h1>
          {submitButton}
        </header>
        <div
          data-testid="create-game-content"
          className="flex-1 min-h-0 overflow-y-auto overscroll-contain p-4"
        >
          {content}
        </div>
      </main>
    );
  }
  return (
    <>
      <div
        className="menu-chrome-top-left menu-chrome-back"
        style={{ zIndex: Z_INDEX.TOP_MENU_BAR }}
      >
        <BackButton onClick={() => navigate("/")} />
      </div>
      <main
        className="menu-shell menu-enter text-white text-left relative"
        style={{ zIndex: Z_INDEX.UI_BASE }}
      >
        <h1 className="menu-title">Create game</h1>
        {content}
      </main>
    </>
  );
}
