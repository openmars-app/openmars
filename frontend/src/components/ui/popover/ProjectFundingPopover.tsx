import React from "react";
import { GameDto } from "@/types/generated/api-types.ts";
import { GamePopover } from "../GamePopover";
import ProjectFundingList, {
  ProjectSeatPaymentPicker,
  useProjectSeatPurchase,
  type ProjectSeatPurchase,
} from "./content/ProjectFundingList.tsx";

interface ProjectFundingPopoverProps {
  isVisible: boolean;
  onClose: () => void;
  gameState?: GameDto;
  anchorRef: React.RefObject<HTMLButtonElement | null>;
}

const ProjectFundingPopover: React.FC<ProjectFundingPopoverProps> = ({
  isVisible,
  onClose,
  gameState,
  anchorRef,
}) => {
  const purchase = useProjectSeatPurchase(gameState);

  return (
    <ProjectFundingPopoverView
      isVisible={isVisible}
      onClose={onClose}
      gameState={gameState}
      anchorRef={anchorRef}
      purchase={purchase}
    />
  );
};

export const ProjectFundingPopoverView: React.FC<
  ProjectFundingPopoverProps & { purchase: ProjectSeatPurchase }
> = ({ isVisible, onClose, gameState, anchorRef, purchase }) => {
  return (
    <>
      <GamePopover
        isVisible={isVisible}
        onClose={onClose}
        position={{ type: "anchor", anchorRef, placement: "below" }}
        theme="colonies"
        excludeRef={anchorRef}
        header={undefined}
        width={560}
        maxHeight="80dvh"
        animation="slideDown"
        className="game-popover-list !bg-space-black-darker"
      >
        <ProjectFundingList gameState={gameState} purchase={purchase} density="popover" />
      </GamePopover>

      <ProjectSeatPaymentPicker purchase={purchase} />
    </>
  );
};

export default ProjectFundingPopover;
