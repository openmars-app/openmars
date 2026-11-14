//! Curved visible/occluded grid, public DTO colours and bounded shared highlights.
use bevy::{
    asset::RenderAssetUsages,
    mesh::{Indices, PrimitiveTopology},
    prelude::*,
};
use openmars_scene::view::BodyId;
use openmars_scene::{
    ScenePlayer, SceneTile,
    geometry::{AxialPoint, BodyProjection},
    highlights::tangent_basis,
};
use std::{collections::BTreeMap, ops::Range};
fn error(message: &str) -> String {
    message.into()
}
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct GridTile {
    pub coordinate: AxialPoint,
    /// Reference linear color, resolved from the visible owner's public color.
    pub border: [f32; 3],
    pub fill: [f32; 4],
    pub ocean: bool,
    pub ocean_space: bool,
    pub volcanic: bool,
    pub reserved: Option<[f32; 3]>,
}
impl GridTile {
    pub fn from_scene(tile: &SceneTile, players: &[ScenePlayer]) -> Self {
        let color = |id: &String| {
            players
                .iter()
                .find(|player| &player.id == id)
                .and_then(|player| Srgba::hex(&player.color).ok())
                .map(|color| {
                    LinearRgba::from(color).to_f32_array()[..3]
                        .try_into()
                        .unwrap()
                })
        };
        let kind = tile
            .occupant
            .as_deref()
            .unwrap_or(if tile.reserved_by.is_some() {
                "restricted-tile"
            } else {
                "empty"
            });
        let base = match kind {
            "ocean-tile" => 0x1e88e5,
            "greenery-tile" => 0x43a047,
            "city-tile" => 0xff6f00,
            "special" => 0x8e24aa,
            "volcano-tile" => 0x4a3728,
            "nuclear-zone-tile" => 0x2a1a0f,
            "mining-tile" => 0x5c3a1e,
            "restricted-tile" => 0x5a4030,
            "ecological-zone-tile" | "natural-preserve-tile" => 0x2d6e2e,
            "world-tree-tile" => 0x1a3a12,
            "mohole-tile" => 0x3a2a1a,
            "empty" | "industrial-center-tile" => 0x6d4c41,
            _ => 0x8e24aa,
        };
        let mut fill = linear_hex(base);
        if matches!(kind, "empty" | "industrial-center-tile") {
            for c in &mut fill {
                *c *= 0.8
            }
        }
        let owner = tile.owner_id.as_ref().or(tile.reserved_by.as_ref());
        let border = owner.and_then(color).unwrap_or_else(|| {
            if kind == "empty" {
                linear_hex(0x67432e)
            } else {
                fill.map(|c| c * 0.25)
            }
        });
        let authored = matches!(
            kind,
            "ocean-tile"
                | "greenery-tile"
                | "city-tile"
                | "volcano-tile"
                | "nuclear-zone-tile"
                | "industrial-center-tile"
                | "mining-tile"
                | "restricted-tile"
                | "ecological-zone-tile"
                | "natural-preserve-tile"
                | "world-tree-tile"
                | "mohole-tile"
        ) || tile.visual.as_ref().is_some_and(|v| v.structure.is_some());
        // Only the original generic special branch retains its flat .7-opacity
        // purple hex. Authored layers never receive a replacement proxy solid.
        let alpha = if kind == "empty" {
            0.12
        } else if authored {
            0.
        } else {
            0.7
        };
        Self {
            coordinate: AxialPoint {
                q: tile.q,
                r: tile.r,
            },
            border,
            fill: [fill[0], fill[1], fill[2], alpha],
            ocean: kind == "ocean-tile",
            ocean_space: tile.terrain.as_str() == "ocean-space",
            volcanic: tile.tags.iter().any(|t| t == "volcanic"),
            reserved: tile
                .reserved_by
                .as_ref()
                .filter(|_| tile.owner_id.is_none() && kind != "restricted-tile")
                .map(|id| color(id).unwrap_or_else(|| linear_hex(0x67432e))),
        }
    }
}
fn linear_hex(hex: u32) -> [f32; 3] {
    [16, 8, 0].map(|n| {
        let c = ((hex >> n) & 255) as f32 / 255.0;
        if c <= 0.04045 {
            c / 12.92
        } else {
            ((c + 0.055) / 1.055).powf(2.4)
        }
    })
}

