import type { BodyId, FocusTarget, GraphicsSettings } from "./scene.generated";
/** Actual live blast dependencies, encoded compositor work and GPU lifetime. */
export interface BlastDiagnostics {
  ready: boolean;
  sites: number;
  target: { extent: [number, number]; bytes: number; format: string; encoded: boolean } | null;
  bytes: number;
  allocations: number;
  retirements: number;
  retired: number;
  encodedFrames: number;
  draws: number;
  uniformBytes: number;
  uniformAllocations: number;
  uniformRetirements: number;
  buffer: string;
  shake: [number, number] | null;
  sample: {
    coordinate: [number, number, number];
    seed: number;
    entity: string;
    warmFrames: number;
    age: number | null;
    missile: [number, number, number];
    missileVisible: boolean;
    waveTag: number;
  }[];
}

/** Renderer-local crater event timelines gated by queued GPU preparation. */
export interface NuclearTimelineDiagnostics {
  events: {
    coordinate: [number, number, number];
    start: number | null;
    motionStart: number | null;
  }[];
}

export interface CraterDiagnostics {
  ready: boolean;
  sites: number;
  pending: number;
  building: boolean;
  uniforms: number;
  meshBytes: number;
  geometryBuilds: number;
  uniformWrites: number;
  retiredUniforms: number;
  sample: {
    coordinate: [number, number, number];
    seed: number;
    entity: string;
    buffer: string;
    meshes: string[];
    warmFrames: number;
    coverage: number;
    entryScale: number;
    fragments: number;
    fragmentSample: { entity: string; settledPosition: number[]; seedColor: number[] }[];
  }[];
}

/** Native worksite ownership and public appearance, never authoritative game state. */
export interface WorksiteDiagnostics {
  ready: boolean;
  sites: number;
  building: boolean;
  uniforms: number;
  meshBytes: number;
  geometryBuilds: number;
  uniformWrites: number;
  retiredUniforms: number;
  highlightBuffer: string | null;
  sample: {
    coordinate: [number, number, number];
    kind: "Mining" | "Reserved";
    seed: number;
    entity: string;
    mesh: string;
    buffer: string;
    born: number;
    destroy: number | null;
    cart: number[];
    collapseParts: number;
    collapse: number[][];
    color: number[];
    warmFrames: number;
  }[];
}

/** Actual special-surface ownership and uploaded shader state. */
export interface SpecialSurfaceDiagnostics extends Omit<WorksiteDiagnostics, "sample"> {
  accessoryMeshes: number;
  fenceBytes: number;
  sample: {
    coordinate: [number, number, number];
    kind: "Volcano" | "Mohole";
    seed: number;
    entity: string;
    mesh: string;
    buffer: string;
    born: number;
    destroy: number | null;
    clock: number[];
    collapse: number[][];
    warmFrames: number;
  }[];
}

/** Bounded shared sprite ownership and actual native simulation samples. */
export interface ParticleDiagnostics {
  ready: boolean;
  sprites: number;
  pools: number;
  vents: number;
  pending: number;
  uniforms: number;
  spawns: number;
  retiredUniforms: number;
  paletteBuffer: string | null;
  imageMips: number | null;
  imageSize: [number, number] | null;
  sample: {
    coordinate: [number, number, number];
    kind: "Placement" | "Volcano" | "Mining" | "Mohole";
    seed: number;
    born: number;
    poolBorn: number;
    warmFrames: number;
    positions: number[][];
  }[];
  ventSample: {
    coordinate: [number, number, number];
    warmFrames: number;
    sprites: number;
    sample: { node: string; position: number[]; scale: number; tag: number }[];
  }[];
}

/** Read-only render diagnostics; these never feed authoritative gameplay state. */
export interface WorldTreeDiagnostics {
  ready: boolean;
  trees: number;
  uniforms: number;
  uniformWrites: number;
  retiredUniforms: number;
  geometryBuilds: number;
  mesh: string | null;
  groundMesh: string | null;
  sample: {
    coordinate: [number, number, number];
    seed: number;
    entity: string;
    buffer: string;
    born: number;
    destroy: number | null;
    collapse: number[];
    groundCollapse: number[];
    depths: number[];
    motionClock: number;
    warmFrames: number;
  }[];
}

export interface FaunaDiagnostics {
  ready: boolean;
  birds: number;
  spawns: number;
  graphs: number;
  clips: number;
  worldAssets: number;
  inverseBindposes: number;
  flocks: number;
  clipDuration: number | null;
  gpuSkins: number;
  skinBufferBytes: number;
  skinBufferIds: [string, string] | null;
  sample: {
    tile: [number, number, number];
    index: number;
    root: string;
    player: string | null;
    skin: string | null;
    wing: number;
    joints: number[][] | null;
    position: number[] | null;
    visible: boolean;
    warmFrames: number;
  }[];
}

/** Authored celestial bodies and queued GPU warm-up readiness. */
export interface SpaceDiagnostics {
  ready: boolean;
  focus: FocusTarget;
  seconds: number;
  bodies: {
    body: BodyId;
    entity: string;
    position: [number, number, number];
    rotation: [number, number, number, number];
    point: [number, number] | null;
    parts: number;
    surface: string | null;
    warmFrames: number;
  }[];
  sun: { point: [number, number] | null; radius: number | null };
  sunParts: number;
  sunWarmFrames: number;
  orbitLines: number;
  orbitWarmFrames: number;
  warmCamera: boolean;
  phobos: { radius: number; contentRadius: number; vertices: number; triangles: number } | null;
}

