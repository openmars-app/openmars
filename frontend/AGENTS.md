# Frontend - Open Mars Game UI

React frontend with 3D Mars visualization using Three.js/React Three Fiber. Real-time multiplayer via WebSocket state synchronization from Go backend.

## Development Server Policy

**CRITICAL**: NEVER restart the frontend dev server yourself. ALWAYS ask the user if you think a restart is needed.

- Vite hot reload handles all code changes automatically
- Hot Module Replacement reloads React components, CSS, and TypeScript instantly
- Your role is to write code, not manage processes

## Architecture

### Directory Structure

```
frontend/
├── src/
│   ├── components/        # React components
│   │   ├── 3d/            # Three.js background components
│   │   ├── game/          # Core game UI (board, view, controls)
│   │   ├── layout/        # Layout components (panels, main)
│   │   ├── pages/         # Top-level page components
│   │   └── ui/            # Reusable UI components (cards, display, modals)
│   ├── contexts/          # React contexts for state management
│   ├── hooks/             # Custom React hooks
│   ├── services/          # API and WebSocket services
│   ├── types/             # TypeScript types (includes generated/ from backend)
│   └── utils/             # Utility functions and helpers
├── public/
│   ├── assets/            # Static game assets (images, icons)
│   └── models/            # 3D models for Three.js
```

### State Management

- **No local game state**: Backend is source of truth
- **Real-time sync**: All state changes via WebSocket events
- **Unidirectional flow**: Server → WebSocket → React state → UI
- **localStorage**: Game/player ID persistence for reconnection

#### Zustand stores

- `gameStore` — server game data (`game`, `currentPlayer`, `chatMessages`, etc.). No phase fields; this is pure domain data.
- `appPhaseStore` — **the single source of truth for "what screen is the user on"**. See "App Phase State Machine" below.
- `uiOverlayStore` — booleans for in-game overlays (modals, selection popovers).
- `cardPlayFlowStore` — state for the card-play interaction flow.
- `spectateStore` — current spectated player.

#### App Phase State Machine

**`useAppPhaseStore`** (`src/stores/appPhaseStore.ts`) holds a discriminated-union `AppPhase` that names every screen the user can be on. **Never derive "what screen is this" from a combination of booleans across stores** — always read `phase.kind` from `useAppPhaseStore`.

```ts
type AppPhase =
  | { kind: "menu"; route: "landing" | "create" | "join" | "cards" | "reconnecting" }
  | { kind: "checking" | "connecting" | "selecting" | "joining" | "spectating"; gameId: string }
  | { kind: "lobby" | "loading" | "fadeOutLobby" | "marsRevealed" | "showcase" | "animateUI" | "playing" | "completed"; gameId: string };
```

The legal transitions form a state machine driven from two places only:

- `App.jsx` — sets `{ kind: "menu", route }` from `location.pathname` on menu routes.
- `useGameInitialization` — sets `checking → connecting → selecting/joining/spectating` while bootstrapping the game from URL / route state / saved session.
- `useGameTransitions` — drives every other transition based on server-side `gameStatus` / `gamePhase` and the `isSkyboxReady` / `isGpuReady` flags.

**No other code writes the phase.** Components read `phase.kind` and selector helpers (`showsSpaceBackground`, `showsMenuChrome`, `isInGameWorld`, `gameIdOf`).

Selector helpers live next to the type in `appPhaseStore.ts`, so adding a phase forces you to update every projection (TypeScript exhaustiveness).

#### State machine constraints (do not break)

