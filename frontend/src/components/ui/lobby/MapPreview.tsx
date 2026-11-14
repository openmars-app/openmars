import React from "react";
import type { MapPreviewTile } from "../../../types/generated/api-types.ts";

interface MapPreviewProps {
  tiles: MapPreviewTile[];
}

const HEX_SIZE = 14;
const SQRT3 = Math.sqrt(3);

function hexToPixel(q: number, r: number): { x: number; y: number } {
  const x = HEX_SIZE * SQRT3 * (q + r / 2);
  const y = ((HEX_SIZE * 3) / 2) * r;
  return { x, y };
}

function hexPoints(cx: number, cy: number): string {
  const points: string[] = [];
  for (let i = 0; i < 6; i++) {
    const angle = (Math.PI / 180) * (60 * i - 30);
    const px = cx + HEX_SIZE * Math.cos(angle);
    const py = cy + HEX_SIZE * Math.sin(angle);
    points.push(`${px},${py}`);
  }
  return points.join(" ");
}

function getTileColor(tile: MapPreviewTile): string {
  if (tile.type === "ocean-space") {
    return "#1a5276";
  }
  if (tile.type === "restricted-tile") {
    return "#4a1a1a";
  }
  if (tile.volcanic) {
    return "#8b3a2a";
  }
  return "#5c4a32";
}

const MapPreview: React.FC<MapPreviewProps> = ({ tiles }) => {
  const visibleTiles = tiles.filter((t) => t.type !== "empty" && t.type !== "restricted-tile");

  if (visibleTiles.length === 0) {
    return null;
  }

  const positions = visibleTiles.map((tile) => {
    const pos = hexToPixel(tile.q, tile.r);
    return { tile, ...pos };
  });

  const xs = positions.map((p) => p.x);
  const ys = positions.map((p) => p.y);
  const minX = Math.min(...xs) - HEX_SIZE - 2;
  const maxX = Math.max(...xs) + HEX_SIZE + 2;
  const minY = Math.min(...ys) - HEX_SIZE - 2;
  const maxY = Math.max(...ys) + HEX_SIZE + 2;

  const width = maxX - minX;
  const height = maxY - minY;

  return (
    <svg
      role="img"
      aria-label="Map preview"
      viewBox={`${minX} ${minY} ${width} ${height}`}
      width={width * 1.2}
      height={height * 1.2}
      className="block"
    >
      {positions.map((p) => (
        <polygon
          key={`${p.tile.q},${p.tile.r},${p.tile.s}`}
          points={hexPoints(p.x, p.y)}
          fill={getTileColor(p.tile)}
          stroke="rgba(255,255,255,0.15)"
          strokeWidth={0.5}
        />
      ))}
    </svg>
  );
};

export default MapPreview;
