import { webSocketService } from "@/services/webSocketService.ts";
import React from "react";
import { GameDto } from "@/types/generated/api-types.ts";
import { GamePopover } from "../GamePopover";
import { MilestoneListView } from "./content/MilestoneList.tsx";

interface MilestonePopoverProps {
  isVisible: boolean;
  onClose: () => void;
  gameState?: GameDto;
  anchorRef: React.RefObject<HTMLButtonElement | null>;
}

const MilestonePopover: React.FC<MilestonePopoverProps> = ({
  isVisible,
  onClose,
  gameState,
  anchorRef,
}) => {
  return (
    <MilestonePopoverView
      isVisible={isVisible}
      onClose={onClose}
      gameState={gameState}
      anchorRef={anchorRef}
      onClaimMilestone={(id) => {
        void webSocketService.claimMilestone(id);
      }}
    />
  );
};

export const MilestonePopoverView: React.FC<
  MilestonePopoverProps & { onClaimMilestone: (id: string) => void }
> = ({ isVisible, onClose, gameState, anchorRef, onClaimMilestone }) => {
  return (
    <GamePopover
      isVisible={isVisible}
      onClose={onClose}
      position={{ type: "anchor", anchorRef, placement: "below" }}
      theme="colonies"
      excludeRef={anchorRef}
      header={undefined}
      width={500}
      maxHeight="80dvh"
      animation="slideDown"
      className="game-popover-list"
    >
      <MilestoneListView
        gameState={gameState}
        density="popover"
        onClaimMilestone={onClaimMilestone}
      />
    </GamePopover>
  );
};

export default MilestonePopover;
