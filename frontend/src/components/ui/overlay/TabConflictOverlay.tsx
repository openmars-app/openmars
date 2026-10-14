import GameButton from "@/components/ui/buttons/GameButton.tsx";
import GameMenuModal from "./GameMenuModal.tsx";
import { Z_INDEX } from "@/constants/zIndex.ts";

interface TabConflictOverlayProps {
  activeGameInfo: { gameId: string; playerName: string };
  onTakeOver: () => void;
  onCancel: () => void;
}

export default function TabConflictOverlay({
  activeGameInfo,
  onTakeOver,
  onCancel,
}: TabConflictOverlayProps) {
  return (
    <GameMenuModal
      title="Game Already Active"
      showBackdrop
      zIndex={Z_INDEX.ERROR_OVERLAYS}
      footer={
        <div className="flex justify-end gap-3 pt-5 compact:pt-3">
          <GameButton emphasis="secondary" onClick={onCancel}>
            Cancel
          </GameButton>
          <GameButton onClick={onTakeOver}>Continue Here</GameButton>
        </div>
      }
    >
      <div className="space-y-4 text-sm text-white/80">
        <p>A game is already running in another tab or window.</p>
        <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-3 border border-white/15 bg-white/[0.03] p-4">
          <dt className="text-white/50">Player</dt>
          <dd className="min-w-0 text-right break-words">{activeGameInfo.playerName}</dd>
          <dt className="text-white/50">Game ID</dt>
          <dd className="min-w-0 text-right break-all font-mono">{activeGameInfo.gameId}</dd>
        </dl>
        <p>Continuing here will close the game in the other tab.</p>
      </div>
    </GameMenuModal>
  );
}
