Highlights since v8.2.0.

## Added

- A Rust Bevy renderer builds separate WebGPU and WebGL2 WASM modules.
  WebGPU is preferred, with automatic WebGL2 fallback when unavailable.
- A typed scene contract connects authoritative Go DTOs to Bevy through
  React. Renderer input returns through the existing gameplay actions.
- A dedicated WASM worker prepares terrain geometry without blocking
  the browser UI. Tile revisions publish independently.
- Settings groups Controls, Graphics, Audio and AI. Graphics includes
  Low, Medium and High presets, render resolution, texture quality,
  effect detail, anti-aliasing, FPS limits and space brightness.
- The Performance window separates browser and engine metrics, with
  recording, downloadable traces and recording across the next reload.
- Renderer diagnostics cover startup, device loss, retry and resource
  retirement. Profiling correlates requests with terrain preparation.

## Changed

- Bevy replaces the Three.js board and menu renderer. The Go backend
  retains game rules and authoritative state; React retains networking
  and the game interface. The old frontend snapshot and migration-only
  renderer comparison tests and fixtures are removed.
- Shared foliage, city and special-tile materials and pipelines prepare
  before gameplay. Planet destinations prepare before camera travel.
- Assets retain GLB source models in Git LFS. Offline preparation builds
  KTX2 environment maps, terrain arrays and texture-quality variants.
- Encoded asset caching uses quality-dependent memory budgets. Scene
  replacement retires decoded images, buffers and temporary views.
- Camera orbit and zoom use smoothing and saved sensitivity controls.
  Audio controls and bot-thought visibility move into Settings.
- Compact lobbies separate Players, Chat, Map and Settings; desktop
  keeps its combined setup form. Graphics menus support touch and keys.
- The Performance window uses readable monospaced values. Renderer
  timings report CPU work; unavailable GPU measurements stay explicit.

## Fixed

- Rapid tile placements no longer wait for a quiet period before their
  animations start. Newer terrain work cannot replace unrelated tiles.
- First city, greenery and Mohole placements no longer compile their
  shared render pipelines during the placement frame.
- Reloading a game keeps its loading screen through world preparation
  instead of briefly revealing the HUD over a black canvas.
- Reconnects restore player and spectator sessions without overlapping
  requests. Spectators receive the map identity needed by the renderer.
- Clicking a visible planet changes focus again. Travel retains world
  resources instead of showing another loading screen.
- The Sun corona no longer covers the stars with an opaque black disk.
  Space brightness adjusts the background without changing lighting.
- Renderer failures retain useful diagnostics and allow retry with
  fresh resources. Scene changes cancel obsolete preparation work.

## Build

- Bun runs frontend, asset and engine orchestration scripts. The Rust
  toolchain and wasm-bindgen version are pinned for reproducible builds.
- CI validates both renderer targets, generated scene types, native
  geometry contracts, asset preparation and the browser bridge.
- Production packaging includes both renderer modules and the terrain
  worker, with WASM MIME types, gzip delivery and hashed asset caching.
- Headless browser coverage exercises both backends, game sessions,
  resource lifetimes, quality settings and compact layouts.
