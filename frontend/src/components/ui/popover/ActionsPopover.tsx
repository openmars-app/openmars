import React from "react";
import { PlayerActionDto, GameDto } from "../../../types/generated/api-types.ts";
import { GamePopover } from "../GamePopover";
import ActionsList from "./content/ActionsList.tsx";

interface ActionsPopoverProps {
  isVisible: boolean;
  onClose: () => void;
  actions: PlayerActionDto[];
  playerName?: string;
  onActionSelect?: (action: PlayerActionDto) => void;
  anchorRef: React.RefObject<HTMLElement | null>;
  gameState?: GameDto;
}

const ActionsPopover: React.FC<ActionsPopoverProps> = ({
  isVisible,
  onClose,
  actions,
  onActionSelect,
  anchorRef,
  gameState,
}) => {
  const handleActionClick = (action: PlayerActionDto) => {
    if (onActionSelect) {
      onActionSelect(action);
      onClose();
    }
  };

  return (
    <GamePopover
      className="game-popover-list"
      isVisible={isVisible}
      onClose={onClose}
      position={{ type: "anchor", anchorRef, placement: "above" }}
      theme="actions"
      header={{
        title: "Card Actions",
        badge: `${actions.filter((a) => a.available).length} available`,
      }}
      arrow={{ enabled: true, position: "right", offset: 30 }}
      width={320}
      maxHeight={400}
    >
      <ActionsList
        actions={actions}
        gameState={gameState}
        onActionSelect={handleActionClick}
        density="popover"
      />
    </GamePopover>
  );
};

export default ActionsPopover;
