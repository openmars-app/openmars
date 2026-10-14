import React from "react";
import { VPGranterDto } from "@/types/generated/api-types.ts";
import { GamePopover } from "../GamePopover";
import VictoryPointsBreakdown from "./content/VictoryPointsBreakdown.tsx";

interface VictoryPointsPopoverProps {
  isVisible: boolean;
  onClose: () => void;
  vpGranters: VPGranterDto[];
  totalVP: number;
  anchorRef: React.RefObject<HTMLElement | null>;
}

const VictoryPointsPopover: React.FC<VictoryPointsPopoverProps> = ({
  isVisible,
  onClose,
  vpGranters,
  totalVP,
  anchorRef,
}) => {
  return (
    <GamePopover
      className="game-popover-list"
      isVisible={isVisible}
      onClose={onClose}
      position={{ type: "anchor", anchorRef, placement: "above" }}
      theme="victoryPoints"
      header={{
        title: `${totalVP} VP`,
        badge: `${vpGranters.length} source${vpGranters.length !== 1 ? "s" : ""}`,
      }}
      width={320}
      maxHeight={400}
    >
      <VictoryPointsBreakdown vpGranters={vpGranters} density="popover" />
    </GamePopover>
  );
};

export default VictoryPointsPopover;
