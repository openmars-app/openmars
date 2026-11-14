<div align="center">

<h1><img src="assets/original/branding/red-frontier.png" alt="Open Mars" width="720" /></h1>

**Play the beloved board game online with a fully interactive 3D Mars.**

Real-time multiplayer. 450+ cards. Six expansions. One red planet.

[![License](https://img.shields.io/github/license/openmars-app/openmars)](LICENSE)
[![Stars](https://img.shields.io/github/stars/openmars-app/openmars)](https://github.com/openmars-app/openmars/stargazers)
[![Issues](https://img.shields.io/github/issues/openmars-app/openmars)](https://github.com/openmars-app/openmars/issues)
[![Coverage](https://img.shields.io/endpoint?url=https://openmars-app.github.io/openmars/coverage.json)](https://openmars-app.github.io/openmars/)

**Original soundtrack by [Lucas](mailto:lucas.gussua@gmail.com)** -- thank you for scoring the red planet.

</div>

---

> **Disclaimer** -- This is a fan-made, open-source project and is not affiliated with
> FryxGames, Asmodee Digital, or Steam. If you enjoy this, please
> [buy the physical board game](https://www.fryxgames.se/terraforming-mars/) -- it's excellent.

---

<p align="center">
  <img src="docs/img/hero.png" alt="The current 3D Mars board with cities, forests and oceans, surrounded by the game interface" width="1000" />
</p>

## What is this?

Open Mars is a digital board game based on Terraforming Mars, playable in your browser. The Mars surface is rendered as an interactive 3D hex grid -- you rotate, zoom, and place tiles directly on the planet. Games run in real time over WebSockets with no account required.

This is not a simplified clone. It implements the complete rule set across six expansions with hundreds of cards, corporations, milestones, awards, and colonies.

## Features

### A living 3D board

Rotate Mars, watch cities rise and forests spread, and travel around the solar system. Rust and Bevy render the world through WebGPU, with automatic WebGL2 fallback. Tile placement, asteroid impacts and the original soundtrack bring the board to life.

### Cards and expansions

Play with 450+ cards across the base game, Corporate Era, Prelude, Venus Next, Colonies and Turmoil. Browse cards by type, cost, tags and game mode, with their effects and requirements shown on the card.

<p align="center">
  <img src="docs/img/cards.png" alt="Project cards showing simple effects, triggered abilities, resource storage and alternative actions" width="1000" />
</p>

### Play together

Create a lobby, choose your map and game options, and invite friends with a game link. Sessions update in real time and support reconnection. On mobile, Players, Chat, Map and Settings have their own tabs.

<p align="center">
  <img src="docs/img/lobby.png" alt="Game lobby with player invitations, map selection and expansion settings" width="1000" />
</p>

### Make it your game

Choose a graphics preset or adjust resolution, textures, effects and frame rate individually. Set camera sensitivity, tune the space background, and keep audio and AI preferences together in Settings.

## Quick Start

```bash
git clone https://github.com/openmars-app/openmars.git
cd openmars
just deps        # Install dependencies
just dev         # Launch frontend (3000) + backend (3001)
```

Requires [Go](https://go.dev), [Bun](https://bun.sh), [Rust via rustup](https://rustup.rs) and [just](https://just.systems). The engine pins its Rust toolchain; `just deps` installs the WASM target and binding tools. Source media uses [Git LFS](https://git-lfs.com); run `git lfs pull` before building.

Open `http://localhost:3000`, create a game, and invite friends with the game code.

## Tech Stack

| Layer    | Technology                              |
|----------|-----------------------------------------|
| Frontend | React, TypeScript, Tailwind CSS |
| Renderer | Rust, Bevy, WebAssembly, WebGPU / WebGL2 |
| Backend  | Go, Gorilla WebSocket, Chi router       |
| Tooling  | Tygo (Go-to-TS codegen), Air (hot reload) |

## Contributing

Contributions are welcome. The Go backend (`backend/`) owns game rules, React (`frontend/`) owns the interface and networking, and Bevy (`engine/`) owns the 3D world. Generated types connect the API and scene contracts.

```bash
just test               # Run backend tests
just lint               # Lint Go + TypeScript
just format             # Format everything
just prepare-for-commit # Format, regenerate types and run every CI check
```

Run `just` for the full command reference.

## License

MIT -- see [LICENSE](LICENSE) for details.

---

<div align="center">

*Not affiliated with FryxGames or Asmodee Digital.*

</div>