1. **Phase writes are one-way.** Components must NOT call `setPhase`; only `useGameInitialization` and `useGameTransitions` may. If you need to drive a transition, add it to `useGameTransitions` as a `useEffect` keyed on the relevant inputs.
2. **Reset world-ready flags between games.** `isSkyboxReady`, `isGpuReady`, `marsRevealedReady` are cleared in `setInitPhase("checking", ...)`. If you add new ready-flags, reset them there too — sticky flags across games cause the next game to skip the loading transition entirely.
3. **Animation timers are explicit.** The `loading → fadeOutLobby → marsRevealed → animateUI → playing` chain is driven by fixed timers (1000 / 1500 / 2500 ms) anchored to phase entry. When the server enters `init_apply_corp`/`init_apply_prelude`, `marsRevealed` (or a reconnect) goes to `showcase` instead; `showcase → animateUI` fires when the server leaves the init phases. During `showcase` the HUD is reduced to the hamburger and the left player list, and `CorporationShowcaseOverlay` presents each player's corp, preludes and the final roster. Cleaning them up on phase-kind change is mandatory (each effect's `return () => clearTimeout(...)` is load-bearing — a stale timer firing after a phase change will skip states).
4. **Never gate visibility on "x ≠ idle" or composite booleans.** Use `phase.kind === "playing"`, `isInGameWorld(phase)`, etc. The discriminated union gives you exhaustiveness; ad-hoc booleans don't.

#### Persistent 3D background

`<SpaceBackground>` is mounted **exactly once** at the App root (`AppWithBackground` in `src/App.jsx`) and never unmounts. Visibility is controlled by an opacity wrapper driven by `showsSpaceBackground(phase)` with a 1500ms transition (matches the `fadeOutLobby` animation). This avoids the R3F `<Canvas>` tear-down / rebuild + skybox-reload on every route change. Two consequences:

- The outer wrapper of `GameLayout` MUST stay transparent. The black background of the in-game world goes on the **inner** content div, gated by `phase.kind !== "lobby"`. Adding `bg-*` to the outer `GameLayout` div hides `<SpaceBackground>` during the lobby — don't.
- The skybox cache (`SkyboxCache.ts`) is a singleton; once loaded it stays in GPU memory across phases. `MainContentDisplay`'s separate `<SkyboxLoader>` gets it instantly from the cache when the in-game canvas mounts.

Low graphics tier exception (`GRAPHICS.tier === "low"` from `utils/graphicsQuality.ts`, picked automatically on phones): `<SpaceBackground>` unmounts its `<Canvas>` while `isInGameWorld(phase)` is true, so the menu scene's WebGL context and textures are released during play. The wrapper stays mounted and the canvas is rebuilt when the player returns to the menus. The low tier also replaces the EXR skybox with `ProceduralStarfield`, so there is nothing to reload.

#### LoadingOverlay (useSpinDelay semantics)

`LoadingOverlay` (`src/components/game/view/LoadingOverlay.tsx`) follows the `useSpinDelay` pattern:

- `showDelayMs` (default 500ms) — wait before showing the spinner; if the load completes before, skip the spinner entirely (no flash on fast loads).
- `minDurationMs` (default 200ms) — once shown, stay visible at least this long (no jank on medium loads).

Two call sites override the defaults for guaranteed-visible feedback:

- App-level menu overlay: `showDelayMs={0}` + `minDurationMs={500}` — cold app boot always shows the spinner.
- In-game overlay: `showDelayMs={0}` + `minDurationMs={200}` — every `loading` phase entry shows the spinner. The one exception is arriving at `/game` from create/join with the game in route state: before the lobby, the delay is 600ms, so the quick re-check finishes behind the still-visible space background and no loading screen appears.

Both of those also pass `showProgress`, which replaces the spinner with a percentage and bar, and drops the `message` title in favour of one small detail line: the `subtitle`, else `message`, else what three.js is loading right now ("Loading starfield", "Loading Mars" …). The value is the larger of three.js loader progress (drei `useProgress`, scaled to 90%) and a time-based estimate that keeps creeping toward 95%. It never goes backwards and holds at 99% until `isLoaded`.

## Key Development Patterns

### Code Style

- **CRITICAL**: NO unnecessary comments - code should be self-documenting. Avoid logic comments that just restate what the code does. Only add comments when truly necessary (complex algorithms, non-obvious business rules).
- **CRITICAL**: Always use braces for `if` statements. No single-line or bracketless `if` blocks:
  ```tsx
  // ✅ CORRECT
  if (something) {
    doThing();
  }

  // ❌ WRONG
  if (something) doThing();
  if (something)
    doThing();
  ```
