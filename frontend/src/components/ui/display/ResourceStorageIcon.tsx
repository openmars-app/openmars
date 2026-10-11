import React from "react";
import { ResourceStorageDto } from "@/types/generated/api-types.ts";
import GameIcon from "./GameIcon.tsx";
import DecorBox from "./DecorBox.tsx";
import RevealTrigger from "./RevealTrigger.tsx";

interface ResourceStorageIconProps {
  resourceStorage?: ResourceStorageDto;
  corner?: "bottom-right" | "top-left";
  bare?: boolean;
}

const ResourceStorageIcon: React.FC<ResourceStorageIconProps> = ({
  resourceStorage,
  corner = "bottom-right",
  bare = false,
}) => {
  if (!resourceStorage) {
    return null;
  }

  const resourceIcon = resourceStorage.icon ?? resourceStorage.type;
  const displayLabel = resourceStorage.label ?? resourceStorage.type;
  const label = (
    <>
      <GameIcon iconType={resourceIcon} size="behavior" className="[--behavior-icon-size:14px]" />
      <span className="text-[9px] text-white/50 font-semibold tracking-wider uppercase">
        {displayLabel}
      </span>
      {resourceStorage.capacity !== undefined && (
        <span
          className="font-orbitron text-[9px] text-white/70"
          aria-label={`Capacity ${resourceStorage.capacity}`}
        >
          ≤{resourceStorage.capacity}
        </span>
      )}
    </>
  );

  return (
    <RevealTrigger
      as="div"
      className="relative w-fit"
      content={resourceStorage.description || null}
      placement="below"
    >
      {bare ? (
        <div className="inline-flex items-center gap-1">{label}</div>
      ) : (
        <DecorBox corner={corner}>{label}</DecorBox>
      )}
    </RevealTrigger>
  );
};

export default ResourceStorageIcon;