export interface OffworldDiagnostics {
  root: string | null;
  stationEntity: string | null;
  entranceScale: number;
  entranceStart: number | null;
  stationStart: number | null;
  stationScale: number;
  effects: {
    warmNodes: number;
    materials: number;
    entries: {
      owner: string;
      ordinal: number;
      serial: number;
      complete: boolean;
      last: number | null;
      groups: { kind: string; capacity: number; instances: number; visible: number }[];
    }[];
  };
  fleet: {
    owner: string;
    ordinal: number;
    entity: string | null;
    status: "Parked" | "Unavailable" | "Flying" | "Landed";
    serial: number;
    position: number[];
    rotation: number[];
    started: number | null;
    destination: BodyId | null;
  }[];
  ready: boolean;
  warmFrames: number;
  ships: number;
  stationParts: number;
  stationMaterials: { ior: number; transmission: number; reflectance: number }[];
  warmCamera: boolean;
}

export interface LabelDiagnostics {
  ready: boolean;
  warmFrames: number;
  warmCamera: boolean;
  glyphs: number;
  atlasPages: number;
  fontMaskBytes: number;
  materials: number;
  entries: {
    id: string;
    root: string;
    ordinal: number;
    name: string | null;
    icons: number;
    credits: number[];
    parts: number;
    scale: number | null;
    opacity: number | null;
    fontParts: number;
  }[];
}

export interface SceneDiagnostics {
  graphics: GraphicsSettings;
  boardReady: boolean;
  entryActors: { entity: string; scale: number; hidden: boolean }[];
  entry: { light: number; lightStart: number | null; tileScales: number[] };
  labels: LabelDiagnostics;
  offworld: OffworldDiagnostics;
  celestialGrids: {
    bodies: {
      body: BodyId;
      root: string;
      tiles: number;
      visibility: number;
      revision: number;
      pending: boolean;
      radius: number;
      contentRadius: number;
      buffer: string;
      sample: { id: string; point: [number, number] | null }[];
    }[];
  };
  space: SpaceDiagnostics;
  focus: FocusTarget;
  menu: {
    ready: boolean;
    root: string | null;
    position: number[];
    rotation: number[];
    moon: { parts: number; center: number[]; scale: number } | null;
    warmFrames: number;
    warmCamera: boolean;
    materials: number;
  };
  lighting: {
    background: {
      size: number;
      mips: number;
      faces: number;
      format: string;
      bytes: number;
      cpuBytes: number;
    } | null;
    views: number;
    clusterBufferAllocations: number;
    clusterViews: number;
    maps: {
      id: string;
      size: number;
      mips: number;
      faces: number;
      format: string;
      bytes: number;
    }[];
  };

  atmosphere: {
    ready: boolean;
    bodies: number;
    uniforms: number;
    bytes: number;
    buffer: string | null;
    allocations: number;
    retirements: number;
    retired: number;
    writes: number;
    profiles: [number, number, number, number][] | null;
    sample: { id: string; entities: string[]; warmFrames: number }[];
  };
  asteroids: {
    started: number;
    completed: number;
    canceled: number;
    retired: number;
    shared: boolean;
    active: {
      id: string;
      root: string;
      start: number | null;
      warmFrames: number;
      scale: number;
      parts: number;
      sprites: number;
      impact: [number, number, number];
      position: [number, number, number];
      dustSample: { entity: string; position: [number, number, number]; tag: number }[];
    } | null;
  };
  presentation: { seen: number; queued: number; sounds: number };
  nuclearDebris: {
    coordinate: [number, number, number];
    entity: string;
    position: number[];
    rotation: number[];
    scale: number[];
    tag: number;
  }[];
  blast: BlastDiagnostics;
  vegetationPoses: {
    tile: [number, number, number];
    entity: string;
    kind: string;
    position: number[];
    rotation: number[];
    scale: number[];
  }[];
  nuclearTimeline: NuclearTimelineDiagnostics;
  craters: CraterDiagnostics;
  specialSurfaces: SpecialSurfaceDiagnostics;
  worksites: WorksiteDiagnostics;
  step: number;
  settling: boolean;
  worldTrees: WorldTreeDiagnostics;
  structures: {
    pending: number;
    retiringMeshes: number;
    warmFrames: number;
    ready: boolean;
    plots: number;
    chunks: number;
    materials: number;
    groups: number;
    uniforms: number;
    groupImages: number;
    textureWrites: number;
    uniformWrites: number;
    retiredUniforms: number;
    geometryBuilds: number;
    meshIds: string[];
    bufferIds: string[];
    groupImage: string | null;
    sample: {
      coordinate: [number, number, number];
      name: string;
      parts: number[];
      groups: number;
      born: number;
      collapseTimes: number[];
    }[];
  };
  vegetation: {
    models: {
      name: string;
      extent: number;
      min: number[];
      max: number[];
      vertices: number;
      indices: number;
      parts: number;
    }[];
    sample: {
      kind: string;
      cell: [number, number];
      entity: string;
      variant: number;
      born: number;
    }[];
  };
  terrain: { revision: number; transitioning: boolean };
  camera: {
    framing: boolean;
    yaw: number;
    pitch: number;
    distance: number;
    hovered: string | null;
  };
  tiles: {
    id: string;
    entity: string;
    occupant: string | null;
    point: [number, number] | null;
  }[];
}
