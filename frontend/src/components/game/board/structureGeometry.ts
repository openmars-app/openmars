import * as THREE from "three";
import { MARS_RELIEF_DEPTH } from "./marsRelief";
import { collapseParameters } from "./nuclearTransitions";
import { mergeGeometries } from "three/examples/jsm/utils/BufferGeometryUtils.js";
import {
  insideBuilding,
  onEntrancePath,
  randomSequence,
  CAPITAL_WALL_APOTHEM,
  structureWallDistance,
  structureHexWallApothem,
  type StructureBuilding,
  type StructureLayout,
  type StructurePlot,
} from "./structureLayout";
import {
  boardCenter,
  projectBoardPoint,
  nearestOnRoad,
  createRoadRibbon,
} from "./landscapeGeometry";
export function createGeometry(layout: StructureLayout, plot?: StructurePlot) {
  const { style } = layout;
  const batches: THREE.BufferGeometry[][] = Array.from({ length: 19 }, () => []);
  const box = (x: number, y: number, z: number, w: number, d: number, h: number, mat: number) => {
    const geo = new THREE.BoxGeometry(
      w,
      d,
      h,
      Math.max(1, Math.ceil(w / 0.012)),
      Math.max(1, Math.ceil(d / 0.012)),
    );
    geo.translate(x, y, z + h / 2);
    geo.clearGroups();
    batches[mat].push(geo);
  };
  const disc = (
    x: number,
    y: number,
    z: number,
    radius: number,
    height: number,
    mat: number,
    sides = 16,
  ) => {
    const geo = new THREE.CylinderGeometry(radius, radius, height, sides);
    geo.rotateX(Math.PI / 2);
    geo.translate(x, y, z);
    geo.clearGroups();
    batches[mat].push(geo);
  };
  const beam = (
    from: THREE.Vector3,
    to: THREE.Vector3,
    width: number,
    height: number,
    mat: number,
  ) => {
    const delta = to.clone().sub(from);
    const geo = new THREE.BoxGeometry(
      delta.length(),
      width,
      height,
      Math.max(1, Math.ceil(delta.length() / 0.012)),
    );
    geo.applyQuaternion(
      new THREE.Quaternion().setFromUnitVectors(new THREE.Vector3(1, 0, 0), delta.normalize()),
    );
    geo.translate((from.x + to.x) / 2, (from.y + to.y) / 2, (from.z + to.z) / 2);
    geo.clearGroups();
    batches[mat].push(geo);
  };
  const dome = (x: number, y: number, z: number, radius: number, flatten: number, mat: number) => {
    const large = radius > 0.05;
    let segments = radius > 0.01 ? 20 : 12;
    if (large) {
      segments = 64;
    }
    const shell = new THREE.SphereGeometry(
      radius,
      segments,
      segments / 2,
      0,
      Math.PI * 2,
      0,
      Math.PI / 2,
    );
    shell.rotateX(Math.PI / 2);
    shell.scale(1, 1, flatten);
    shell.translate(x, y, z);
    batches[mat].push(shell);
    let ribs = large ? 6 : 3;
    if (radius < 0.006) {
      ribs = 0;
    }
    for (let i = 0; i < ribs; i++) {
      const arch = new THREE.TorusGeometry(
        radius + 0.0005,
        0.0008,
        large ? 6 : 3,
        segments,
        Math.PI,
      );
      arch.rotateX(Math.PI / 2);
      arch.rotateZ((i * Math.PI) / ribs);
      arch.scale(1, 1, flatten);
      arch.translate(x, y, z);
      batches[4].push(arch);
    }
  };
  const ring = (
    x: number,
    y: number,
    z: number,
    radius: number,
    thickness: number,
    mat: number,
  ) => {
    const geo = new THREE.TorusGeometry(
      radius,
      thickness,
      radius > 0.07 ? 6 : 4,
      radius > 0.07 ? 128 : 24,
    );
    geo.translate(x, y, z);
    batches[mat].push(geo);
  };

  const hasGatedWall = style.perimeter !== "open" && style.cover !== "dome";
  if (style.ground === "recessed") {
    const shape = new THREE.Shape();
    shape.absarc(0, 0, 0.067, 0, Math.PI * 2, false);
    const hole = new THREE.Path();
    hole.absarc(0, 0, 0.054, 0, Math.PI * 2, true);
    shape.holes.push(hole);
    const terrace = new THREE.ExtrudeGeometry(shape, {
      depth: 0.024,
      bevelEnabled: false,
      curveSegments: 48,
    });
    terrace.translate(0, 0, 0.007);
    batches[16].push(terrace);
    disc(0, 0, 0.008, 0.054, 0.002, 12);
    ring(0, 0, 0.032, 0.054, 0.002, 6);
    for (const x of [-0.032, 0.032]) {
      box(x, 0.022, 0.009, 0.024, 0.01, 0.017, 12);
      box(x, 0.0165, 0.012, 0.018, 0.001, 0.009, 11);
      box(x, 0.021, 0.026, 0.028, 0.015, 0.003, 17);
    }
  }
  const curb = createRoadRibbon(layout.roads, 0.0012, 0.0076, false);
  const asphalt = createRoadRibbon(layout.roads, 0, 0.008, false);
  curb.deleteAttribute("connectionBirth");
  asphalt.deleteAttribute("connectionBirth");
  batches[4].push(curb);
  batches[13].push(asphalt);

  const facade = (
    building: StructureBuilding,
    z: number,
    width: number,
    depth: number,
    height: number,
  ) => {
    const geo = new THREE.BoxGeometry(width, height, depth);
    const uv = geo.getAttribute("uv");
    for (let face = 0; face < 6; face++) {
      const span = face < 2 ? depth : width;
      for (let corner = 0; corner < 4; corner++) {
        const i = face * 4 + corner;
        uv.setXY(i, (uv.getX(i) * span) / 0.036, (uv.getY(i) * height) / 0.088);
      }
    }
    geo.rotateX(Math.PI / 2);
    geo.translate(building.x, building.y, z + height / 2);
    geo.clearGroups();
    batches[building.facade].push(geo);
    box(building.x, building.y, z + height, width + 0.001, depth + 0.001, 0.002, 4);
  };

  const dishRandom = randomSequence(7321);
  const dishCandidates = layout.buildings
    .map((building, index) => ({ building, index, rank: dishRandom() }))
    .filter(({ building, index }) => !building.round && index % 2 === 1)
    .sort((a, b) => a.rank - b.rank);
  const dishIndices = new Set(
    dishCandidates.slice(0, Math.round(dishCandidates.length * 0.7)).map(({ index }) => index),
  );
  for (const [index, b] of layout.buildings.entries()) {
    const batchStarts = batches.map((batch) => batch.length);
    const finishBuilding = () => {
      for (const [materialIndex, batch] of batches.entries()) {
        for (let i = batchStarts[materialIndex]; i < batch.length; i++) {
          if (materialIndex < 3 || materialIndex === 18) {
            batch[i].userData.tint = b.tint;
          }
          batch[i].userData.collapseGroup = index + 1;
          batch[i].userData.collapsePivot = [b.x, b.y, b.z];
          batch[i].translate(-b.x, -b.y, 0);
          batch[i].rotateZ(b.rotation);
          batch[i].translate(b.x, b.y, 0);
        }
      }
      if (b.entranceRoad) {
        const target = new THREE.Vector3(b.entranceRoad.x, b.entranceRoad.y, b.z + 0.0004);
        const center = new THREE.Vector3(b.x, b.y, b.z + 0.0004);
        const direction = target.clone().sub(center).normalize();
        const start = center.addScaledVector(direction, b.depth / 2);
        const end = target.addScaledVector(direction, -b.entranceRoad.halfWidth - 0.0005);
        if (end.clone().sub(start).dot(direction) > 0) {
          beam(start, end, 0.004, 0.0008, 4);
        }
      }
    };
    const base = b.z + 0.005;
    if (b.landmark && style.landmark === "headquarters") {
      // One reserved campus footprint keeps streets and neighboring buildings outside the wings.
      box(b.x, b.y, b.z, b.width, b.depth, 0.003, 5);
      const office = (x: number, y: number, z: number, w: number, d: number, h: number) => {
        facade({ ...b, x: b.x + x, y: b.y + y, facade: 2 }, base + z, w, d, h);
        box(b.x + x, b.y + y, base + z + h + 0.002, w * 0.9, d * 0.88, 0.0015, 17);
      };
      // Broad terraces, an offset executive tower and unequal office wings frame the forecourt.
      office(0, 0.026, 0, 0.116, 0.046, 0.03);
      office(-0.009, 0.029, 0.032, 0.085, 0.035, 0.024);
      office(-0.025, 0.032, 0.058, 0.033, 0.027, b.height - 0.058);
      office(-0.046, -0.024, 0, 0.027, 0.054, 0.024);
      office(0.046, -0.014, 0, 0.025, 0.036, 0.035);
      for (const x of [-0.056, -0.026, 0.017, 0.055]) {
        box(b.x + x, b.y + 0.002, base, 0.003, 0.004, 0.031, 18);
      }
      // Recessed glass lobby and a small amber entrance strip.
      box(b.x - 0.004, b.y + 0.001, base + 0.002, 0.026, 0.003, 0.016, 8);
      box(b.x - 0.004, b.y - 0.002, base + 0.018, 0.034, 0.009, 0.002, 17);
      box(b.x - 0.004, b.y - 0.006, base + 0.017, 0.018, 0.001, 0.001, 11);
      box(b.x - 0.004, b.y - 0.025, b.z + 0.003, 0.017, 0.051, 0.001, 4);
      // Rooftop service housings give the terraces readable scale from above.
      for (const [x, y, z] of [
        [0.028, 0.03, 0.034],
        [-0.045, -0.027, 0.028],
        [0.046, -0.015, 0.039],
      ]) {
        box(b.x + x, b.y + y, base + z, 0.008, 0.011, 0.004, 5);
      }
      finishBuilding();
      continue;
    }
    if (b.landmark && style.landmark === "hall") {
      disc(b.x, b.y, b.z + 0.0025, 0.031, 0.005, 4, 48);
      const corners = [
        new THREE.Vector3(-0.022, -0.022, 0),
        new THREE.Vector3(0.022, -0.022, 0),
        new THREE.Vector3(0.022, 0.022, 0),
        new THREE.Vector3(-0.022, 0.022, 0),
      ];
      const face = (points: THREE.Vector3[], mat: number) => {
        const geo = new THREE.BufferGeometry();
        geo.setAttribute(
          "position",
          new THREE.Float32BufferAttribute(
            points.flatMap((p) => [p.x + b.x, p.y + b.y, p.z + base]),
            3,
          ),
        );
        geo.setAttribute(
          "uv",
          new THREE.Float32BufferAttribute(
            points.flatMap((p) => [p.x * 20, p.z * 20]),
            2,
          ),
        );
        geo.setIndex([0, 1, 2, 0, 2, 3]);
        batches[mat].push(geo);
      };
      for (let side = 0; side < 4; side++) {
        const a = corners[side],
          c = corners[(side + 1) % 4];
        const point = (t: number, z: number) =>
          a
            .clone()
            .lerp(c, t)
            .multiplyScalar(1 - z / (b.height * 1.035))
            .setZ(z);
        const left = point(0.02, 0),
          right = point(0.98, 0);
        const shoulder = point(0.98, b.height * (0.85 + side * 0.012));
        const tip = point(0.08, b.height * [1, 0.945, 0.98, 0.915][side]);
        face([left, right, shoulder, tip], 9);
        for (const [from, to] of [
          [left, tip],
          [right, shoulder],
        ]) {
          beam(
            from.clone().add(new THREE.Vector3(b.x, b.y, base)),
            to.clone().add(new THREE.Vector3(b.x, b.y, base)),
            0.00045,
            0.00045,
            17,
          );
        }
        // Fine curtain-wall mullions stop below the staggered open crown.
        for (let z = 0.003; z < b.height * 0.83; z += 0.0035) {
          const from = point(0.025, z).add(new THREE.Vector3(b.x, b.y, base));
          const to = point(0.975, z).add(new THREE.Vector3(b.x, b.y, base));
          beam(from, to, 0.00018, 0.00025, 17);
        }
        for (const t of [0.25, 0.5, 0.75]) {
          beam(
            point(t, 0).add(new THREE.Vector3(b.x, b.y, base)),
            point(t, b.height * 0.84).add(new THREE.Vector3(b.x, b.y, base)),
            0.0002,
            0.0002,
            17,
          );
        }
        const coreTop = point(0.5, b.height * 0.86);
        face([point(0, 0), point(1, 0), coreTop, coreTop], 8);
      }
      finishBuilding();
      continue;
    }
    if (b.landmark && style.landmark === "globe") {
      const radius = b.width * 0.42;
      const center = base + 0.006 + radius;
      disc(b.x, b.y, base + 0.002, b.width / 2, 0.005, 17, 48);
      disc(b.x, b.y, base + 0.005, radius * 0.6, 0.006, 4, 32);
      for (const [scale, material] of [
        [0.975, 9],
        [1, 10],
      ]) {
        const sphere = new THREE.SphereGeometry(radius * scale, 40, 24);
        sphere.translate(b.x, b.y, center);
        batches[material].push(sphere);
      }
      for (let i = 0; i < 6; i++) {
        const rib = new THREE.TorusGeometry(radius + 0.0004, 0.0005, 4, 48);
        rib.rotateX(Math.PI / 2);
        rib.rotateZ((i * Math.PI) / 6);
        rib.translate(b.x, b.y, center);
        batches[17].push(rib);
      }
      for (const latitude of [-0.5, 0, 0.5]) {
        ring(
          b.x,
          b.y,
          center + radius * latitude,
          radius * Math.sqrt(1 - latitude * latitude) + 0.0004,
          0.0005,
          17,
        );
      }
      finishBuilding();
      continue;
    }
    box(b.x, b.y, b.z, b.width + 0.002, b.depth + 0.002, 0.005, 5);
    const shadow = new THREE.PlaneGeometry(b.width + 0.014, b.depth + 0.014);
    shadow.translate(b.x, b.y, b.z + 0.0008);
    batches[14].push(shadow);
    if (b.landmark && style.landmark === "factory") {
      box(b.x, b.y, base, b.width, b.depth, b.height, 17);
      for (const side of [-1, 1]) {
        for (let rib = 1; rib < 7; rib++) {
          box(
            b.x - b.width / 2 + (rib * b.width) / 7,
            b.y + (side * b.depth) / 2,
            base,
            0.001,
            0.001,
            b.height,
            5,
          );
        }
      }
      const roofZ = base + b.height + 0.002;
      const bayDepth = b.depth / 3;
      for (let bay = 0; bay < 3; bay++) {
        const y = b.y - b.depth / 2 + bayDepth * (bay + 0.5);
        for (const side of [-1, 1]) {
          const roof = new THREE.BoxGeometry(b.width + 0.002, bayDepth * 0.57, 0.0015);
          roof.rotateX(side * 0.35);
          roof.translate(b.x, y + side * bayDepth * 0.24, roofZ + 0.002);
          batches[17].push(roof);
        }
        box(b.x, y, roofZ + 0.004, b.width * 0.75, 0.002, 0.001, 9);
      }
      for (const side of [-1, 1]) {
        box(
          b.x + side * b.width * 0.25,
          b.y - b.depth / 2 - 0.001,
          base,
          b.width * 0.23,
          0.001,
          b.height * 0.64,
          8,
        );
      }
      finishBuilding();
      continue;
    }
    if (b.habitat) {
      disc(b.x, b.y, base - 0.001, b.width * 0.53, 0.003, 17, 24);
      const shell = new THREE.SphereGeometry(1, 16, 8, 0, Math.PI * 2, 0, Math.PI * 0.72);
      shell.rotateX(Math.PI / 2);
      shell.scale(b.width * 0.52, b.depth * 0.52, b.height * 0.72);
      shell.translate(b.x, b.y, base + b.height * 0.35);
      batches[18].push(shell);
      dome(b.x, b.y, base + b.height * 0.88, b.width * 0.22, 0.5, 9);
      ring(b.x, b.y, base + b.height * 0.35, b.width * 0.52, 0.0005, 17);
      box(b.x, b.y - b.depth * 0.48, base, b.width * 0.35, 0.004, 0.008, 17);
      box(b.x, b.y - b.depth * 0.51, base + 0.001, b.width * 0.23, 0.001, 0.005, 8);
      finishBuilding();
      continue;
    }
    if (b.round) {
      const geo = new THREE.CylinderGeometry(b.width / 2, b.width / 2, b.height, 20);
      const uv = geo.getAttribute("uv");
      for (let i = 0; i < uv.count; i++) {
        uv.setXY(i, (uv.getX(i) * Math.PI * b.width) / 0.036, (uv.getY(i) * b.height) / 0.088);
      }
      geo.rotateX(Math.PI / 2);
      geo.translate(b.x, b.y, base + b.height / 2);
      geo.clearGroups();
      batches[1].push(geo);
      disc(b.x, b.y, base + b.height, b.width / 2 + 0.003, 0.004, 4);
      if (b.landmark || index % 3 === 0) {
        dome(b.x, b.y, base + b.height + 0.002, b.width / 2, 0.8, 9);
      } else {
        disc(b.x, b.y, base + b.height + 0.003, b.width * 0.4, 0.003, 5);
        disc(b.x, b.y, base + b.height + 0.006, b.width * 0.18, 0.004, 8);
      }
      box(b.x, b.y - b.width / 2, base, 0.006, 0.001, 0.007, 8);
      finishBuilding();
      continue;
    }
    let roofWidth = b.width;
    let roofDepth = b.depth;
    let roof = base + b.height + 0.002;
    if (b.stepped && b.height > 0.045) {
      const lower = Math.round((b.height * 0.55) / 0.011) * 0.011;
      facade(b, base, b.width, b.depth, lower);
      roofWidth *= 0.7;
      roofDepth *= 0.7;
      facade(b, base + 0.002 + lower, roofWidth, roofDepth, b.height - lower);
      roof += 0.002;
    } else {
      facade(b, base, b.width, b.depth, b.height);
    }
    box(b.x, b.y, roof, roofWidth * 0.75, roofDepth * 0.75, 0.001, 5);
    box(b.x - roofWidth * 0.16, b.y, roof + 0.001, roofWidth * 0.35, roofDepth * 0.58, 0.002, 8);
    box(b.x + roofWidth * 0.26, b.y, roof + 0.001, roofWidth * 0.18, roofDepth * 0.3, 0.003, 3);
    box(b.x, b.y - b.depth / 2 - 0.002, base + 0.007, b.width * 0.35, 0.008, 0.002, 6);
    box(b.x, b.y - b.depth / 2 - 0.001, base, 0.007, 0.001, 0.007, 8);
    if (style.details.includes("antennas") && index % 2 === 0) {
      disc(b.x + roofWidth * 0.28, b.y, roof + 0.015, 0.0007, 0.029, 4, 8);
      box(b.x + roofWidth * 0.28, b.y, roof + 0.024, 0.012, 0.001, 0.001, 6);
    }
    if (style.details.includes("dishes") && dishIndices.has(index)) {
      disc(b.x, b.y, roof + 0.007, 0.0012, 0.014, 17, 12);
      const random = randomSequence(931 + index * 137);
      const azimuth = random() * Math.PI * 2;
      const tilt = 0.25 + random() * 0.6;
      const direction = new THREE.Vector3(
        Math.sin(tilt) * Math.cos(azimuth),
        Math.sin(tilt) * Math.sin(azimuth),
        Math.cos(tilt),
      );
      const transform = new THREE.Matrix4().compose(
        new THREE.Vector3(b.x, b.y, roof + 0.014),
        new THREE.Quaternion().setFromUnitVectors(new THREE.Vector3(0, 1, 0), direction),
        new THREE.Vector3(1, 1, 1),
      );
      const dish = new THREE.LatheGeometry(
        [
          new THREE.Vector2(0, 0),
          new THREE.Vector2(0.003, 0.0004),
          new THREE.Vector2(0.006, 0.0016),
          new THREE.Vector2(0.009, 0.0036),
          new THREE.Vector2(0.012, 0.0064),
        ],
        40,
      );
      dish.applyMatrix4(transform);
      batches[18].push(dish);
      const rim = new THREE.TorusGeometry(0.012, 0.0005, 6, 40);
      rim.rotateX(Math.PI / 2);
      rim.translate(0, 0.0064, 0);
      rim.applyMatrix4(transform);
      batches[17].push(rim);
      const receiver = new THREE.Vector3(0, 0.014, 0).applyMatrix4(transform);
      for (let support = 0; support < 3; support++) {
        const angle = (support * Math.PI * 2) / 3;
        const anchor = new THREE.Vector3(
          Math.cos(angle) * 0.011,
          0.0055,
          Math.sin(angle) * 0.011,
        ).applyMatrix4(transform);
        beam(anchor, receiver, 0.0005, 0.0005, 17);
      }
      const feed = new THREE.SphereGeometry(0.0012, 8, 6);
      feed.translate(receiver.x, receiver.y, receiver.z);
      batches[5].push(feed);
    }
    if (style.details.includes("vents")) {
      for (let i = 0; i < 3; i++) {
        disc(b.x - 0.008 + i * 0.008, b.y, roof + 0.006, 0.002, 0.012, 3, 12);
        disc(b.x - 0.008 + i * 0.008, b.y, roof + 0.012, 0.003, 0.002, 6, 12);
      }
    }
    finishBuilding();
  }

  for (const [index, fixture] of layout.fixtures.entries()) {
    const starts = batches.map((batch) => batch.length);
    const { x, y, z, radius, height } = fixture;
    if (fixture.kind === "smokestack") {
      disc(x, y, z + height / 2, radius, height, 5, 16);
      for (const level of [0.15, 0.8]) {
        ring(x, y, z + height * level, radius + 0.0004, 0.0007, 17);
      }
      disc(x, y, z + height, radius * 0.75, 0.0005, 8, 16);
      ring(x, y, z + height + 0.0003, radius * 0.9, 0.001, 17);
    } else if (fixture.kind === "tank") {
      disc(x, y, z + 0.001, radius + 0.002, 0.002, 4, 20);
      disc(x, y, z + height / 2, radius, height, 17, 20);
      dome(x, y, z + height, radius, 0.28, 17);
      ring(x, y, z + height * 0.2, radius + 0.0002, 0.0005, 5);
    } else if (fixture.kind === "ground-exhaust") {
      const angle = fixture.rotation ?? 0;
      const direction = new THREE.Vector3(Math.cos(angle), Math.sin(angle), 0);
      disc(x, y, z + 0.0015, radius, 0.003, 4, 16);
      ring(x, y, z + 0.003, radius * 0.6, 0.001, 17);
      const mouth = new THREE.Vector3(x, y, z + height).addScaledVector(direction, radius * 0.8);
      const path = new THREE.CatmullRomCurve3([
        new THREE.Vector3(x, y, z + 0.002),
        new THREE.Vector3(x, y, z + height * 0.7),
        mouth,
      ]);
      batches[17].push(new THREE.TubeGeometry(path, 12, radius * 0.34, 10, false));
      const rotation = new THREE.Quaternion().setFromUnitVectors(
        new THREE.Vector3(0, 0, 1),
        direction,
      );
      const lip = new THREE.TorusGeometry(radius * 0.35, 0.0007, 4, 16);
      lip.applyQuaternion(rotation);
      lip.translate(mouth.x, mouth.y, mouth.z);
      batches[5].push(lip);
      for (const [scale, material] of [
        [0.34, 8],
        [0.12, 11],
      ]) {
        const opening = new THREE.CircleGeometry(radius * scale, 16);
        opening.applyQuaternion(rotation);
        opening.translate(
          mouth.x + direction.x * 0.0001 * (material === 11 ? 2 : 1),
          mouth.y + direction.y * 0.0001 * (material === 11 ? 2 : 1),
          mouth.z,
        );
        batches[material].push(opening);
      }
    } else if (fixture.kind === "shaft") {
      disc(x, y, z + height / 2, radius, height, 5, 24);
      disc(x, y, z + height, radius * 0.82, 0.0005, 8, 24);
      ring(x, y, z + height + 0.0005, radius * 0.9, 0.001, 17);
      disc(x, y, z + height + 0.001, radius * 0.17, 0.002, 17, 12);
      for (let blade = 0; blade < 5; blade++) {
        const angle = (blade * Math.PI * 2) / 5;
        const vane = new THREE.BoxGeometry(radius * 0.57, radius * 0.22, 0.0008);
        vane.rotateZ(0.5);
        vane.translate(radius * 0.48, 0, 0);
        vane.rotateZ(angle);
        vane.translate(x, y, z + height + 0.0008);
        batches[17].push(vane);
      }
    } else if (fixture.kind === "accent-light") {
      const light = new THREE.BoxGeometry(height, radius, radius * 2);
      light.rotateZ(fixture.rotation ?? 0);
      light.translate(x, y, z);
      batches[fixture.accent === "cyan" ? 9 : 11].push(light);
    }
    for (const [material, batch] of batches.entries()) {
      for (let i = starts[material]; i < batch.length; i++) {
        batch[i].userData.collapseGroup = layout.buildings.length + index + 1;
        batch[i].userData.collapsePivot = [x, y, z];
      }
    }
  }
  for (const pipe of layout.pipes) {
    const points = pipe.points.map((p) => new THREE.Vector3(p.x, p.y, p.z));
    const path = new THREE.CatmullRomCurve3(points);
    batches[17].push(new THREE.TubeGeometry(path, 8, pipe.radius, 8, false));
    for (const t of [0.15, 0.5, 0.85]) {
      const point = path.getPoint(t);
      const collar = new THREE.TorusGeometry(pipe.radius + 0.0004, 0.0004, 4, 10);
      collar.applyQuaternion(
        new THREE.Quaternion().setFromUnitVectors(new THREE.Vector3(0, 0, 1), path.getTangent(t)),
      );
      collar.translate(point.x, point.y, point.z);
      batches[6].push(collar);
      box(point.x, point.y, 0.007, 0.004, 0.003, Math.max(0.001, point.z - 0.007), 5);
    }
  }

  if (style.connections !== "paths") {
    const buildings = layout.buildings.filter((b) => !b.landmark);
    for (let i = 0; i < buildings.length - 1; i += style.connections === "skybridges" ? 6 : 2) {
      const a = buildings[i];
      const b = buildings[i + 1];
      const from = new THREE.Vector3(a.x, a.y, a.z + 0.008);
      const to = new THREE.Vector3(b.x, b.y, b.z + 0.008);
      if (style.connections === "skybridges") {
        const z = Math.min(a.z + a.height, b.z + b.height) * 0.6 + 0.012;
        from.z = to.z = Math.max(z, a.z + 0.01, b.z + 0.01);
      }
      beam(from, to, 0.01, 0.003, 4);
      const perpendicular = new THREE.Vector3(-(b.y - a.y), b.x - a.x, 0)
        .normalize()
        .multiplyScalar(0.0045);
      for (const side of [-1, 1]) {
        const start = from.clone().addScaledVector(perpendicular, side);
        const end = to.clone().addScaledVector(perpendicular, side);
        start.z += 0.004;
        end.z += 0.004;
        beam(start, end, 0.001, 0.006, 8);
      }
      if (style.connections === "enclosed") {
        from.z += 0.008;
        to.z += 0.008;
        beam(from, to, 0.012, 0.002, 4);
      }
    }
  }

  const hexApothem = structureHexWallApothem(layout);
  if (hasGatedWall && hexApothem !== null) {
    const height = style.perimeter === "high-wall" ? 0.025 : 0.012;
    const gatehouses = style.details.includes("gatehouses");
    const halfSide = hexApothem * Math.tan(Math.PI / 6);
    for (let side = 0; side < 6; side++) {
      const angle = (side * Math.PI) / 3;
      const center = new THREE.Vector3(
        Math.cos(angle) * hexApothem,
        Math.sin(angle) * hexApothem,
        0.007 + height / 2,
      );
      const tangent = new THREE.Vector3(-Math.sin(angle), Math.cos(angle), 0);
      const gate = layout.entrances.some((entrance) => Math.abs(entrance - angle) < 1e-6);
      const spans = gate
        ? [
            [-halfSide, -0.01],
            [0.01, halfSide],
          ]
        : [[-halfSide, halfSide]];
      for (const [a, b] of spans) {
        const wallSection = (width: number, z: number, thickness: number, material: number) => {
          const shape = new THREE.Shape();
          const points = [
            [a, -1],
            [b, -1],
            [b, 1],
            [a, 1],
          ].map(([along, edge]) => {
            const apothem = hexApothem + (edge * width) / 2;
            const end =
              Math.abs(Math.abs(along) - halfSide) < 1e-8
                ? Math.sign(along) * apothem * Math.tan(Math.PI / 6)
                : along;
            return new THREE.Vector2(
              Math.cos(angle) * apothem + tangent.x * end,
              Math.sin(angle) * apothem + tangent.y * end,
            );
          });
          shape.setFromPoints(points);
          shape.closePath();
          const section = new THREE.ExtrudeGeometry(shape, {
            depth: thickness,
            bevelEnabled: false,
            steps: 1,
          });
          section.translate(0, 0, z);
          section.clearGroups();
          batches[material].push(section);
        };
        wallSection(gatehouses ? 0.007 : 0.004, 0.007, height, 4);
        wallSection(gatehouses ? 0.008 : 0.005, 0.007 + height, 0.0015, 17);
      }
      if (gate && gatehouses) {
        const starts = batches.map((batch) => batch.length);
        for (const offset of [-0.019, 0.019]) {
          box(hexApothem - 0.007, offset, 0.007, 0.024, 0.018, height + 0.009, 5);
          box(hexApothem - 0.007, offset, 0.007 + height + 0.009, 0.026, 0.02, 0.003, 17);
          box(hexApothem + 0.0051, offset, 0.018, 0.0008, 0.011, 0.006, 8);
          box(hexApothem + 0.006, Math.sign(offset) * 0.011, 0.014, 0.001, 0.001, 0.005, 11);
        }
        box(hexApothem - 0.007, 0, 0.007 + height, 0.022, 0.028, 0.007, 17);
        for (const [material, batch] of batches.entries()) {
          for (let i = starts[material]; i < batch.length; i++) {
            batch[i].rotateZ(angle);
            batch[i].userData.collapseGroup =
              layout.buildings.length + layout.fixtures.length + side + 1;
            batch[i].userData.collapsePivot = [center.x, center.y, 0.007];
          }
        }
      } else if (gate) {
        for (const offset of [-0.011, 0.011]) {
          const post = center.clone().addScaledVector(tangent, offset);
          box(post.x, post.y, 0.007, 0.004, 0.004, height + 0.004, 17);
        }
        const from = center.clone().addScaledVector(tangent, -0.011);
        const to = center.clone().addScaledVector(tangent, 0.011);
        from.z = to.z = 0.007 + height + 0.003;
        beam(from, to, 0.005, 0.003, 17);
      }
    }
  } else if (hasGatedWall) {
    const wallHeight = style.perimeter === "high-wall" ? 0.025 : 0.01;
    const inner = 0.118;
    const outer = 0.124;
    const gateHalfAngle = Math.asin(0.009 / inner);
    const entrances = [...layout.entrances].sort((a, b) => a - b);
    for (let i = 0; i < entrances.length; i++) {
      const current = entrances[i];
      const next = i + 1 < entrances.length ? entrances[i + 1] : entrances[0] + Math.PI * 2;
      const start = current + Math.PI / 2 + gateHalfAngle;
      const arcLength = next - current - gateHalfAngle * 2;
      const wall = new THREE.LatheGeometry(
        [
          new THREE.Vector2(inner, 0),
          new THREE.Vector2(outer, 0),
          new THREE.Vector2(outer, wallHeight),
          new THREE.Vector2(inner, wallHeight),
          new THREE.Vector2(inner, 0),
        ],
        64,
        start,
        arcLength,
      );
      wall.rotateX(Math.PI / 2);
      wall.translate(0, 0, 0.007);
      wall.userData.circularWall = true;
      wall.userData.wallArcLength = arcLength;
      wall.userData.wallRadius = (inner + outer) / 2;
      batches[4].push(wall);
      const cap = new THREE.TorusGeometry(0.121, 0.0015, 8, 64, arcLength);
      cap.rotateZ(start - Math.PI / 2);
      cap.translate(0, 0, 0.007 + wallHeight);
      batches[17].push(cap);
      for (const angle of [start - Math.PI / 2, start + arcLength - Math.PI / 2]) {
        const end = new THREE.Shape();
        end.moveTo(inner, 0);
        end.lineTo(outer, 0);
        end.lineTo(outer, wallHeight);
        end.lineTo(inner, wallHeight);
        end.closePath();
        const endCap = new THREE.ShapeGeometry(end);
        endCap.rotateX(Math.PI / 2);
        endCap.rotateZ(angle);
        endCap.translate(0, 0, 0.007);
        batches[4].push(endCap);
      }
      const gateAngle = current;
      const radial = new THREE.Vector3(Math.cos(gateAngle), Math.sin(gateAngle), 0);
      const tangent = new THREE.Vector3(-radial.y, radial.x, 0);
      for (const side of [-1, 1]) {
        const post = radial
          .clone()
          .multiplyScalar(0.121)
          .addScaledVector(tangent, side * 0.01);
        box(post.x, post.y, 0.007, 0.004, 0.004, wallHeight + 0.002, 17);
      }
      const from = radial.clone().multiplyScalar(0.121).addScaledVector(tangent, -0.012);
      const to = radial.clone().multiplyScalar(0.121).addScaledVector(tangent, 0.012);
      from.z = to.z = 0.007 + wallHeight + 0.002;
      beam(from, to, 0.005, 0.003, 17);
      from.addScaledVector(tangent, 0.005);
      to.addScaledVector(tangent, -0.005);
      from.z -= 0.002;
      to.z -= 0.002;
      beam(from, to, 0.002, 0.0007, 11);
    }
  }
  if (style.cover === "dome") {
    const collar = new THREE.LatheGeometry(
      [
        new THREE.Vector2(0.111, 0),
        new THREE.Vector2(0.117, 0),
        new THREE.Vector2(0.117, 0.017),
        new THREE.Vector2(0.111, 0.017),
        new THREE.Vector2(0.111, 0),
      ],
      192,
    );
    collar.rotateX(Math.PI / 2);
    collar.translate(0, 0, 0.007);
    collar.userData.circularWall = true;
    batches[4].push(collar);
    dome(0, 0, 0.024, 0.113, 0.92, 10);
    ring(0, 0, 0.024, 0.114, 0.0012, 6);
    ring(0, 0, 0.076, 0.098, 0.0007, 4);
    for (const angle of layout.entrances) {
      const starts = batches.map((batch) => batch.length);
      box(0.114, 0, 0.007, 0.025, 0.024, 0.022, 4);
      box(0.127, 0, 0.01, 0.001, 0.015, 0.016, 8);
      box(0.128, 0, 0.027, 0.002, 0.018, 0.001, 11);
      for (const [index, batch] of batches.entries()) {
        for (let i = starts[index]; i < batch.length; i++) {
          batch[i].rotateZ(angle);
        }
      }
    }
  }

  if (style.landmark === "hall") {
    const segments = 96,
      rows = 10;
    const inner = 0.018;
    const roofPoint = (angle: number, t: number) => {
      const x = Math.cos(angle),
        y = Math.sin(angle);
      const outer =
        CAPITAL_WALL_APOTHEM /
        Math.max(
          Math.abs(x),
          Math.abs(x * 0.5 + (y * Math.sqrt(3)) / 2),
          Math.abs(x * 0.5 - (y * Math.sqrt(3)) / 2),
        );
      const hole = inner / Math.max(Math.abs(x), Math.abs(y));
      const r = THREE.MathUtils.lerp(hole, outer, t);
      return new THREE.Vector3(x * r, y * r, 0.05 + 0.01 * (1 - t * t));
    };
    const positions: number[] = [],
      uvs: number[] = [],
      indices: number[] = [];
    for (let row = 0; row <= rows + 1; row++) {
      for (let i = 0; i <= segments; i++) {
        const p = roofPoint((i * Math.PI * 2) / segments, Math.min(row / rows, 1));
        if (row > rows) {
          p.z = 0.019;
        }
        positions.push(p.x, p.y, p.z);
        uvs.push(i / segments, row / rows);
        if (row <= rows && i < segments) {
          const a = row * (segments + 1) + i,
            b = a + segments + 1;
          indices.push(a, b, b + 1, a, b + 1, a + 1);
        }
      }
    }
    const roof = new THREE.BufferGeometry();
    roof.setAttribute("position", new THREE.Float32BufferAttribute(positions, 3));
    roof.setAttribute("uv", new THREE.Float32BufferAttribute(uvs, 2));
    roof.setIndex(indices);
    batches[10].push(roof);
    for (let rib = 0; rib < 6; rib++) {
      const angle = Math.PI / 6 + (rib * Math.PI) / 3;
      for (let step = 0; step < rows; step++) {
        beam(
          roofPoint(angle, step / rows),
          roofPoint(angle, (step + 1) / rows),
          0.00065,
          0.00065,
          17,
        );
      }
      const corner = roofPoint(angle, 1);
      beam(corner, corner.clone().setZ(0.019), 0.0007, 0.0007, 17);
      const next = roofPoint(angle + Math.PI / 3, 1);
      beam(corner, next, 0.0008, 0.0008, 17);
    }
    for (let side = 0; side < 4; side++) {
      const angle = Math.PI / 4 + (side * Math.PI) / 2;
      beam(roofPoint(angle, 0), roofPoint(angle + Math.PI / 2, 0), 0.001, 0.001, 17);
    }
  } else if (style.cover === "flat-glass") {
    const radius = style.ground === "recessed" ? 0.054 : 0.113;
    const z = style.ground === "recessed" ? 0.033 : 0.025;
    const glass = new THREE.CircleGeometry(radius, 96);
    glass.translate(0, 0, z);
    batches[10].push(glass);
    ring(0, 0, z + 0.001, radius, 0.002, 17);
    for (let i = -2; i <= 2; i++) {
      const offset = (i * radius) / 3;
      const reach = Math.sqrt(radius * radius - offset * offset);
      box(offset, 0, z, 0.002, reach * 2, 0.003, 17);
      box(0, offset, z, reach * 2, 0.002, 0.003, 17);
      for (let j = -2; j <= 2; j++) {
        const y = (j * radius) / 3;
        if (Math.hypot(offset, y) < radius - 0.004) {
          box(offset, y, z + 0.003, 0.004, 0.004, 0.001, 5);
          disc(offset, y, z + 0.004, 0.0008, 0.001, 17, 8);
        }
      }
    }
    for (let i = 0; i < 12; i++) {
      const angle = (i / 12) * Math.PI * 2;
      const x = Math.cos(angle) * (radius + 0.002);
      const y = Math.sin(angle) * (radius + 0.002);
      disc(x, y, z + 0.002, 0.0025, 0.002, 5, 12);
      disc(x, y, z + 0.0035, 0.001, 0.001, 17, 8);
    }
    ring(0, 0, z - 0.006, radius - 0.005, 0.0006, 11);
  }
  if (style.details.includes("pipes") && style.ground === "recessed") {
    const inletHeight = style.ground === "recessed" ? 0.029 : 0.015;
    for (const side of [-1, 1]) {
      for (let run = 0; run < 2; run++) {
        const x = side * (0.074 + run * 0.007);
        const path = new THREE.CatmullRomCurve3([
          new THREE.Vector3(side * 0.079, -0.05, 0.022 + run * 0.004),
          new THREE.Vector3(x, -0.035, 0.016),
          new THREE.Vector3(x, 0.019, 0.016),
          new THREE.Vector3(side * 0.065, 0.033 + run * 0.006, 0.019),
          new THREE.Vector3(side * 0.045, 0.03 + run * 0.006, inletHeight),
        ]);
        batches[17].push(new THREE.TubeGeometry(path, 40, 0.0018, 8, false));
        for (const t of [0.12, 0.4, 0.7, 0.95]) {
          const collar = new THREE.TorusGeometry(0.0023, 0.0006, 6, 12);
          const point = path.getPointAt(t);
          collar.applyQuaternion(
            new THREE.Quaternion().setFromUnitVectors(
              new THREE.Vector3(0, 0, 1),
              path.getTangentAt(t),
            ),
          );
          collar.translate(point.x, point.y, point.z);
          batches[6].push(collar);
        }
        for (const y of [-0.02, 0.01]) {
          box(x, y, 0.007, 0.005, 0.003, 0.007, 5);
        }
      }
      disc(side * 0.074, -0.006, 0.021, 0.0008, 0.008, 17, 8);
      ring(side * 0.074, -0.006, 0.025, 0.004, 0.0007, 6);
      box(side * 0.074, -0.006, 0.0245, 0.008, 0.001, 0.001, 17);
    }
  }
  if (style.details.includes("pipes") && style.ground !== "recessed") {
    let count = 0;
    for (const a of layout.buildings.filter((b) => b.habitat)) {
      const nearby = layout.buildings
        .filter((b) => b !== a && !b.landmark)
        .sort((b, c) => Math.hypot(a.x - b.x, a.y - b.y) - Math.hypot(a.x - c.x, a.y - c.y));
      for (const b of nearby) {
        if (Math.hypot(b.x - a.x, b.y - a.y) > 0.047) {
          break;
        }
        const points = Array.from({ length: 12 }, (_, i) => ({
          x: a.x + ((b.x - a.x) * i) / 11,
          y: a.y + ((b.y - a.y) * i) / 11,
        }));
        if (
          points.some((p) => {
            const road = nearestOnRoad(p, layout.roads);
            return road.distance < road.halfWidth + 0.002;
          })
        ) {
          continue;
        }
        if (
          points.some((p) =>
            layout.buildings.some((c) => c !== a && c !== b && insideBuilding(c, p.x, p.y, 0.002)),
          )
        ) {
          continue;
        }
        const path = new THREE.CatmullRomCurve3([
          new THREE.Vector3(a.x, a.y, a.z + 0.008),
          new THREE.Vector3(a.x + (b.x - a.x) * 0.2, a.y + (b.y - a.y) * 0.2, a.z + 0.004),
          new THREE.Vector3(a.x + (b.x - a.x) * 0.8, a.y + (b.y - a.y) * 0.8, b.z + 0.004),
          new THREE.Vector3(b.x, b.y, b.z + 0.008),
        ]);
        batches[17].push(new THREE.TubeGeometry(path, 12, 0.0012, 6, false));
        count++;
        break;
      }
      if (count >= 3) {
        break;
      }
    }
  }
  if (style.details.includes("shafts") && style.plan !== "grid") {
    for (const x of [-0.079, 0.079]) {
      disc(x, -0.05, 0.017, 0.012, 0.02, 5, 12);
      disc(x, -0.05, 0.028, 0.016, 0.003, 6, 12);
      ring(x, -0.05, 0.029, 0.01, 0.001, 11);
      box(x, -0.05, 0.03, 0.004, 0.004, 0.015, 4);
    }
  }
  if (style.lighting === "bright") {
    for (const x of [-0.051, 0.051]) {
      box(x, 0, 0.009, 0.0015, 0.215, 0.001, 11);
    }
  }

  if (style.exposure === "surface") {
    const candidates: { x: number; y: number; dx: number; dy: number }[] = [];
    for (const road of layout.roads) {
      let accumulated = 0;
      for (let i = 1; i < road.points.length; i++) {
        const a = road.points[i - 1],
          b = road.points[i];
        const length = Math.hypot(b.x - a.x, b.y - a.y);
        if (length < 1e-6) {
          continue;
        }
        const nx = -(b.y - a.y) / length,
          ny = (b.x - a.x) / length;
        const spacing = style.lighting === "bright" ? 0.028 : 0.038;
        for (let distance = spacing - accumulated; distance < length; distance += spacing) {
          for (const side of [-1, 1]) {
            const across = side * (road.width / 2 + 0.0025);
            candidates.push({
              x: a.x + ((b.x - a.x) * distance) / length + nx * across,
              y: a.y + ((b.y - a.y) * distance) / length + ny * across,
              dx: -nx * side,
              dy: -ny * side,
            });
          }
        }
        accumulated = (accumulated + length) % spacing;
      }
    }
    const placed: { x: number; y: number }[] = [];
    for (const { x, y, dx, dy } of candidates) {
      const nearest = nearestOnRoad({ x, y }, layout.roads);
      const onRoad = nearest.distance < nearest.halfWidth + 0.002;
      const radius = Math.hypot(x, y);
      const underBuilding = layout.buildings.some(
        (b) => insideBuilding(b, x, y, 0.006) || onEntrancePath(b, x, y, 0.004),
      );
      const underPlant = layout.plants.some((p) => Math.hypot(p.x - x, p.y - y) < 0.004);
      const nearLamp = placed.some((p) => Math.hypot(p.x - x, p.y - y) < 0.018);
      const nearWall =
        (style.cover === "dome" && radius > 0.102) ||
        (style.landmark === "hall" && structureWallDistance(layout, x, y) > -0.008);
      if (onRoad || underBuilding || underPlant || nearLamp || nearWall) {
        continue;
      }
      placed.push({ x, y });
      beam(
        new THREE.Vector3(x - dx * 0.001, y - dy * 0.001, 0.0074),
        new THREE.Vector3(x + dx * 0.0015, y + dy * 0.0015, 0.0074),
        0.0035,
        0.0008,
        4,
      );
      disc(x, y, 0.0165, 0.0006, 0.018, 5, 8);
      beam(
        new THREE.Vector3(x, y, 0.025),
        new THREE.Vector3(x + dx * 0.005, y + dy * 0.005, 0.025),
        0.001,
        0.001,
        5,
      );
      beam(
        new THREE.Vector3(x + dx * 0.003, y + dy * 0.003, 0.0244),
        new THREE.Vector3(x + dx * 0.006, y + dy * 0.006, 0.0244),
        0.002,
        0.0007,
        11,
      );
      const lightPool = new THREE.PlaneGeometry(0.014, 0.008);
      lightPool.rotateZ(Math.atan2(dy, dx));
      lightPool.translate(x + dx * 0.006, y + dy * 0.006, 0.009);
      batches[15].push(lightPool);
    }
  }

  const collapseBounds = new Map<number, THREE.Box3>();
  for (const geometry of batches.flat()) {
    const id = geometry.userData.collapseGroup ?? 0;
    geometry.computeBoundingBox();
    const bounds = collapseBounds.get(id) ?? new THREE.Box3();
    bounds.union(geometry.boundingBox!);
    collapseBounds.set(id, bounds);
  }
  return batches.map((geometries, materialIndex) => {
    if (geometries.length === 0) {
      return null;
    }
    const prepared = geometries.map((geo) => {
      geo.computeVertexNormals();
      const color = new THREE.Color(geo.userData.tint ?? "#ffffff");
      const colors = new Float32Array(geo.getAttribute("position").count * 3);
      for (let i = 0; i < colors.length; i += 3) {
        colors[i] = color.r;
        colors[i + 1] = color.g;
        colors[i + 2] = color.b;
      }
      geo.setAttribute("color", new THREE.BufferAttribute(colors, 3));
      const frostVertices = geo.getAttribute("position");
      const frostExposure = new Float32Array(frostVertices.count).fill(1);
      if (style.cover !== "none" && materialIndex !== 10) {
        const radius = style.cover === "flat-glass" && style.ground === "recessed" ? 0.054 : 0.113;
        for (let i = 0; i < frostVertices.count; i++) {
          const r = Math.hypot(frostVertices.getX(i), frostVertices.getY(i));
          let roofHeight = style.ground === "recessed" ? 0.033 : 0.025;
          if (style.cover === "dome") {
            roofHeight = 0.024 + Math.sqrt(Math.max(0, 1 - (r / radius) ** 2)) * radius * 0.92;
          }
          if (r < radius - 0.002 && frostVertices.getZ(i) < roofHeight - 0.001) {
            frostExposure[i] = 0.1;
          }
        }
      }
      geo.setAttribute("structureFrostExposure", new THREE.BufferAttribute(frostExposure, 1));
      if ([3, 4, 5, 7, 13, 16].includes(materialIndex)) {
        const position = geo.getAttribute("position");
        const uv = geo.getAttribute("uv");
        const repeat = materialIndex === 16 ? 12 : 35;
        for (let i = 0; i < position.count; i++) {
          const normal = geo.getAttribute("normal");
          const x = position.getX(i);
          const y = position.getY(i);
          const z = position.getZ(i);
          if (Math.abs(normal.getZ(i)) > 0.65) {
            const center = plot
              ? (plot.surface?.center ?? boardCenter(plot.coordinate))
              : { x: 0, y: 0 };
            uv.setXY(i, (x + center.x) * repeat, (y + center.y) * repeat);
          } else if (geo.userData.circularWall) {
            uv.setXY(
              i,
              uv.getX(i) *
                (geo.userData.wallArcLength ?? Math.PI * 2) *
                (geo.userData.wallRadius ?? 0.114) *
                repeat,
              z * repeat,
            );
          } else if (Math.abs(normal.getX(i)) > Math.abs(normal.getY(i))) {
            uv.setXY(i, y * repeat, z * repeat);
          } else {
            uv.setXY(i, x * repeat, z * repeat);
          }
        }
      }
      if (plot) {
        const center = plot.surface?.center ?? boardCenter(plot.coordinate);
        const inverse = new THREE.Quaternion()
          .setFromUnitVectors(new THREE.Vector3(0, 0, 1), plot.normal)
          .invert();
        const vertices = geo.getAttribute("position");
        const projected = new THREE.Vector3();
        for (let i = 0; i < vertices.count; i++) {
          const point = projectBoardPoint(
            center.x + vertices.getX(i),
            center.y + vertices.getY(i),
            vertices.getZ(i) - 0.007,
            projected,
            plot.surface?.radius,
          )
            .sub(plot.worldPosition)
            .applyQuaternion(inverse);
          vertices.setXYZ(i, point.x, point.y, point.z);
        }
        geo.computeVertexNormals();
      }
      const group = geo.userData.collapseGroup ?? 0;
      const pivot = new THREE.Vector3(
        ...((geo.userData.collapsePivot ?? [0, 0, 0]) as [number, number, number]),
      );
      const bounds = collapseBounds.get(group)!;
      const depth =
        Math.max(0, bounds.max.z - pivot.z) +
        bounds.getSize(new THREE.Vector3()).length() * 0.12 +
        0.08;
      if (plot) {
        const center = plot.surface?.center ?? boardCenter(plot.coordinate);
        projectBoardPoint(
          center.x + pivot.x,
          center.y + pivot.y,
          pivot.z - 0.007,
          pivot,
          plot.surface?.radius,
        );
        pivot
          .sub(plot.worldPosition)
          .applyQuaternion(
            new THREE.Quaternion()
              .setFromUnitVectors(new THREE.Vector3(0, 0, 1), plot.normal)
              .invert(),
          );
      }
      const motion = collapseParameters(layout.seed + group * 97);
      const count = geo.getAttribute("position").count;
      const pivots = new Float32Array(count * 3);
      const motions = new Float32Array(count * 4);
      for (let i = 0; i < count; i++) {
        pivots.set(pivot.toArray(), i * 3);
        motions.set(
          [
            motion.delay,
            Math.cos(motion.direction) * motion.tilt,
            Math.sin(motion.direction) * motion.tilt,
            depth,
          ],
          i * 4,
        );
      }
      geo.setAttribute("structureCollapsePivot", new THREE.BufferAttribute(pivots, 3));
      geo.setAttribute("structureCollapseMotion", new THREE.BufferAttribute(motions, 4));
      if (!geo.index) {
        return geo;
      }
      const converted = geo.toNonIndexed();
      geo.dispose();
      return converted;
    });
    const merged = mergeGeometries(prepared);
    for (const geometry of prepared) {
      geometry.dispose();
    }
    return merged;
  });
}

/** Sink the whole structure below the deepest ground before its shared rise animation. */
export function structureEmergenceDepth(geometries: (THREE.BufferGeometry | null)[]) {
  let highest = 0;
  for (const geometry of geometries) {
    if (geometry) {
      geometry.computeBoundingBox();
      highest = Math.max(highest, geometry.boundingBox!.max.z);
    }
  }
  return Math.max(0.08, highest + MARS_RELIEF_DEPTH + 0.015);
}
