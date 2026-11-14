//! Shared immutable field textures and real forward terrain materials.
use bevy::{
    ecs::system::SystemParamItem,
    render::{
        render_resource::{
            AsBindGroupError, BindGroupBuilder, BindGroupLayout, BindGroupLayoutEntry, BindingType,
            Buffer, BufferBindingType, BufferId, ShaderStages, UniformBuffer,
            UnpreparedBindingResource,
        },
        renderer::{RenderDevice, RenderQueue},
    },
};
use bevy::{
    mesh::MeshVertexBufferLayoutRef,
    pbr::{MaterialPipeline, MaterialPipelineKey},
    prelude::*,
    reflect::TypePath,
    render::render_resource::{
        AsBindGroup, CompareFunction, Face, RenderPipelineDescriptor, ShaderType,
        SpecializedMeshPipelineError,
    },
    shader::{ShaderDefVal, ShaderRef},
};

#[derive(Clone, Copy, Debug, ShaderType)]
pub struct HighlightUniform {
    pub body_inverse: Mat4,
    pub grid: Vec4,
    pub projection: Vec4,
}
impl Default for HighlightUniform {
    fn default() -> Self {
        Self {
            body_inverse: Mat4::IDENTITY,
            grid: Vec4::new(0., 0., 1., 1.),
            projection: Vec4::new(2.02, -1., 0., 0.),
        }
    }
}

/// A batch owns these two buffers. Frame updates write their existing storage;
/// they never mark materials changed or allocate a replacement GPU uniform.
pub struct TerrainBuffers {
    parameters: UniformBuffer<TerrainUniform>,
    highlights: UniformBuffer<HighlightUniform>,
}
impl TerrainBuffers {
    pub fn new(
        parameters: TerrainUniform,
        device: &RenderDevice,
        queue: &RenderQueue,
    ) -> Result<Self, String> {
        Self::allocate(
            parameters,
            device,
            queue,
            ["terrain parameters", "terrain highlights"],
        )
    }
    pub fn new_bake(
        parameters: TerrainUniform,
        device: &RenderDevice,
        queue: &RenderQueue,
    ) -> Result<Self, String> {
        Self::allocate(
            parameters,
            device,
            queue,
            ["bake parameters", "bake highlights"],
        )
    }
    fn allocate(
        parameters: TerrainUniform,
        device: &RenderDevice,
        queue: &RenderQueue,
        labels: [&str; 2],
    ) -> Result<Self, String> {
        parameters.validate()?;
        let mut result = Self {
            parameters: parameters.into(),
            highlights: HighlightUniform::default().into(),
        };
        result.parameters.set_label(Some(labels[0]));
        result.highlights.set_label(Some(labels[1]));
        result.parameters.write_buffer(device, queue);
        result.highlights.write_buffer(device, queue);
        Ok(result)
    }
    pub fn write(
        &mut self,
        parameters: TerrainUniform,
        highlights: HighlightUniform,
        device: &RenderDevice,
        queue: &RenderQueue,
    ) -> Result<(), String> {
        parameters.validate()?;
        self.parameters.set(parameters);
        self.parameters.write_buffer(device, queue);
        self.highlights.set(highlights);
        self.highlights.write_buffer(device, queue);
        Ok(())
    }
    pub fn parameters(&self) -> Buffer {
        self.parameters
            .buffer()
            .expect("initialized uniform")
            .clone()
    }
    pub fn highlights(&self) -> Buffer {
        self.highlights
            .buffer()
            .expect("initialized uniform")
            .clone()
    }
    pub fn ids(&self) -> [BufferId; 2] {
        [self.parameters().id(), self.highlights().id()]
    }
    /// The renderer calls this only after removing all batch/globe users and
    /// submitting their last GPU uses. destroy() prevents future submissions;
    /// storage needed by previously submitted uses remains owned by the GPU.
    pub fn retire(self) {
        self.parameters().destroy();
        self.highlights().destroy();
    }
}

