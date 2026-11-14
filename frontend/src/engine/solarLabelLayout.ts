import type { BodyMarker } from "./scene.generated";
import type { HudInsets } from "../hooks/useHudInsets";

export interface OverlayObstacle {
  x: number;
  y: number;
  width: number;
  height: number;
}

/** Keep source world anchors while separating their readable click targets. */
export function placeSolarLabels(
  markers: BodyMarker[],
  compact: boolean,
  width: number,
  height: number,
  insets: HudInsets,
  obstacles: OverlayObstacle[] = [],
) {
  const labelWidth = compact ? 132 : 112;
  const labelHeight = compact ? 56 : 52;
  const halfWidth = labelWidth / 2;
  const halfHeight = labelHeight / 2;
  const left = insets.left + halfWidth + 8;
  const right = width - insets.right - halfWidth - 8;
  const top = insets.top + halfHeight + 8;
  const bottom = height - insets.bottom - halfHeight - 8;
  const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(max, value));
  const grid: { x: number; y: number }[] = [];
  for (let y = top; y <= bottom; y += labelHeight + 6) {
    for (let x = left; x <= right; x += labelWidth + 8) grid.push({ x, y });
  }
  const pack = (gridOnly: boolean) => {
    const placed: { marker: BodyMarker; x: number; y: number; width: number; height: number }[] =
      [];
    for (const marker of markers) {
      const idealX = marker.point[0];
      const idealY = marker.point[1] - (compact ? 24 : 14);
      const candidates = [...grid];
      if (!gridOnly) {
        for (let row = -8; row <= 8; row++) {
          for (let column = -3; column <= 3; column++) {
            candidates.push({
              x: clamp(idealX + column * (labelWidth + 8), left, right),
              y: clamp(idealY + row * (labelHeight + 6), top, bottom),
            });
          }
        }
      }
      candidates.sort(
        (a, b) => Math.hypot(a.x - idealX, a.y - idealY) - Math.hypot(b.x - idealX, b.y - idealY),
      );
      const position = candidates.find(
        ({ x, y }) =>
          placed.every(
            (other) =>
              Math.abs(other.x - x) >= labelWidth + 6 || Math.abs(other.y - y) >= labelHeight + 6,
          ) &&
          obstacles.every(
            (other) =>
              x + halfWidth + 6 <= other.x ||
              x - halfWidth - 6 >= other.x + other.width ||
              y + halfHeight + 6 <= other.y ||
              y - halfHeight - 6 >= other.y + other.height,
          ),
      );
      if (position) placed.push({ marker, ...position, width: labelWidth, height: labelHeight });
    }
    return placed;
  };
  const placed = pack(false);
  return placed.length === markers.length ? placed : pack(true);
}