#[derive(Default)]
struct Builder {
    positions: Vec<[f32; 3]>,
    normals: Vec<[f32; 3]>,
    uv: Vec<[f32; 2]>,
    cells: Vec<[f32; 2]>,
    colors: Vec<[f32; 4]>,
    indices: Vec<u32>,
}
impl Builder {
    fn vertex(
        &mut self,
        projection: BodyProjection,
        tile: &GridTile,
        p: [f64; 2],
        z: f64,
        color: [f32; 4],
    ) -> Result<u32, String> {
        let center = projection
            .project(projection.hex_center(tile.coordinate), 0.0)
            .map_err(|_| error("grid center projection"))?;
        let n = center.map(|x| x / projection.radius());
        let (x, y) = tangent_basis(n);
        let v = std::array::from_fn::<_, 3, _>(|i| center[i] + x[i] * p[0] + y[i] * p[1]);
        let len = v[0].hypot(v[1]).hypot(v[2]);
        let normal = v.map(|v| v / len);
        let id = self.positions.len() as u32;
        self.positions
            .push(normal.map(|n| (n * (projection.radius() + z)) as f32));
        self.normals.push(normal.map(|n| n as f32));
        self.uv
            .push([(p[0] / 0.332 + 0.5) as f32, (p[1] / 0.332 + 0.5) as f32]);
        let offset = projection.coord_offset();
        self.cells.push([
            (i64::from(tile.coordinate.q) - i64::from(offset.q)) as f32,
            (i64::from(tile.coordinate.r) - i64::from(offset.r)) as f32,
        ]);
        self.colors.push(color);
        Ok(id)
    }
    fn ring(&mut self, p: BodyProjection, t: &GridTile) -> Result<Range<usize>, String> {
        let start = self.positions.len();
        for i in 0..=24 {
            let a = corner(i / 4);
            let b = corner(i / 4 + 1);
            let f = (i % 4) as f64 / 4.0;
            let v = [a[0] + (b[0] - a[0]) * f, a[1] + (b[1] - a[1]) * f];
            for radius in [0.156, 0.166] {
                self.vertex(
                    p,
                    t,
                    v.map(|v| v * radius),
                    0.0181,
                    [
                        t.border[0],
                        t.border[1],
                        t.border[2],
                        if radius > 0.161 { 0.25 } else { 0.0 },
                    ],
                )?;
            }
        }
        for i in 0..24 {
            let a = start as u32 + i * 2;
            self.indices.extend([a, a + 1, a + 3, a, a + 3, a + 2])
        }
        Ok(start..self.positions.len())
    }
    fn triangle(
        &mut self,
        p: BodyProjection,
        t: &GridTile,
        points: [[f64; 2]; 3],
        color: [f32; 3],
    ) -> Result<(), String> {
        for point in points {
            let id = self.vertex(p, t, point, 0.02, [color[0], color[1], color[2], 3.0])?;
            self.indices.push(id)
        }
        Ok(())
    }
    fn fill(
        &mut self,
        p: BodyProjection,
        t: &GridTile,
        mode: f32,
        color: [f32; 3],
        radius: f64,
        center: [f64; 2],
    ) -> Result<(), String> {
        let div = 6;
        for side in 0..6 {
            let a = corner(side).map(|v| v * radius);
            let b = corner(side + 1).map(|v| v * radius);
            let start = self.positions.len() as u32;
            for row in 0..=div {
                for col in 0..=row {
                    let u = (row - col) as f64 / div as f64;
                    let v = col as f64 / div as f64;
                    self.vertex(
                        p,
                        t,
                        [
                            center[0] + a[0] * u + b[0] * v,
                            center[1] + a[1] * u + b[1] * v,
                        ],
                        if mode >= 4.0 { 0.0176 } else { 0.0196 },
                        [color[0], color[1], color[2], mode],
                    )?;
                }
            }
            let at = |row: u32, col: u32| start + row * (row + 1) / 2 + col;
            for row in 0..div {
                for col in 0..=row {
                    self.indices
                        .extend([at(row, col), at(row + 1, col), at(row + 1, col + 1)]);
                    if col < row {
                        self.indices
                            .extend([at(row, col), at(row + 1, col + 1), at(row, col + 1)]);
                    }
                }
            }
        }
        Ok(())
    }
}
fn corner(i: u32) -> [f64; 2] {
    let a = std::f64::consts::FRAC_PI_2 + f64::from(i % 6) * std::f64::consts::TAU / 6.0;
    [a.cos(), a.sin()]
}
fn grid_mesh(
    projection: BodyProjection,
    tiles: &[GridTile],
) -> Result<(Mesh, Vec<std::ops::Range<usize>>), String> {
    let mut b = Builder::default();
    let mut borders = Vec::new();
    for t in tiles {
        if t.fill[3] > 0.0 {
            b.fill(
                projection,
                t,
                4.0 + t.fill[3],
                t.fill[..3].try_into().expect("RGB"),
                0.166,
                [0.0; 2],
            )?;
        }
        if !t.ocean {
            let vertices = b.ring(projection, t)?;
            borders.push(vertices);
        }
        if t.ocean_space && !t.ocean && t.fill[3] > 0.0 {
            b.fill(projection, t, 1.0, [0.12, 0.25, 0.42], 0.166, [0.0; 2])?;
        }
        if t.volcanic && t.fill[3] > 0.0 {
            b.fill(projection, t, 2.0, [0.75, 0.12, 0.05], 0.166, [0.0; 2])?;
        }

        if let Some(color) = t.reserved {
            b.triangle(
                projection,
                t,
                [[0.078, 0.02], [0.082, 0.02], [0.082, 0.08]],
                linear_hex(0x333333),
            )?;
            b.triangle(
                projection,
                t,
                [[0.078, 0.02], [0.082, 0.08], [0.078, 0.08]],
                linear_hex(0x333333),
            )?;
            b.triangle(
                projection,
                t,
                [[0.125, 0.07], [0.0875, 0.091650635], [0.0875, 0.048349365]],
                color,
            )?;
        }
    }
    let mut mesh = Mesh::new(
        PrimitiveTopology::TriangleList,
        RenderAssetUsages::MAIN_WORLD | RenderAssetUsages::RENDER_WORLD,
    );
    mesh.insert_attribute(Mesh::ATTRIBUTE_POSITION, b.positions);
    mesh.insert_attribute(Mesh::ATTRIBUTE_NORMAL, b.normals);
    mesh.insert_attribute(Mesh::ATTRIBUTE_UV_0, b.uv);
    mesh.insert_attribute(Mesh::ATTRIBUTE_UV_1, b.cells);
    mesh.insert_attribute(Mesh::ATTRIBUTE_COLOR, b.colors);
    mesh.insert_indices(Indices::U32(b.indices));
    Ok((mesh, borders))
}