macro_rules! stable_material {
    ($name:ident, $textures:ident, $($binding:literal => $field:ident : $uniform:ty),+) => {
        #[derive(Asset, TypePath, Clone, Debug)]
        pub struct $name {
            #[dependency]
            pub textures: $textures,
            $(pub $field: Buffer),+
        }
        impl AsBindGroup for $name {
            type Data = <$textures as AsBindGroup>::Data;
            type Param = <$textures as AsBindGroup>::Param;
            fn label() -> &'static str { stringify!($name) }
            fn bind_group_data(&self) -> Self::Data { self.textures.bind_group_data() }
            fn build_bind_group(&self, layout: &BindGroupLayout, device: &RenderDevice,
                param: &mut SystemParamItem<'_, '_, Self::Param>, _: bool, output: &mut BindGroupBuilder)
                -> Result<(), AsBindGroupError> {
                self.textures.build_bind_group(layout, device, param, true, output)?;
                $(output.push(($binding, UnpreparedBindingResource::Buffer(self.$field.clone())));)+
                Ok(())
            }
            fn bind_group_layout_entries(device: &RenderDevice, _: bool) -> Vec<BindGroupLayoutEntry> {
                let mut entries = $textures::bind_group_layout_entries(device, true);
                $(entries.push(BindGroupLayoutEntry {
                    binding: $binding, visibility: ShaderStages::VERTEX_FRAGMENT,
                    ty: BindingType::Buffer { ty: BufferBindingType::Uniform, has_dynamic_offset: false,
                        min_binding_size: Some(<$uniform>::min_size()) }, count: None,
                });)+
                entries.sort_by_key(|entry| entry.binding);
                entries
            }
        }
    }
}
stable_material!(GroundMaterial, GroundTextures, 0 => parameters: TerrainUniform, 16 => highlights: HighlightUniform, 31 => atmosphere: crate::atmosphere::AtmosphereUniform);
stable_material!(OceanMaterial, OceanTextures, 0 => parameters: TerrainUniform, 16 => highlights: HighlightUniform, 31 => atmosphere: crate::atmosphere::AtmosphereUniform);
stable_material!(LakeMaskMaterial, LakeMaskTextures, 0 => parameters: TerrainUniform);

