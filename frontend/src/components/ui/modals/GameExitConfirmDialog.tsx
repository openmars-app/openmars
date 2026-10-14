import { Z_INDEX } from "@/constants/zIndex";
import GameMenuModal from "../overlay/GameMenuModal";
import GameButton from "../buttons/GameButton";

export default function GameExitConfirmDialog({
  kind,
  onCancel,
  onConfirm,
}: {
  kind: "leave" | "close" | "end";
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const label = { leave: "Leave", close: "Close", end: "End game" }[kind];
  return (
    <GameMenuModal
      title={kind === "end" ? "End game?" : `${label} game?`}
      showBackdrop
      onClose={onCancel}
      zIndex={Z_INDEX.CONFIRMATION_MODAL}
    >
      <p className="text-white/80 text-center mb-6">
        {kind === "end"
          ? "This will end the game for all players. This action cannot be undone."
          : "You can reconnect to the game again without losing any progress."}
      </p>
      <div className="flex gap-4 justify-center">
        <GameButton emphasis="secondary" onClick={onCancel}>
          Cancel
        </GameButton>
        <GameButton tone="error" onClick={onConfirm}>
          {label}
        </GameButton>
      </div>
    </GameMenuModal>
  );
}
