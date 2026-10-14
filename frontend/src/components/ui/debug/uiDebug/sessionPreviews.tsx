import { EnterCodePopoverView } from "@/components/ui/popover/EnterCodePopover";
import { JoinGamePopoverView } from "@/components/ui/popover/JoinGamePopover";
import { SpectatePopoverView } from "@/components/ui/popover/SpectatePopover";
import { ServerSwitchPopoverView } from "@/components/ui/popover/ServerSwitchPopover";
import { WaitingRoomOverlayView } from "@/components/ui/overlay/WaitingRoomOverlay";
import { ServerSelectorScreenView } from "@/components/ui/servers/ServerSelectorScreen";
import { ServerListView } from "@/components/ui/servers/ServerList";
import type { ServerProbe } from "@/utils/serverProbe";
import GameButton from "@/components/ui/buttons/GameButton";
import GameModeRules from "@/components/ui/lobby/GameModeRules";
import { useMemo, useRef, useState } from "react";
import type {
  GameDto,
  GameHistoryEntryDto,
  InitPhaseDto,
  UpdateGameSettingsRequest,
} from "@/types/generated/api-types";
import { createPreviewGame, previewProduction, previewResources } from "./fixtures";
import type { PreviewDefinition, PreviewProps } from "./types";
import GameMenuModal from "@/components/ui/overlay/GameMenuModal";
import GameSetupControls from "@/components/ui/lobby/GameSetupControls";
import { LobbySettingsOverlayView } from "@/components/ui/overlay/LobbySettingsOverlay";
import { DemoSetupOverlayView } from "@/components/ui/overlay/DemoSetupOverlay";
import StartingCardSelectionOverlay from "@/components/ui/overlay/StartingCardSelectionOverlay";
import ProductionCardSelectionOverlay from "@/components/ui/overlay/ProductionCardSelectionOverlay";
import { ProductionPhaseModalView } from "@/components/ui/modals/ProductionPhaseModal";
import CorporationOverlay from "@/components/ui/overlay/CorporationOverlay";
import { CorporationShowcaseOverlayView } from "@/components/ui/overlay/CorporationShowcaseOverlay";
import { JoinGameOverlayView } from "@/components/ui/overlay/JoinGameOverlay";
import { SpectateGameOverlayView } from "@/components/ui/overlay/SpectateGameOverlay";
import { PlayerSelectionOverlayView } from "@/components/ui/overlay/PlayerSelectionOverlay";
import { GameConfirmDialogView } from "@/components/layout/main/GameConfirmDialogs";
import PassConfirmDialog from "@/components/ui/modals/PassConfirmDialog";
import { ChangelogModalView, type ChangelogState } from "@/components/ui/overlay/ChangelogModal";
import { ServerDownDialogView } from "@/components/ui/overlay/ServerDownDialog";
import TabConflictOverlay from "@/components/ui/overlay/TabConflictOverlay";
import { InstallHowToSheetView } from "@/components/ui/InstallHowToSheet";
import EndGameBottomBar from "@/components/ui/endgame/EndGameBottomBar";
import { VPCountingProvider, type VPCountingState } from "@/contexts/VPCountingContext";
import { useLayoutMode } from "@/hooks/useLayoutMode";

function updateSettings(game: GameDto, patch: UpdateGameSettingsRequest): GameDto {
  const { claudeOAuthToken, ...values } = patch;
  return {
    ...game,
    settings: {
      ...game.settings,
      ...values,
      hasClaudeOAuthToken:
        claudeOAuthToken === undefined
          ? game.settings.hasClaudeOAuthToken
          : !!claudeOAuthToken.trim(),
    },
  };
}

function SetupPreview({ cards, presetId, onComplete }: PreviewProps) {
  const { isCompact } = useLayoutMode();
  const [game, setGame] = useState(() => createPreviewGame(cards));
  const change = (patch: UpdateGameSettingsRequest) =>
    setGame((current) => updateSettings(current, patch));
  return (
    <GameMenuModal
      title="Game setup"
      layout="wide"
      onClose={() => onComplete()}
      showCloseButton
      showBackdrop
    >
      <GameSetupControls
        maps={game.settings.availableMaps}
        modes={game.settings.availableGameModes}
        mapId={game.settings.mapId}
        gameModeId={game.settings.gameModeId}
        gameModeParameters={game.settings.gameModeParameters}
        features={game.settings.features}
        maxPlayers={game.settings.maxPlayers}
        onChange={change}
        disabled={presetId === "read-only"}
        layout={isCompact ? "stacked" : "split"}
      />
    </GameMenuModal>
  );
}

