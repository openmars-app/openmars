import { useEffect, useRef } from "react";
import type { GameDto } from "@/types/generated/api-types.ts";
import type { GameEvent } from "./useGameEvent.ts";

// Snapshots carry stable event IDs. Joining or reconnecting establishes a new
// baseline; presentations never replay history or hold up an engine transition.
export function useModeEvents(
  game: GameDto | null,
  connected: boolean,
  enqueue: (event: Omit<GameEvent, "id">) => void,
) {
  const seen = useRef<{ gameId: string; ids: Set<string> } | null>(null);
  useEffect(() => {
    if (!connected || !game) {
      seen.current = null;
      return;
    }
    const events = game.modeRuntime?.events ?? [];
    if (seen.current?.gameId !== game.id) {
      seen.current = { gameId: game.id, ids: new Set(events.map((event) => String(event.id))) };
      return;
    }
    for (const event of events) {
      const id = String(event.id);
      if (seen.current.ids.has(id)) continue;
      seen.current.ids.add(id);
      enqueue({
        title: event.title,
        description: event.description,
        presentation: event.presentation,
        duration: 5000,
      });
    }
  }, [game, connected, enqueue]);
}
