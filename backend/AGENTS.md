# Backend instructions

## Commands and server lifecycle

Use root recipes `just backend test`, `just backend lint`, and `just backend check`. Test arguments replace the default `./test/...` pattern; for example, `just backend test -v -run TestKick ./test/action/...`. See the [module recipes](justfile) for coverage and other commands.

The user normally runs Air through `just dev` or `just backend dev`. [Air configuration](.air.toml) watches Go changes but excludes `assets/` and does not watch JSON. Do not assume a card-data edit has reached an already running server. If verifying it requires restarting the user's server, obtain authorization unless already given. Test harnesses may start and stop their own isolated servers.

## Ownership and operations

- Actions in `internal/action/` orchestrate operations through domain methods and shared behavior execution. Reuse `BaseAction` dependencies where applicable.
- Keep transport parsing and response delivery in handlers; keep game rules in backend domain/action code. Domain methods and card behavior appliers can mutate state.
- Use [state calculation](internal/action/state_calculator.go) for playability, costs, errors, and availability presented to clients. Keep execution validation consistent with those results.
- Passive effects react through domain events. Do not add card-by-card passive-effect checks to individual actions.
- Follow [game-state instructions](internal/game/AGENTS.md) for domain mutation, events, and client updates.
- Game access from outside the WebSocket hub goroutine must use the existing serialization path, `Hub.Do`, as bot tools do. Never call `Hub.Do` from the hub goroutine itself: it waits on that same goroutine. See [hub.go](internal/delivery/websocket/core/hub.go).
- Wire new dependencies in [internal/app](internal/app); do not introduce parallel registries or transport-specific game-rule implementations.

## New WebSocket operations

1. Read the actor from `connection.Identity()`, never from a client-supplied player identity.
2. Parse and validate the payload, then delegate to the action.
3. Send rejection errors with `connection.SendError(message.Type, ...)` so only the sender receives them.
4. After a successful state-changing action completes, send personalized updates through `broadcaster.BroadcastGameState`. Follow the existing operation's recipient policy rather than sending one shared DTO to everyone.
5. Register the handler in [registry.go](internal/delivery/websocket/registry.go), update the client contract when needed, and add rejection cases to [contract_test.go](test/e2e/contract_test.go).

[play_card.go](internal/delivery/websocket/handler/card/play_card.go) demonstrates the handler/action/broadcast boundary. HTTP handlers likewise delegate to actions and map results to DTOs.

## Client contract

Only the DTO package is configured for TypeScript generation in [tygo.yaml](tygo.yaml). Internal model additions are not automatically public.

For client-visible changes, update the DTO and mapper together, then run `just generate` from the repository root. This also formats the generated file; do not run bare `tygo generate` or edit generated TypeScript manually. See [DTO instructions](internal/delivery/dto/AGENTS.md) for tags and visibility rules.

## Cards and modes

Use the existing primitives in [internal/game/cards](internal/game/cards) and the [asset authoring guide](assets/AGENTS.md). Card loading, validation, and resolved mode catalogs live in that package. For new condition types or fields, follow the [shared-condition checklist](internal/game/shared/AGENTS.md).

Bot game access uses the same hub serialization as player actions. The implementation is in [internal/service/bot](internal/service/bot); personas and strategy guidance are in [assets/bot](assets/bot).

## Logging

- Use structured `log/slog` fields for IDs, counts, and error context.
- Use concise sentence-case messages without emojis, decorative prefixes, or “successfully”.
- Give each successful operation one final Info summary; intermediate steps, queries, and routine transport details belong at Debug.
- Warn for rejected input and recoverable failures; Error for unexpected failures or broken invariants.
- Do not log the same error at multiple layers; let the outermost handling layer report it.

## Tests

Place tests under `test/` and reuse `testutil`. Add tests for new backend features and cover both accepted and rejected operations. Use the [test guide](test/AGENTS.md) for real-server tests and deterministic synchronization. Run affected tests during iteration; the root preparation policy governs staging and publishing.
