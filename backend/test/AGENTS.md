# Backend test instructions

Place tests in the matching area under this directory and reuse [testutil](testutil) for setup and assertions. Tests normally log errors only; set `OPENMARS_TEST_LOG_LEVEL=debug` when investigating a failure.

From the repository root, `just backend test` runs the backend suite with race detection and random order. Arguments replace the default package pattern; use an explicit affected package during iteration.

## Real-server functional tests

[e2e/harness](e2e/harness) starts production wiring on isolated `httptest` servers and uses real HTTP/WebSocket clients. These servers are separate from the user's development server.

- `srv.Play(t, edit, names...)` reaches the first action phase; `srv.Lobby` stops in the lobby. Set development-mode requirements explicitly through the harness options.
- Send, then use `Await`, `AwaitState`, or `AwaitError`. Synchronize on events and barriers, not sleeps.
- `c.Sync(t)` returns messages queued for that client so far. Different connections are not ordered: after one client acts, sync that client or await its reply before syncing others.
- Use `ExpectQuiet` to verify that a client received nothing, and `srv.CloseAndWait(t, c)` to wait for the server to process a disconnect.
- Use `harness.Timeout()` where a timeout is needed; it accounts for slower race/coverage runs.
- Add rejection payloads for new client message types to [contract_test.go](e2e/contract_test.go). `TestContract_EveryMessageTypeIsCovered` checks registry coverage. Verify that rejected operations do not change state or notify unrelated players.

## Behavior and card tests

[Action behavior tests](action/behavior) use constructed conditions, grouped by category, to exercise individual fields and their interactions. Follow the [shared-condition checklist](../internal/game/shared/AGENTS.md) for new types or fields.

Card/corporation integration tests in [card_packs](action/card_packs/AGENTS.md) follow existing family files. Passive effects and mode lifecycle tests belong alongside their existing suites; not every test involving a card belongs in `card_packs`.
