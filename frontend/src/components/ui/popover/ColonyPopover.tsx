import React from "react";
import { GameDto } from "@/types/generated/api-types.ts";
import { GamePopover } from "../GamePopover";
import ColonyList, {
  ColonyModeToggle,
  ColonyStorageWarning,
  TradePaymentSelector,
  useColonyControls,
  type ColonyControls,
} from "./content/ColonyList.tsx";

interface ColonyPopoverProps {
  isVisible: boolean;
  onClose: () => void;
  gameState?: GameDto;
  anchorRef: React.RefObject<HTMLButtonElement | null>;
}

const ColonyPopover: React.FC<ColonyPopoverProps> = ({
  isVisible,
  onClose,
  gameState,
  anchorRef,
}) => {
  const controls = useColonyControls(gameState);
  return (
    <ColonyPopoverView
      isVisible={isVisible}
      onClose={onClose}
      gameState={gameState}
      anchorRef={anchorRef}
      controls={controls}
    />
  );
};

export const ColonyPopoverView: React.FC<ColonyPopoverProps & { controls: ColonyControls }> = ({
  isVisible,
  onClose,
  gameState,
  anchorRef,
  controls,
}) => {
  const toggleButton = <ColonyModeToggle controls={controls} density="popover" />;

  return (
    <>
      <GamePopover
        isVisible={isVisible}
        onClose={onClose}
        position={{ type: "anchor", anchorRef, placement: "below" }}
        theme="colonies"
        excludeRef={anchorRef}
        header={
          controls.mode === "trade"
            ? {
                title: "",
                badge: <TradePaymentSelector controls={controls} density="popover" />,
                rightContent: toggleButton,
              }
            : {
                title: "",
                rightContent: toggleButton,
              }
        }
        width={560}
        maxHeight="80dvh"
        animation="slideDown"
        className="game-popover-list !bg-space-black-darker"
      >
        <ColonyList gameState={gameState} controls={controls} density="popover" />
      </GamePopover>

      <ColonyStorageWarning controls={controls} />
    </>
  );
};

export default ColonyPopover;
