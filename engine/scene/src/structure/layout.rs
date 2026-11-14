//! The shared authored structure generator. A card name never selects geometry.
//! Independent architecture, fixture and planting streams preserve stable detail.
use super::*;
use crate::geometry::BoardPoint;
use crate::geometry::placement::random_sequence;
use std::f64::consts::{PI, TAU};

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum LayoutError {
    Style,
    Capacity,
    InvalidLayout,
}
pub const CAPITAL_WALL_APOTHEM: f64 = 0.127;
const TINTS: [&str; 8] = [
    "#edece8", "#737b82", "#353c42", "#aaa59c", "#50595b", "#c7ccce", "#45484e", "#929a9b",
];
fn point(x: f64, y: f64) -> BoardPoint {
    BoardPoint { x, y }
}
fn road(width: f64, points: Vec<BoardPoint>) -> RoadPath {
    RoadPath {
        points,
        width,
        end_width: None,
        kind: RoadKind::Road,
    }
}
fn rotate(p: BoardPoint, angle: f64) -> BoardPoint {
    let (s, c) = angle.sin_cos();
    point(p.x * c - p.y * s, p.x * s + p.y * c)
}
#[derive(Clone, Copy)]
struct RoadHit {
    point: BoardPoint,
    half_width: f64,
    distance: f64,
}
fn nearest(p: BoardPoint, roads: &[RoadPath]) -> RoadHit {
    let mut hit = RoadHit {
        point: p,
        half_width: 0.,
        distance: f64::INFINITY,
    };
    for road in roads {
        for segment in road.points.windows(2) {
            let a = segment[0];
            let b = segment[1];
            let dx = b.x - a.x;
            let dy = b.y - a.y;
            let length = dx * dx + dy * dy;
            let t = if length > 0. {
                ((p.x - a.x) * dx + (p.y - a.y) * dy) / length
            } else {
                0.
            }
            .clamp(0., 1.);
            let value = point(a.x + dx * t, a.y + dy * t);
            let distance = (p.x - value.x).hypot(p.y - value.y);
            if distance - road.width / 2. < hit.distance - hit.half_width {
                hit = RoadHit {
                    point: value,
                    half_width: road.width / 2.,
                    distance,
                };
            }
        }
    }
    hit
}
pub fn wall_apothem(layout: &StructureLayout) -> Option<f64> {
    if layout.style.landmark == StructureLandmark::Hall {
        Some(CAPITAL_WALL_APOTHEM)
    } else if layout.style.plan == StructurePlan::Grid
        && layout.style.ground == StructureGround::Level
        && layout.style.cover != StructureCover::Dome
    {
        Some(0.132)
    } else {
        None
    }
}
pub fn wall_distance(layout: &StructureLayout, x: f64, y: f64) -> f64 {
    match wall_apothem(layout) {
        Some(apothem) => {
            x.abs()
                .max((x * 0.5 + y * 0.866025404).abs())
                .max((-x * 0.5 + y * 0.866025404).abs())
                - apothem
        }
        None => x.hypot(y) - 0.121,
    }
}
pub fn inside_building(building: &StructureBuilding, x: f64, y: f64, padding: f64) -> bool {
    let dx = x - building.x;
    let dy = y - building.y;
    if building.landmark && building.round {
        return dx.hypot(dy) < building.width.max(building.depth) / 2. + padding;
    }
    let (s, c) = building.rotation.sin_cos();
    (dx * c + dy * s).abs() < building.width / 2. + padding
        && (-dx * s + dy * c).abs() < building.depth / 2. + padding
}
fn entrance_path(building: &StructureBuilding, x: f64, y: f64, clearance: f64) -> bool {
    let Some(target) = building.entrance_road else {
        return false;
    };
    let dx = target.x - building.x;
    let dy = target.y - building.y;
    let length = dx * dx + dy * dy;
    if length < 0.000001 {
        return false;
    }
    let t = (((x - building.x) * dx + (y - building.y) * dy) / length).clamp(0., 1.);
    (x - building.x - dx * t).hypot(y - building.y - dy * t) < clearance
}
pub fn inside_gatehouse(layout: &StructureLayout, x: f64, y: f64, padding: f64) -> bool {
    let Some(wall) = wall_apothem(layout) else {
        return false;
    };
    layout.style.details.contains(&StructureDetail::Gatehouses)
        && layout.entrances.iter().any(|angle| {
            let (s, c) = angle.sin_cos();
            (x * c + y * s - (wall - 0.007)).abs() < 0.012 + padding
                && (-x * s + y * c).abs() < 0.028 + padding
        })
}
pub fn inside_structure(layout: &StructureLayout, x: f64, y: f64, padding: f64) -> bool {
    inside_gatehouse(layout, x, y, padding)
        || layout
            .buildings
            .iter()
            .any(|b| inside_building(b, x, y, padding) || entrance_path(b, x, y, padding))
        || layout
            .fixtures
            .iter()
            .any(|f| (f.x - x).hypot(f.y - y) < f.radius + padding)
        || layout.pipes.iter().any(|pipe| {
            pipe.points.windows(2).any(|points| {
                let a = points[0];
                let b = points[1];
                let dx = b.x - a.x;
                let dy = b.y - a.y;
                let length = dx * dx + dy * dy;
                let t = if length > 0. {
                    ((x - a.x) * dx + (y - a.y) * dy) / length
                } else {
                    0.
                }
                .clamp(0., 1.);
                (x - a.x - t * dx).hypot(y - a.y - t * dy) < pipe.radius + padding
            })
        })
}
fn resolve(request: &StructureStyleRequest) -> Result<StructureStyle, LayoutError> {
    let mut value = serde_json::json!({"plan":"grid","density":"balanced","heights":"mixed","ground":"level","exposure":"surface","perimeter":"open","cover":"none","landmark":"none","connections":"paths","landscaping":"parks","lighting":"normal","perimeterRoad":"none","entranceMin":2,"entranceMax":3,"details":["pipes"]});
    let authored = serde_json::to_value(request).map_err(|_| LayoutError::Style)?;
    for (key, entry) in authored.as_object().ok_or(LayoutError::Style)? {
        // Zero is the source model's absent entrance value (the original Go
        // authoring contract omits it); it is not a new zero-entrance layout.
        if matches!(key.as_str(), "entranceMin" | "entranceMax") && entry.as_i64() == Some(0) {
            continue;
        }
        value[key] = entry.clone();
    }
    let mut style: StructureStyle =
        serde_json::from_value(value).map_err(|_| LayoutError::Style)?;
    if style.details.len() > MAX_STRUCTURE_ITEMS {
        return Err(LayoutError::Capacity);
    }
    if style.perimeter == StructurePerimeter::Open {
        style.perimeter = StructurePerimeter::LowWall;
    }
    if style.cover == StructureCover::Dome {
        style.plan = StructurePlan::Radial;
        style.entrance_min = 2;
        style.entrance_max = 2;
    }
    if style.exposure == StructureExposure::MostlyBuried
        && style.ground == StructureGround::Recessed
    {
        style.plan = StructurePlan::Radial;
    }
    if style.landmark == StructureLandmark::Hall {
        style.plan = StructurePlan::Grid;
        style.perimeter = StructurePerimeter::LowWall;
        style.cover = StructureCover::FlatGlass;
    }
    Ok(style)
}
#[derive(Clone, Copy)]
struct Block {
    x: f64,
    y: f64,
    width: f64,
    depth: f64,
}
struct Grid {
    blocks: Vec<Block>,
    roads: Vec<RoadPath>,
}
struct Edge {
    fixed: f64,
    horizontal: bool,
    spans: Vec<(f64, f64)>,
}
fn grid(seed: u32, rotation: f64, style: &StructureStyle) -> Grid {
    let large = style.exposure == StructureExposure::MostlyBuried;
    let factory = style.landmark == StructureLandmark::Factory;
    let dense = large || (style.density == StructureDensity::Dense && !factory);
    let mut rng = random_sequence(seed ^ 0x3489);
    let mut ys = if dense {
        vec![
            -0.14, -0.116, -0.09, -0.0675, -0.045, -0.0225, 0., 0.0225, 0.045, 0.0675, 0.09, 0.116,
            0.14,
        ]
    } else {
        vec![
            -0.14, -0.103, -0.069, -0.033, 0.004, 0.043, 0.077, 0.111, 0.14,
        ]
    };
    if large {
        ys = vec![-0.14, -0.106, -0.07, 0., 0.07, 0.106, 0.14];
    } else if factory {
        ys = vec![-0.14, -0.111, -0.08, -0.025, 0.03, 0.082, 0.111, 0.14];
    }
    let mut blocks = Vec::new();
    let mut edges: Vec<Edge> = Vec::new();
    let mut edge = |a: BoardPoint, b: BoardPoint| {
        let horizontal = a.y == b.y;
        let fixed = if horizontal { a.y } else { a.x };
        let span = if horizontal { (a.x, b.x) } else { (a.y, b.y) };
        if let Some(row) = edges
            .iter_mut()
            .find(|row| row.horizontal == horizontal && row.fixed == fixed)
        {
            row.spans.push(span)
        } else {
            edges.push(Edge {
                fixed,
                horizontal,
                spans: vec![span],
            });
        }
    };
    for pair in ys.windows(2) {
        let bottom = pair[0];
        let top = pair[1];
        let (mut left, mut right) = (-0.15_f64, 0.15_f64);
        for side in 0..6 {
            let angle = f64::from(side) * PI / 3. - rotation;
            let (nx, ny) = (angle.cos(), angle.sin());
            for y in [bottom, top] {
                if nx.abs() > 1e-6 {
                    let x = (0.124 - ny * y) / nx;
                    if nx > 0. {
                        right = right.min(x)
                    } else {
                        left = left.max(x)
                    }
                }
            }
        }
        left += rng() * 0.005;
        right -= rng() * 0.005;
        if right - left < 0.024 {
            continue;
        }
        let spacing = if large {
            0.064
        } else if factory {
            0.031
        } else if dense {
            0.0225
        } else {
            0.034
        };
        let cells = (if large {
            ((right - left) / spacing).floor()
        } else {
            ((right - left) / spacing).round()
        })
        .max(1.) as usize;
        let mut xs = vec![left];
        for column in 1..cells {
            xs.push(
                left + (right - left) * column as f64 / cells as f64
                    + (rng() - 0.5) * if dense { 0.003 } else { 0.01 },
            );
        }
        xs.push(right);
        let mut column = 0;
        while column < cells {
            let x1 = xs[column];
            let mut x2 = xs[column + 1];
            if !dense
                && !large
                && column < cells - 1
                && if factory {
                    top - bottom > 0.045 && column % 3 == 1
                } else {
                    rng() < 0.15
                }
            {
                column += 1;
                x2 = xs[column + 1];
            }
            let x = (x1 + x2) / 2.;
            let y = (bottom + top) / 2.;
            let outer = dense && !large && x.hypot(y) > 0.09;
            if !outer || rng() > 0.22 {
                let setback = if dense { 0.0065 } else { 0.008 };
                blocks.push(Block {
                    x,
                    y,
                    width: x2 - x1 - setback,
                    depth: top - bottom - setback,
                });
            }
            edge(point(x1, bottom), point(x2, bottom));
            edge(point(x1, top), point(x2, top));
            edge(point(x1, bottom), point(x1, top));
            edge(point(x2, bottom), point(x2, top));
            column += 1;
        }
    }
    let mut roads = Vec::new();
    for mut row in edges {
        row.spans.sort_by(|a, b| a.0.total_cmp(&b.0));
        let mut merged: Vec<(f64, f64)> = vec![];
        for span in row.spans {
            if let Some(last) = merged.last_mut().filter(|last| span.0 <= last.1 + 1e-8) {
                last.1 = last.1.max(span.1)
            } else {
                merged.push(span)
            }
        }
        for (from, to) in merged {
            let points = if row.horizontal {
                vec![point(from, row.fixed), point(to, row.fixed)]
            } else {
                vec![point(row.fixed, from), point(row.fixed, to)]
            };
            roads.push(road(if dense { 0.004 } else { 0.005 }, points));
        }
    }
    Grid { blocks, roads }
}
fn capital() -> (Grid, Vec<f64>) {
    let edges = [-0.15, -0.106, -0.074, -0.043, 0.043, 0.074, 0.106, 0.15];
    let limit = CAPITAL_WALL_APOTHEM - 0.008;
    let normals = (0..6)
        .map(|i| {
            point(
                (f64::from(i) * PI / 3.).cos(),
                (f64::from(i) * PI / 3.).sin(),
            )
        })
        .collect::<Vec<_>>();
    let mut roads = vec![];
    for offset in &edges[1..edges.len() - 1] {
        for horizontal in [true, false] {
            let (mut low, mut high) = (-0.2_f64, 0.2_f64);
            for n in &normals {
                let slope = if horizontal { n.x } else { n.y };
                let fixed = if horizontal { n.y } else { n.x };
                if slope.abs() < 1e-6 {
                    continue;
                }
                let end = (CAPITAL_WALL_APOTHEM - fixed * offset) / slope;
                if slope > 0. {
                    high = high.min(end)
                } else {
                    low = low.max(end)
                }
            }
            roads.push(road(
                0.0035,
                if horizontal {
                    vec![point(low, *offset), point(high, *offset)]
                } else {
                    vec![point(*offset, low), point(*offset, high)]
                },
            ));
        }
    }
    let mut blocks = vec![];
    for row in 0..edges.len() - 1 {
        for col in 0..edges.len() - 1 {
            if row == 3 && col == 3 {
                continue;
            }
            let x = (edges[col] + edges[col + 1]) / 2.;
            let y = (edges[row] + edges[row + 1]) / 2.;
            let width = edges[col + 1] - edges[col] - 0.012;
            let depth = edges[row + 1] - edges[row] - 0.012;
            let mut scale = 0.78_f64;
            for n in &normals {
                scale = scale.min(
                    (limit - n.x * x - n.y * y) / (n.x.abs() * width / 2. + n.y.abs() * depth / 2.),
                );
            }
            if width.min(depth) * scale >= 0.012 {
                blocks.push(Block {
                    x,
                    y,
                    width: width * scale,
                    depth: depth * scale,
                });
            }
        }
    }
    let gates = vec![0., 2. * PI / 3., 4. * PI / 3.];
    for angle in &gates {
        let gate = point(
            angle.cos() * CAPITAL_WALL_APOTHEM,
            angle.sin() * CAPITAL_WALL_APOTHEM,
        );
        let target = nearest(gate, &roads);
        roads.push(road(
            0.006,
            vec![
                target.point,
                point(target.point.x, gate.y),
                point(gate.x * 1.03, gate.y),
            ],
        ));
    }
    (Grid { blocks, roads }, gates)
}
fn building(
    buildings: &mut Vec<StructureBuilding>,
    style: &StructureStyle,
    rng: &mut impl FnMut() -> f64,
    x: f64,
    y: f64,
    width: f64,
    depth: f64,
) {
    let mut height = 0.033 + rng() * 0.041;
    if style.heights == StructureHeights::Low {
        height = 0.025 + rng() * 0.016;
    } else if style.heights == StructureHeights::Tall {
        height = 0.045 + rng() * 0.062;
    }
    if style.exposure == StructureExposure::MostlyBuried {
        height = 0.009 + rng() * 0.01;
    }
    let height_scale = if style.plan == StructurePlan::Grid
        && style.density == StructureDensity::Dense
        && style.heights == StructureHeights::Tall
    {
        0.75
    } else {
        1.
    };
    // Preserve the source RNG evaluation order: dimensions, stepped, facade,
    // then round. Texture/shader choices cannot consume the planting stream.
    buildings.push(StructureBuilding {
        x,
        y,
        z: 0.007,
        width: width * (0.92 + rng() * 0.12),
        depth: depth * (0.92 + rng() * 0.12),
        height: ((height / 0.011).round() * 0.011).max(0.011) * height_scale,
        stepped: rng() > 0.35,
        facade: (rng() * 3.).floor() as u8,
        tint: TINTS[buildings.len() % 8].into(),
        landmark: false,
        round: rng() < 0.22,
        habitat: false,
        rotation: 0.,
        entrance_road: None,
    });
}
fn initial(seed: u32, style: StructureStyle, rotation: f64) -> StructureLayout {
    let mut rng = random_sequence(seed);
    let mut buildings = vec![];
    if style.landmark == StructureLandmark::Hall {
        for block in capital().0.blocks {
            building(
                &mut buildings,
                &style,
                &mut rng,
                block.x,
                block.y,
                block.width,
                block.depth,
            );
            let b = buildings.last_mut().unwrap();
            b.width = block.width;
            b.depth = block.depth;
            b.round = false;
        }
    } else if style.exposure == StructureExposure::MostlyBuried && style.plan != StructurePlan::Grid
    {
        for i in 0..5 {
            let a = f64::from(i) * PI / 4.;
            building(
                &mut buildings,
                &style,
                &mut rng,
                a.cos() * 0.09,
                a.sin() * 0.09,
                0.026,
                0.023,
            );
        }
    } else if style.plan == StructurePlan::Grid {
        for block in grid(seed, rotation, &style).blocks {
            building(
                &mut buildings,
                &style,
                &mut rng,
                block.x,
                block.y,
                block.width,
                block.depth,
            );
            let b = buildings.last_mut().unwrap();
            b.width = block.width;
            b.depth = block.depth;
        }
    } else {
        let sparse = style.density == StructureDensity::Sparse;
        for (radius, count) in [
            (
                if style.cover == StructureCover::Dome {
                    0.104
                } else {
                    0.1
                },
                if sparse { 16 } else { 24 },
            ),
            (0.055, if sparse { 10 } else { 14 }),
            (0.023, if sparse { 0 } else { 6 }),
        ] {
            for i in 0..count {
                let angle = f64::from(i) / f64::from(count) * TAU + (rng() - 0.5) * 0.12;
                let x = angle.cos() * radius;
                let y = angle.sin() * radius;
                if style.landmark != StructureLandmark::None
                    && x.abs() < 0.035
                    && y > -0.025
                    && y < 0.06
                {
                    continue;
                }
                if style.cover == StructureCover::Dome && radius > 0.09 {
                    building(
                        &mut buildings,
                        &style,
                        &mut rng,
                        x * 0.95,
                        y * 0.95,
                        0.013,
                        0.013,
                    );
                    buildings.last_mut().unwrap().height = 0.022;
                } else {
                    let size = if radius < 0.03 { 0.018 } else { 0.022 };
                    let w = size + rng() * 0.002;
                    let d = size + rng() * 0.002;
                    building(&mut buildings, &style, &mut rng, x, y, w, d);
                }
            }
        }
    }
    if style.landmark == StructureLandmark::Factory {
        buildings.sort_by(|a, b| (b.width * b.depth).total_cmp(&(a.width * a.depth)));
        let dense = style.density == StructureDensity::Dense;
        buildings.truncate(if dense { 28 } else { 12 });
        for (index, b) in buildings.iter_mut().enumerate() {
            b.landmark = index < if dense { 6 } else { 3 };
            b.round = false;
            b.stepped = false;
            b.height = if b.landmark {
                0.022 + rng() * 0.014
            } else {
                0.011 + rng() * 0.015
            };
            b.facade = 2;
            b.tint = ["#b6c0c2", "#8b969c", "#c2c0b6"][index % 3].into();
            b.width *= if b.landmark { 0.94 } else { 0.8 };
            b.depth *= if b.landmark { 0.94 } else { 0.8 };
            if index < 2 {
                let width = b.width;
                b.width *= 0.7;
                b.x -= width * 0.15;
            }
        }
    } else if style.landmark != StructureLandmark::None {
        let (mut x, mut y, mut size, mut height) = (0., 0.018, 0.039, 0.03);
        if style.landmark == StructureLandmark::Hall {
            y = 0.;
            size = 0.062;
            height = 0.175;
        }
        if style.landmark == StructureLandmark::Globe {
            let a = rng() * TAU;
            let offset = 0.022 + rng() * 0.018;
            x = a.cos() * offset;
            y = a.sin() * offset;
            size = 0.084;
            height = 0.077;
            buildings.retain(|b| {
                (b.x - x).hypot(b.y - y) >= size / 2. + b.width.hypot(b.depth) / 2. + 0.004
            });
        }
        if style.landmark == StructureLandmark::Headquarters {
            x = (rng() - 0.5) * 0.018;
            y = 0.014 + rng() * 0.012;
            size = 0.125;
            height = 0.108;
            buildings.retain(|b| {
                (b.x - x).abs() >= size / 2. + b.width / 2. + 0.012
                    || (b.y - y).abs() >= 0.054 + b.depth / 2. + 0.012
            });
        }
        buildings.push(StructureBuilding {
            x,
            y,
            z: 0.007,
            width: size,
            depth: if style.landmark == StructureLandmark::Headquarters {
                0.108
            } else {
                size
            },
            height,
            stepped: false,
            facade: 0,
            tint: "#ffffff".into(),
            landmark: true,
            round: style.landmark != StructureLandmark::Headquarters,
            habitat: false,
            rotation: 0.,
            entrance_road: None,
        });
    }
    if style.exposure == StructureExposure::MostlyBuried && style.plan == StructurePlan::Grid {
        let count = buildings.len();
        for index in 0..count {
            let lot = buildings[index].clone();
            let b = &mut buildings[index];
            if b.width > b.depth {
                b.x += b.width * 0.22
            } else {
                b.y += b.depth * 0.22
            }
            b.width = (b.width * 0.4).min(0.028);
            b.depth = (b.depth * 0.4).min(0.025);
            b.round = false;
            b.stepped = false;
            let mut annex = b.clone();
            annex.x = lot.x;
            annex.y = lot.y;
            annex.width = 0.018;
            annex.depth = 0.022;
            if lot.width > lot.depth {
                annex.x -= lot.width * 0.3
            } else {
                annex.y -= lot.depth * 0.3
            }
            annex.height = 0.009 + rng() * 0.008;
            buildings.push(annex);
        }
    }
    StructureLayout {
        name: String::new(),
        seed,
        roads: vec![],
        entrances: vec![],
        style,
        buildings,
        fixtures: vec![],
        pipes: vec![],
        plants: vec![],
        parks: vec![],
    }
}
fn circle(roads: &mut Vec<RoadPath>, radius: f64, offset: BoardPoint) {
    roads.push(road(
        0.007,
        (0..=96)
            .map(|i| {
                let a = f64::from(i) * PI / 48.;
                point(a.cos() * radius + offset.x, a.sin() * radius + offset.y)
            })
            .collect(),
    ));
}
fn trim_globe(roads: &[RoadPath], building: &StructureBuilding) -> Vec<RoadPath> {
    let mut trimmed = vec![];
    for road in roads {
        let radius = building.width / 2. + road.width / 2. + 0.002;
        for pair in road.points.windows(2) {
            let (a, b) = (pair[0], pair[1]);
            let (dx, dy) = (b.x - a.x, b.y - a.y);
            let (fx, fy) = (a.x - building.x, a.y - building.y);
            let length = dx * dx + dy * dy;
            if length < 1e-12 {
                continue;
            }
            let projection = -(fx * dx + fy * dy) / length;
            let discriminant =
                projection * projection - (fx * fx + fy * fy - radius * radius) / length;
            let intervals = if discriminant <= 0. {
                vec![(0., 1.)]
            } else {
                let root = discriminant.sqrt();
                vec![
                    (0., (projection - root).clamp(0., 1.)),
                    ((projection + root).clamp(0., 1.), 1.),
                ]
            };
            for (start, end) in intervals {
                if end - start > 1e-6 {
                    trimmed.push(RoadPath {
                        points: vec![
                            point(a.x + dx * start, a.y + dy * start),
                            point(a.x + dx * end, a.y + dy * end),
                        ],
                        ..road.clone()
                    });
                }
            }
        }
    }
    trimmed
}
fn trim_headquarters(roads: &[RoadPath], building: &StructureBuilding) -> Vec<RoadPath> {
    let mut trimmed = vec![];
    let w = building.width / 2. + 0.007;
    let d = building.depth / 2. + 0.007;
    for road in roads {
        for pair in road.points.windows(2) {
            let (a, b) = (pair[0], pair[1]);
            let (mut entry, mut exit) = (0.0_f64, 1.0_f64);
            for (start, end, center, half) in [(a.x, b.x, building.x, w), (a.y, b.y, building.y, d)]
            {
                let delta = end - start;
                if delta.abs() < 1e-10 {
                    if (start - center).abs() > half {
                        entry = 2.;
                    }
                } else {
                    let t0 = (center - half - start) / delta;
                    let t1 = (center + half - start) / delta;
                    entry = entry.max(t0.min(t1));
                    exit = exit.min(t0.max(t1));
                }
            }
            let intervals = if entry >= exit {
                vec![(0., 1.)]
            } else {
                vec![(0., entry), (exit, 1.)]
            };
            for (start, end) in intervals {
                if end - start > 1e-6 {
                    trimmed.push(RoadPath {
                        points: vec![
                            point(a.x + (b.x - a.x) * start, a.y + (b.y - a.y) * start),
                            point(a.x + (b.x - a.x) * end, a.y + (b.y - a.y) * end),
                        ],
                        ..road.clone()
                    });
                }
            }
        }
    }
    trimmed.push(road(
        0.005,
        vec![
            point(building.x - w, building.y - d),
            point(building.x + w, building.y - d),
            point(building.x + w, building.y + d),
            point(building.x - w, building.y + d),
            point(building.x - w, building.y - d),
        ],
    ));
    trimmed
}
/// Complete resolved layout, suitable for the terrain footprint, batched mesh
/// owner and planting exclusions. No GPU or application global is consulted.
pub fn generate(
    seed: u32,
    request: &StructureStyleRequest,
    name: &str,
) -> Result<StructureLayoutRef, LayoutError> {
    if name.len() > 128 {
        return Err(LayoutError::Capacity);
    }
    let style = resolve(request)?;
    let mut choices = random_sequence(seed ^ 0x196c31);
    let min = style.entrance_min.clamp(1, 4);
    let max = style.entrance_max.min(4).max(min);
    let count = min + (choices() * f64::from(max - min + 1)).floor() as i32;
    let rotation = if style.landmark == StructureLandmark::Hall {
        0.
    } else {
        choices() * TAU
    };
    let mut layout = initial(seed ^ 0x5742, style, rotation);
    layout.name = name.into();
    layout.seed = seed;
    layout.entrances = (0..count)
        .map(|i| rotation + f64::from(i) * TAU / f64::from(count))
        .collect();
    let style = layout.style.clone();
    let mut roads = vec![];
    if style.landmark == StructureLandmark::Hall {
        let capital = capital();
        roads = capital.0.roads;
        layout.entrances = capital.1;
    } else if style.exposure == StructureExposure::MostlyBuried && style.plan != StructurePlan::Grid
    {
        circle(&mut roads, 0.077, point(0., 0.));
    } else if style.plan == StructurePlan::Grid {
        roads = grid(seed ^ 0x5742, rotation, &style).roads;
    } else if style.plan == StructurePlan::Courtyard {
        roads.push(road(
            0.007,
            vec![
                point(-0.056, -0.056),
                point(0.056, -0.056),
                point(0.056, 0.056),
                point(-0.056, 0.056),
                point(-0.056, -0.056),
            ],
        ));
    } else {
        circle(
            &mut roads,
            0.079,
            if style.plan == StructurePlan::Clustered {
                point(0.007, -0.004)
            } else {
                point(0., 0.)
            },
        );
    }
    if style.perimeter_road == StructurePerimeterRoad::Ring && style.plan != StructurePlan::Grid {
        circle(&mut roads, 0.108, point(0., 0.));
    }
    if style.landmark == StructureLandmark::Globe {
        roads = trim_globe(
            &roads,
            layout
                .buildings
                .iter()
                .find(|b| b.landmark)
                .ok_or(LayoutError::InvalidLayout)?,
        );
    }
    if style.landmark == StructureLandmark::Headquarters {
        roads = trim_headquarters(
            &roads,
            layout
                .buildings
                .iter()
                .find(|b| b.landmark)
                .ok_or(LayoutError::InvalidLayout)?,
        );
    }
    layout.roads = roads
        .iter()
        .map(|road| RoadPath {
            points: road.points.iter().map(|p| rotate(*p, rotation)).collect(),
            ..road.clone()
        })
        .collect();
    if let Some(wall) = wall_apothem(&layout)
        && style.landmark != StructureLandmark::Hall
    {
        let sides: &[u32] = match layout.entrances.len() {
            1 => &[0],
            2 => &[0, 3],
            3 => &[0, 2, 4],
            _ => &[0, 1, 3, 4],
        };
        layout.entrances = sides
            .iter()
            .map(|side| f64::from(*side) * PI / 3.)
            .collect();
        let gates = layout
            .entrances
            .iter()
            .map(|angle| {
                let p = point(angle.cos() * wall, angle.sin() * wall);
                road(0.006, vec![p, nearest(p, &layout.roads).point])
            })
            .collect::<Vec<_>>();
        layout.roads.extend(gates);
    }
    if style.exposure == StructureExposure::MostlyBuried && style.plan != StructurePlan::Grid {
        for b in &mut layout.buildings {
            b.x *= 1.15;
            b.y *= 1.15;
        }
    }
    layout.buildings.retain(|b| {
        if b.landmark {
            return true;
        }
        let n = nearest(point(b.x, b.y), &roads);
        let half = if (n.point.x - b.x).abs() > (n.point.y - b.y).abs() {
            b.width / 2.
        } else {
            b.depth / 2.
        };
        n.distance > half + n.half_width + 0.001
    });
    for b in &mut layout.buildings {
        let p = rotate(point(b.x, b.y), rotation);
        b.x = p.x;
        b.y = p.y;
        b.rotation += rotation;
        if b.landmark && style.landmark == StructureLandmark::Headquarters {
            continue;
        }
        let target = nearest(point(b.x, b.y), &layout.roads);
        if style.plan == StructurePlan::Grid {
            let delta = rotate(point(target.point.x - b.x, target.point.y - b.y), -rotation);
            if delta.x.abs() > delta.y.abs() {
                std::mem::swap(&mut b.width, &mut b.depth);
            }
        }
        b.entrance_road = Some(EntranceRoad {
            x: target.point.x,
            y: target.point.y,
            half_width: target.half_width,
        });
        b.rotation = (target.point.y - b.y).atan2(target.point.x - b.x) + PI / 2.;
    }
    if style.details.contains(&StructureDetail::Gatehouses) {
        let old = std::mem::take(&mut layout.buildings);
        layout.buildings = old
            .into_iter()
            .filter(|b| !inside_gatehouse(&layout, b.x, b.y, b.width.hypot(b.depth) / 2. + 0.003))
            .collect();
    }
    if style.exposure != StructureExposure::MostlyBuried
        && style.landmark != StructureLandmark::Factory
    {
        let mut architecture = random_sequence(seed ^ 0x78219);
        let cluster = point(
            (rotation + 0.7).cos() * 0.048,
            (rotation + 0.7).sin() * 0.048,
        );
        let mut candidates = layout
            .buildings
            .iter()
            .enumerate()
            .filter(|(_, b)| !b.landmark)
            .map(|(i, b)| (i, (b.x - cluster.x).hypot(b.y - cluster.y)))
            .collect::<Vec<_>>();
        candidates.sort_by(|a, b| a.1.total_cmp(&b.1));
        let towers = if style.heights == StructureHeights::Mixed
            && !matches!(
                style.landmark,
                StructureLandmark::Hall | StructureLandmark::Headquarters
            ) {
            4
        } else {
            0
        };
        for (i, (index, _)) in candidates.into_iter().enumerate() {
            let b = &mut layout.buildings[index];
            if style.heights == StructureHeights::Mixed {
                b.height = if i < towers {
                    0.05 + architecture() * 0.024
                } else {
                    0.012 + (architecture() * 3.).floor() * 0.009
                };
            }
            if i >= towers
                && !matches!(
                    style.landmark,
                    StructureLandmark::Hall | StructureLandmark::Headquarters
                )
                && i % 4 == 0
                && style.heights != StructureHeights::Tall
                && b.width.max(b.depth) / b.width.min(b.depth) < 1.4
            {
                b.habitat = true;
                b.round = true;
                b.stepped = false;
                b.height = b.width.min(b.depth) * 0.65;
            }
            if style.landmark == StructureLandmark::Headquarters {
                b.round = false;
                b.habitat = false;
            }
        }
    }
    fixtures(&mut layout);
    let mut planting = random_sequence(seed ^ 0x87cd);
    let target = match style.landscaping {
        StructureLandscaping::Lush => 140,
        StructureLandscaping::Parks => 110,
        StructureLandscaping::Sparse => 8,
    };
    for _ in 0..4000 {
        if layout.plants.len() >= target {
            break;
        }
        let x = (planting() - 0.5) * 0.254;
        let y = (planting() - 0.5) * 0.254;
        let scale = 0.25 + planting() * 0.17;
        if wall_distance(&layout, x, y) > -0.012
            || nearest(point(x, y), &layout.roads).distance < 0.013
            || (style.landmark == StructureLandmark::Hall && x.abs().max(y.abs()) < 0.045)
            || (style.ground == StructureGround::Recessed && x.hypot(y) < 0.085)
        {
            continue;
        }
        if inside_structure(&layout, x, y, 0.006)
            || layout
                .plants
                .iter()
                .any(|p| (p.x - x).hypot(p.y - y) < 0.006)
        {
            continue;
        }
        layout.plants.push(StructurePlant {
            x,
            y,
            z: 0.0085,
            scale,
            kind: if planting() > 0.5 {
                StructurePlantKind::Tree
            } else {
                StructurePlantKind::Bush
            },
        });
    }
    StructureLayoutRef::new(layout).map_err(|_| LayoutError::InvalidLayout)
}
fn connect(
    layout: &StructureLayout,
    buildings: &[StructureBuilding],
    fixture: &StructureFixture,
) -> Option<StructurePipe> {
    for (index, b) in buildings.iter().enumerate() {
        let (dx, dy) = (fixture.x - b.x, fixture.y - b.y);
        let (s, c) = b.rotation.sin_cos();
        let (lx, ly) = (dx * c + dy * s, -dx * s + dy * c);
        let safe = |v: f64| if v == 0. { 1e-8 } else { v };
        let scale = (b.width / 2. / safe(lx).abs()).min(b.depth / 2. / safe(ly).abs());
        let start = StructurePoint3 {
            x: b.x + dx * scale,
            y: b.y + dy * scale,
            z: b.z + 0.004,
        };
        let length = (fixture.x - start.x).hypot(fixture.y - start.y);
        if length > 0.075 || length < fixture.radius + 0.003 {
            continue;
        }
        let end_scale = 1. - fixture.radius / length;
        let end = StructurePoint3 {
            x: start.x + (fixture.x - start.x) * end_scale,
            y: start.y + (fixture.y - start.y) * end_scale,
            z: fixture.z + 0.004,
        };
        let clear = (0..25).all(|i| {
            let t = f64::from(i) / 24.;
            let p = point(
                start.x + (end.x - start.x) * t,
                start.y + (end.y - start.y) * t,
            );
            let road = nearest(p, &layout.roads);
            road.distance
                > road.half_width
                    + if layout
                        .style
                        .details
                        .contains(&StructureDetail::GroundExhausts)
                    {
                        0.0045
                    } else {
                        0.003
                    }
                && wall_distance(layout, p.x, p.y) < -0.007
                && !buildings.iter().enumerate().any(|(other, b)| {
                    other != index
                        && (inside_building(b, p.x, p.y, 0.003)
                            || entrance_path(b, p.x, p.y, 0.003))
                })
                && !layout
                    .fixtures
                    .iter()
                    .any(|f| (f.x - p.x).hypot(f.y - p.y) < f.radius + 0.002)
        });
        if clear {
            return Some(StructurePipe {
                points: vec![start, end],
                radius: 0.0015,
            });
        }
    }
    None
}
fn fixtures(layout: &mut StructureLayout) {
    let style = layout.style.clone();
    let mut random = random_sequence(layout.seed ^ 0x723dfa);
    let mut buildings = layout.buildings.clone();
    buildings.sort_by(|a, b| (b.width * b.depth).total_cmp(&(a.width * a.depth)));
    let Some(host) = buildings.first() else {
        return;
    };
    let exposed =
        style.exposure == StructureExposure::Surface && style.cover == StructureCover::None;
    if exposed && style.details.contains(&StructureDetail::Smokestacks) {
        let halls = buildings.iter().filter(|b| b.landmark).collect::<Vec<_>>();
        for i in 0..2 {
            let b = if halls.len() >= 2 { halls[i] } else { host };
            let radius = 0.0065_f64.min(b.width / 7.).min(b.depth / 7.);
            let local_x = if halls.len() >= 2 {
                0.
            } else {
                (i as f64 * 2. - 1.) * b.width * 0.27
            };
            let local_y = b.depth * 0.2;
            let p = rotate(point(local_x, local_y), b.rotation);
            let x = b.x + p.x;
            let y = b.y + p.y;
            let z = b.z + 0.005 + b.height;
            let height = 0.028 + random() * 0.01;
            layout.fixtures.push(StructureFixture {
                kind: StructureFixtureKind::Smokestack,
                rotation: None,
                accent: None,
                emitter: Some(FixtureEmitter {
                    kind: FixtureEmitterKind::Smoke,
                    x,
                    y,
                    z: z + height + 0.001,
                }),
                x,
                y,
                z,
                radius,
                height,
            });
        }
    }
    if style.details.contains(&StructureDetail::AccentLights) {
        let shops = buildings.iter().filter(|b| !b.landmark).collect::<Vec<_>>();
        for i in 0..4.min(shops.len()) {
            let b = shops[i * shops.len() / 4];
            layout.fixtures.push(StructureFixture {
                kind: StructureFixtureKind::AccentLight,
                x: b.x + b.rotation.sin() * (b.depth / 2. + 0.001),
                y: b.y - b.rotation.cos() * (b.depth / 2. + 0.001),
                z: b.z + 0.005 + b.height * 0.72,
                radius: 0.0012,
                height: 0.007,
                rotation: Some(b.rotation),
                accent: Some(if i % 2 == 0 {
                    FixtureAccent::Cyan
                } else {
                    FixtureAccent::Amber
                }),
                emitter: None,
            });
        }
    }
    let exhausts = style.cover == StructureCover::None
        && style.details.contains(&StructureDetail::GroundExhausts);
    let tanks = exposed && style.details.contains(&StructureDetail::Tanks);
    let wanted = if exhausts {
        8
    } else if tanks {
        2
    } else {
        0
    };
    let mut candidates = (0..4000)
        .map(|_| point((random() - 0.5) * 0.23, (random() - 0.5) * 0.23))
        .collect::<Vec<_>>();
    if exhausts {
        let distance = |p: BoardPoint| {
            buildings
                .iter()
                .map(|b| (p.x - b.x).hypot(p.y - b.y))
                .fold(f64::INFINITY, f64::min)
        };
        candidates.sort_by(|a, b| distance(*b).total_cmp(&distance(*a)));
    }
    let mut placed = 0;
    for p in &candidates {
        if placed >= wanted {
            break;
        }
        let radius = if exhausts { 0.0065 } else { 0.005 };
        let road = nearest(*p, &layout.roads);
        if wall_distance(layout, p.x, p.y) > -radius - 0.006
            || road.distance < road.half_width + radius + 0.002
            || inside_structure(layout, p.x, p.y, radius + 0.003)
        {
            continue;
        }
        let mut fixture = StructureFixture {
            kind: if exhausts {
                StructureFixtureKind::GroundExhaust
            } else {
                StructureFixtureKind::Tank
            },
            x: p.x,
            y: p.y,
            z: 0.007,
            radius,
            height: if exhausts { 0.013 } else { 0.019 },
            rotation: Some(host.rotation),
            accent: None,
            emitter: None,
        };
        let pipe = if style.details.contains(&StructureDetail::Pipes) {
            connect(layout, &buildings, &fixture)
        } else {
            None
        };
        if style.details.contains(&StructureDetail::Pipes) && pipe.is_none() {
            continue;
        }
        if exhausts && let Some(pipe) = &pipe {
            let a = pipe.points[0];
            let b = pipe.points[1];
            let angle = (b.y - a.y).atan2(b.x - a.x);
            fixture.rotation = Some(
                host.rotation + (((angle - host.rotation) / (PI / 2.)) + 0.5).floor() * (PI / 2.),
            );
        }
        if exhausts && placed < 2 {
            let rotation = fixture.rotation.unwrap();
            fixture.emitter = Some(FixtureEmitter {
                kind: FixtureEmitterKind::Vapor,
                x: fixture.x + rotation.cos() * radius * 0.8,
                y: fixture.y + rotation.sin() * radius * 0.8,
                z: fixture.z + fixture.height,
            });
        }
        layout.fixtures.push(fixture);
        if let Some(pipe) = pipe {
            if exhausts {
                let a = pipe.points[0];
                let b = pipe.points[1];
                let length = (b.x - a.x).hypot(b.y - a.y);
                for side in [-1., 1.] {
                    let dx = -(b.y - a.y) / length * 0.0024 * side;
                    let dy = (b.x - a.x) / length * 0.0024 * side;
                    layout.pipes.push(StructurePipe {
                        points: pipe
                            .points
                            .iter()
                            .map(|p| StructurePoint3 {
                                x: p.x + dx,
                                y: p.y + dy,
                                z: p.z,
                            })
                            .collect(),
                        radius: pipe.radius,
                    });
                }
            } else {
                layout.pipes.push(pipe);
            }
        }
        placed += 1;
    }
    if style.plan == StructurePlan::Grid && style.details.contains(&StructureDetail::Shafts) {
        let mut count = 0;
        for p in candidates {
            let radius = 0.009;
            let road = nearest(p, &layout.roads);
            if wall_distance(layout, p.x, p.y) > -radius - 0.006
                || road.distance < road.half_width + radius + 0.002
                || inside_structure(layout, p.x, p.y, radius + 0.003)
            {
                continue;
            }
            layout.fixtures.push(StructureFixture {
                kind: StructureFixtureKind::Shaft,
                x: p.x,
                y: p.y,
                z: 0.007,
                radius,
                height: 0.006,
                rotation: None,
                accent: None,
                emitter: None,
            });
            count += 1;
            if count == 6 {
                break;
            }
        }
    }
}
