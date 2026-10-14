import React, { useRef } from "react";
import { GameDto, StateDiffDto } from "@/types/generated/api-types.ts";
import { useGameLogs } from "@/hooks/useGameLogs.ts";
import { GamePopover } from "../GamePopover";
import GameLogList from "./content/GameLogList.tsx";

interface LogPopoverProps {
  isVisible: boolean;
  onClose: () => void;
  anchorRef: React.RefObject<HTMLElement | null>;
  gameId: string;
  gameState?: GameDto;
}

const LogPopover: React.FC<LogPopoverProps> = ({
  isVisible,
  onClose,
  anchorRef,
  gameId,
  gameState,
}) => {
  const logs = useGameLogs(gameId);
  return (
    <LogPopoverView
      isVisible={isVisible}
      onClose={onClose}
      anchorRef={anchorRef}
      gameState={gameState}
      logs={logs}
    />
  );
};

export const LogPopoverView: React.FC<
  Omit<LogPopoverProps, "gameId"> & { logs: StateDiffDto[] }
> = ({ isVisible, onClose, anchorRef, gameState, logs }) => {
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  return (
    <GamePopover
      isVisible={isVisible}
      onClose={onClose}
      position={{ type: "anchor", anchorRef, placement: "above" }}
      theme="log"
      header={{
        title: "Game Log",
        badge: logs.length > 0 ? `${logs.length} entries` : undefined,
      }}
      arrow={{ enabled: true, position: "right", offset: 30 }}
      width={350}
      maxHeight={400}
      contentRef={scrollContainerRef}
    >
      <GameLogList
        logs={logs}
        gameState={gameState}
        density="popover"
        scrollContainerRef={scrollContainerRef}
        active={isVisible}
      />
    </GamePopover>
  );
};

export default LogPopover;