use crate::{
    scene::SceneState,
    terrain::materials::{GridMaterial, GridPass, GridTextures, HighlightUniform},
    world::WorldView,
};
use bevy::{
    camera::visibility::{NoFrustumCulling, RenderLayers},
    render::{
        RenderApp,
        render_asset::RenderAssets,
        render_resource::{
            Buffer, Extent3d, TexelCopyBufferLayout, TextureDimension, TextureFormat, UniformBuffer,
        },
        renderer::{RenderDevice, RenderQueue},
        texture::GpuImage,
    },
};
use openmars_scene::highlights::{HighlightBoard, HighlightState};

struct PendingGrid {
    tiles: Vec<GridTile>,
    mesh: Handle<Mesh>,
}
struct GridOwner {
    root: Entity,
    board: HighlightBoard,
    tiles: Vec<GridTile>,
    data: Handle<Image>,
    mesh: Handle<Mesh>,
    pending: Option<PendingGrid>,
    materials: [Handle<GridMaterial>; 2],
    uniform: UniformBuffer<HighlightUniform>,
    uploaded_revision: u64,
    bytes: Vec<u8>,
    visibility: f64,
    last_step: Option<u64>,
    revision: u64,
}
#[derive(Resource, Default)]
pub struct GridRuntime {
    owner: Option<GridOwner>,
    celestial: BTreeMap<BodyId, GridOwner>,
    next_revision: u64,
    retired: Vec<Buffer>,
    pub uniform_writes: u64,
    pub texture_writes: u64,
    pub mesh_builds: u64,
}
impl GridRuntime {
    pub fn label_frame(
        &self,
        id: BodyId,
        world: &World,
    ) -> Option<(BodyProjection, Transform, f32)> {
        let owner = if id == BodyId::Mars {
            self.owner.as_ref()?
        } else {
            self.celestial.get(&id)?
        };
        Some((
            owner.board.projection(),
            *world.get::<Transform>(owner.root)?,
            owner.visibility as f32,
        ))
    }
    pub fn ready(&self) -> bool {
        self.owner.as_ref().is_some_and(|o| o.pending.is_none())
            && self.celestial.values().all(|o| o.pending.is_none())
    }
    pub fn celestial_parts(&self, id: BodyId, world: &World) -> (u64, Vec<Entity>) {
        self.celestial.get(&id).map_or((0, Vec::new()), |owner| {
            (
                owner.revision,
                world
                    .get::<Children>(owner.root)
                    .map_or_else(Vec::new, |c| c.iter().collect()),
            )
        })
    }
    pub fn diagnostics(
        &self,
        world: &World,
        camera: Option<(&Camera, &GlobalTransform)>,
        scale: Vec2,
    ) -> serde_json::Value {
        serde_json::json!({"bodies":self.celestial.iter().map(|(id,o)|{
          let transform=world.get::<Transform>(o.root).copied().unwrap_or_default();
          let sample:Vec<_>=o.tiles.iter().map(|tile|{
            let projection=o.board.projection();
            let local=projection.project(projection.hex_center(tile.coordinate),0.02).unwrap();
            let position=transform.transform_point(Vec3::from_array(local.map(|v|v as f32)));
            let point=camera.and_then(|(c,t)|c.world_to_viewport(t,position).ok()).map(|p|(p*scale).to_array());
            serde_json::json!({"id":format!("{},{},{}",tile.coordinate.q,tile.coordinate.r,-tile.coordinate.q-tile.coordinate.r),"point":point})
          }).collect();
          serde_json::json!({"body":id,"root":o.root.to_string(),"tiles":o.tiles.len(),"visibility":o.visibility,"revision":o.revision,"pending":o.pending.is_some(),"radius":o.board.projection().radius(),"contentRadius":o.board.projection().content_radius(),"buffer":format!("{:?}",o.uniform.buffer().unwrap().id()),"sample":sample})
        }).collect::<Vec<_>>()})
    }
    pub fn uniform(&self) -> HighlightUniform {
        self.owner
            .as_ref()
            .map_or_else(HighlightUniform::default, |owner| *owner.uniform.get())
    }
    pub fn data(&self) -> Option<Handle<Image>> {
        self.owner.as_ref().map(|owner| owner.data.clone())
    }
    pub fn uniforms(&self) -> usize {
        usize::from(self.owner.is_some()) + self.celestial.len() + self.retired.len()
    }
}
fn new_grid(
    world: &mut World,
    tiles: Vec<GridTile>,
    noise: Handle<Image>,
    projection: BodyProjection,
    revision: u64,
    device: &RenderDevice,
    queue: &RenderQueue,
) -> Result<GridOwner, String> {
    if tiles.len() > 4096 {
        return Err(error("Grid source capacity exceeded"));
    }
    let board = HighlightBoard::new(
        projection,
        &tiles.iter().map(|tile| tile.coordinate).collect::<Vec<_>>(),
    )
    .map_err(|e| format!("Grid layout: {e:?}"))?;
    let extent = board.texture_extent();
    let bytes: Vec<u8> = board
        .data()
        .iter()
        .flat_map(|value| value.to_le_bytes())
        .collect();
    let mut image = Image::new(
        Extent3d {
            width: extent[0],
            height: extent[1],
            depth_or_array_layers: 1,
        },
        TextureDimension::D2,
        bytes.clone(),
        TextureFormat::Rgba32Float,
        RenderAssetUsages::RENDER_WORLD,
    );
    image.texture_descriptor.label = Some("tile highlight data");
    let data = world.resource_mut::<Assets<Image>>().add(image);
    let (mesh, _) = grid_mesh(projection, &tiles)?;
    let mesh = world.resource_mut::<Assets<Mesh>>().add(mesh);
    let mut uniform: UniformBuffer<HighlightUniform> = HighlightUniform::default().into();
    uniform.set_label(Some("grid highlights"));
    uniform.write_buffer(device, queue);
    let atmosphere = world
        .resource::<crate::atmosphere::AtmosphereRuntime>()
        .binding()?;
    let materials = [GridPass::Visible, GridPass::Occluded].map(|pass| {
        world
            .resource_mut::<Assets<GridMaterial>>()
            .add(GridMaterial {
                textures: GridTextures {
                    highlight_data: data.clone(),
                    noise: noise.clone(),
                    pass,
                },
                highlights: uniform.buffer().unwrap().clone(),
                atmosphere: atmosphere.clone(),
            })
    });
    let root = world
        .spawn((Transform::default(), Visibility::default()))
        .id();
    for (index, material) in materials.iter().enumerate() {
        world.spawn((
            Mesh3d(mesh.clone()),
            MeshMaterial3d(material.clone()),
            Transform::default(),
            NoFrustumCulling,
            RenderLayers::layer(2),
            ChildOf(root),
            if index == 0 {
                crate::transparent::TransparentOrder::GRID_VISIBLE
            } else {
                crate::transparent::TransparentOrder::GRID_OCCLUDED
            },
        ));
    }
    Ok(GridOwner {
        root,
        board,
        tiles,
        data,
        mesh,
        pending: None,
        materials,
        uniform,
        uploaded_revision: 1,
        bytes,
        visibility: 1.,
        last_step: None,
        revision,
    })
}

