import { webSocketService } from "@/services/webSocketService.ts";
import React from "react";
import { GameDto } from "@/types/generated/api-types.ts";
import { GamePopover } from "../GamePopover";
import { AwardListView } from "./content/AwardList.tsx";

interface AwardPopoverProps {
  isVisible: boolean;
  onClose: () => void;
  gameState?: GameDto;
  anchorRef: React.RefObject<HTMLButtonElement | null>;
}

const AwardPopover: React.FC<AwardPopoverProps> = ({
  isVisible,
  onClose,
  gameState,
  anchorRef,
}) => {
  return (
    <AwardPopoverView
      isVisible={isVisible}
      onClose={onClose}
      gameState={gameState}
      anchorRef={anchorRef}
      onFundAward={(id) => {
        void webSocketService.fundAward(id);
      }}
    />
  );
};

export const AwardPopoverView: React.FC<
  AwardPopoverProps & { onFundAward: (id: string) => void }
> = ({ isVisible, onClose, gameState, anchorRef, onFundAward }) => {
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
      <AwardListView gameState={gameState} density="popover" onFundAward={onFundAward} />
    </GamePopover>
  );
};

export default AwardPopover;