function SettingsPreview({ cards, presetId, onComplete }: PreviewProps) {
  const [game, setGame] = useState(() => {
    const initial = createPreviewGame(cards);
    initial.settings.hasClaudeOAuthToken = presetId === "token-configured";
    return initial;
  });
  const [thoughts, setThoughts] = useState(false);
  return (
    <LobbySettingsOverlayView
      game={game}
      playerId={game.currentPlayer.id}
      isOpen
      onClose={() => onComplete("Settings closed")}
      showBotThoughts={thoughts}
      onBotThoughtsChange={setThoughts}
      onUpdateSettings={(patch) => setGame((current) => updateSettings(current, patch))}
    />
  );
}

function DemoPreview({ cards, presetId, onComplete }: PreviewProps) {
  const game = useMemo(() => createPreviewGame(cards), [cards]);
  return (
    <DemoSetupOverlayView
      game={game}
      playerId={presetId === "guest" ? "preview-other-1" : game.currentPlayer.id}
      isOpen
      onClose={() => onComplete("Demo setup closed")}
      catalog={cards}
      allMilestones={[]}
      allAwards={[]}
      onSubmit={async (choices) => {
        onComplete(`Selected ${choices.cardIds.length} project cards`);
      }}
    />
  );
}

function StartingPreview({ cards, presetId, onComplete }: PreviewProps) {
  return (
    <StartingCardSelectionOverlay
      isOpen
      availableCorporations={cards
        .filter((card) => card.type === "corporation")
        .slice(0, presetId === "many-cards" ? 4 : 2)}
      availablePreludes={
        presetId === "classic" ? [] : cards.filter((card) => card.type === "prelude").slice(0, 4)
      }
      maxSelectablePreludes={presetId === "classic" ? 0 : 2}
      cards={cards
        .filter((card) => card.type !== "corporation" && card.type !== "prelude")
        .slice(0, presetId === "many-cards" ? 16 : 10)}
      playerCredits={42}
      onConfirm={(_, preludes, selected) =>
        onComplete(`Selected ${preludes.length} preludes and ${selected.length} cards`)
      }
      onHide={() => onComplete("Selection hidden")}
    />
  );
}

function ResearchPreview({ cards, presetId, onComplete }: PreviewProps) {
  return (
    <ProductionCardSelectionOverlay
      isOpen
      cards={cards
        .filter((card) => card.type !== "corporation" && card.type !== "prelude")
        .slice(0, presetId === "long" ? 12 : 4)}
      playerCredits={presetId === "unaffordable" ? 1 : 18}
      costPerCard={3}
      allowRandomBuy={presetId === "random-buy"}
      onSelectCards={(ids, options) =>
        onComplete(options?.randomBuy ? "Random card purchased" : `${ids.length} cards purchased`)
      }
      onReturn={() => onComplete("Returned to production")}
    />
  );
}

function ProductionPreview({ cards, presetId, onComplete }: PreviewProps) {
  const game = useMemo(() => {
    const result = createPreviewGame(cards);
    result.currentPhase = "production-and-card-draw";
    result.isLastRound = presetId === "last-round";
    const phase = {
      productionUsed: { ...previewProduction },
      selectionComplete: false,
      beforeResources: { ...previewResources },
      afterResources: { ...previewResources, credits: 73, energy: 2, heat: 19 },
      resourceDelta: { credits: 31, steel: 2, titanium: 1, plants: 3, energy: -1, heat: 7 },
      energyConverted: 3,
      creditsIncome: 31,
    };
    result.currentPlayer.productionPhase = {
      ...phase,
      availableCards: result.currentPlayer.cards.slice(0, 4),
    };
    result.otherPlayers.forEach((player, index) => {
      player.productionPhase = { ...phase, selectionComplete: index === 0 };
    });
    return result;
  }, [cards, presetId]);
  return (
    <ProductionPhaseModalView
      isOpen
      gameState={game}
      soundsEnabled={false}
      onClose={() => onComplete("Production finished")}
      onHide={() => onComplete("Production hidden")}
      onSelectCards={async (ids) => onComplete(`${ids.length} cards selected`)}
    />
  );
}

