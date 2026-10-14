import React from "react";
import { GamePopover } from "../GamePopover";
import TagsSummary, { summarizeTags, type TagCount } from "./content/TagsSummary.tsx";

interface TagsPopoverProps {
  isVisible: boolean;
  onClose: () => void;
  tagCounts: TagCount[];
  anchorRef: React.RefObject<HTMLElement | null>;
}

const TagsPopover: React.FC<TagsPopoverProps> = ({ isVisible, onClose, tagCounts, anchorRef }) => {
  const { totalTags } = summarizeTags(tagCounts);

  return (
    <GamePopover
      className="game-popover-list"
      isVisible={isVisible}
      onClose={onClose}
      position={{ type: "anchor", anchorRef, placement: "above" }}
      theme="tags"
      header={{ title: "Tags", badge: `${totalTags} total` }}
      arrow={{ enabled: true, position: "right", offset: 30 }}
      width={320}
      maxHeight={400}
    >
      <TagsSummary tagCounts={tagCounts} density="popover" />
    </GamePopover>
  );
};

export default TagsPopover;
