import { cardPreviews } from "./cardPreviews";
import { colonyPreviews } from "./colonyPreviews";
import { sessionPreviews } from "./sessionPreviews";
import type { PreviewDefinition } from "./types";

import GameExitConfirmDialog from "../../modals/GameExitConfirmDialog";
import { createElement } from "react";

export const previews: PreviewDefinition[] = [
  ...cardPreviews,
  ...colonyPreviews,
  ...sessionPreviews,
  {
    id: "game-exit",
    name: "Leave, close or end game",
    category: "Session",
    componentPaths: ["components/ui/modals/GameExitConfirmDialog.tsx"],
    presets: ["leave", "close", "end"].map((id) => ({
      id,
      name: `${id[0].toUpperCase()}${id.slice(1)} game`,
    })),
    Component: ({ presetId, onComplete }) =>
      createElement(GameExitConfirmDialog, {
        kind: presetId as "leave" | "close" | "end",
        onCancel: () => onComplete("Cancelled"),
        onConfirm: () => onComplete(`${presetId} game confirmed`),
      }),
  },
];
