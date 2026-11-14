# Asset tools

Approved masters live in `assets/original/`, with export settings in the root asset catalog and profiles. Install frontend dependencies with Bun and fetch source media with `git lfs pull`. Bun runs the image exporter, build scripts and tests.

Models remain GLB sources in Git LFS. Preparation preserves geometry and node names while resizing embedded textures to core PNG images for Bevy's GLTF loader. The skybox EXR is converted offline to a linear RGBA16F KTX2 cubemap with mip levels by `engine/asset-pipeline`, adapted from the sibling renderer. This native tool builds automatically with the pinned Rust toolchain; browsers download the prepared cubemap. The terrain color/detail arrays are prepared offline from catalog layer references as lossless zlib KTX2, with periodic sampling, linear-light color mipmaps and straight alpha. Both renderers load the same 1024px arrays. Each layer source digest participates in cache invalidation, and source changes during preparation fail instead of publishing an inconsistent result. Models, textures and UI images share the generated registry and hashed URLs.

- `just frontend assets`: incrementally generate hashed runtime files and the shared registry.
- `just frontend assets-check`: validate source coverage, card identities, and generated files.
- `just frontend assets-test`: run exporter tests.
- `just frontend assets-preview`: generate `output/asset-preview/index.html` for local review.

`just dev`, `just frontend dev`, production builds, and the frontend dev/build/typecheck scripts prepare assets automatically. Vite watches originals and settings during development. Generated output is ignored; edit sources and export settings instead. CI and Docker generate assets before building the application.

The generated renderer contract declares required asset identities, decoding color space, array layer counts and byte limits. The encoded LRU cache is capped at 64, 160 or 320 MiB for Low, Medium or High texture quality; individual files are limited to 64 MiB. These are download-cache limits; decoded CPU/GPU images have separate residency and retirement checks.

## Engine inspection

`just frontend inspect` exercises the production Bevy WebGPU and WebGL2 canvases on desktop, iPhone landscape, and Android landscape Chromium profiles. It also checks automatic fallback, unavailable renderers, retry, disposal and pause/resume. These mobile profiles emulate layout, not Safari or physical devices.

The development-only `/dev/engine` page uses the same canvas, loading status, and lifecycle as the app. Use `?renderer=webgpu` or `?renderer=webgl2` to force a backend in development; production always tries WebGPU before WebGL2. Each backend has a separate WASM artifact built from the same engine source. Screenshots are saved under the repository's `tmp/bevy-scaffold/` directory. Open those images to review the actual rendering.

The previous renderer, inspector, and migration comparison fixtures have been removed. Tests cover the current engine’s behavior, preparation, and resource lifetimes.