/// All members are vec4-aligned for the real WebGL uniform-buffer path.
#[derive(Clone, Copy, Debug, ShaderType)]
pub struct TerrainUniform {
    /// body radius, patch size, layer capacity, canonical world seconds
    pub geometry: Vec4,
    /// frost, ice, greening, meadow
    pub climate: Vec4,
    /// ice age, clarity, algae, waves
    pub water: Vec4,
    /// foam, shimmer, ice reach, chill
    pub water_extra: Vec4,
    /// Camera XYZ and pause/reduced-motion-aware decorative seconds.
    pub camera: Vec4,
    pub sun_direction: Vec4,
    pub sun_color: Vec4,
    pub shade_bounds: Vec4,
    pub bake_bounds: Vec4,
    /// bake valid, field activation world seconds, unused, unused
    pub bake: Vec4,
}
impl TerrainUniform {
    pub fn from_climate(
        climate: openmars_scene::climate::ClimateState,
        camera: Vec3,
        sun: Vec3,
        sun_color: Vec3,
        intensity: f32,
    ) -> Self {
        Self {
            geometry: Vec4::new(
                2.02,
                openmars_scene::geometry::terrain::PATCH_SIZE as f32,
                1.0,
                0.0,
            ),
            climate: Vec4::from_array(climate.ground_channels().map(|v| v as f32)),
            water: Vec4::new(
                climate.ice_age as f32,
                climate.clarity as f32,
                climate.algae as f32,
                climate.waves as f32,
            ),
            water_extra: Vec4::new(
                climate.foam as f32,
                0.0,
                openmars_scene::landscape::WATER_RADIUS as f32,
                climate.frost as f32,
            ),
            camera: camera.extend(0.0),
            sun_direction: sun.extend(intensity),
            sun_color: sun_color.extend(1.0),
            shade_bounds: Vec4::new(0.0, 0.0, 1.0, 1.0),
            bake_bounds: Vec4::new(0.0, 0.0, 1.0, 1.0),
            bake: Vec4::ZERO,
        }
    }
    pub fn validate(&self) -> Result<(), String> {
        if ![
            self.geometry,
            self.climate,
            self.water,
            self.water_extra,
            self.camera,
            self.sun_direction,
            self.sun_color,
            self.shade_bounds,
            self.bake_bounds,
            self.bake,
        ]
        .into_iter()
        .all(|v| v.is_finite())
            || self.geometry.x <= 0.0
            || self.geometry.z < 1.0
            || self.sun_direction.truncate().length_squared() < 1e-12
            || self.sun_direction.w < 0.0
            || self.shade_bounds.z <= 0.0
            || self.shade_bounds.w <= 0.0
            || self.bake_bounds.z <= 0.0
            || self.bake_bounds.w <= 0.0
        {
            return Err("invalid terrain uniform".into());
        }
        Ok(())
    }
}
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum GroundPass {
    Opaque,
    Edge,
    Basin,
    Globe,
}
#[derive(Asset, TypePath, AsBindGroup, Clone, Debug)]
#[bind_group_data(GroundPass)]
pub struct GroundTextures {
    #[dependency]
    #[texture(17, filterable = false)]
    pub highlight_data: Option<Handle<Image>>,
    #[dependency]
    #[texture(1, dimension = "2d_array")]
    #[sampler(2)]
    pub terrain: Handle<Image>,
    #[dependency]
    #[texture(3, dimension = "2d_array")]
    pub surface_fields: Handle<Image>,
    #[dependency]
    #[texture(5, dimension = "2d_array")]
    #[sampler(6)]
    pub color_atlas: Handle<Image>,
    #[dependency]
    #[texture(7, dimension = "2d_array")]
    pub detail_atlas: Handle<Image>,
    #[dependency]
    #[texture(8)]
    #[sampler(9)]
    pub mars: Handle<Image>,
    #[dependency]
    #[texture(10)]
    #[sampler(11)]
    pub plant_shade: Handle<Image>,
    // The framebuffer belongs to WorldSurface; this strong handle shares that
    // owner's lifetime without pretending it is a manifest/cache dependency.
    #[texture(12)]
    pub surface_mask: Handle<Image>,
    #[dependency]
    #[texture(13)]
    #[sampler(15)]
    pub bake_albedo: Handle<Image>,
    #[dependency]
    #[texture(14)]
    pub bake_surface: Handle<Image>,
    pub pass: GroundPass,
}
impl From<&GroundTextures> for GroundPass {
    fn from(m: &GroundTextures) -> Self {
        m.pass
    }
}
impl Material for GroundMaterial {
    // This surface owns its actual custom depth/mask path. The stock mesh
    // prepass would omit its projection/displacement and write false depth.
    fn enable_prepass() -> bool {
        false
    }
    fn enable_shadows() -> bool {
        false
    }

    fn vertex_shader() -> ShaderRef {
        "embedded://openmars_engine/terrain/shaders/ground.wesl".into()
    }
    fn fragment_shader() -> ShaderRef {
        Self::vertex_shader()
    }
    fn alpha_mode(&self) -> AlphaMode {
        if self.textures.pass == GroundPass::Edge {
            AlphaMode::Blend
        } else {
            AlphaMode::Opaque
        }
    }
    fn specialize(
        _: &MaterialPipeline,
        d: &mut RenderPipelineDescriptor,
        _: &MeshVertexBufferLayoutRef,
        key: MaterialPipelineKey<Self>,
    ) -> Result<(), SpecializedMeshPipelineError> {
        d.primitive.cull_mode = Some(Face::Back);
        if let Some(depth) = &mut d.depth_stencil {
            depth.depth_write_enabled = Some(key.bind_group_data != GroundPass::Edge);
            depth.depth_compare = Some(CompareFunction::GreaterEqual);
        }
        let name = match key.bind_group_data {
            GroundPass::Opaque => "TERRAIN_OPAQUE",
            GroundPass::Edge => "TERRAIN_EDGE",
            GroundPass::Basin => "TERRAIN_BASIN",
            GroundPass::Globe => "TERRAIN_GLOBE",
        };
        d.vertex
            .shader_defs
            .push(ShaderDefVal::Bool(name.into(), true));
        if let Some(fragment) = &mut d.fragment {
            fragment
                .shader_defs
                .push(ShaderDefVal::Bool(name.into(), true));
        }
        Ok(())
    }
}
#[derive(Asset, TypePath, AsBindGroup, Clone, Debug)]
pub struct OceanTextures {
    #[dependency]
    #[texture(17, filterable = false)]
    pub highlight_data: Option<Handle<Image>>,
    #[dependency]
    #[texture(1, dimension = "2d_array")]
    #[sampler(2)]
    pub terrain: Handle<Image>,
    #[dependency]
    #[texture(5, dimension = "2d_array")]
    #[sampler(6)]
    pub color_atlas: Handle<Image>,
    #[dependency]
    #[texture(7, dimension = "2d_array")]
    pub detail_atlas: Handle<Image>,
    #[texture(12)]
    pub surface_mask: Handle<Image>,
}
impl Material for OceanMaterial {
    // This surface owns its actual custom depth/mask path. The stock mesh
    // prepass would omit its projection/displacement and write false depth.
    fn enable_prepass() -> bool {
        false
    }
    fn enable_shadows() -> bool {
        false
    }

