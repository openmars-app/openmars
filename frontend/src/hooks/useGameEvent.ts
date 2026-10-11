import { useCallback, useEffect, useRef, useState } from "react";
import type { GameDto, ModePresentationEffectDto } from "@/types/generated/api-types.ts";
import { useUIOverlayStore } from "@/stores/uiOverlayStore.ts";

export interface GameEvent {
  id: number;
  title: string;
  soundId?: string;
  achievementName?: string;
  playerName?: string;
  playerColor?: string;
  duration: number;
  size?: "normal" | "large";
  description?: string;
  effects?: ModePresentationEffectDto[];
}

interface PendingAnnouncement extends Omit<GameEvent, "id"> {
  modeEvent?: boolean;
  turn?: { playerId: string; generation: number };
}

export function useGameEvent(game: GameDto | null, connected: boolean) {
  const queue = useRef<PendingAnnouncement[]>([]);
  const previousGame = useRef<GameDto | null>(null);
  const seenModeEvents = useRef(new Set<string>());
  const readyAt = useRef<number | null>(null);
  const nextId = useRef(0);
  const [currentEvent, setCurrentEvent] = useState<GameEvent | null>(null);
  const [queueVersion, setQueueVersion] = useState(0);
  const enqueue = useCallback((event: Omit<GameEvent, "id">) => {
    queue.current.push(event);
    setQueueVersion((version) => version + 1);
  }, []);
  const productionOpen = useUIOverlayStore((state) => state.showProductionPhaseModal);
  const dismissCurrent = useCallback(() => setCurrentEvent(null), []);

  useEffect(() => {
    const previous = previousGame.current;
    if (!connected || !game || previous?.id !== game.id) {
      queue.current = [];
      readyAt.current = null;
      previousGame.current = connected ? game : null;
      seenModeEvents.current = new Set(
        (game?.modeRuntime?.events ?? []).map((event) => String(event.id)),
      );
      setCurrentEvent(null);
      return;
    }
    previousGame.current = game;
    const fresh: PendingAnnouncement[] = [];
    for (const event of game.modeRuntime?.events ?? []) {
      if (seenModeEvents.current.has(String(event.id))) {
        continue;
      }
      seenModeEvents.current.add(String(event.id));
      fresh.push({
        modeEvent: true,
        title: event.title,
        soundId: event.soundId,
        description: event.description,
        effects: event.effects,
        duration:
          event.state === "earth-contact" || event.kind === "deadline-warning" ? 10000 : 5000,
      });
    }
    const prevMilestones = previous.milestones;
    const newMilestones = game.milestones;
    if (prevMilestones && newMilestones) {
      for (const ms of newMilestones) {
        if (ms.isClaimed && ms.claimedBy) {
          const prev = prevMilestones.find((p) => p.type === ms.type);
          if (prev && !prev.isClaimed) {
            const allPlayers = [game.currentPlayer, ...(game.otherPlayers ?? [])];
            const claimPlayer = allPlayers.find((p) => p.id === ms.claimedBy);
            fresh.push({
              title: "MILESTONE CLAIMED",
              soundId: "award-funded",
              achievementName: ms.name,
              playerName: claimPlayer?.name ?? "Unknown",
              playerColor: claimPlayer?.color ?? "#64c8ff",
              duration: 4000,
            });
          }
        }
      }
    }

    const prevAwards = previous.awards;
    const newAwards = game.awards;
    if (prevAwards && newAwards) {
      for (const aw of newAwards) {
        if (aw.isFunded && aw.fundedBy) {
          const prev = prevAwards.find((p) => p.type === aw.type);
          if (prev && !prev.isFunded) {
            const allPlayers = [game.currentPlayer, ...(game.otherPlayers ?? [])];
            const fundPlayer = allPlayers.find((p) => p.id === aw.fundedBy);
            fresh.push({
              title: "AWARD FUNDED",
              soundId: "award-funded",
              achievementName: aw.name,
              playerName: fundPlayer?.name ?? "Unknown",
              playerColor: fundPlayer?.color ?? "#64c8ff",
              duration: 4000,
            });
          }
        }
      }
    }

    if (!previous.isLastRound && game.isLastRound) {
      fresh.push({ title: "LAST ROUND", duration: 4000 });
    }

    const prevProjects = previous.projectFunding;
    const newProjects = game.projectFunding;
    if (prevProjects && newProjects) {
      for (const proj of newProjects) {
        const prev = prevProjects.find((p) => p.id === proj.id);
        if (prev && proj.seatOwners.length > prev.seatOwners.length) {
          const newOwner = proj.seatOwners[proj.seatOwners.length - 1];
          const allPlayers = [game.currentPlayer, ...(game.otherPlayers ?? [])];
          const fundPlayer = allPlayers.find((p) => p.id === newOwner.playerId);
          fresh.push({
            title: "PROJECT FUNDED",
            achievementName: proj.name,
            playerName: fundPlayer?.name ?? "Unknown",
            playerColor: fundPlayer?.color ?? "#64c8ff",
            duration: 4000,
          });
        }
      }
    }

    const myPlayerId = game.isSpectator ? undefined : game.currentPlayer?.id;
    const myTurn = game.currentPhase === "action" && game.currentTurn === myPlayerId;
    const wasMyTurn = previous.currentPhase === "action" && previous.currentTurn === myPlayerId;
    if (myPlayerId && myTurn && (!wasMyTurn || previous.generation !== game.generation)) {
      fresh.push({
        title: "YOUR TURN",
        soundId: "your-turn",
        duration: 2500,
        turn: { playerId: myPlayerId, generation: game.generation },
      });
    }
    queue.current = [...queue.current, ...fresh].filter(
      (event) =>
        !event.turn ||
        (myTurn && event.turn.playerId === myPlayerId && event.turn.generation === game.generation),
    );
    // Mode events retain server order and always precede queued turn announcements.
    queue.current.sort((a, b) => Number(Boolean(a.turn)) - Number(Boolean(b.turn)));
    const ready =
      game.currentPhase === "action" ||
      game.currentPhase === "final_phase" ||
      game.currentPhase === "complete";
    if (!ready || productionOpen) {
      readyAt.current = null;
      return;
    }
    if (currentEvent || queue.current.length === 0) {
      return;
    }
    const event = queue.current[0];
    if (event.modeEvent && readyAt.current === null) {
      readyAt.current = Date.now() + 1000;
    }
    const delay = event.modeEvent ? Math.max(0, (readyAt.current ?? 0) - Date.now()) : 0;
    const timer = setTimeout(() => {
      queue.current.shift();
      readyAt.current = null;
      setCurrentEvent({ ...event, id: nextId.current++ });
    }, delay);
    return () => clearTimeout(timer);
  }, [game, connected, productionOpen, currentEvent, queueVersion]);

  return { currentEvent, dismissCurrent, enqueue };
}
