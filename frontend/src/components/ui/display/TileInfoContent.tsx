import React from "react";
import GameIcon from "./GameIcon.tsx";

export interface TileTooltipData {
  tileType: string;
  displayName?: string;
  placedName?: string;
  ownerName?: string;
  ownerColor?: string;
  reservedByName?: string;
  isOceanSpace: boolean;
  isVolcanic: boolean;
  bonuses: { [key: string]: number };
}

const TILE_TYPE_LABELS: Record<string, string> = {
  empty: "Land Space",
  ocean: "Ocean",
  city: "City",
  greenery: "Greenery",
  volcano: "Volcano",
  "nuclear-zone": "Nuclear Zone",
  mining: "Mining Area",
  "industrial-center": "Industrial Center",
  restricted: "Reserved Area",
  special: "Special",
  "ecological-zone": "Ecological Zone",
  "natural-preserve": "Natural Preserve",
};

export function getTileInfoTitle(data: TileTooltipData): string {
  if (data.tileType === "city") {
    return "City";
  }
  if (data.tileType === "empty" && !data.displayName) {
    return data.isOceanSpace ? "Ocean Space" : "Land Space";
  }
  return data.displayName || TILE_TYPE_LABELS[data.tileType] || data.tileType;
}

interface TileInfoContentProps {
  data: TileTooltipData;
  showTitle?: boolean;
}

const TileInfoContent: React.FC<TileInfoContentProps> = ({ data, showTitle = true }) => {
  const isCity = data.tileType === "city";
  const isEmptySpace = data.tileType === "empty";
  const bonusEntries = Object.entries(data.bonuses);

  return (
    <>
      {showTitle && (
        <div className="font-orbitron font-bold text-xs text-white mb-1">
          {getTileInfoTitle(data)}
        </div>
      )}

      {isCity && data.placedName && (
        <div className="font-orbitron text-[10px] compact:text-[11px] text-white/60 mb-1 break-words">
          {data.placedName}
        </div>
      )}

      {data.isVolcanic && isEmptySpace && (
        <div className="text-[10px] compact:text-[11px] text-orange-400 mb-1">Volcanic</div>
      )}

      {data.ownerName && (
        <div className="flex items-center gap-1.5 mb-1">
          <div
            className="w-2 h-2 rounded-full shrink-0"
            style={{ backgroundColor: data.ownerColor || "#888" }}
          />
          <span className="text-white/70">{data.ownerName}</span>
        </div>
      )}

      {data.reservedByName && !data.ownerName && (
        <div className="text-white/50 text-[10px] compact:text-[11px] mb-1">
          Reserved by {data.reservedByName}
        </div>
      )}

      {bonusEntries.length > 0 && (
        <div className="flex items-center gap-1.5 mt-1">
          <span className="text-white/50 text-[10px] compact:text-[11px]">Bonus:</span>
          {bonusEntries.map(([type, amount]) => (
            <GameIcon key={type} iconType={type} amount={amount} size="small" />
          ))}
        </div>
      )}
    </>
  );
};

export default TileInfoContent;