function CorporationPreview({ cards, presetId, onComplete }: PreviewProps) {
  const game = createPreviewGame(cards);
  if (presetId === "solo") {
    game.otherPlayers = [];
  }
  return (
    <CorporationOverlay
      visible
      onClose={() => onComplete()}
      currentPlayer={game.currentPlayer}
      otherPlayers={game.otherPlayers}
    />
  );
}

function ShowcasePreview({ cards, presetId, onComplete }: PreviewProps) {
  const [step, setStep] = useState(0);
  const game = createPreviewGame(cards);
  game.currentPhase = presetId === "preludes" ? "init_apply_prelude" : "init_apply_corp";
  const phase: InitPhaseDto = {
    currentPlayerId: game.currentPlayer.id,
    currentPlayerIndex: 0,
    totalPlayers: game.playerOrder.length,
    waitingForConfirm: true,
    confirmVersion: step,
    hasPreludePhase: true,
    stage: presetId === "roster" ? "roster" : "applied",
    preludes: cards.filter((card) => card.type === "prelude").slice(0, 2),
    preludesPlayed: step,
    hasPendingSelection: false,
    pendingSourceCardId: "",
  };
  game.initPhase = phase;
  return (
    <CorporationShowcaseOverlayView
      game={game}
      initPhase={phase}
      onAdvance={() => {
        if (step >= 1) {
          onComplete("Showcase complete");
        } else {
          setStep(step + 1);
        }
      }}
    />
  );
}

function JoinPreview({ presetId, onComplete }: PreviewProps) {
  const [name, setName] = useState(presetId === "empty" ? "" : "Avery");
  const join = async () => {
    if (name.trim()) {
      onComplete(`Joined as ${name.trim()}`);
    }
  };
  return (
    <JoinGameOverlayView
      playerName={name}
      setPlayerName={setName}
      isLoading={presetId === "loading"}
      errorMessage={
        presetId === "error" ? "This game is full. Try another game or join as a spectator." : ""
      }
      loadingMessage="Joining game…"
      handleJoin={join}
      handleKeyDown={(event) => {
        if (event.key === "Enter") {
          void join();
        }
      }}
      onCancel={() => onComplete("Join cancelled")}
    />
  );
}

function SpectatePreview({ presetId, onComplete }: PreviewProps) {
  const [name, setName] = useState(presetId === "empty" ? "" : "Avery");
  return (
    <SpectateGameOverlayView
      spectatorName={name}
      onNameChange={setName}
      isConnecting={presetId === "loading"}
      onSpectate={() => onComplete(`Spectating as ${name}`)}
      onCancel={() => onComplete("Cancelled")}
    />
  );
}

function ReconnectPreview({ cards, presetId, onComplete }: PreviewProps) {
  const [name, setName] = useState("");
  const game = createPreviewGame(cards);
  game.currentPlayer.isConnected = false;
  if (presetId === "long-names") {
    game.otherPlayers.forEach((player) => {
      player.name += " of the Interplanetary Terraforming Consortium";
    });
  }
  if (presetId === "all-connected") {
    game.currentPlayer.isConnected = true;
    game.otherPlayers.forEach((player) => {
      player.isConnected = true;
    });
  }
  return (
    <PlayerSelectionOverlayView
      game={game}
      spectatorName={name}
      onNameChange={setName}
      isConnecting={false}
      onSelectPlayer={(_, playerName) => onComplete(`Reconnected as ${playerName}`)}
      onSpectate={() => onComplete(`Spectating as ${name}`)}
      onCancel={() => onComplete()}
    />
  );
}

function ConfirmationPreview({ presetId, onComplete }: PreviewProps) {
  return (
    <GameConfirmDialogView
      pending={{
        kind: presetId === "kick" ? "kick" : "convertToBot",
        playerId: "preview-other-1",
        playerName: "Morgan",
      }}
      dismiss={() => onComplete("Cancelled")}
      onConfirm={() =>
        onComplete(presetId === "kick" ? "Player removed" : "Player converted to bot")
      }
    />
  );
}

function PassPreview({ presetId, onComplete }: PreviewProps) {
  return (
    <PassConfirmDialog
      confirmation={{
        open: true,
        reason:
          presetId === "long"
            ? "You still have playable cards, unused card actions, and enough resources for a standard project. Passing will end your actions for this generation."
            : "You still have available actions this generation.",
        confirm: () => onComplete("Passed"),
        dismiss: () => onComplete("Continued turn"),
      }}
    />
  );
}

