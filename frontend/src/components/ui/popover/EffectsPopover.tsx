import React from "react";
import { PlayerEffectDto } from "../../../types/generated/api-types.ts";
import { GamePopover } from "../GamePopover";
import EffectsList from "./content/EffectsList.tsx";

interface EffectsPopoverProps {
  isVisible: boolean;
  onClose: () => void;
  effects: PlayerEffectDto[];
  playerName?: string;
  anchorRef: React.RefObject<HTMLElement | null>;
}

const EffectsPopover: React.FC<EffectsPopoverProps> = ({
  isVisible,
  onClose,
  effects,
  anchorRef,
}) => {
  return (
    <GamePopover
      className="game-popover-list"
      isVisible={isVisible}
      onClose={onClose}
      position={{ type: "anchor", anchorRef, placement: "above" }}
      theme="effects"
      header={{
        title: "Card Effects",
        badge: `${effects.length} active`,
      }}
      arrow={{ enabled: true, position: "right", offset: 30 }}
      width={320}
      maxHeight={400}
    >
      <EffectsList effects={effects} density="popover" />
    </GamePopover>
  );
};

export default EffectsPopover;
