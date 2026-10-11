# Asset tools

Approved masters live in `assets/original/`, with export settings in the root asset catalog and profiles. Install frontend dependencies with Bun and fetch source media with `git lfs pull`. The image exporter runs under Node 24+; Bun remains the package manager and test runner.

- `just frontend assets`: incrementally generate hashed runtime files and the shared registry.
- `just frontend assets-check`: validate source coverage, card identities, and generated files.
- `just frontend assets-test`: run exporter tests.
- `just frontend assets-preview`: generate `output/asset-preview/index.html` for local review.

`just dev`, `just frontend dev`, production builds, and the frontend dev/build/typecheck scripts prepare assets automatically. Vite watches originals and settings during development. Generated output is ignored; edit sources and export settings instead. CI and Docker generate assets before building the application.

## Board inspection

`just frontend inspect` runs the real game board renderer without a backend or a game session. It starts its own temporary loopback Vite server and fresh, muted browsers. It leaves existing dev servers and saved player preferences alone.

```sh
just frontend inspect --list
just frontend inspect --card 123 --open
just frontend inspect --tile industrial-center
just frontend inspect --tile volcano --state placement --frames 0,30,120 --json
just frontend inspect --scene structures --open
just frontend inspect --card 123 --scene neighbors --climate terraformed
just frontend inspect --card 123 --scene destruction --state placement --frames 0,60,240,600
```

Select by card ID or tile type. Cards resolve their actual placement and authored structure style from `cards.json`. Cards with several supported placements require `--tile` too. Off-Mars targets are excluded. A special tile type uses its uniquely authored card style when available. The neighbors scene adds a second subject tile, and destruction replaces the subject with a nuclear zone. The default comparison board contains the subject, a city, greenery, an ocean and empty hexes. The structure styles scene replaces the former `?landscape=showcase` branch with ordinary board data.

Captures default to gameplay and close cameras, desktop Firefox, and iPhone and Android landscape emulation in Chromium. The iPhone profile checks layout and low-tier graphics, not Safari compatibility. Use `--views gameplay,close`, `--devices desktop,iphone,android`, `--climate barren|growing|terraformed`, `--seed 42`, and `--color '#e43c32'` to narrow or vary a capture. `--open` launches an interactive inspector with subject selection, camera controls, an exact frame input, playback, step and replay controls. Copying its link preserves the selected frame and configuration.

Frames advance at a fixed 60 fps. The initial board, GPU warmup and asynchronous landscape preparation finish before frame zero. Placement state then introduces the subject through the same tile-change detection used in play. Seeking backwards rebuilds the scene and replays it. CPU preparation can take wall-clock time without advancing the scene clock. Repeated frames are intended to agree on the same browser and GPU. Different browsers and GPUs can produce different pixels.

Screenshots and a `report.json` go to ignored `frontend/output/inspection/<target>/<run>/`. `--output DIR` changes the destination. `--json` prints the report on stdout and diagnostics on stderr. Reports include target, seed, climate, frame, viewport, graphics tier, renderer, browser, warnings and capture errors. A successful capture means rendering completed. **Open and inspect the images before calling the result visually verified.** The report deliberately leaves visual review marked as required. The temporary server URL ends when the command exits. Use the recorded configuration or copied path on an active dev server to reopen it.

The `/dev/board` entry and its fixtures are excluded from production builds. The inspector does not implement game rules, load saves, or send game commands. Add scenarios as typed fixtures using production components, not separate preview geometry, materials or lighting.

Validation: `bun test scripts/inspect` checks target resolution and fixtures. `just frontend e2e tests/inspection.spec.ts --workers 1` checks isolation, frame replay and reachable controls. Geometry scripts under `verify` supplement these captures.
