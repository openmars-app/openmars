import { MARS_RELIEF_DEPTH, marsReliefAtBoard, type MarsRelief } from "./marsRelief";
import { DataUtils } from "three";
import { HexGrid2D } from "../../../utils/hex-grid-2d";
import {
  boardCenter,
  BOARD_SCALE,
  TILE_RADIUS,
  hashSeed,
  nearestOnRoad,
} from "./landscapeGeometry";
import {
  insideStructure,
  insideBuilding,
  onEntrancePath,
  structureWallDistance,
  structureHexWallApothem,
} from "./structureLayout";
import type {
  LandscapeInput,
  LandscapePatch,
  LandscapePlant,
  LandscapeSource,
} from "./landscapeTypes";

export const PATCH_SIZE = 0.6 * BOARD_SCALE;
export const FIELD_SAMPLES = 128;
export const FIELD_BORDER = 2;
export const FIELD_SIZE = FIELD_SAMPLES + FIELD_BORDER * 2;
export const FIELD_STEP = PATCH_SIZE / (FIELD_SAMPLES - 1);
export const WATER_LEVEL = -0.008;
export const LAKE_BED_LEVEL = -0.024;
export const BEACH_WIDTH_MIN = 0.05;
export const BEACH_WIDTH_MAX = 0.075;
export const LAKE_MASK_REACH = BEACH_WIDTH_MAX + 0.015;
export const LAKE_GROUND_REACH = BEACH_WIDTH_MAX + 0.04;
export const TRANSITION_MS = 600;
const REACH = PATCH_SIZE * 1.1;
export const WATER_RADIUS = 0.17 * BOARD_SCALE;
// Shore distance of land without a patch; matches LandscapeSurface.clearTerrain.
export const EMPTY_SHORE = 0.2;
const keyOf = HexGrid2D.coordinateToKey;
const clamp = (x: number) => Math.max(0, Math.min(1, x));
export function smooth(a: number, b: number, x: number) {
  const t = clamp((x - a) / (b - a));
  return t * t * (3 - 2 * t);
}
export function basinHeight(shore: number, beachWidth: number, landHeight: number) {
  if (shore < 0) {
    return WATER_LEVEL + (LAKE_BED_LEVEL - WATER_LEVEL) * smooth(0, 0.07, -shore);
  }
  // Keep the dry bank above water, then grade back into any deeper terrain beyond the beach.
  const bankHeight = Math.max(landHeight, WATER_LEVEL + 0.002);
  const bank = WATER_LEVEL + (bankHeight - WATER_LEVEL) * smooth(0, beachWidth, shore);
  const blend = smooth(beachWidth, LAKE_GROUND_REACH, shore);
  return bank + (landHeight - bank) * blend;
}
export function landscapeHeightAt(
  patches: Map<string, LandscapePatch>,
  x: number,
  y: number,
  relief?: MarsRelief,
) {
  const patch = patches.get(`${Math.floor(x / PATCH_SIZE)}:${Math.floor(y / PATCH_SIZE)}`);
  if (!patch) {
    return marsReliefAtBoard(relief, x, y) + 0.0003;
  }
  return sampleTerrain(patch, x, y, 0);
}
export function landscapeShoreAt(patches: Map<string, LandscapePatch>, x: number, y: number) {
  const patch = patches.get(`${Math.floor(x / PATCH_SIZE)}:${Math.floor(y / PATCH_SIZE)}`);
  if (!patch) {
    return EMPTY_SHORE;
  }
  return sampleTerrain(patch, x, y, 1);
}
function sampleTerrain(patch: LandscapePatch, x: number, y: number, channel: number) {
  const px = (x - patch.x) / FIELD_STEP + FIELD_BORDER;
  const py = (y - patch.y) / FIELD_STEP + FIELD_BORDER;
  const ix = Math.floor(px),
    iy = Math.floor(py);
  const read = (dx: number, dy: number) =>
    DataUtils.fromHalfFloat(patch.terrain[((iy + dy) * FIELD_SIZE + ix + dx) * 4 + channel]);
  const a = read(0, 0) * (1 - (px - ix)) + read(1, 0) * (px - ix);
  const b = read(0, 1) * (1 - (px - ix)) + read(1, 1) * (px - ix);
  return a * (1 - (py - iy)) + b * (py - iy);
}
// Weight of the next field at linear progress `elapsed`; see landscapeWeight in
// landscape-field.glsl, which this mirrors.
export function lakeBlendWeight(previous: number, next: number, elapsed: number) {
  if (next >= previous) {
    return elapsed * elapsed * (3 - 2 * elapsed);
  }
  const front = -WATER_RADIUS + elapsed * (EMPTY_SHORE + WATER_RADIUS);
  const settle = Math.max(next, -WATER_RADIUS);
  return Math.max(smooth(settle, settle + 0.1, front), smooth(0.85, 1, elapsed));
}
function hash(x: number, y: number, seed: number) {
  let n = Math.imul(x, 374761393) ^ Math.imul(y, 668265263) ^ seed;
  n = Math.imul(n ^ (n >>> 13), 1274126177);
  return ((n ^ (n >>> 16)) >>> 0) / 4294967296;
}
export function landscapeNoise(x: number, y: number, seed: number) {
  const ix = Math.floor(x),
    iy = Math.floor(y);
  const curve = (v: number) => v * v * v * (v * (v * 6 - 15) + 10);
  const u = curve(x - ix),
    v = curve(y - iy);
  const a = hash(ix, iy, seed),
    b = hash(ix + 1, iy, seed);
  const c = hash(ix, iy + 1, seed),
    d = hash(ix + 1, iy + 1, seed);
  return (a + (b - a) * u) * (1 - v) + (c + (d - c) * u) * v;
}
function fractal(x: number, y: number, seed: number) {
  return (
    landscapeNoise(x, y, seed) * 0.57 +
    landscapeNoise(x * 2.1, y * 2.1, seed + 41) * 0.28 +
    landscapeNoise(x * 4.3, y * 4.3, seed + 79) * 0.15
  );
}
function hexDistance(x: number, y: number, radius = PATCH_SIZE * 0.5) {
  return (
    Math.max(
      Math.abs(x),
      Math.abs(x * 0.5 + y * 0.866025404),
      Math.abs(x * -0.5 + y * 0.866025404),
    ) -
    radius * 0.866025404
  );
}
function segmentDistance(x: number, y: number, ax: number, ay: number, bx: number, by: number) {
  const dx = bx - ax,
    dy = by - ay;
  const t = clamp(((x - ax) * dx + (y - ay) * dy) / Math.max(dx * dx + dy * dy, 1e-10));
  return Math.hypot(x - ax - t * dx, y - ay - t * dy);
}
type Source = LandscapeSource & { x: number; y: number; key: string };
export interface LandscapeSample {
  relief: number;
  height: number;
  shore: number;
  coverage: number;
  forest: number;
  foundation: number;
  moisture: number;
  dry: number;
  lush: number;
  litter: number;
  mud: number;
  meadow: number;
  mound: number;
  greenery: number;
  source?: Source;
  blocked: boolean;
}
const ENVIRONMENT_STRIDE = 8;
function environmentAt(x: number, y: number, seed: number, target: Float64Array, offset: number) {
  const broad = fractal(x * 2.6, y * 2.6, seed);
  const wx = x + (landscapeNoise(x * 5, y * 5, seed + 1) - 0.5) * 0.1;
  const wy = y + (landscapeNoise(x * 5 + 19, y * 5, seed + 2) - 0.5) * 0.1;
  const soil = fractal(wx * 11, wy * 11, seed + 3);
  const fine = landscapeNoise(x * 73, y * 73, seed + 4);
  const ox = x + (fractal(x * 18, y * 18, seed + 5) - 0.5) * 0.08;
  const oy = y + (fractal(x * 18 + 31.7, y * 18, seed + 5) - 0.5) * 0.08;
  // Rounded hills about 0.11 wide, spanning roughly ten mesh quads.
  const mound = fractal(wx * 9, wy * 9, seed + 6);
  target.set([broad, wx, wy, soil, fine, ox, oy, mound], offset);
}
export function createLandscapeSampler(input: LandscapeInput) {
  const sources: Source[] = input.sources
    .map((s) => ({ ...s, ...boardCenter(s.coordinate), key: keyOf(s.coordinate) }))
    .sort((a, b) => a.key.localeCompare(b.key));
  const waters = sources.filter((s) => s.kind === "ocean");
  const waterMap = new Map(waters.map((s) => [s.key, s]));
  const links = waters.flatMap((a) =>
    HexGrid2D.getNeighbors(a.coordinate).flatMap((c) => {
      const b = waterMap.get(keyOf(c));
      return b && a.key < b.key ? [{ a, b }] : [];
    }),
  );
  const waterTriangles = links.flatMap(({ a, b }) =>
    HexGrid2D.getNeighbors(a.coordinate).flatMap((coordinate) => {
      const c = waterMap.get(keyOf(coordinate));
      if (
        !c ||
        c.key <= b.key ||
        Math.max(
          Math.abs(c.coordinate.q - b.coordinate.q),
          Math.abs(c.coordinate.r - b.coordinate.r),
          Math.abs(c.coordinate.s - b.coordinate.s),
        ) !== 1
      ) {
        return [];
      }
      const orientation = Math.sign((b.x - a.x) * (c.y - a.y) - (b.y - a.y) * (c.x - a.x));
      return [{ a, b, c, scale: orientation / Math.hypot(b.x - a.x, b.y - a.y) }];
    }),
  );
  const structures = sources.filter((s) => s.kind === "structure");
  const structureMap = new Map(structures.map((s) => [s.key, s]));
  const structureLinks = structures.flatMap((a) =>
    HexGrid2D.getNeighbors(a.coordinate).flatMap((c) => {
      const b = structureMap.get(keyOf(c));
      return b &&
        a.key < b.key &&
        a.layout?.style.landmark !== "hall" &&
        b.layout?.style.landmark !== "hall"
        ? [{ a, b }]
        : [];
    }),
  );
  const greenery = sources.filter((s) =>
    ["greenery", "special-greenery", "volcano"].includes(s.kind),
  );
  const greeneryMap = new Map(greenery.map((s) => [s.key, s]));
  const structureGreeneryLinks = structures.flatMap((structure) =>
    HexGrid2D.getNeighbors(structure.coordinate).flatMap((coordinate) => {
      const green = greeneryMap.get(keyOf(coordinate));
      if (!green || green.kind === "volcano") {
        return [];
      }
      const dx = green.x - structure.x,
        dy = green.y - structure.y;
      const length = Math.hypot(dx, dy);
      const circular = structure.layout && structureHexWallApothem(structure.layout) === null;
      return [{ structure, circular, dx: dx / length, dy: dy / length, midpoint: length / 2 }];
    }),
  );
  const greeneryLinks = greenery.flatMap((a) =>
    HexGrid2D.getNeighbors(a.coordinate).flatMap((c) => {
      const b = greeneryMap.get(keyOf(c));
      return b && a.key < b.key ? [{ a, b }] : [];
    }),
  );
  const shorelineLinks = greenery.flatMap((a) =>
    HexGrid2D.getNeighbors(a.coordinate).flatMap((c) => {
      const b = waterMap.get(keyOf(c));
      return b ? [{ a, b }] : [];
    }),
  );
  const seed = input.seed;
  const scratch = new Float64Array(ENVIRONMENT_STRIDE);
  return (
    x: number,
    y: number,
    placement = true,
    environment?: Float64Array,
    offset = 0,
  ): LandscapeSample => {
    const data = environment ?? scratch;
    if (!environment) {
      environmentAt(x, y, seed, data, 0);
    }
    const broad = data[offset],
      wx = data[offset + 1],
      wy = data[offset + 2],
      soil = data[offset + 3],
      fine = data[offset + 4],
      ox = data[offset + 5],
      oy = data[offset + 6],
      mound = data[offset + 7];
    let shore = 1;
    for (const s of waters) {
      shore = Math.min(shore, Math.hypot(ox - s.x, oy - s.y) - WATER_RADIUS);
    }
    for (const { a, b } of links) {
      shore = Math.min(shore, segmentDistance(ox, oy, a.x, a.y, b.x, b.y) - WATER_RADIUS);
    }
    // Three occupied neighbors enclose water, not a shallow pocket between capsules.
    for (const { a, b, c, scale } of waterTriangles) {
      const inset = Math.min(
        ((b.x - a.x) * (oy - a.y) - (b.y - a.y) * (ox - a.x)) * scale,
        ((c.x - b.x) * (oy - b.y) - (c.y - b.y) * (ox - b.x)) * scale,
        ((a.x - c.x) * (oy - c.y) - (a.y - c.y) * (ox - c.x)) * scale,
      );
      if (inset >= 0) {
        shore = Math.min(shore, -WATER_RADIUS - inset);
      }
    }
    shore = Math.min(0.2, shore + (fine - 0.5) * 0.012);
    const moisture =
      (1 - smooth(0.005, 0.06 + soil * 0.09, Math.max(shore, 0))) * (0.55 + broad * 0.45);
    let growth = 0,
      greenery = 0,
      forest = 0,
      strongest = 0,
      source: Source | undefined;
    let blocked = false,
      structurePad = 0,
      foundation = 0,
      exclusion = 0;
    for (const s of sources) {
      const lx = x - s.x,
        ly = y - s.y;
      if (Math.abs(lx) > REACH || Math.abs(ly) > REACH) {
        continue;
      }
      const distance = hexDistance(lx, ly);
      if (s.kind === "ocean") {
        continue;
      }
      if (s.kind === "excluded") {
        // Special tiles keep their own ground; the landscape around them reaches irregularly up to,
        // and a little into, their edge instead of leaving a bare ring.
        const edge = hexDistance(wx - s.x, wy - s.y, TILE_RADIUS) + (soil - 0.5) * 0.04;
        exclusion = Math.max(exclusion, 1 - smooth(-0.045, -0.005, edge));
        blocked ||= distance < 0.008;
        continue;
      }
      const structure = s.kind === "structure";
      const capital = s.layout?.style.landmark === "hall";
      const circular = structure && s.layout && structureHexWallApothem(s.layout) === null;
      const footprintDistance =
        circular && s.layout ? structureWallDistance(s.layout, lx, ly) : distance;
      if (structure) {
        const workedEdge =
          footprintDistance + (soil - 0.5) * (circular ? 0.006 : 0.018) + (fine - 0.5) * 0.004;
        if (!capital) {
          blocked ||= footprintDistance < 0.03;
          const paving = circular
            ? 1 - smooth(0.004, 0.024, workedEdge)
            : 1 - smooth(-0.025, 0.023, workedEdge);
          foundation = Math.max(foundation, paving);
        }
      }
      const suitability = smooth(0.16, 0.75, soil * 0.7 + broad * 0.2 + moisture * 0.35);
      const warpedDistance = hexDistance(wx - s.x, wy - s.y);
      let influence =
        (1 - smooth(-0.06, 0.025, warpedDistance)) * (1 - smooth(0, PATCH_SIZE * 0.25, distance));
      if (circular) {
        influence = 1 - smooth(0.004, 0.028, footprintDistance);
      }
      if (!structure) {
        // Ground spills irregularly into neighbouring tiles, like the beach around lakes.
        const edge =
          hexDistance(wx - s.x, wy - s.y, TILE_RADIUS) + (soil - 0.5) * 0.05 + (fine - 0.5) * 0.006;
        influence = 1 - smooth(-0.02, 0.05, edge);
      }
      const canopy =
        (1 - smooth(-0.035, 0.006, warpedDistance)) *
        (0.65 + 0.35 * smooth(0.22, 0.66, soil + moisture * 0.18));
      let fertility = structure ? 0.48 : 1;
      if (structure && s.layout) {
        const style = s.layout.style;
        const r = Math.hypot(lx, ly);
        const hole = style.ground === "recessed" ? 0.095 : 0;
        if (r < hole) {
          exclusion = 1;
          blocked = true;
        }
        if (style.landscaping === "sparse") {
          fertility = 0.1;
        }
        if (capital) {
          fertility = 1;
          influence = 1 - smooth(-0.015, 0.018, structureWallDistance(s.layout, lx, ly));
        }
        if (placement || capital) {
          const road = nearestOnRoad({ x: lx, y: ly }, s.layout.roads);
          const built = insideStructure(s.layout, lx, ly, 0.003);
          const wallDistance = structureWallDistance(s.layout, lx, ly);
          if (capital) {
            const paved = s.layout.buildings.some(
              (b) => insideBuilding(b, lx, ly, 0.0008) || onEntrancePath(b, lx, ly, 0.0018),
            );
            const plaza = Math.max(Math.abs(lx), Math.abs(ly)) < 0.04475;
            blocked ||= plaza;
            const paving = Math.max(
              paved || plaza ? 1 : 0,
              1 - smooth(road.halfWidth, road.halfWidth + 0.0014, road.distance),
              1 - smooth(0.002, 0.005, Math.abs(wallDistance)),
            );
            foundation = Math.max(foundation, paving * influence);
          }
          if (
            built ||
            road.distance < road.halfWidth + 0.003 ||
            (style.perimeter !== "open" && Math.abs(wallDistance) < 0.01)
          ) {
            blocked = true;
          }
        }
        const pad = circular
          ? 1 - smooth(0, 0.026, footprintDistance)
          : 1 - smooth(-0.028, 0.015, distance);
        structurePad = Math.max(structurePad, pad);
      }
      if (s.kind === "volcano" && Math.hypot(lx, ly) < 0.117) {
        exclusion = 1;
        blocked = true;
      }
      const strength = influence * fertility;
      if (strength > strongest) {
        source = s;
        strongest = strength;
      }
      growth = 1 - (1 - growth) * (1 - strength * (0.88 + suitability * 0.12));
      if (s.kind === "greenery" || s.kind === "special-greenery") {
        greenery = Math.max(greenery, influence);
      }
      forest = Math.max(forest, canopy * (structure ? 0.12 : 1));
    }
    for (const { a, b } of greeneryLinks) {
      const distance = segmentDistance(x, y, a.x, a.y, b.x, b.y) + (soil - 0.5) * 0.012;
      growth = Math.max(growth, 1 - smooth(0.085, 0.115, distance));
      greenery = Math.max(greenery, 1 - smooth(0.085, 0.115, distance));
    }
    let shoreGrowth = 0;
    for (const { a, b } of shorelineLinks) {
      const distance = segmentDistance(x, y, a.x, a.y, b.x, b.y) + (soil - 0.5) * 0.012;
      shoreGrowth = Math.max(shoreGrowth, 1 - smooth(0.11, 0.145, distance));
    }
    shoreGrowth *= smooth(0.001, 0.006, shore);
    growth = Math.max(growth, shoreGrowth);
    for (const { a, b } of structureLinks) {
      const distance = segmentDistance(x, y, a.x, a.y, b.x, b.y) + (soil - 0.5) * 0.008;
      const infill = 1 - smooth(0.085, 0.115, distance);
      foundation = Math.max(foundation, infill);
      structurePad = Math.max(structurePad, infill);
    }
    for (const { structure, circular, dx, dy, midpoint } of structureGreeneryLinks) {
      const lx = x - structure.x,
        ly = y - structure.y;
      const along = lx * dx + ly * dy + (soil - 0.5) * 0.006;
      const across = Math.abs(lx * dy - ly * dx);
      const wall = structure.layout
        ? structureWallDistance(structure.layout, lx, ly)
        : hexDistance(lx, ly, TILE_RADIUS) + 0.01;
      if (circular) {
        const halfEdge = midpoint / Math.sqrt(3);
        const edge =
          (1 - smooth(halfEdge, halfEdge + 0.025, across)) *
          smooth(0, 0.025, along) *
          (1 - smooth(midpoint + 0.04, midpoint + 0.08, along));
        // Complement the circular paving fade so greenery meets the wall without a bare strip.
        const workedEdge = wall + (soil - 0.5) * 0.006 + (fine - 0.5) * 0.004;
        growth = Math.max(growth, edge * smooth(0.004, 0.024, workedEdge));
        continue;
      }
      const edge =
        (1 - smooth(0.08, 0.115, across)) *
        smooth(midpoint - 0.07, midpoint - 0.035, along) *
        smooth(0, 0.015, wall) *
        (1 - smooth(midpoint + 0.04, midpoint + 0.08, along));
      const green = smooth(midpoint - 0.035, midpoint + 0.025, along);
      growth = Math.max(growth, edge * green);
      if (structure.layout?.style.landmark !== "hall") {
        const paving = edge * (1 - smooth(midpoint - 0.035, midpoint + 0.035, along));
        foundation = Math.max(foundation, paving);
        structurePad = Math.max(structurePad, paving);
      }
    }
    const shoreMeadow =
      1 - smooth(BEACH_WIDTH_MAX + 0.008, BEACH_WIDTH_MAX + 0.05 + soil * 0.02, Math.max(shore, 0));
    const wetGrass = shoreMeadow * (0.72 + soil * 0.2) * smooth(0.003, 0.014, shore);
    const coverage = clamp(Math.max(growth, wetGrass) * (1 - exclusion));
    forest *= 1 - exclusion;
    foundation *= (1 - exclusion) * smooth(-0.002, 0.012, shore);
    const wetSoil =
      moisture *
      (1 - smooth(0.002, 0.04, Math.max(shore, 0))) *
      (0.3 + soil * 0.7) *
      (1 - shoreGrowth * 0.75);
    const fringeSoil =
      coverage * (1 - smooth(0.5, 0.88, coverage)) * (1 - smooth(0.05, 0.45, forest));
    const mud = Math.max(wetSoil, fringeSoil * 0.4);
    const litter = Math.min(coverage * 0.28, forest * (0.15 + soil * 0.14));
    const lush = Math.min(
      Math.max(0, coverage - litter - mud),
      coverage * (0.8 + moisture * 0.12) * smooth(0.08, 0.65, coverage),
    );
    const dry = Math.max(0, coverage - lush - litter - mud);
    const beachWidth = BEACH_WIDTH_MAX + (BEACH_WIDTH_MIN - BEACH_WIDTH_MAX) * clamp(lush + litter);
    // Hummocks only rise inside greeneries and flatten out before the beach.
    const hummocks =
      greenery *
      (1 - exclusion) *
      0.027 *
      clamp((mound - 0.25) / 0.45) *
      smooth(beachWidth, beachWidth + 0.03, shore);
    const relief = marsReliefAtBoard(input.relief, x, y);
    const naturalHeight =
      relief * (1 - exclusion) + 0.0003 + coverage * (0.0012 + broad * 0.0048) + hummocks;
    let height = basinHeight(shore, beachWidth, naturalHeight);
    height = height * (1 - exclusion) + naturalHeight * exclusion;
    height = height * (1 - structurePad) + 0.0003 * structurePad;
    if (blocked) {
      forest = 0;
    }
    return {
      relief,
      height,
      shore,
      coverage,
      forest,
      foundation,
      moisture,
      dry,
      lush,
      litter,
      mud,
      meadow: clamp(wetGrass * (1 - growth) * (1 - exclusion)),
      mound,
      greenery: clamp(greenery * (1 - exclusion)),
      source,
      blocked: blocked || shore < beachWidth * 0.8 || exclusion > 0.2,
    };
  };
}