function ChangelogPreview({ presetId, onComplete }: PreviewProps) {
  let state: ChangelogState;
  if (presetId === "loading") {
    state = { status: "loading" };
  } else if (presetId === "error") {
    state = { status: "error" };
  } else {
    state = {
      status: "ready",
      entries:
        presetId === "empty"
          ? []
          : Array.from({ length: presetId === "long" ? 8 : 1 }, (_, index) => ({
              version: `0.${24 - index}.0`,
              intro: "New ways to explore Mars and smoother game interactions.",
              sections: [
                {
                  title: "Game improvements",
                  major: true,
                  intro: "Updated card and colony interactions.",
                  items: [
                    "Card receipts show every newly acquired card.",
                    "Colony controls stay reachable on smaller screens.",
                  ],
                },
                {
                  title: "Fixes",
                  major: false,
                  intro: "",
                  items: [
                    "Clearer action descriptions.",
                    "Reliable reconnection after a connection interruption.",
                  ],
                },
              ],
            })),
    };
  }
  return (
    <ChangelogModalView
      state={state}
      onClose={() => onComplete()}
      onUpdate={() => onComplete("Update requested")}
    />
  );
}

function ServerPreview({ presetId, onComplete }: PreviewProps) {
  return (
    <ServerDownDialogView
      serverName={
        presetId === "long-name" ? "The Interplanetary Terraforming Community Server" : "Olympus"
      }
      onRetry={() => onComplete("Retry requested")}
      onSwitchServer={() => onComplete("Server switch requested")}
    />
  );
}

function TabPreview({ presetId, onComplete }: PreviewProps) {
  return (
    <TabConflictOverlay
      activeGameInfo={{
        gameId:
          presetId === "long" ? "preview-game-with-a-long-session-identifier" : "preview-game",
        playerName: "Avery",
      }}
      onTakeOver={() => onComplete("Takeover requested")}
      onCancel={() => onComplete("Cancelled")}
    />
  );
}

function InstallPreview({ presetId, onComplete }: PreviewProps) {
  return (
    <InstallHowToSheetView
      platform={presetId === "android" ? "firefox-android" : "ios"}
      onClose={() => onComplete()}
    />
  );
}