    fn vertex_shader() -> ShaderRef {
        "embedded://openmars_engine/terrain/shaders/ocean.wesl".into()
    }
    fn fragment_shader() -> ShaderRef {
        Self::vertex_shader()
    }
    fn alpha_mode(&self) -> AlphaMode {
        AlphaMode::Blend
    }
    fn specialize(
        _: &MaterialPipeline,
        d: &mut RenderPipelineDescriptor,
        _: &MeshVertexBufferLayoutRef,
        _: MaterialPipelineKey<Self>,
    ) -> Result<(), SpecializedMeshPipelineError> {
        d.primitive.cull_mode = Some(Face::Back);
        if let Some(depth) = &mut d.depth_stencil {
            depth.depth_write_enabled = Some(false);
            depth.depth_compare = Some(CompareFunction::GreaterEqual);
        }
        Ok(())
    }
}
pub struct TerrainMaterialPlugin;
#[derive(Asset, TypePath, AsBindGroup, Clone, Debug)]
pub struct LakeMaskTextures {
    #[dependency]
    #[texture(1, dimension = "2d_array")]
    #[sampler(2)]
    pub terrain: Handle<Image>,
}
impl Material for LakeMaskMaterial {
    // This surface owns its actual custom depth/mask path. The stock mesh
    // prepass would omit its projection/displacement and write false depth.
    fn enable_prepass() -> bool {
        false
    }
    fn enable_shadows() -> bool {
        false
    }

