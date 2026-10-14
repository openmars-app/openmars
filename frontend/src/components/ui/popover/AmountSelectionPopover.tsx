import React, { useState, useEffect } from "react";
import type { ResourceType } from "@/types/generated/api-types.ts";
import { getResourceName } from "@/utils/resourceColors.ts";
import GameIcon from "../display/GameIcon.tsx";
import GameButton from "../buttons/GameButton.tsx";
import {
  GameFlowPopover,
  GameFlowTitle,
  GameFlowBody,
  GameFlowFooter,
} from "./GameFlowPopover.tsx";

interface AmountSelectionPopoverProps {
  resourceType: ResourceType;
  maxAmount: number;
  onAmountSelect: (amount: number) => void;
  onCancel: () => void;
  isVisible: boolean;
}

const AmountSelectionPopover: React.FC<AmountSelectionPopoverProps> = ({
  resourceType,
  maxAmount,
  onAmountSelect,
  onCancel,
  isVisible,
}) => {
  const [amount, setAmount] = useState(maxAmount);

  useEffect(() => {
    if (isVisible) {
      setAmount(maxAmount);
    }
  }, [isVisible, maxAmount]);

  const handleConfirm = () => {
    onAmountSelect(amount);
  };

  return (
    <GameFlowPopover isVisible={isVisible} onClose={onCancel} className="min-w-[280px]">
      <GameFlowTitle>
        <h3 className="m-0 font-orbitron text-white text-base font-bold text-shadow-glow">
          Select Amount
        </h3>
      </GameFlowTitle>

      <GameFlowBody className="p-5! flex flex-col items-center gap-4">
        <div className="text-white/80 text-sm text-center flex items-center justify-center gap-1.5">
          <span>Select the amount of {getResourceName(resourceType).toLowerCase()}</span>
          <GameIcon iconType={resourceType} size="small" />
        </div>

        <div className="grid grid-cols-[44px_minmax(0,1fr)_44px] items-center gap-3 w-full">
          <GameButton
            emphasis="quiet"
            className="w-9 h-9 compact:size-11 rounded-none bg-space-blue-600/50 border border-space-blue-500/60 text-white text-lg font-bold cursor-pointer transition-all duration-200 hover:bg-space-blue-500/60 disabled:opacity-30 disabled:cursor-default"
            onClick={() => setAmount((a) => Math.max(0, a - 1))}
            disabled={amount <= 0}
          >
            -
          </GameButton>

          <div className="min-w-0 text-center whitespace-nowrap tabular-nums">
            <span className="text-white text-3xl font-orbitron font-bold text-shadow-glow">
              {amount}
            </span>
            <span className="text-white/40 text-sm ml-1">/ {maxAmount}</span>
          </div>

          <GameButton
            emphasis="quiet"
            className="w-9 h-9 compact:size-11 rounded-none bg-space-blue-600/50 border border-space-blue-500/60 text-white text-lg font-bold cursor-pointer transition-all duration-200 hover:bg-space-blue-500/60 disabled:opacity-30 disabled:cursor-default"
            onClick={() => setAmount((a) => Math.min(maxAmount, a + 1))}
            disabled={amount >= maxAmount}
          >
            +
          </GameButton>
        </div>

        {maxAmount > 1 && (
          <input
            type="range"
            min={0}
            max={maxAmount}
            value={amount}
            onChange={(e) => setAmount(parseInt(e.target.value))}
            className="w-full compact:h-11 accent-space-blue-500"
          />
        )}
      </GameFlowBody>

      <GameFlowFooter className="gap-3">
        <GameButton emphasis="secondary" tone="info" size="sm" onClick={onCancel}>
          Cancel
        </GameButton>
        <GameButton
          emphasis="primary"
          tone="info"
          size="sm"
          clickSound={false}
          onClick={handleConfirm}
        >
          Confirm
        </GameButton>
      </GameFlowFooter>
    </GameFlowPopover>
  );
};

export default AmountSelectionPopover;