function EndgamePreview({ cards, presetId }: PreviewProps) {
  const [panel, setPanel] = useState<"score" | "graphs" | "replay">(
    presetId === "graphs" || presetId === "replay" ? presetId : "score",
  );
  const [index, setIndex] = useState(3);
  const [playing, setPlaying] = useState(false);
  const [speed, setSpeed] = useState(0.5);
  const game = useMemo(() => {
    const result = createPreviewGame(cards);
    result.status = "completed";
    result.finalScores = [result.currentPlayer, ...result.otherPlayers].map((player, position) => ({
      playerId: player.id,
      playerName: player.name,
      placement: position + 1,
      isWinner: position === 0,
      vpBreakdown: {
        terraformRating: 30,
        cardVP: 20 - position * 2,
        cardVPDetails: [],
        milestoneVP: 5,
        awardVP: 5,
        greeneryVP: 8,
        greeneryVPDetails: [],
        cityVP: 10,
        cityVPDetails: [],
        totalVP: 78 - position * 2,
      },
    }));
    return result;
  }, [cards]);
  const history: GameHistoryEntryDto[] = useMemo(
    () =>
      Array.from({ length: 12 }, (_, sequence) => ({
        sequence,
        timestamp: new Date(Date.UTC(2026, 0, 1, 12, sequence)).toISOString(),
        generation: Math.floor(sequence / 2) + 1,
        phase: "action",
        temperature: -30 + sequence * 2,
        oxygen: sequence,
        oceans: Math.floor(sequence / 2),
        venus: sequence * 2,
        actionNumber: sequence,
        board: { tiles: [] },
        players: Object.fromEntries(
          [game.currentPlayer, ...game.otherPlayers].map((player, playerIndex) => [
            player.id,
            {
              id: player.id,
              name: player.name,
              color: player.color,
              terraformRating: 20 + sequence,
              ...previewResources,
              credits: 20 + sequence * 3 + playerIndex,
              playedCardCount: sequence,
              production: previewProduction,
              permanentProduction: previewProduction,
              dependencyStatuses: {},
              playedCardIds: [],
              handCardIds: [],
              corporationId: player.corporation?.id ?? "",
              resourceStorage: {},
              totalVP: 20 + sequence * 4 - playerIndex,
            },
          ]),
        ),
        milestones: [],
        awards: [],
        settings: game.settings,
      })),
    [game],
  );
  const [state, setState] = useState<VPCountingState>({
    isActive: presetId === "counting",
    currentPhaseIndex: 0,
    currentPhaseId: "tr",
    activePlayerId: game.currentPlayer.id,
    playerAccumulatedVP: {},
    highlightedTiles: new Set(),
    secondaryHighlightedTiles: new Set(),
    isFinishing: false,
    isComplete: presetId !== "counting",
  });
  const phases = [
    { id: "tr", label: "Terraforming", color: "#e76f51" },
    { id: "cards", label: "Cards", color: "#4fa3df" },
    { id: "awards", label: "Awards", color: "#a6ce39" },
  ];
  return (
    <VPCountingProvider
      state={state}
      controls={{
        phases,
        start: () => setState({ ...state, isActive: true, isComplete: false }),
        skip: () => setState({ ...state, isActive: false, isComplete: true }),
        goToPhase: (phaseIndex) =>
          setState({
            ...state,
            currentPhaseIndex: phaseIndex,
            currentPhaseId: phases[phaseIndex].id,
          }),
      }}
    >
      <EndGameBottomBar
        game={game}
        playerId={game.currentPlayer.id}
        historyEntries={presetId === "no-history" ? [] : history}
        activePanel={panel}
        onPanelChange={setPanel}
        isReplayActive={panel === "replay"}
        replayIndex={index}
        replayTotal={history.length}
        replayPlaying={playing}
        replaySpeed={speed}
        onReplayPlay={() => setPlaying(true)}
        onReplayPause={() => setPlaying(false)}
        onReplaySeek={setIndex}
        onReplayStepForward={() => setIndex(Math.min(history.length - 1, index + 1))}
        onReplayStepBackward={() => setIndex(Math.max(0, index - 1))}
        onReplaySpeedChange={setSpeed}
      />
    </VPCountingProvider>
  );
}

function WaitingPreview({ cards, presetId, onComplete }: PreviewProps) {
  const [game, setGame] = useState(() => {
    const initial = createPreviewGame(cards);
    initial.status = "lobby";
    initial.settings.hasClaudeOAuthToken = true;
    initial.spectators = [{ id: "preview-spectator", name: "Taylor", color: "#cccccc" }];
    if (presetId === "solo") {
      initial.otherPlayers = [];
      initial.playerOrder = [initial.currentPlayer.id];
      initial.turnOrder = [...initial.playerOrder];
    }
    if (presetId === "bot-loading" || presetId === "bot-failed") {
      initial.otherPlayers[0] = {
        ...initial.otherPlayers[0],
        playerType: "bot",
        botStatus: presetId === "bot-failed" ? "failed" : "loading",
        botPersona: "balanced",
      };
    }
    if (presetId === "demo") {
      initial.settings.demoGame = true;
    }
    return initial;
  });
  const [thoughts, setThoughts] = useState(false);
  const playerId = presetId === "guest" ? "preview-other-1" : game.currentPlayer.id;
  const update = (patch: UpdateGameSettingsRequest) =>
    setGame((current) => updateSettings(current, patch));
  const mapPanel = (layout?: "stacked" | "split") => (
    <GameSetupControls
      maps={game.settings.availableMaps}
      modes={game.settings.availableGameModes}
      mapId={game.settings.mapId}
      gameModeId={game.settings.gameModeId}
      gameModeParameters={game.settings.gameModeParameters}
      features={game.settings.features}
      maxPlayers={game.settings.maxPlayers}
      onChange={playerId === game.hostPlayerId ? update : undefined}
      layout={layout}
    />
  );
  return (
    <div className="fixed inset-0 bg-black">
      <WaitingRoomOverlayView
        game={game}
        playerId={playerId}
        chat={
          <div className="border border-white/15 p-3 text-sm text-white/70">
            Taylor: Ready to terraform Mars?
          </div>
        }
        onStart={() => onComplete("Game started")}
        onLeave={() => onComplete("Left lobby")}
        onCopyInvite={async () => {}}
        desktopMenu={
          <div className="fixed top-4 left-4">
            <GameButton onClick={() => onComplete("Menu opened")}>Menu</GameButton>
          </div>
        }
        compactMenu={
          <GameButton size="sm" onClick={() => onComplete("Menu opened")}>
            Menu
          </GameButton>
        }
        mapPanel={mapPanel}
        onColor={(color, id) =>
          setGame((current) => ({
            ...current,
            currentPlayer: !id ? { ...current.currentPlayer, color } : current.currentPlayer,
            otherPlayers: current.otherPlayers.map((player) =>
              player.id === id ? { ...player, color } : player,
            ),
          }))
        }
        onKick={(id) =>
          setGame((current) => ({
            ...current,
            otherPlayers: current.otherPlayers.filter((player) => player.id !== id),
          }))
        }
        onKickSpectator={(id) =>
          setGame((current) => ({
            ...current,
            spectators: current.spectators.filter((player) => player.id !== id),
          }))
        }
        onAddBot={() =>
          setGame((current) => {
            const id = `preview-bot-${current.otherPlayers.length}`;
            return {
              ...current,
              otherPlayers: [
                ...current.otherPlayers,
                {
                  ...createPreviewGame(cards).otherPlayers[0],
                  id,
                  name: "New bot",
                  playerType: "bot",
                  botStatus: "ready",
                },
              ],
              playerOrder: [...current.playerOrder, id],
              turnOrder: [...current.turnOrder, id],
            };
          })
        }
        renderSettings={(close) => (
          <LobbySettingsOverlayView
            game={game}
            playerId={playerId}
            isOpen
            onClose={close}
            showBotThoughts={thoughts}
            onBotThoughtsChange={setThoughts}
            onUpdateSettings={update}
          />
        )}
        renderDemo={(close) => (
          <DemoSetupOverlayView
            game={game}
            playerId={playerId}
            isOpen
            onClose={close}
            catalog={cards}
            allMilestones={[]}
            allAwards={[]}
            onSubmit={async () => {
              setGame((current) => ({
                ...current,
                currentPlayer: { ...current.currentPlayer, demoReady: true },
              }));
            }}
          />
        )}
      />
    </div>
  );
}

