## Fixed

- Restore the desktop tile-placement prompt in the shared React HUD.
  It was removed with the old renderer view during the Bevy migration.
  The prompt follows HUD visibility and shows the pending tile's icon.

## Build

- Cache Rust compilation, wasm-bindgen and prepared assets between CI jobs.
- Store Docker build layers in the registry for reuse across release tags.
