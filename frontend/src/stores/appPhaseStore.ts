import { create } from "zustand";

export type MenuRoute =
  | "landing"
  | "load"
  | "resume"
  | "create"
  | "join"
  | "cards"
  | "reconnecting";

export type AppPhase =
  | { kind: "menu"; route: MenuRoute }
  | { kind: "checking"; gameId: string }
  | { kind: "connecting"; gameId: string }
  | { kind: "selecting"; gameId: string }
  | { kind: "joining"; gameId: string }
  | { kind: "spectating"; gameId: string }
  | { kind: "lobby"; gameId: string }
  | { kind: "loading"; gameId: string }
  | { kind: "fadeOutLobby"; gameId: string }
  | { kind: "marsRevealed"; gameId: string }
  | { kind: "showcase"; gameId: string }
  | { kind: "animateUI"; gameId: string }
  | { kind: "playing"; gameId: string }
  | { kind: "completed"; gameId: string };

export type AppPhaseKind = AppPhase["kind"];

interface AppPhaseState {
  phase: AppPhase;
  marsRevealedReady: boolean;
  lobbyMounted: boolean;

  setPhase: (next: AppPhase) => void;
  setMenuRoute: (route: MenuRoute) => void;
  setMarsRevealedReady: (ready: boolean) => void;
  setLobbyMounted: (mounted: boolean) => void;
  resetToMenu: (route?: MenuRoute) => void;
}

export const useAppPhaseStore = create<AppPhaseState>((set) => ({
  phase: { kind: "menu", route: "landing" },
  marsRevealedReady: false,
  lobbyMounted: false,

  setPhase: (phase) => set({ phase }),
  setMenuRoute: (route) =>
    set((s) => (s.phase.kind === "menu" ? { phase: { kind: "menu", route } } : s)),
  setMarsRevealedReady: (marsRevealedReady) => set({ marsRevealedReady }),
  setLobbyMounted: (lobbyMounted) => set({ lobbyMounted }),
  resetToMenu: (route = "landing") =>
    set({
      phase: { kind: "menu", route },
      marsRevealedReady: false,
      lobbyMounted: false,
    }),
}));

export const gameIdOf = (p: AppPhase): string | null => ("gameId" in p ? p.gameId : null);

export const isMenu = (p: AppPhase): boolean => p.kind === "menu";

export const isInitializing = (p: AppPhase): boolean =>
  p.kind === "checking" ||
  p.kind === "connecting" ||
  p.kind === "selecting" ||
  p.kind === "joining" ||
  p.kind === "spectating";

export const isLobby = (p: AppPhase): boolean => p.kind === "lobby";

export const isStartingTransition = (p: AppPhase): boolean =>
  p.kind === "loading" || p.kind === "fadeOutLobby" || p.kind === "marsRevealed";

export const isInGameWorld = (p: AppPhase): boolean =>
  p.kind === "marsRevealed" ||
  p.kind === "showcase" ||
  p.kind === "animateUI" ||
  p.kind === "playing" ||
  p.kind === "completed";

export const needsGameScene = (p: AppPhase): boolean =>
  p.kind === "loading" || p.kind === "fadeOutLobby" || isInGameWorld(p);

export const showsMenuBackground = (p: AppPhase): boolean =>
  (p.kind === "menu" && p.route !== "cards") ||
  p.kind === "checking" ||
  p.kind === "connecting" ||
  p.kind === "selecting" ||
  p.kind === "joining" ||
  p.kind === "spectating" ||
  p.kind === "lobby" ||
  p.kind === "loading";

export const showsMenuChrome = (p: AppPhase): boolean => p.kind === "menu";
