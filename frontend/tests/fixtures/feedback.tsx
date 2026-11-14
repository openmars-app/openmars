import React from "react";
import { createRoot } from "react-dom/client";
import "../../src/index.css";
import DebugDropdown from "../../src/components/ui/debug/DebugDropdown";
import { WindowManagerProvider } from "../../src/components/ui/debug/WindowManager";
import EngineCanvas from "../../src/engine/EngineCanvas";
import { toSceneSnapshot } from "../../src/engine/scene";
import { audioService } from "../../src/services/audioService";
import { globalWebSocketManager } from "../../src/services/globalWebSocketManager";
import { assetUrl } from "../../src/assets";
import { Z_INDEX } from "../../src/constants/zIndex";
import { sceneGame } from "./scene";
import type { AdminCommandRequest } from "../../src/types/generated/api-types";

function gameData(id: string, prefix: string) {
  const game = sceneGame(id);
  game.settings.developmentMode = true;
  game.currentPlayer = {
    ...game.currentPlayer,
    id: `${prefix}-emil`,
    name: "Emil",
    terraformRating: 35,
    resources: { credits: 111, steel: 2, titanium: 3, plants: 4, energy: 5, heat: 6 },
    production: { credits: 7, steel: 8, titanium: 9, plants: 10, energy: 11, heat: 12 },
  };
  game.otherPlayers = [
    {
      ...game.currentPlayer,
      id: `${prefix}-sam`,
      name: "Sam",
      handCardCount: 0,
      terraformRating: 44,
      resources: { ...game.currentPlayer.resources, credits: 222 },
    },
  ];
  game.board.tiles.find((t) => t.coordinates.q === 0 && t.coordinates.r === 0)!.occupiedBy = {
    type: "city-tile",
    tags: [],
  };
  return game;
}

export function mountFeedback(canvas: boolean) {
  document.documentElement.style.setProperty("--surface-layer", String(Z_INDEX.CONTROL_DECORATION));
  const host = document.createElement("div");
  document.body.append(host);
  const root = createRoot(host);
  let game = gameData("first-game", "first");
  let visible = !canvas,
    connected = true;
  const commands: AdminCommandRequest[] = [];
  const original = globalWebSocketManager.sendAdminCommand;
  globalWebSocketManager.sendAdminCommand = async (command) => {
    commands.push(command);
    return "recorded";
  };
  audioService.setEnabled(true);
  audioService.setVolume(0.4);
  const status = { phase: "starting" };
  const render = () =>
    root.render(
      <React.StrictMode>
        <WindowManagerProvider>
          {canvas && (
            <EngineCanvas
              scene={toSceneSnapshot(game, null)}
              connected={connected}
              menuPlacement={null}
              entry={{ tiles: "settled", lighting: "steady" }}
              onStatus={(s) => {
                status.phase = s.phase;
              }}
              onEvent={() => {}}
            />
          )}
          <DebugDropdown
            isVisible={visible}
            onClose={() => {
              visible = false;
              render();
            }}
            gameState={game}
          />
        </WindowManagerProvider>
      </React.StrictMode>,
    );
  render();
  return {
    status,
    commands,
    replaceGame() {
      game = gameData("second-game", "second");
      render();
    },
    replaceRoster() {
      game = {
        ...game,
        currentPlayer: { ...game.currentPlayer, id: "third-emil" },
        otherPlayers: game.otherPlayers.map((p) => ({ ...p, id: "third-sam" })),
      };
      render();
    },
    removeSam() {
      game = { ...game, otherPlayers: [] };
      render();
    },
    updateCredits() {
      game = {
        ...game,
        currentPlayer: {
          ...game.currentPlayer,
          resources: { ...game.currentPlayer.resources, credits: 333 },
        },
      };
      render();
    },
    show() {
      visible = true;
      render();
    },
    hide() {
      visible = false;
      render();
    },
    reconnect(value: boolean) {
      connected = value;
      render();
    },
    rerender: render,
    place(q: number, r: number, type = "city-tile") {
      game = {
        ...game,
        board: {
          ...game.board,
          tiles: game.board.tiles.map((t) =>
            t.coordinates.q === q && t.coordinates.r === r
              ? { ...t, occupiedBy: { type, tags: [] } }
              : t,
          ),
        },
      };
      render();
    },
    enabled(value: boolean) {
      audioService.setEnabled(value);
    },
    soundUrls() {
      return (["construction-1", "construction-2", "water-placement"] as const).map(
        (id) => new URL(assetUrl(`audio/effects/${id}`), location.href).href,
      );
    },
    dispose() {
      root.unmount();
      host.remove();
      globalWebSocketManager.sendAdminCommand = original;
    },
  };
}
