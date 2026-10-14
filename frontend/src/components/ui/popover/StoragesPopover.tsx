import React from "react";
import { OtherPlayerDto, PlayerDto } from "../../../types/generated/api-types.ts";
import { GamePopover } from "../GamePopover";
import StoragesList, { useStorageItems, type StorageItem } from "./content/StoragesList.tsx";

interface StoragesPopoverProps {
  isVisible: boolean;
  onClose: () => void;
  player: PlayerDto | OtherPlayerDto;
  anchorRef: React.RefObject<HTMLElement | null>;
}

const StoragesPopover: React.FC<StoragesPopoverProps> = ({
  isVisible,
  onClose,
  player,
  anchorRef,
}) => {
  const storageItems = useStorageItems(player.resourceStorage, isVisible);

  return (
    <StoragesPopoverView
      isVisible={isVisible}
      onClose={onClose}
      anchorRef={anchorRef}
      storageItems={storageItems}
    />
  );
};

export const StoragesPopoverView: React.FC<
  Omit<StoragesPopoverProps, "player"> & { storageItems: StorageItem[] }
> = ({ isVisible, onClose, anchorRef, storageItems }) => {
  return (
    <GamePopover
      className="game-popover-list"
      isVisible={isVisible}
      onClose={onClose}
      position={{ type: "anchor", anchorRef, placement: "above" }}
      theme="storages"
      header={{
        title: "Card Storages",
        badge: `${storageItems.length} card${storageItems.length !== 1 ? "s" : ""}`,
      }}
      arrow={{ enabled: true, position: "right", offset: 30 }}
      width={320}
      maxHeight={400}
    >
      <StoragesList items={storageItems} density="popover" />
    </GamePopover>
  );
};

export default StoragesPopover;