- **CRITICAL**: NEVER nest ternary operators. Break them into separate variables or use `if`/`else`:
  ```tsx
  // ✅ CORRECT
  const order = hasA ? a : b;
  const result = order ? transform(order) : fallback;

  // ❌ WRONG
  const result = hasA ? a : hasB ? b : fallback;
  ```

### Z-Index Policy

**CRITICAL**: NEVER hardcode z-index values. Always use the centralized `Z_INDEX` constants from `@/constants/zIndex.ts`.

```tsx
// ✅ CORRECT
import { Z_INDEX } from "@/constants/zIndex.ts";

<div style={{ zIndex: Z_INDEX.STANDARD_MODAL }}>...</div>

// ❌ WRONG — hardcoded Tailwind z-index
<div className="z-[1000]">...</div>
<div className="z-50">...</div>

// ❌ WRONG — hardcoded inline z-index
<div style={{ zIndex: 99999 }}>...</div>
```

The `Z_INDEX` constants define layering groups (base, UI, navigation, overlay, modal, critical). Use `getZIndex(level, offset)` when you need a small offset from a defined level. Add new constants to `zIndex.ts` if no existing one fits — never invent magic numbers inline.

### Cursor Policy

**CRITICAL**: Only use these three cursor types:
- `cursor-default` — default arrow cursor
- `cursor-pointer` — clickable/interactive elements
- `cursor-text` — text inputs

NEVER use `cursor-not-allowed`, `cursor-grab`, `cursor-wait`, or any other custom cursor types.

### Component Development

1. **Inspect existing design language** before creating new components
2. **Reuse over creation**: Check for existing components first
3. **No emojis in UI**: Use GameIcon component or assets instead
4. **No browser tooltips**: NEVER use `title` attributes on HTML elements for hover tooltips. The UI should be self-explanatory without them.
5. Use `void <function>()` to explicitly discard promises in event handlers

### Displaying Resources, Costs, and Gains

**CRITICAL**: ALWAYS reuse existing components for displaying game resources.

**GameIcon** - Single icon with optional value:

```tsx
<GameIcon iconType="credits" amount={25} size="medium" />
<GameIcon iconType="energy-production" amount={3} size="medium" />  // Auto brown background
```

Sizes: 'small' (24px), 'medium' (32px), 'large' (40px). Add new icons to `utils/` icon store.

**BehaviorSection** - Card behavior layouts:

```tsx
<BehaviorSection behavior={cardBehavior} />
```

Use for any display with multiple resources, costs/gains, or input/output relationships. See `components/ui/cards/BehaviorSection/CLAUDE.md` for details.

### Styling with Tailwind CSS v4

**CRITICAL**: Uses Tailwind CSS v4 with CSS-based configuration.

- **Configuration**: `@theme {}` block in main CSS file
- **NO CSS Modules**: NEVER create `.module.css` files
- **NO JavaScript Config**: `tailwind.config.js` is ignored

Custom theme values (colors, fonts, shadows) defined in `@theme {}` block.

### Typography & Fonts

The project uses two font styles:

**Game Font (Orbitron)** - `font-orbitron`
- Use for: titles, headers, numbers, labels, button text, UI elements
- Futuristic, bold, sci-fi aesthetic
- Available weights: 400-900
- Example: `className="font-orbitron font-bold"`

**Default Font (System)**
- Use for: long descriptions, card text, tooltips, explanatory content
- More readable for extended text
- Default if no font class specified

**Guidelines:**
- Numbers and statistics: Always use `font-orbitron`
- Short labels (1-3 words): Use `font-orbitron`
- Descriptions (sentences/paragraphs): Use default font
- Interactive elements (buttons): Use `font-orbitron`

### 3D Rendering

