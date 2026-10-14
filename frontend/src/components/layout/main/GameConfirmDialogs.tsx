import { Z_INDEX } from "@/constants/zIndex.ts";
import { globalWebSocketManager } from "@/services/globalWebSocketManager.ts";
import { useConfirmDialogStore, type ConfirmDialogRequest } from "@/stores/confirmDialogStore.ts";
import GameMenuModal from "../../ui/overlay/GameMenuModal.tsx";
import GameButton from "../../ui/buttons/GameButton.tsx";

async function runConfirmedAction(request: ConfirmDialogRequest) {
  try {
    if (request.kind === "kick") {
      await globalWebSocketManager.kickPlayer(request.playerId);
    } else {
      await globalWebSocketManager.convertToBot(request.playerId);
    }
  } catch (error) {
    console.error("Failed to execute action:", error);
  }
}

export default function GameConfirmDialogs() {
  const pending = useConfirmDialogStore((s) => s.pending);
  const dismiss = useConfirmDialogStore((s) => s.dismiss);

  if (!pending) {
    return null;
  }

  const handleConfirm = () => {
    dismiss();
    void runConfirmedAction(pending);
  };

  return <GameConfirmDialogView pending={pending} dismiss={dismiss} onConfirm={handleConfirm} />;
}

export function GameConfirmDialogView({
  pending,
  dismiss,
  onConfirm,
}: {
  pending: ConfirmDialogRequest;
  dismiss: () => void;
  onConfirm: () => void;
}) {
  if (pending.kind === "kick") {
    return (
      <GameMenuModal
        title="Kick player?"
        showBackdrop={true}
        onClose={dismiss}
        zIndex={Z_INDEX.CONFIRMATION_MODAL}
      >
        <p className="text-white/80 text-center mb-6">
          <span className="font-bold text-white">{pending.playerName}</span> will be removed from
          the game and cannot rejoin.
        </p>
        <div className="flex gap-4 justify-center">
          <GameButton emphasis="secondary" onClick={dismiss}>
            Cancel
          </GameButton>
          <GameButton tone="error" onClick={onConfirm}>
            Kick
          </GameButton>
        </div>
      </GameMenuModal>
    );
  }

  return (
    <GameMenuModal
      title="Convert to bot?"
      showBackdrop={true}
      onClose={dismiss}
      zIndex={Z_INDEX.CONFIRMATION_MODAL}
    >
      <p className="text-white/80 text-center mb-6">
        <span className="font-bold text-white">{pending.playerName}</span> will be replaced by a
        bot. This cannot be undone.
      </p>
      <div className="flex gap-4 justify-center">
        <GameButton emphasis="secondary" onClick={dismiss}>
          Cancel
        </GameButton>
        <GameButton tone="error" onClick={onConfirm}>
          Convert
        </GameButton>
      </div>
    </GameMenuModal>
  );
}