function ServerSelectorPreview({ presetId, onComplete }: PreviewProps) {
  const servers = [
    { alias: "olympus", url: "https://olympus.invalid" },
    { alias: "tharsis", url: "https://tharsis.invalid" },
    { alias: "hellas", url: "https://hellas.invalid" },
  ];
  const probes: Record<string, ServerProbe> =
    presetId === "loading"
      ? {}
      : {
          olympus: {
            status: "online",
            latencyMs: 24,
            meta: { name: "Olympus", alias: "olympus", version: "0.24.0" },
          },
          tharsis: { status: "down" },
          hellas: {
            status: "online",
            latencyMs: 130,
            meta: { name: "Hellas Community", alias: "hellas", version: "0.24.0" },
          },
        };
  return (
    <ServerSelectorScreenView onClose={() => onComplete()}>
      <ServerListView
        servers={presetId === "empty" ? [] : servers}
        probes={probes}
        currentAlias="olympus"
        onSelect={(alias) => onComplete(`Selected ${alias}`)}
        rowClassName="min-h-12 !px-1 border-b border-white/10"
      />
    </ServerSelectorScreenView>
  );
}

function RulesPreview({ cards, presetId, onComplete }: PreviewProps) {
  const mode = createPreviewGame(cards).settings.availableGameModes[0];
  if (presetId === "long") {
    mode.rules = Array.from({ length: 10 }, (_, index) => ({
      title: `Terraforming rule ${index + 1}`,
      text: "Spend resources, play projects, and build cities and greenery tiles to contribute to the terraforming of Mars. Each action changes the shared planet and develops your corporation.",
      advanced: index > 3,
    }));
  }
  return <GameModeRules mode={mode} parameters={{}} open onClose={() => onComplete()} />;
}