Uses React Three Fiber for Three.js integration. Hex coordinates use cube system (q, r, s) where q + r + s = 0, with utilities in `utils/`.

**Controls**: Custom pan/zoom (no orbital rotation), parallax background for depth.

### Mobile / compact layout

Phones get a separate "compact" layout. Keep it working whenever you touch UI code.

- **One switch**: `useLayoutMode()` (`hooks/useLayoutMode.ts`) is the only source of truth; non-React code uses `getLayoutMode()`. Compact = `(pointer: coarse)` and the physical screen's short side < 600px (`screen.width/height`; mobile browsers inflate `innerWidth` when zoomed out or when content overflows), or the `?compact` URL flag. Never read `window.innerWidth` to choose a layout (clamping/positioning math is fine). Tablets keep the desktop layout.
- **Landscape only**: on a phone in portrait, `RotateDeviceOverlay` covers every page (menus, lobby and game); the manifest asks installed apps for landscape too. Design every phone screen for landscape only.
- **Separate shell**: in compact mode `GameInterface` renders `MobileGameLayout` (`components/mobile/`) instead of `GameLayout`. Desktop HUD components (`TopMenuBar`, `BottomResourceBar`, `CardFanOverlay`, `LeftSidebar`, `RightSidebar`) get **no** compact branches. Shared overlays and prompts (selection overlays, `GameFlowPopover`, `CardInspection`, `ProductionPhaseModal`, endgame) get a compact presentation.
- **Mobile building blocks**: `MobileScreen` (full-screen view opened from the dock; pauses the 3D canvas), `MobileCardDetail` (one card in inspection presentation with optional notice/actions column; `CardDetailPortal` mounts it from shared components), `mobileUiStore` (open screen and its params), `useMobileGame()` (game data and handlers for screens).
- **Styling**: use the `compact:` Tailwind variant (matches `<html data-layout="compact">`) instead of width breakpoints like `max-[768px]:`, which miss landscape phones. Use `useLayoutMode()` only when behavior differs. Position anything over the board with `--hud-top-h`, `--hud-dock-h`, `--hud-rail-left-w`, `--hud-rail-right-w` (0px on desktop) and `--safe-top|right|bottom|left`.
- **No hover, right-click or keyboard-only features**: hover info goes through `RevealTrigger` / `useTapReveal` (tap to reveal), context menus through `useLongPress` plus a visible button, hotkeys need an on-screen control.
- **Graphics tier**: `GRAPHICS` from `utils/graphicsQuality.ts` is `low` on phones (override with `?gfx=low|high`); low tier means smaller textures, fewer particles and no EXR skybox. Pause rendering with `useRenderPause(reason, active)` from `stores/renderPauseStore.ts` while something covers the canvas.
- **Sizes**: touch targets at least 44×44px, text inputs at least 16px (prevents iOS focus zoom), text at least 11px, and `dvh` for anything viewport-tall (never `vh` or `h-screen`).
- **Full-screen forms and dialogs**: substantial menu forms and dialogs on phones must use a full-screen compact shell with a fixed top bar, safe-area padding, one scrolling content region, and reachable actions. Never shrink or vertically center a desktop modal on a phone. Account for the software keyboard when inputs are present.
- **Visual verification for every UI change**: inspect desktop first, then iPhone and Android landscape layouts. Check initial, loading, error, expanded, long-content, and keyboard-open states as applicable; confirm controls are visible or reachable by scrolling and nothing is clipped or hidden under fixed chrome. Automated assertions supplement actual screenshot inspection. Preserve the portrait rotation overlay.
- **Testing**: append `?compact` to any URL, or use DevTools device emulation in landscape. `tests/mobile-*.spec.ts` check this automatically (see "Testing with Playwright").
- **Install bar**: `InstallAppBar` offers "Play full screen as an app" on the compact landing page 2s after load; "Not now" hides it for 30 days (`openmars.installBar.dismissedUntil` in `localStorage`). `utils/installApp.ts` keeps Chromium's `beforeinstallprompt` (registered from `index.jsx`) and picks the install path: the real prompt, or the how-to sheet (`InstallHowToSheet`) on iOS and Firefox Android. The menus show "Install app" whenever installing is possible.
- **Testing install**: Chrome only offers install on HTTPS or `localhost`. On an Android phone, run `adb reverse tcp:3000 tcp:3000` and open `http://localhost:3000`, or allow the dev origin with `chrome://flags/#unsafely-treat-insecure-origin-as-secure`. iOS Add to Home Screen works over plain HTTP. Clear site data to bring a dismissed bar back.


