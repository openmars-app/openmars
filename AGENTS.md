# Open Mars agent instructions

Open Mars is a multiplayer Terraforming Mars game with a Go backend, React frontend, and 3D board.

## Commands

Run commands from the repository root unless stated otherwise. Tasks use `just`; run `just` to list recipes. Modules are `backend`, `frontend`, and `proxy` (the gateway). Inside a module directory, omit its prefix.

```sh
just deps                    # Install dependencies for all modules
just dev                     # Backend :3001 and frontend :3000
just dev --proxy             # Also start the gateway on :4000
just backend test            # Backend tests with race detection and random order
just backend test -v -run TestKick ./test/action/... # Args replace the default package pattern
just test                    # Backend and gateway tests
just check                   # Run the repository's CI validation recipes
just lint                    # Lint all modules
just generate                # Generate and format TypeScript from client-facing Go DTOs
just build                   # Build backend, frontend, and gateway
```

Go tools are pinned in [backend/go.mod](backend/go.mod) and [backend/golangci-lint.mod](backend/golangci-lint.mod); recipes use `go tool`, not global installations.

## Working agreement

- Keep changes within the requested task. Prefer existing files and abstractions; create files when the change needs them. Do not create documentation unless requested.
- Move APIs forward without compatibility shims, deprecated fields, old message types, or compatibility branches. When replacing a contract, remove its old form and update consumers together.
- Never push directly to `main`; use a feature branch and pull request.
- Do not stop or restart the user's development servers without their authorization. Disposable test servers and the frontend inspector may manage their own processes.
- New backend features require tests under `backend/test/`.
- The backend owns game rules and authoritative game state. Frontend interaction and animation state are local; gameplay changes go through the backend.
- Client-facing DTOs and mappers define the TypeScript contract. Regenerate with `just generate` when that contract changes; an internal model change does not automatically require exposing it.

## Before staging or publishing

`just prepare-for-commit` must pass for the final file state before `git add`, `git commit`, `git push`, or PR creation. It formats sources, regenerates types, and runs `just check`.

A successful run covers subsequent Git operations while the file state is unchanged. Subsequent file edits invalidate it and require another run. Resolve check failures before proceeding; Git operations alone do not invalidate a successful run.

## Scoped guidance

Read the applicable instruction files when working in their directories, including deeper files for the affected area:

- [Backend](backend/AGENTS.md): actions, transports, concurrency, and server lifecycle.
- [Card and mode data](backend/assets/AGENTS.md): authoring semantics and validation.
- [Game state](backend/internal/game/AGENTS.md), [shared conditions](backend/internal/game/shared/AGENTS.md), and [DTOs](backend/internal/delivery/dto/AGENTS.md): domain events, condition extensions, and the client contract.
- [Backend tests](backend/test/AGENTS.md) and [card-family tests](backend/test/action/card_packs/AGENTS.md): harness synchronization and organization.
- [Frontend](frontend/AGENTS.md): app phases, mobile layout, UI conventions, and verification.
- [Board rendering](frontend/src/components/game/board/AGENTS.md) and [card behavior rendering](frontend/src/components/ui/cards/BehaviorSection/AGENTS.md): rendering-specific constraints.

`AGENTS.md` is the maintained source; edit it directly and retain the matching `CLAUDE.md` symlinks.

## Releases

For release work, use the existing [release-creator skill](.claude/skills/release-creator/SKILL.md). Release notes live in `changelog/<tag>/`: developer notes in `CHANGELOG.md`, player notes in `CHANGELOG-USER.md`. Use `just changelog-check <tag>` to validate them. The [release recipe](justfile) tags an up-to-date `main` after the release PR is merged; tag messages contain only the version title.
