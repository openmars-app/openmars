# Frontend instructions

## Development and verification

The user normally runs Vite with hot module replacement. Do not stop or restart their server without authorization. Tests and the board inspector may manage their own isolated servers.

For frontend source changes, run `just frontend check` and `just frontend build` from the repository root. The check recipe includes formatting, lint, type checking, asset validation, asset tests, and inspector tests. There is no general app `test` script; use the commands in [justfile](justfile). The root preparation policy still governs staging and publishing.

For a visual change, capture the final result with the production renderer, open the images, and correct visible problems before reporting completion. Assertions, successful screenshot generation, geometry checks, and isolated previews do not establish visual correctness. Report which images you inspected and any verification gaps.

Inspect desktop, iPhone landscape, and Android landscape. Check initial, loading, error, expanded, long-content, and keyboard-open states as applicable. Confirm controls remain reachable and content is not clipped by fixed chrome. Preserve the portrait rotation overlay.

For board graphics, follow the [board review instructions](src/components/game/board/AGENTS.md) and [inspector commands](scripts/README.md). The inspector can start and stop its own temporary server; this does not authorize restarting the user's server.

## State ownership

- Backend state and rules are authoritative. Send gameplay operations through the existing services; do not recreate game rules or fabricate server-state updates locally.
- Import client contract types from the generated backend types. Change DTOs and regenerate when that contract needs to change.
- Local interaction, overlay, and animation state belongs in the existing frontend stores.
- Use server events for synchronization, not delays waiting for an expected response. Timers controlling display duration are valid; clean them up when their owning state changes.

## App phases and background

Read [appPhaseStore.ts](src/stores/appPhaseStore.ts) for the current union and selectors. Determine screens through `phase.kind` and its helpers, not combinations of booleans across stores.

Only these owners write the app phase:

- [App.jsx](src/App.jsx) sets menu phases from routing.
- [useGameInitialization](src/hooks/useGameInitialization.ts) handles bootstrapping.
- [useGameTransitions](src/hooks/useGameTransitions.ts) handles the remaining lifecycle and animation transitions.

Add lifecycle transitions in the transition hook. Update selector projections when adding a phase. Reset world-ready flags, including new ones, when initializing another game; retained readiness can skip that game's loading sequence. Anchor animation timers to phase entry and cancel them on cleanup.

Keep the `SpaceBackground` wrapper mounted once at the App root. Control visibility through phase selectors. Keep the outer `GameLayout` transparent; the inner game-world content owns its background so the lobby does not obscure the space scene.

Preserve the low-graphics exception: the background wrapper stays mounted, but its menu Canvas is released during gameplay and rebuilt on return. Do not defeat that GPU-memory saving by forcing both canvases to stay active.

## UI conventions

- Reuse `GameButton`, `BackButton`, `GameMenuModal`, and existing popovers. Check actual component props rather than copying old examples; see [GameButton](src/components/ui/buttons/GameButton.tsx).
- Use `GameIcon` or the existing semantic display components for game icons, not emojis or ad hoc image tags. Use [BehaviorSection](src/components/ui/cards/BehaviorSection/AGENTS.md) for behavior relationships and resource layouts.
- Do not use native HTML `title` tooltips. Information must also be accessible on touch devices.
- Use only default, pointer, and text cursors.
- Use centralized [Z_INDEX](src/constants/zIndex.ts) constants and `getZIndex` for layering; add a named constant when needed instead of hard-coded values.
- Use Tailwind with CSS-based `@theme` configuration in [index.css](src/index.css). Do not add CSS Modules or a JavaScript Tailwind configuration.
- Use Orbitron for titles, numbers, short labels, and buttons; use the default font for descriptions and other long text.
- Keep braces around `if` blocks and avoid nested ternaries. Explicitly discard intentionally unawaited event-handler promises with `void`.

## Mobile and compact layout

- [useLayoutMode](src/hooks/useLayoutMode.ts) is the layout authority; use `getLayoutMode` outside React. It accounts for coarse pointers, physical screen size, and the `?compact` override. Do not choose layout from `window.innerWidth`; positioning math is fine.
- Phone screens are landscape-only. Keep `RotateDeviceOverlay` covering portrait menus, lobby, and game.
- `GameInterface` selects `MobileGameLayout` instead of `GameLayout`. Keep compact branches out of desktop-only HUD components; give shared prompts, inspections, and overlays compact presentations.
- Reuse `MobileScreen`, `MobileCardDetail` / `CardDetailPortal`, `mobileUiStore`, and `useMobileGame` for mobile flows.
- Use the `compact:` Tailwind variant rather than viewport-width breakpoints. Use layout hooks for behavior differences.
- Position board overlays using the HUD variables `--hud-top-h`, `--hud-dock-h`, `--hud-rail-left-w`, `--hud-rail-right-w`, plus `--safe-top`, `--safe-right`, `--safe-bottom`, and `--safe-left`.
- Provide touch equivalents: `RevealTrigger` / `useTapReveal` for hover information, `useLongPress` plus a visible button for context menus, and on-screen controls for hotkeys.
- Respect [GRAPHICS](src/utils/graphicsQuality.ts). Use `useRenderPause(reason, active)` from [renderPauseStore](src/stores/renderPauseStore.ts) when an overlay covers the canvas.
- Use touch targets of at least 44×44px, input text of at least 16px, and other text of at least 11px. Use `dvh` for viewport-height surfaces.
- Substantial forms and dialogs need a full-screen compact shell, fixed top bar, safe-area padding, one scrolling content region, and reachable actions. Account for the software keyboard rather than shrinking a desktop modal.

## Assets and feedback

Use approved masters in root `assets/original/`, the root `assets/catalog.json`, and the [asset pipeline](scripts/README.md). Generated files are not authoring inputs. Energy artwork uses the `power` asset name.

For sounds, add source audio and catalog entries through that pipeline, then register playback in [audioService](src/services/audioService.ts) and use the existing sound hook.

`useNotifications` provides menu feedback, but `NotificationContainer` is hidden on game routes. Use in-game overlays or prompts for gameplay feedback.

## Browser checks

Use `?quick` for functional browser inspection when skipped graphics are irrelevant. Omit it when reviewing those visuals or their performance; board visual review uses the production-renderer inspector.

Playwright specs in [tests](tests) are checked for formatting, lint, and types by the normal gate, but execution is opt-in:

```sh
just frontend e2e
E2E_BACKEND=1 just frontend e2e
```

Backend-dependent specs need a running backend. [playwright.config.ts](playwright.config.ts) starts its own Vite server on port 3100 by default; `E2E_PORT` changes it and `E2E_REUSE_SERVER=1` permits reuse. Do not restart the user's port-3000 server to run these tests.

The mobile browser profiles emulate iPhone and Android layout using Chromium; they do not establish Safari compatibility. Keep the portrait-overlay checks and supplement assertions with actual image inspection.