### Sound System

```tsx
import { useSoundEffects } from '../hooks/useSoundEffects';

const { playSound, playProductionSound } = useSoundEffects();
void playProductionSound();
```

Add new sounds to `public/assets/audio/` and register in the audio service preload list.

### Notifications

Use `useNotifications()` from `contexts/NotificationContext.tsx` for error and warning messages. Notifications are displayed by `NotificationContainer` at the bottom-left of the screen (top centre on phones).

- **Only shown outside game pages**: `NotificationContainer` returns `null` on `/game/*` routes, so notifications fired during gameplay are silently ignored. Use in-game UI (modals, overlays) for in-game feedback instead.
- **Types**: `"error"` (red), `"warning"` (yellow) and `"info"` (dark panel).
- **Auto-dismiss**: Default 3000ms. Pass `duration: 0` for persistent notifications that require manual dismiss (e.g. "Server is down").

```tsx
const { showNotification } = useNotifications();

showNotification({ message: "Name too short", type: "error" });
showNotification({ message: "Server is down", type: "error", duration: 0 });
```

### Updates and changelog

`useVersionCheck` (called in `App.jsx`) compares `APP_VERSION` with the server's `/api/v1/meta` version on load, every 5 minutes, when the tab becomes visible and after WebSocket reconnects. When they differ, `UpdatePill` offers What's new and Update (`location.reload()`, which also works in the installed app because there is no service worker), and `MenuPopoverVersion` shows "Update to vX". `ChangelogModal` shows the player notes (`changelog/<tag>/CHANGELOG-USER.md`) from `GET /api/v1/changelog`, opened from the Changelog menu item or once after the client was updated. It is a wide `GameMenuModal` on desktop and a full-screen view with a Changelog top bar on phones. State lives in `versionStore`; `localbuild` never counts as out of date.

### WebSocket Communication

WebSocket service in `services/` handles real-time game state sync.

**Outbound**: `join-game`, `player-reconnect`, `select-corporation`, `skip-action`, `start-game`
**Inbound**: `game-updated`, `player-connected`, `player-reconnected`, `player-disconnected`

## Testing & Pre-Commit Gate

**CRITICAL**: There is NO frontend unit-test runner — there is no `test` recipe or
script. The authoritative gate for any frontend-only change is, both passing:

```bash
just frontend check   # format-check, oxlint, typecheck, asset checks
just frontend build
```

Do NOT look for or invent a `bun run test` / `vitest` / `jest` command. The end-to-end
"does it actually work" check is a **human visual smoke** (or Playwright MCP), not an
automated assertion — call it out explicitly at sign-off.

## Testing with Playwright

Use Playwright MCP tools for live debugging:

- `mcp__playwright__browser_navigate`: Navigate to URLs
- `mcp__playwright__browser_snapshot`: Capture page state
- `mcp__playwright__browser_click`: Click elements
- `mcp__playwright__browser_take_screenshot`: Capture visuals

**Always add `?quick` to the URL** when opening the app in a browser to inspect it (e.g. `http://localhost:3000/?quick`, `http://localhost:3000/game/abc?quick`). Quick mode skips the menu's 3D background, the skybox and the GPU warmup, and loads low-res planet textures, so pages load much faster. The param stays in the URL on its own as you navigate in the app (`src/utils/quickMode.ts`). Leave it out only when the task is about how those visuals look or perform.

### Playwright test suite