// Called only after the preceding GPU submission completed. Texture/mesh
// replacements and buffer destruction therefore cannot race their old draws.
struct GridFrame {
    transform: Transform,
    step: u64,
    shown: bool,
    reduced: bool,
    layer: Option<usize>,
    entrance: crate::entry::Sample,
}
#[derive(Default)]
struct GridUpload {
    image: Option<(bevy::asset::AssetId<Image>, GpuImage)>,
    mesh: Option<bevy::asset::AssetId<Mesh>>,
}
// Mutation and logical retirement follow prior submissions on one ordered queue.
pub fn prepare(
    app: &mut App,
    step: u64,
    device: &RenderDevice,
    queue: &RenderQueue,
) -> Result<(), String> {
    let world = app.world();
    let render = app.sub_app(RenderApp).world();
    let images = render.resource::<RenderAssets<GpuImage>>();
    let meshes = render.resource::<RenderAssets<bevy::render::mesh::RenderMesh>>();
    let runtime = world.resource::<GridRuntime>();
    let uploads: BTreeMap<_, _> = runtime
        .owner
        .iter()
        .map(|o| (BodyId::Mars, o))
        .chain(runtime.celestial.iter().map(|(&id, o)| (id, o)))
        .map(|(id, o)| {
            (
                id,
                GridUpload {
                    image: images
                        .get(o.data.id())
                        .cloned()
                        .map(|gpu| (o.data.id(), gpu)),
                    mesh: o
                        .pending
                        .as_ref()
                        .and_then(|p| meshes.get(p.mesh.id()).map(|_| p.mesh.id())),
                },
            )
        })
        .collect();
    let mut runtime = app
        .world_mut()
        .remove_resource::<GridRuntime>()
        .ok_or("Grid runtime absent")?;
    let result = update(app.world_mut(), &mut runtime, uploads, step, device, queue);
    app.world_mut().insert_resource(runtime);
    result
}
fn update(
    world: &mut World,
    runtime: &mut GridRuntime,
    mut uploads: BTreeMap<BodyId, GridUpload>,
    step: u64,
    device: &RenderDevice,
    queue: &RenderQueue,
) -> Result<(), String> {
    for buffer in runtime.retired.drain(..) {
        buffer.destroy();
    }
    let Some(scene) = world.resource::<SceneState>().0.clone() else {
        return Ok(());
    };
    let view = world.resource::<WorldView>();
    let hovered = view.hovered.clone();
    let focus = view.focus;
    let reduced = world.resource::<crate::scene::MotionPreference>().0;
    for id in crate::space::config::BODIES {
        let mut owner = if id == BodyId::Mars {
            runtime.owner.take()
        } else {
            runtime.celestial.remove(&id)
        };
        let tiles: Vec<_> = scene
            .tiles
            .iter()
            .filter(|t| {
                if id == BodyId::Mars {
                    crate::world::is_mars(t)
                } else {
                    t.location == id.location() && t.terrain != "empty"
                }
            })
            .collect();
        let frame = if id == BodyId::Mars {
            Some((BodyProjection::MARS, Transform::IDENTITY))
        } else {
            world
                .resource::<crate::space::scene::CelestialRuntime>()
                .grid_frame(id)
        };
        if (id != BodyId::Mars && tiles.is_empty()) || frame.is_none() {
            if let Some(owner) = owner {
                world.despawn(owner.root);
                runtime
                    .retired
                    .push(owner.uniform.buffer().unwrap().clone());
            }
            if id != BodyId::Mars {
                crate::space::scene::bind_grid_highlights(world, id, None);
            }
            continue;
        }
        let (projection, transform) = frame.unwrap();
        let appearance: Vec<_> = tiles
            .iter()
            .map(|tile| GridTile::from_scene(tile, &scene.players))
            .collect();
        let states = tiles
            .iter()
            .map(|tile| {
                (
                    AxialPoint {
                        q: tile.q,
                        r: tile.r,
                    },
                    HighlightState {
                        hovered: hovered.as_ref() == Some(&tile.id),
                        inspected: scene.selected_tile.as_ref() == Some(&tile.id),
                        available: scene.selectable_tiles.contains(&tile.id),
                        selected: scene.selected_tile.as_ref() == Some(&tile.id),
                        ..Default::default()
                    },
                )
            })
            .collect();
        if owner.as_ref().is_some_and(|o| {
            o.board.projection() != projection
                || o.tiles
                    .iter()
                    .map(|t| t.coordinate)
                    .ne(appearance.iter().map(|t| t.coordinate))
        }) {
            let old = owner.take().unwrap();
            world.despawn(old.root);
            runtime.retired.push(old.uniform.buffer().unwrap().clone());
        }
        let shown = crate::space::picking::active(id, focus);
        if owner.is_none() {
            let Some(noise) = crate::assets::texture(world, "textures/terrain/detail-array") else {
                continue;
            };
            runtime.next_revision += 1;
            owner = Some(new_grid(
                world,
                appearance.clone(),
                noise,
                projection,
                runtime.next_revision,
                device,
                queue,
            )?);
            owner.as_mut().unwrap().visibility = f64::from(shown);
            runtime.mesh_builds += 1;
        }
        let layer = if id == BodyId::Mars {
            Some(
                if world
                    .resource::<crate::terrain::TerrainState>()
                    .contact
                    .is_some()
                {
                    0
                } else {
                    2
                },
            )
        } else {
            None
        };
        let gpu = uploads.remove(&id).unwrap_or_default();
        let mut owner = owner.unwrap();
        let result = update_owner(
            world,
            &mut owner,
            appearance,
            states,
            &gpu,
            GridFrame {
                transform,
                step,
                shown,
                reduced,
                layer,
                entrance: if id == BodyId::Mars {
                    world.resource::<crate::entry::WorldEntrance>().tiles
                } else {
                    crate::entry::Sample::Settled
                },
            },
            runtime,
            device,
            queue,
        );
        if id == BodyId::Mars {
            runtime.owner = Some(owner);
        } else {
            crate::space::scene::bind_grid_highlights(
                world,
                id,
                Some((owner.uniform.buffer().unwrap().clone(), owner.data.clone())),
            );
            runtime.celestial.insert(id, owner);
        }
        result?;
    }
    Ok(())
}
#[allow(clippy::too_many_arguments)]
fn update_owner(
    world: &mut World,
    owner: &mut GridOwner,
    tiles: Vec<GridTile>,
    states: Vec<(AxialPoint, HighlightState)>,
    gpu: &GridUpload,
    frame: GridFrame,
    counts: &mut GridRuntime,
    device: &RenderDevice,
    queue: &RenderQueue,
) -> Result<(), String> {
    let uploaded_grid = gpu.mesh;
    let gpu = gpu.image.as_ref();
    if owner.tiles == tiles {
        owner.pending = None;
    } else if owner
        .pending
        .as_ref()
        .is_none_or(|pending| pending.tiles != tiles)
    {
        let (mesh, _) = grid_mesh(owner.board.projection(), &tiles)?;
        owner.pending = Some(PendingGrid {
            mesh: world.resource_mut::<Assets<Mesh>>().add(mesh),
            tiles,
        });
        counts.mesh_builds += 1;
    }
    // Keep the displayed grid immutable while preparing its replacement. A new
    // Mesh3d identity also refreshes the renderer's vertex/index allocation data.
    if owner
        .pending
        .as_ref()
        .is_some_and(|pending| Some(pending.mesh.id()) == uploaded_grid)
    {
        let pending = owner.pending.take().unwrap();
        let children: Vec<_> = world.get::<Children>(owner.root).unwrap().iter().collect();
        for entity in children {
            world
                .entity_mut(entity)
                .insert(Mesh3d(pending.mesh.clone()));
        }
        owner.mesh = pending.mesh;
        owner.tiles = pending.tiles;
        counts.next_revision += 1;
        owner.revision = counts.next_revision;
    }
    let dt = owner
        .last_step
        .map_or(0, |last| frame.step.saturating_sub(last));
    owner.last_step = Some(frame.step);
    let target = f64::from(frame.shown);
    if frame.reduced {
        owner.visibility = target;
    } else {
        owner.visibility = target + (owner.visibility - target) * 0.95_f64.powf(dt as f64);
    }
    for (ordinal, (coordinate, state)) in states.into_iter().enumerate() {
        owner
            .board
            .set_entrance_scale(coordinate, f64::from(frame.entrance.scale(ordinal)))
            .map_err(|e| format!("Grid entrance: {e:?}"))?;
        owner
            .board
            .set_visibility(coordinate, owner.visibility)
            .map_err(|e| format!("Grid visibility: {e:?}"))?;

        owner
            .board
            .set(coordinate, state)
            .map_err(|e| format!("Grid state: {e:?}"))?;
    }
    owner
        .board
        .tick(frame.step, frame.shown)
        .map_err(|e| format!("Grid clock: {e:?}"))?;
    if owner.uploaded_revision != owner.board.revision()
        && let Some(gpu) = gpu
            .filter(|(id, _)| *id == owner.data.id())
            .map(|(_, gpu)| gpu)
            .filter(|gpu| {
                gpu.texture_descriptor.size.width == owner.board.texture_extent()[0]
                    && gpu.texture_descriptor.size.height == owner.board.texture_extent()[1]
            })
    {
        for (bytes, value) in owner
            .bytes
            .as_chunks_mut::<4>()
            .0
            .iter_mut()
            .zip(owner.board.data())
        {
            bytes.copy_from_slice(&value.to_le_bytes());
        }
        let extent = owner.board.texture_extent();
        queue.write_texture(
            gpu.texture.as_image_copy(),
            &owner.bytes,
            TexelCopyBufferLayout {
                offset: 0,
                bytes_per_row: Some(extent[0] * 16),
                rows_per_image: Some(extent[1]),
            },
            Extent3d {
                width: extent[0],
                height: extent[1],
                depth_or_array_layers: 1,
            },
        );
        owner.uploaded_revision = owner.board.revision();
        counts.texture_writes += 1;
    }
    owner.uniform.set(HighlightUniform {
        body_inverse: frame.transform.to_matrix().inverse(),
        grid: Vec4::from_array(owner.board.grid().map(|value| value as f32)),
        projection: Vec4::new(
            owner.board.projection().radius() as f32,
            f32::from(owner.board.projection().y_sign()),
            frame.step as f32 / 60.,
            f32::from(u8::from(owner.board.active())),
        ),
    });
    owner.uniform.write_buffer(device, queue);
    counts.uniform_writes += 1;
    world.entity_mut(owner.root).insert(frame.transform);
    if let Some(layer) = frame.layer {
        let children: Vec<_> = world.get::<Children>(owner.root).unwrap().iter().collect();
        for entity in children {
            world.entity_mut(entity).insert(RenderLayers::layer(layer));
        }
    }
    // Keep both material handles owned even when their mesh has no triangles.
    debug_assert_eq!(owner.materials.len(), 2);
    Ok(())
}
pub fn reset(world: &mut World) {
    let mut runtime = world.remove_resource::<GridRuntime>().unwrap();
    let owners = runtime
        .owner
        .take()
        .into_iter()
        .chain(std::mem::take(&mut runtime.celestial).into_values());
    for owner in owners {
        runtime
            .retired
            .push(owner.uniform.buffer().unwrap().clone());
        world.despawn(owner.root);
    }
    world.insert_resource(runtime);
}