    fn vertex_shader() -> ShaderRef {
        "embedded://openmars_engine/terrain/shaders/lake_mask.wesl".into()
    }
    fn fragment_shader() -> ShaderRef {
        Self::vertex_shader()
    }
    fn specialize(
        _: &MaterialPipeline,
        d: &mut RenderPipelineDescriptor,
        _: &MeshVertexBufferLayoutRef,
        _: MaterialPipelineKey<Self>,
    ) -> Result<(), SpecializedMeshPipelineError> {
        if let Some(depth) = &mut d.depth_stencil {
            depth.depth_write_enabled = Some(false);
            depth.depth_compare = Some(CompareFunction::Always);
        }
        if let Some(fragment) = &mut d.fragment {
            for target in fragment.targets.iter_mut().flatten() {
                target.write_mask = bevy::render::render_resource::ColorWrites::GREEN;
                target.blend = None;
            }
        }
        Ok(())
    }
}
impl Plugin for TerrainMaterialPlugin {
    fn build(&self, app: &mut App) {
        bevy::shader::load_shader_library!(app, "shaders/field.wesl");
        bevy::shader::load_shader_library!(app, "shaders/highlight.wesl");
        bevy::asset::embedded_asset!(app, "shaders/ground.wesl");
        bevy::asset::embedded_asset!(app, "shaders/ocean.wesl");
        bevy::asset::embedded_asset!(app, "shaders/lake_mask.wesl");
        bevy::asset::embedded_asset!(app, "shaders/grid.wesl");
        app.add_plugins((
            MaterialPlugin::<GroundMaterial>::default(),
            MaterialPlugin::<OceanMaterial>::default(),
            MaterialPlugin::<LakeMaskMaterial>::default(),
            MaterialPlugin::<GridMaterial>::default(),
        ));
    }
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum GridPass {
    Visible,
    Occluded,
}
#[derive(Asset, TypePath, AsBindGroup, Clone, Debug)]
#[bind_group_data(GridPass)]
pub struct GridTextures {
    #[dependency]
    #[texture(17, filterable = false)]
    pub highlight_data: Handle<Image>,
    #[dependency]
    #[texture(18, dimension = "2d_array")]
    #[sampler(19)]
    pub noise: Handle<Image>,
    pub pass: GridPass,
}
impl From<&GridTextures> for GridPass {
    fn from(textures: &GridTextures) -> Self {
        textures.pass
    }
}
stable_material!(GridMaterial, GridTextures, 16 => highlights: HighlightUniform, 31 => atmosphere: crate::atmosphere::AtmosphereUniform);
impl Material for GridMaterial {
    // This surface owns its actual custom depth/mask path. The stock mesh
    // prepass would omit its projection/displacement and write false depth.
    fn enable_prepass() -> bool {
        false
    }
    fn enable_shadows() -> bool {
        false
    }

    fn vertex_shader() -> ShaderRef {
        "embedded://openmars_engine/terrain/shaders/grid.wesl".into()
    }
    fn fragment_shader() -> ShaderRef {
        Self::vertex_shader()
    }
    fn alpha_mode(&self) -> AlphaMode {
        AlphaMode::Blend
    }
    fn depth_bias(&self) -> f32 {
        20.0
    }
    fn specialize(
        _: &MaterialPipeline,
        d: &mut RenderPipelineDescriptor,
        _: &MeshVertexBufferLayoutRef,
        key: MaterialPipelineKey<Self>,
    ) -> Result<(), SpecializedMeshPipelineError> {
        d.primitive.cull_mode = None;
        if let Some(depth) = &mut d.depth_stencil {
            depth.depth_write_enabled = Some(false);
            depth.depth_compare = Some(if key.bind_group_data == GridPass::Occluded {
                CompareFunction::Less
            } else {
                CompareFunction::GreaterEqual
            });
        }
        if key.bind_group_data == GridPass::Occluded {
            let def = ShaderDefVal::Bool("GRID_OCCLUDED".into(), true);
            d.vertex.shader_defs.push(def.clone());
            if let Some(f) = &mut d.fragment {
                f.shader_defs.push(def)
            }
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use bevy::render::render_resource::encase;
    #[test]
    fn uniform_payload_matches_wesl_offsets_and_webgl_alignment() {
        let mut parameters = TerrainUniform::from_climate(
            openmars_scene::climate::from_parameters(
                openmars_scene::climate::ClimateParameters::BARREN,
            )
            .unwrap(),
            Vec3::new(1., 2., 3.),
            Vec3::Z,
            Vec3::ONE,
            1.,
        );
        parameters.geometry.w = 123.;
        parameters.bake.y = 120.;
        parameters.validate().unwrap();
        let mut encoded = encase::UniformBuffer::new(Vec::<u8>::new());
        encoded.write(&parameters).unwrap();
        let bytes = encoded.into_inner();
        assert_eq!(bytes.len(), 160);
        let at = |offset| f32::from_le_bytes(bytes[offset..offset + 4].try_into().unwrap());
        assert_eq!(at(12), 123.);
        assert_eq!([at(64), at(68), at(72)], [1., 2., 3.]);
        assert_eq!(at(148), 120.);
        let mut encoded = encase::UniformBuffer::new(Vec::<u8>::new());
        encoded.write(&HighlightUniform::default()).unwrap();
        assert_eq!(encoded.into_inner().len(), 96);
        parameters.camera.x = f32::NAN;
        assert!(parameters.validate().is_err());
    }
}
