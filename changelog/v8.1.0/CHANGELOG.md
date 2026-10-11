Highlights since v8.0.0.

## Added

- Classic and Earth Withdrawal use curated decks and rules from
  `gamemodes.json`. `GET /api/v1/game-modes` exposes the catalog.
- Earth Withdrawal adds 44 projects, four preludes and two corporations.
  Its runtime tracks contact windows, dependent effects and imports.
- Earth support starts at 2 M€ and increases each connected generation.
  All players receive it after everyone finishes generation setup.
- Withdrawal deadlines have configurable warning and contact-loss sounds.
  Mode announcements finish before turn announcements begin.
- Hosts can export portable JSON saves and import them into resume
  lobbies. Saves restore pending decisions, history and game logs.
- Save routes are `GET /api/v1/games/{gameId}/save` and
  `POST /api/v1/game-saves/validate` or `/api/v1/game-saves/import`.
  Imports require matching rules and content, with a 128 MiB limit.
- Resume lobbies support seat claiming and release, plus bot
  reconnection with newly supplied credentials.
- The frontend inspector renders deterministic board fixtures and
  captures screenshots or animation frames without starting a game.

## Changed

- Setup replaces `cardPacks` and `venusNextEnabled` with `gameModeId`,
  `gameModeParameters` and `features`. Classic offers Venus and Colonies
  independently; Project Funding remains an experimental option.
- Cards replace `pack` with `gameModes`, `requiresFeatures` and
  `requiresSystems`. The backend validates mode-specific availability.
- New games default to Classic with Preludes and Development Mode on.
- Optional effects use `optional` and a dedicated confirmation message.
  Shared action groups present alternatives as one action.
- The shared structure renderer replaces the city-specific pipeline.
  Tile visuals rename `city` to `structure`; Industrial Center and other
  complexes use the same terrain blending and neighbor connections.
- WebSocket commands execute through the hub with connection-bound
  identities. Game creation uses HTTP; obsolete WebSocket aliases are
  removed, including the separate standard-project action messages.
- Industrial Center gains factories and pipework. Commercial District,
  Lava Tube Settlement and Corporate Stronghold have distinct layouts.
- Nuke and Nuclear Zone are available in both game modes.
- Cards and notifications share condition, state and VP symbols.
  Earth Withdrawal cards have new artwork.
- Game setup uses collapsible sections on phones. The Game mode view
  explains rules and compares connected and disconnected production.
- The main menu and loading screen use the new Open Mars branding.

## Fixed

- Cards gained from tile bonuses now open the received-cards dialog.
- Production no longer flashes back after submitting card selection.
  Final production shows Next instead of Buy cards.
- End-game score counting waits for production to close and survives
  interrupted rendering. The winner appears after the count finishes.
- Optional payments no longer display an empty choice, and unavailable
  Earth-dependent actions explain which connection requirement failed.
- VP tooltips include every scoring condition. Connection badges no
  longer shift resource icons out of alignment.
- Effect notifications stay beneath modal backdrops.
- Tile placement tools include the special tiles in the game catalog.
- Save validation parses integer map keys explicitly instead of
  attempting an invalid reflection conversion.

## Build

- Go builds and CI use 1.27.2, with `golang.org/x/tools` at v0.51.0.
- HTTP and WebSocket functional tests exercise real client connections.
  Save lifecycle tests cover export, import and resumed play.
- Race tests allow 20 minutes per package for save lifecycle checks.
- Artwork generation isolates candidate files from production assets
  and records prompts and provenance. Ambiguous timeouts no longer
  automatically resubmit generation requests.
