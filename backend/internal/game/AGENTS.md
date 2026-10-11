# Game state instructions

## Mutation and events

- Keep game state in the existing Game, Player, Board, Deck, and related domain owners. Mutate through their methods so validation, bookkeeping, and domain events stay consistent.
- Actions orchestrate operations; domain methods and behavior appliers perform their state changes. Do not introduce a second game-state store.
- Each game has its own event bus. Keep subscriptions and event data scoped to that game.
- Publish the relevant domain events when adding state transitions so passive effects and generational tracking continue to run. Do not manually repeat those effects in actions.

[EventBus.Publish](../events/event_bus.go) invokes subscribers synchronously, after releasing its subscription lock. Subscribers can trigger further domain work. Preserve this ordering; do not turn state reactions into background goroutines or hold the subscription lock while invoking handlers.

## Client updates

Domain events drive game reactions; the transport sends client state after action execution completes. Follow [play_card.go](../delivery/websocket/handler/card/play_card.go) and [Broadcaster](../delivery/websocket/broadcaster.go):

- Ensure new client-visible operations reach the existing broadcast path after their state changes and synchronous effects finish.
- Keep broadcasting at the operation boundary, rather than broadcasting every intermediate mutation or adding transport callbacks to the event bus.
- Use personalized DTOs and the existing spectator mapping. Preserve hidden information and operation-specific recipients.
- When changing pending choices, transitions, or effects, verify that the resulting state reaches the affected clients.

## Concurrency

Live game operations run on the WebSocket hub goroutine. Code outside it, including bot tools, uses [Hub.Do](../delivery/websocket/core/hub.go) for serialized game access. Do not call that blocking method from inside the hub goroutine. Event-bus locking protects subscriptions; it does not make arbitrary concurrent game mutations safe.

## Extending behavior

For condition types or fields, read [shared instructions](shared/AGENTS.md). For card or mode definitions, read [asset instructions](../../assets/AGENTS.md). Keep domain logic independent of client DTOs; expose new state only when the client contract needs it.

Add domain/action tests under `backend/test/`; use functional tests for changes to client-visible updates or cross-player effects. See [test instructions](../../test/AGENTS.md).
