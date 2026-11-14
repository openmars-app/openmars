<div align="center">

<h1><img src="assets/original/branding/red-frontier.png" alt="Open Mars" width="720" /></h1>

**Terraforming Mars in your browser.**

Play with friends or AI opponents on a 3D Mars board. No account needed.

[![License](https://img.shields.io/github/license/openmars-app/openmars)](LICENSE)
[![Stars](https://img.shields.io/github/stars/openmars-app/openmars)](https://github.com/openmars-app/openmars/stargazers)
[![Issues](https://img.shields.io/github/issues/openmars-app/openmars)](https://github.com/openmars-app/openmars/issues)
[![Coverage](https://img.shields.io/endpoint?url=https://openmars-app.github.io/openmars/coverage.json)](https://openmars-app.github.io/openmars/)

Original music by [Lucas](mailto:lucas.gussua@gmail.com). Thanks for the soundtrack!

</div>

> Open Mars is a fan-made, open-source project, unaffiliated with FryxGames,
> Asmodee Digital, or Steam. If you enjoy playing, please support the creators
> and [buy the physical board game](https://www.fryxgames.se/terraforming-mars/).

<p align="center">
  <img src="docs/img/hero.png" alt="A multiplayer game on the 3D Mars board, with cities, forests and oceans" width="1000" />
</p>

## Play on Mars

Build cities, plant forests and fill oceans as you compete to terraform the planet. Rotate Mars and zoom in to place tiles directly on its surface, or take a look around the solar system between turns. Cities rise and forests grow as you build them.

You can play on desktop or on a phone in landscape.

### Cards and expansions

Choose from hundreds of cards, with corporations, milestones and awards to shape your strategy. Open Mars includes the base game, Corporate Era, Prelude, Venus Next, Colonies and Turmoil.

<p align="center">
  <img src="docs/img/cards.png" alt="Project cards with simple effects, triggered abilities, resource storage and alternative actions" width="1000" />
</p>

### Multiplayer and bots

Create a lobby, pick a map and your game options, then share the invite link. Friends and AI opponents can play in the same game. If you lose your connection, you can rejoin and carry on.

<p align="center">
  <img src="docs/img/lobby.png" alt="Game lobby with player invitations, map selection and expansion settings" width="1000" />
</p>

## Run locally

Install [Go](https://go.dev), [Bun](https://bun.sh), [Rust via rustup](https://rustup.rs), [just](https://just.systems) and [Git LFS](https://git-lfs.com).

```bash
git clone https://github.com/openmars-app/openmars.git
cd openmars
git lfs pull
just deps
just dev
```

Open `http://localhost:3000` to create a game. The backend runs on port 3001.

The repository pins the Rust toolchain. `just deps` installs the engine's WebAssembly target and binding tools, along with the other project dependencies. Git LFS downloads the source models, images and audio needed to build.

## How it's built

| Part | Technology |
|------|------------|
| Interface | React, TypeScript, Tailwind CSS |
| 3D world | Rust, Bevy, WebAssembly, WebGPU / WebGL2 |
| Server | Go, Gorilla WebSocket, Gorilla Mux |
| Development tools | Tygo for generated TypeScript, Air for backend reloads |

The Go backend handles game rules and state. React handles the interface and networking; Bevy renders the 3D world. The renderer uses WebGPU when available and falls back to WebGL2.

## Contributing

Game logic lives in `backend/`, the interface in `frontend/`, and the renderer in `engine/`. Bug reports and pull requests are welcome.

```bash
just test               # Backend and gateway tests
just lint               # Lint all modules
just format             # Format source files
just prepare-for-commit # Format, regenerate types and run the CI checks
```

Run `just` to see all available commands.

## License

[MIT](LICENSE).