`tests/` holds Playwright specs (`playwright.config.ts`). They are type-checked, linted and formatted by the normal gate but are not part of it; run them explicitly:

```bash
just frontend e2e                          # mobile Chromium projects
E2E_BACKEND=1 just frontend e2e            # also the in-game spec (backend on :3001)
```

- Projects: `firefox` (desktop), `mobile-landscape-ios` (iPhone 14 landscape size), `mobile-landscape-android` (Pixel 7 landscape), `mobile-portrait` (iPhone 14 portrait; checks that the rotate overlay covers the menus). The mobile projects run on Chromium with the device's viewport, scale factor, `isMobile` and `hasTouch` (no WebKit).
- Playwright starts its own Vite server on port 3100 (`E2E_PORT` to change it, `E2E_REUSE_SERVER=1` to reuse one already running there); it never touches the dev server on 3000. The bundled browser revision must match `@playwright/test` (pinned in `package.json`).
- `mobile-menus.spec.ts`: no horizontal overflow, touch targets ≥ 44×32px, inputs ≥ 16px, fixed menu chrome never permanently covers a button, `<html data-layout>` matches the device. Routes other than `/cards` skip when the Go backend isn't reachable.
- `mobile-game.spec.ts`: creates an Earth Withdrawal solo match without Preludes, plays through starting selection and checks the compact HUD (no overlap, nothing outside the viewport, scenario status and rules, every dock screen opens and closes, rotate overlay in portrait). Skipped unless `E2E_BACKEND=1` and the backend runs on port 3001.

## Important Notes

### State Management Rules

**CRITICAL**: No timeouts or arbitrary delays for **server state synchronization**. Game data must come from the backend via WebSocket — never fake it with a `setTimeout` waiting for a response.

This does NOT forbid timers in the **app-phase state machine**. The lobby → game-world transition (`loading → fadeOutLobby → marsRevealed → animateUI → playing`) is intentionally driven by fixed-duration timers anchored to phase entry, because those are animation durations, not server-state waits. See "App Phase State Machine" above.

Rule of thumb: a timer that controls how long something is *displayed* is fine; a timer that *replaces* a server event is a bug.

### Design Principles

- **No emojis in UI**: Use GameIcon or assets
- **GameIcon first**: Never use direct `<img>` tags for game icons
- **Tailwind CSS only**: No CSS Modules
- **Type safety**: Always use generated types from backend

### Menu and Modal Components

**CRITICAL**: Reuse existing components for main menu and game modals.

**GameButton** - All buttons across the app:

```tsx
import GameButton from "../buttons/GameButton.tsx";

<GameButton size="lg" onClick={handleClick}>START GAME</GameButton>
<GameButton buttonType="secondary" size="sm">Cancel</GameButton>
<GameButton variant="success" size="sm">Confirm</GameButton>
<GameButton buttonType="textonly" size="sm">Hide</GameButton>
<GameButton variant="error">Kick</GameButton>
```

Types: `primary` (default, filled accent), `secondary` (dark bg, accent border), `textonly` (no bg/border).
Variants: `info` (default, blue), `success` (green), `warn` (yellow), `error` (red).
Sizes: `sm`, `md`, `lg`.

**BackButton** - Navigation back buttons:

```tsx
import BackButton from "../buttons/BackButton.tsx";
<BackButton onClick={handleBack} />
```

**GameMenuModal** - All main menu modals and overlays:

```tsx
import GameMenuModal from "./GameMenuModal.tsx";

<GameMenuModal
  title="Modal Title"
  subtitle="Optional subtitle"
  onBack={() => navigate("/")}
  visible={isVisible}
>
  {/* Modal content */}
</GameMenuModal>
```

Provides consistent styling, animations, Back button (top-left), and Settings button (top-right).

## Related Documentation

- **Root CLAUDE.md**: Project overview and commands
- **backend/CLAUDE.md**: Backend architecture and event system
- **backend/assets/CLAUDE.md**: Card database documentation