function scatterPatch(
  x: number,
  y: number,
  sample: ReturnType<typeof createLandscapeSampler>,
  seed: number,
  hasCapital: boolean,
) {
  const plants: LandscapePlant[] = [];
  const rocks: { x: number; y: number; r: number }[] = [];
  for (const [kind, spacing, capitalOnly] of [
    ["rock", 0.095, false],
    ["tree", 0.016, false],
    ["pine", 0.014, false],
    ["bush", 0.014, false],
    ["clover", 0.031, false],
    ["flower", 0.046, false],
    ["tree", 0.01, true],
    ["bush", 0.012, true],
  ] as const) {
    if (capitalOnly && !hasCapital) {
      continue;
    }
    for (let ix = Math.floor(x / spacing); ix <= Math.ceil((x + PATCH_SIZE) / spacing); ix++) {
      for (let iy = Math.floor(y / spacing); iy <= Math.ceil((y + PATCH_SIZE) / spacing); iy++) {
        const salt = kind === "pine" ? 977 : kind.length;
        const h = hash(ix, iy, seed + salt * 311),
          h2 = hash(ix, iy, seed + salt * 829);
        const px = (ix + 0.2 + h * 0.6) * spacing,
          py = (iy + 0.2 + h2 * 0.6) * spacing;
        if (px < x || px >= x + PATCH_SIZE || py < y || py >= y + PATCH_SIZE) {
          continue;
        }
        const f = sample(px, py),
          s = f.source;
        if (!s || f.blocked || rocks.some((r) => Math.hypot(px - r.x, py - r.y) < r.r + 0.008)) {
          continue;
        }
        const structure = s.kind === "structure";
        const capital = structure && s.layout?.style.landmark === "hall";
        if ((kind === "tree" || kind === "bush") && capital !== capitalOnly) {
          continue;
        }
        let threshold = 0;
        if (kind === "tree") {
          threshold = capital ? f.coverage * 0.95 : f.forest * 0.665;
          if (!structure) {
            const edge = hexDistance(px - s.x, py - s.y, TILE_RADIUS);
            threshold *= 1 - 0.92 * smooth(-0.025, 0.006, edge);
          }
        }
        // Hardy pines are the first trees. They grow in groves, with only a few strays between.
        if (kind === "pine" && !structure && s.kind !== "volcano") {
          const edge = hexDistance(px - s.x, py - s.y, TILE_RADIUS);
          const grove = smooth(0.38, 0.64, fractal(px * 11, py * 11, seed + 41));
          threshold = f.coverage * 0.8 * (0.1 + 0.9 * grove) * (1 - smooth(-0.03, 0.0, edge));
        }
        if (kind === "bush") {
          threshold = f.coverage * (capital ? 0.9 : 0.12 + f.forest * 0.18);
        }
        if (kind === "clover") {
          threshold = f.lush * 0.45;
        }
        if (kind === "flower" && s.kind === "special-greenery") {
          threshold = f.coverage * 0.5;
        }
        if (kind === "rock" && !structure && s.kind !== "volcano") {
          threshold = f.coverage * 0.11;
        }
        if (h2 > threshold) {
          continue;
        }
        const cellSeed = hashSeed(`${seed}:${kind}:${ix}:${iy}`);
        let scale = structure ? 0.27 + h * 0.12 : 0.75 + h * 0.35;
        if (!structure && kind === "tree") {
          scale *= 0.6;
        }
        if (kind === "pine") {
          scale *= 0.7;
        }
        if (!structure && kind === "bush") {
          scale *= 0.75;
        }
        plants.push({
          id: `${kind}:${ix}:${iy}:${capitalOnly}`,
          tileKey: s.key,
          kind,
          x: px,
          y: py,
          height: f.height,
          seed: cellSeed,
          scale,
          rank: structure || kind === "rock" ? 0 : h2 / threshold,
        });
        if (kind === "rock") {
          rocks.push({ x: px, y: py, r: 0.025 * scale });
        }
      }
    }
  }
  return plants;
}

