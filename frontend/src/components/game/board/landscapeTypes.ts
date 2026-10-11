import type { MarsRelief } from "./marsRelief";
import type { HexCoordinate } from "../../../utils/hex-grid-2d";
import type { StructureLayout } from "./structureLayout";

export type LandscapeKind =
  | "structure"
  | "greenery"
  | "special-greenery"
  | "ocean"
  | "volcano"
  | "excluded";
export interface LandscapeSource {
  coordinate: HexCoordinate;
  kind: LandscapeKind;
  seed: number;
  layout?: StructureLayout;
}
export interface LandscapeInput {
  relief?: MarsRelief;
  seed: number;
  sources: LandscapeSource[];
}
export interface LandscapePlant {
  id: string;
  tileKey: string;
  kind: "tree" | "pine" | "bush" | "clover" | "flower" | "rock";
  x: number;
  y: number;
  height: number;
  scale: number;
  seed: number;
  // Position of the plant within its cell's density threshold; climate gates show plants with rank <= gate.
  rank: number;
}
export interface LandscapePatch {
  key: string;
  signature: string;
  x: number;
  y: number;
  terrain: Uint16Array;
  materials: Uint8Array;
  // R: shoreline meadow, G: hummock noise, B: greenery weight, A: normalized Mars depth.
  detail: Uint8Array;
  plants: LandscapePlant[];
  water: boolean;
}
export interface LandscapeRequest extends LandscapeInput {
  id: number;
  known: Record<string, string>;
}
export interface LandscapeDelta {
  relief?: MarsRelief;
  id: number;
  seed: number;
  sources: LandscapeSource[];
  patches: LandscapePatch[];
  removed: string[];
  preparationMs: number;
}
export interface LandscapeState {
  relief?: MarsRelief;
  id: number;
  seed: number;
  sources: LandscapeSource[];
  patches: Map<string, LandscapePatch>;
  plants: LandscapePlant[];
}
export const EMPTY_LANDSCAPE: LandscapeState = {
  id: 0,
  seed: 0,
  sources: [],
  patches: new Map(),
  plants: [],
};