function SessionPopoverPreview({
  presetId,
  onComplete,
  kind,
}: PreviewProps & { kind: "code" | "join" | "spectate" | "server" }) {
  const anchor = useRef<HTMLButtonElement>(null);
  const [value, setValue] = useState(
    presetId === "empty" ? "" : kind === "code" ? "abf5e21c-7029-4adc-b72c-ac1096018f8d" : "Avery",
  );
  const [open, setOpen] = useState(true);
  const close = () => onComplete("Closed");
  const accept = () => {
    if (value.trim()) {
      onComplete(`${kind}: ${value.trim()}`);
    }
  };
  const common = { isVisible: open, onClose: close, anchorRef: anchor };
  let content;
  if (kind === "code") {
    content = (
      <EnterCodePopoverView
        {...common}
        gameId={value}
        onCodeChange={setValue}
        isLoading={presetId === "loading"}
        onConnect={accept}
      />
    );
  } else if (kind === "join") {
    content = (
      <JoinGamePopoverView
        {...common}
        playerName={value}
        setPlayerName={setValue}
        isLoading={presetId === "loading"}
        handleJoin={async () => accept()}
        handleKeyDown={(event) => {
          if (event.key === "Enter") {
            accept();
          }
        }}
        errorMessage=""
        loadingMessage="Joining game…"
      />
    );
  } else if (kind === "spectate") {
    content = (
      <SpectatePopoverView {...common} name={value} onNameChange={setValue} onSpectate={accept} />
    );
  } else {
    content = (
      <ServerSwitchPopoverView {...common}>
        <ServerListView
          servers={[
            { alias: "olympus", url: "https://olympus.invalid" },
            { alias: "tharsis", url: "https://tharsis.invalid" },
          ]}
          probes={
            presetId === "loading"
              ? {}
              : {
                  olympus: {
                    status: "online",
                    latencyMs: 20,
                    meta: { alias: "olympus", name: "Olympus", version: "0.24.0" },
                  },
                  tharsis: { status: "down" },
                }
          }
          currentAlias=""
          onSelect={(alias) => onComplete(`Selected ${alias}`)}
          rowClassName="px-4 py-3 text-sm"
        />
      </ServerSwitchPopoverView>
    );
  }
  return (
    <>
      <div className="absolute left-1/3 top-1/4">
        <GameButton ref={anchor} onClick={() => setOpen(true)}>
          Open {kind}
        </GameButton>
      </div>
      {content}
    </>
  );
}

const presets = (...items: string[]) =>
  items.map((id) => ({
    id,
    name: id
      .split("-")
      .map((word) => word[0].toUpperCase() + word.slice(1))
      .join(" "),
  }));