export class LandscapeBuilder {
  private cache = new Map<string, LandscapePatch>();
  private signatures = new Map<string, string>();
  private environment = new Map<string, Float64Array>();
  private seed: number | undefined;
  build(input: LandscapeInput) {
    if (this.seed !== input.seed) {
      this.environment.clear();
      this.seed = input.seed;
    }
    const patches = new Map<string, LandscapePatch>();
    const signatures = new Map<string, string>();
    const sources = [...input.sources].sort((a, b) =>
      keyOf(a.coordinate).localeCompare(keyOf(b.coordinate)),
    );
    const keys = new Set<string>();
    for (const source of sources) {
      if (source.kind === "excluded") {
        continue;
      }
      const p = boardCenter(source.coordinate);
      for (
        let x = Math.floor((p.x - REACH) / PATCH_SIZE);
        x <= Math.floor((p.x + REACH) / PATCH_SIZE);
        x++
      ) {
        for (
          let y = Math.floor((p.y - REACH) / PATCH_SIZE);
          y <= Math.floor((p.y + REACH) / PATCH_SIZE);
          y++
        ) {
          keys.add(`${x}:${y}`);
        }
      }
    }
    for (const key of [...keys].sort()) {
      const [ix, iy] = key.split(":").map(Number),
        x = ix * PATCH_SIZE,
        y = iy * PATCH_SIZE;
      const local = sources.filter((s) => {
        const p = boardCenter(s.coordinate);
        return (
          p.x >= x - REACH - FIELD_STEP * 2 &&
          p.x <= x + PATCH_SIZE + REACH + FIELD_STEP * 2 &&
          p.y >= y - REACH - FIELD_STEP * 2 &&
          p.y <= y + PATCH_SIZE + REACH + FIELD_STEP * 2
        );
      });
      const content = JSON.stringify([input.seed, input.relief?.revision, local]);
      const signature = `${hashSeed(content).toString(36)}:${hashSeed(content + "landscape").toString(36)}`;
      const previous = this.cache.get(key);
      signatures.set(key, signature);
      if (this.signatures.get(key) === signature) {
        if (previous) {
          patches.set(key, previous);
        }
        continue;
      }
      const sample = createLandscapeSampler({
        seed: input.seed,
        sources: local,
        relief: input.relief,
      });
      let environment = this.environment.get(key);
      if (!environment) {
        environment = new Float64Array(FIELD_SIZE * FIELD_SIZE * ENVIRONMENT_STRIDE);
        for (let row = 0; row < FIELD_SIZE; row++) {
          for (let col = 0; col < FIELD_SIZE; col++) {
            environmentAt(
              x + (col - FIELD_BORDER) * FIELD_STEP,
              y + (row - FIELD_BORDER) * FIELD_STEP,
              input.seed,
              environment,
              (row * FIELD_SIZE + col) * ENVIRONMENT_STRIDE,
            );
          }
        }
        this.environment.set(key, environment);
      }
      const terrain = new Uint16Array(FIELD_SIZE * FIELD_SIZE * 4),
        materials = new Uint8Array(terrain.length),
        detail = new Uint8Array(terrain.length);
      let visible = false,
        water = false;
      for (let row = 0; row < FIELD_SIZE; row++) {
        for (let col = 0; col < FIELD_SIZE; col++) {
          const f = sample(
              x + (col - FIELD_BORDER) * FIELD_STEP,
              y + (row - FIELD_BORDER) * FIELD_STEP,
              false,
              environment,
              (row * FIELD_SIZE + col) * ENVIRONMENT_STRIDE,
            ),
            offset = (row * FIELD_SIZE + col) * 4;
          terrain[offset] = DataUtils.toHalfFloat(f.height);
          terrain[offset + 1] = DataUtils.toHalfFloat(f.shore);
          terrain[offset + 2] = DataUtils.toHalfFloat(f.coverage);
          terrain[offset + 3] = DataUtils.toHalfFloat(f.foundation);
          materials[offset] = Math.round(clamp(f.dry) * 255);
          materials[offset + 1] = Math.round(clamp(f.lush) * 255);
          materials[offset + 2] = Math.round(clamp(f.litter) * 255);
          materials[offset + 3] = Math.round(clamp(f.mud) * 255);
          detail[offset] = Math.round(f.meadow * 255);
          detail[offset + 1] = Math.round(clamp(f.mound) * 255);
          detail[offset + 2] = Math.round(f.greenery * 255);
          detail[offset + 3] = Math.round(clamp(-f.relief / MARS_RELIEF_DEPTH) * 255);
          visible ||= f.coverage > 0.001 || f.foundation > 0.001 || f.shore < LAKE_GROUND_REACH;
          water ||= f.shore < 0.005;
        }
      }
      if (visible) {
        patches.set(key, {
          key,
          signature,
          x,
          y,
          terrain,
          materials,
          detail,
          plants: scatterPatch(
            x,
            y,
            sample,
            input.seed,
            local.some((s) => s.kind === "structure" && s.layout?.style.landmark === "hall"),
          ),
          water,
        });
      }
    }
    for (const key of this.environment.keys()) {
      if (!keys.has(key)) {
        this.environment.delete(key);
      }
    }
    this.cache = patches;
    this.signatures = signatures;
    return patches;
  }
}