#[cfg(test)]
mod tests {
    use super::*;
    use bevy::mesh::VertexAttributeValues;
    fn tile(coordinate: AxialPoint) -> GridTile {
        GridTile {
            coordinate,
            border: [0.2, 0.1, 0.05],
            fill: [0.1, 0.1, 0.1, 0.12],
            ocean: false,
            ocean_space: false,
            volcanic: false,
            reserved: None,
        }
    }
    #[test]
    fn dto_owner_colour_is_linear_and_reserved_ocean_is_not_a_water_occupant() {
        let mut tile: SceneTile = serde_json::from_value(serde_json::json!({
            "id":"0,0,0", "q":0,"r":0,"s":0,"location":"mars","terrain":"ocean-space",
            "tags":["volcanic"],"bonuses":[],"occupantTags":[],"ownerId":"p1"
        }))
        .unwrap();
        let players = [ScenePlayer {
            id: "p1".into(),
            color: "#808080".into(),
        }];
        let grid = GridTile::from_scene(&tile, &players);
        assert_eq!(grid.border, linear_hex(0x808080));
        assert!(grid.border[0] < 0.22);
        assert!(grid.ocean_space && grid.volcanic && !grid.ocean);
        tile.occupant = Some("ocean-tile".into());
        let grid = GridTile::from_scene(&tile, &players);
        assert!(grid.ocean);
        assert_eq!(grid.fill[3], 0.);
        tile.occupant = Some("city-tile".into());
        assert_eq!(GridTile::from_scene(&tile, &players).fill[3], 0.);
    }
    #[test]
    fn generic_special_keeps_source_purple_surface_without_covering_authored_layers() {
        let mut tile: SceneTile = serde_json::from_value(serde_json::json!({
            "id":"0,0,0","q":0,"r":0,"s":0,"location":"mars","terrain":"land",
            "occupant":"commercial-district-tile","tags":[],"bonuses":[],"occupantTags":[]
        }))
        .unwrap();
        let purple = linear_hex(0x8e24aa);
        assert_eq!(
            GridTile::from_scene(&tile, &[]).fill,
            [purple[0], purple[1], purple[2], 0.7]
        );
        tile.occupant = Some("industrial-center-tile".into());
        assert_eq!(GridTile::from_scene(&tile, &[]).fill[3], 0.);
        tile.occupant = None;
        assert_eq!(GridTile::from_scene(&tile, &[]).fill[3], 0.12);
    }
    #[test]
    fn border_is_curved_and_ocean_tiles_have_no_land_border() {
        let coordinate = AxialPoint { q: 4, r: -2 };
        let (mesh, ranges) = grid_mesh(BodyProjection::MARS, &[tile(coordinate)]).unwrap();
        assert_eq!(ranges.len(), 1);
        let VertexAttributeValues::Float32x3(positions) =
            mesh.attribute(Mesh::ATTRIBUTE_POSITION).unwrap()
        else {
            panic!("positions")
        };
        for point in &positions[ranges[0].clone()] {
            assert!((Vec3::from(*point).length() - (2.02 + 0.0181)).abs() < 3e-6)
        }
        let mut water = tile(coordinate);
        water.ocean = true;
        water.fill[3] = 0.0;
        let (_, ranges) = grid_mesh(BodyProjection::MARS, &[water]).unwrap();
        assert!(ranges.is_empty());
    }
    #[test]
    fn celestial_grid_keeps_its_radius_orientation_and_coordinate_offset() {
        let p = BodyProjection::new(0.55, 0.55, 1, AxialPoint { q: 12, r: -7 }).unwrap();
        let (mesh, ranges) = grid_mesh(p, &[tile(AxialPoint { q: 13, r: -7 })]).unwrap();
        let VertexAttributeValues::Float32x3(positions) =
            mesh.attribute(Mesh::ATTRIBUTE_POSITION).unwrap()
        else {
            panic!("positions")
        };
        for point in &positions[ranges[0].clone()] {
            assert!((Vec3::from(*point).length() - (0.55 + 0.0181)).abs() < 2e-6)
        }
        let VertexAttributeValues::Float32x2(cells) = mesh.attribute(Mesh::ATTRIBUTE_UV_1).unwrap()
        else {
            panic!("cells")
        };
        assert!(cells.iter().all(|p| *p == [1.0, 0.0]));
    }
}
