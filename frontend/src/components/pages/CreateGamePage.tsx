import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { apiService } from "@/services/apiService";
import type { GameDto, GameOptionsDto, GameSetupDto } from "@/types/generated/api-types";
import { useJoinGame } from "@/hooks/useJoinGame";
import { useLayoutMode } from "@/hooks/useLayoutMode";
import { MAX_PLAYER_NAME_LENGTH } from "@/constants/gameConstants";
import { Z_INDEX } from "@/constants/zIndex";
import BackButton from "../ui/buttons/BackButton";
import GameButton from "../ui/buttons/GameButton";
import GameSetupControls from "../ui/lobby/GameSetupControls";

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
  const { playerName, setPlayerName, isLoading, handleJoin } = useJoinGame({ game: created });
  const { isCompact } = useLayoutMode();
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
  // Phones keep it beside the name field; desktop puts it at the bottom of the left column
  const submitButton = (
    <GameButton
      type="submit"
      size={isCompact ? "md" : "lg"}
      loading={busy}
      disabled={playerName.trim().length < 2}
      className={isCompact ? "shrink-0" : "mt-auto self-end"}
    >
      {created && !busy ? "Join lobby" : "Create lobby"}
    </GameButton>
  );
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
        <h1 className="menu-title compact:mb-4">Create game</h1>
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
              <p className="text-white/60">Loading options…</p>
            )}
          </div>
        ) : (
          <form
            onSubmit={(event) => void submit(event)}
            className={isCompact ? "game-panel p-4" : "game-panel p-6 sm:p-10 menu-grid"}
          >
            <div className="flex flex-col gap-8 compact:gap-4 min-w-0">
              <div className="flex items-stretch gap-3">
                <input
                  className="game-input text-lg flex-1"
                  placeholder="Your name"
                  aria-label="Your name"
                  value={playerName}
                  onChange={(e) => setPlayerName(e.target.value)}
                  disabled={busy || !!created}
                  autoFocus
                  required
                  minLength={2}
                  maxLength={MAX_PLAYER_NAME_LENGTH}
                  autoComplete="nickname"
                />
                {isCompact && submitButton}
              </div>
              {!isCompact && (
                <details className="text-sm">
                  <summary className="cursor-pointer font-orbitron text-white/70">
                    Game options
                  </summary>
                  <fieldset disabled={busy || !!created} className="flex flex-col gap-5 mt-6">
                    <label className="flex items-center justify-between gap-4">
                      Players
                      <input
                        className="game-input w-20"
                        type="number"
                        min={1}
                        max={
                          options.gameModes.find((mode) => mode.id === setup.gameModeId)
                            ?.maxPlayers ?? 5
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
                      <label key={key} className="flex items-center justify-between gap-4">
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
                </details>
              )}
              {!isCompact && submitButton}
            </div>
            <div className="compact:mt-5">
              <GameSetupControls
                maps={options.availableMaps}
                mapId={setup.mapId}
                modes={options.gameModes}
                gameModeId={setup.gameModeId}
                gameModeParameters={setup.gameModeParameters}
                features={setup.features}
                maxPlayers={setup.maxPlayers}
                disabled={busy || !!created}
                onChange={(patch) => setSetup({ ...setup, ...patch })}
              />
            </div>
          </form>
        )}
      </main>
    </>
  );
}