const path = (name: string) => `components/ui/${name}.tsx`;
export const sessionPreviews: PreviewDefinition[] = [
  {
    id: "enter-code",
    name: "Enter game code popover",
    category: "Session",
    componentPaths: [path("popover/EnterCodePopover")],
    presets: presets("empty", "filled", "loading"),
    Component: (props) => <SessionPopoverPreview {...props} kind="code" />,
  },
  {
    id: "join-popover",
    name: "Join game popover",
    category: "Session",
    componentPaths: [path("popover/JoinGamePopover")],
    presets: presets("empty", "named", "loading"),
    Component: (props) => <SessionPopoverPreview {...props} kind="join" />,
  },
  {
    id: "spectate-popover",
    name: "Spectate game popover",
    category: "Session",
    componentPaths: [path("popover/SpectatePopover")],
    presets: presets("empty", "named"),
    Component: (props) => <SessionPopoverPreview {...props} kind="spectate" />,
  },
  {
    id: "server-switch",
    name: "Switch server popover",
    category: "Application",
    componentPaths: [path("popover/ServerSwitchPopover")],
    presets: presets("mixed", "loading"),
    Component: (props) => <SessionPopoverPreview {...props} kind="server" />,
  },
  {
    id: "waiting-room",
    name: "Game lobby and waiting room",
    category: "Session",
    componentPaths: [path("overlay/WaitingRoomOverlay")],
    presets: presets("host", "guest", "solo", "bot-loading", "bot-failed", "demo"),
    Component: WaitingPreview,
  },
  {
    id: "server-selector",
    name: "Server selector",
    category: "Application",
    componentPaths: [path("servers/ServerSelectorScreen"), path("servers/ServerList")],
    presets: presets("mixed", "loading", "empty"),
    Component: ServerSelectorPreview,
  },
  {
    id: "mode-rules",
    name: "Game mode rules",
    category: "Session",
    componentPaths: [path("lobby/GameModeRules")],
    presets: presets("standard", "long"),
    Component: RulesPreview,
  },
  {
    id: "game-setup",
    name: "Game setup",
    category: "Session",
    componentPaths: [path("lobby/GameSetupControls"), path("lobby/LobbyMapInfoPanel")],
    presets: presets("editable", "read-only"),
    Component: SetupPreview,
  },
  {
    id: "lobby-settings",
    name: "Lobby settings",
    category: "Session",
    componentPaths: [path("overlay/LobbySettingsOverlay")],
    presets: presets("general", "token-configured"),
    Component: SettingsPreview,
  },
  {
    id: "demo-setup",
    name: "Demo setup",
    category: "Session",
    componentPaths: [path("overlay/DemoSetupOverlay")],
    presets: presets("host", "guest"),
    Component: DemoPreview,
  },
  {
    id: "starting-selection",
    name: "Corporation, prelude and starting cards",
    category: "Game phases",
    componentPaths: [path("overlay/StartingCardSelectionOverlay")],
    presets: presets("prelude", "classic", "many-cards"),
    Component: StartingPreview,
  },
  {
    id: "research",
    name: "Research card purchase",
    category: "Game phases",
    componentPaths: [path("overlay/ProductionCardSelectionOverlay")],
    presets: presets("standard", "unaffordable", "random-buy", "long"),
    Component: ResearchPreview,
  },
  {
    id: "production",
    name: "Production phase",
    category: "Game phases",
    componentPaths: [path("modals/ProductionPhaseModal")],
    presets: presets("standard", "last-round"),
    Component: ProductionPreview,
  },
  {
    id: "corporations",
    name: "Corporation roster",
    category: "Game phases",
    componentPaths: [path("overlay/CorporationOverlay")],
    presets: presets("multiplayer", "solo"),
    Component: CorporationPreview,
  },
  {
    id: "showcase",
    name: "Corporation and prelude showcase",
    category: "Game phases",
    componentPaths: [path("overlay/CorporationShowcaseOverlay")],
    presets: presets("corporation", "preludes", "roster"),
    Component: ShowcasePreview,
  },
  {
    id: "join",
    name: "Join game",
    category: "Session",
    componentPaths: [path("overlay/JoinGameOverlay")],
    presets: presets("empty", "named", "loading", "error"),
    Component: JoinPreview,
  },
  {
    id: "spectate",
    name: "Spectate game",
    category: "Session",
    componentPaths: [path("overlay/SpectateGameOverlay")],
    presets: presets("empty", "named", "loading"),
    Component: SpectatePreview,
  },
  {
    id: "reconnect",
    name: "Reconnect as player",
    category: "Session",
    componentPaths: [path("overlay/PlayerSelectionOverlay")],
    presets: presets("disconnected", "all-connected", "long-names"),
    Component: ReconnectPreview,
  },
  {
    id: "player-confirmation",
    name: "Player management confirmation",
    category: "Session",
    componentPaths: ["components/layout/main/GameConfirmDialogs.tsx"],
    presets: presets("kick", "convert-to-bot"),
    Component: ConfirmationPreview,
  },
  {
    id: "pass-confirmation",
    name: "Pass with actions available",
    category: "Game phases",
    componentPaths: [path("modals/PassConfirmDialog")],
    presets: presets("standard", "long"),
    Component: PassPreview,
  },
  {
    id: "changelog",
    name: "Changelog",
    category: "Application",
    componentPaths: [path("overlay/ChangelogModal")],
    presets: presets("ready", "long", "loading", "error", "empty"),
    Component: ChangelogPreview,
  },
  {
    id: "server-down",
    name: "Server unavailable",
    category: "Application",
    componentPaths: [path("overlay/ServerDownDialog")],
    presets: presets("standard", "long-name"),
    Component: ServerPreview,
  },
  {
    id: "tab-conflict",
    name: "Game active in another tab",
    category: "Application",
    componentPaths: [path("overlay/TabConflictOverlay")],
    presets: presets("standard", "long"),
    Component: TabPreview,
  },
  {
    id: "install-help",
    name: "Install app instructions",
    category: "Application",
    componentPaths: [path("InstallHowToSheet")],
    presets: presets("ios", "android"),
    Component: InstallPreview,
  },
  {
    id: "endgame",
    name: "Endgame scoring, graphs and replay",
    category: "Game phases",
    componentPaths: [
      path("endgame/EndGameBottomBar"),
      path("endgame/GameGraphs"),
      path("endgame/ReplayControls"),
      path("endgame/VPPhaseTabsOverlay"),
    ],
    presets: presets("score", "counting", "graphs", "replay", "no-history"),
    Component: EndgamePreview,
  },
];
