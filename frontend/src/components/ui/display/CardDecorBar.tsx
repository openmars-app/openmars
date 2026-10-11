import React, { useRef } from "react";
import DecorBox from "./DecorBox.tsx";
import RevealTrigger from "./RevealTrigger.tsx";
import VictoryPointIcon from "./VictoryPointIcon.tsx";
import ResourceStorageIcon from "./ResourceStorageIcon.tsx";
import { ResourceStorageDto } from "@/types/generated/api-types.ts";

interface CardDecorBarProps {
  vpConditions?: any[];
  resourceStorage?: ResourceStorageDto;
  corner?: "bottom-right" | "top-left";
}

const CardDecorBar: React.FC<CardDecorBarProps> = ({
  vpConditions,
  resourceStorage,
  corner = "bottom-right",
}) => {
  const ref = useRef<HTMLDivElement>(null);

  const hasVp = vpConditions && vpConditions.length > 0;
  const hasStorage = !!resourceStorage;

  if (!hasVp && !hasStorage) {
    return null;
  }

  const vpDescription: string | null =
    vpConditions
      ?.map((condition: any) => condition.description)
      .filter(Boolean)
      .join(" ") || null;

  return (
    <div className="relative w-fit" ref={ref}>
      <DecorBox corner={corner}>
        {hasVp && (
          <RevealTrigger
            className="inline-flex"
            content={vpDescription}
            placement="below"
            anchorRef={ref}
          >
            <VictoryPointIcon vpConditions={vpConditions} bare />
          </RevealTrigger>
        )}
        {hasVp && hasStorage && <div className="w-px h-3 bg-white/20 mx-1" />}
        {hasStorage && <ResourceStorageIcon resourceStorage={resourceStorage} bare />}
      </DecorBox>
    </div>
  );
};

export default CardDecorBar;
