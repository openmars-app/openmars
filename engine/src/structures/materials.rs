//! Original structure PBR with shared, persistent animation/frost parameters.
use super::{texture_library::TextureLibrary, textures::Slot};
use bevy::{
    ecs::system::SystemParamItem,
    mesh::MeshVertexBufferLayoutRef,
    pbr::{
        ExtendedMaterial, MaterialExtension, MaterialExtensionKey, MaterialExtensionPipeline,
        MeshPipelineKey,
    },
    prelude::*,
    render::{
        render_resource::*,
        renderer::{RenderDevice, RenderQueue},
    },
    shader::{ShaderDefVal, ShaderRef},
};
#[derive(Clone, Copy, Debug, ShaderType)]
pub struct StructureUniform {
    pub clock: Vec4,
    pub surface: Vec4,
    pub bump: Vec4,
}
/// Only five distinct frost/glass/bump combinations are needed for all surfaces.
/// A material never changes when its scene clock or climate advances.
pub struct StructureBuffer {
    uniform: UniformBuffer<StructureUniform>,
    id: BufferId,
}
impl StructureBuffer {
    pub fn new(value: StructureUniform, device: &RenderDevice, queue: &RenderQueue) -> Self {
        let mut uniform = UniformBuffer::from(value);
        uniform.set_label(Some("structure parameters"));
        uniform.write_buffer(device, queue);
        let id = uniform.buffer().unwrap().id();
        Self { uniform, id }
    }
    pub fn write(
        &mut self,
        clock: Vec4,
        device: &RenderDevice,
        queue: &RenderQueue,
    ) -> Result<(), String> {
        self.uniform.get_mut().clock = clock;
        self.uniform.write_buffer(device, queue);
        if self.uniform.buffer().unwrap().id() != self.id {
            return Err("Structure uniform identity changed".into());
        }
        Ok(())
    }
    pub fn buffer(&self) -> Buffer {
        self.uniform.buffer().unwrap().clone()
    }
}
#[derive(AsBindGroup, Asset, TypePath, Clone, Debug)]
pub struct StructureTextures {
    #[dependency]
    #[texture(101, filterable = false)]
    pub groups: Handle<Image>,
    #[dependency]
    #[texture(102)]
    #[sampler(103)]
    pub frost_noise: Handle<Image>,
}
#[derive(Asset, TypePath, Clone, Debug)]
pub struct StructureExtension {
    pub parameters: Buffer,
    #[dependency]
    pub textures: StructureTextures,
    pub atmosphere: bevy::render::render_resource::Buffer,
}
impl AsBindGroup for StructureExtension {
    type Data = <StructureTextures as AsBindGroup>::Data;
    type Param = <StructureTextures as AsBindGroup>::Param;
    fn label() -> &'static str {
        "StructureExtension"
    }
    fn bind_group_data(&self) -> Self::Data {
        self.textures.bind_group_data()
    }
    fn build_bind_group(
        &self,
        layout: &BindGroupLayout,
        device: &RenderDevice,
        param: &mut SystemParamItem<'_, '_, Self::Param>,
        _: bool,
        out: &mut BindGroupBuilder,
    ) -> Result<(), AsBindGroupError> {
        self.textures
            .build_bind_group(layout, device, param, true, out)?;
        out.push((
            100,
            UnpreparedBindingResource::Buffer(self.parameters.clone()),
        ));
        crate::atmosphere::material::bind_haze(&self.atmosphere, out);
        Ok(())
    }
    fn bind_group_layout_entries(device: &RenderDevice, _: bool) -> Vec<BindGroupLayoutEntry> {
        let mut entries = StructureTextures::bind_group_layout_entries(device, true);
        entries.push(BindGroupLayoutEntry {
            binding: 100,
            visibility: ShaderStages::VERTEX_FRAGMENT,
            ty: BindingType::Buffer {
                ty: BufferBindingType::Uniform,
                has_dynamic_offset: false,
                min_binding_size: Some(StructureUniform::min_size()),
            },
            count: None,
        });
        entries.push(crate::atmosphere::material::haze_layout());
        entries.sort_by_key(|entry| entry.binding);
        entries
    }
}
pub type StructureMaterial = ExtendedMaterial<StandardMaterial, StructureExtension>;
impl MaterialExtension for StructureExtension {
    fn vertex_shader() -> ShaderRef {
        "embedded://openmars_engine/structures/shaders/vertex.wesl".into()
    }
    fn prepass_vertex_shader() -> ShaderRef {
        Self::vertex_shader()
    }
    fn fragment_shader() -> ShaderRef {
        "embedded://openmars_engine/structures/shaders/fragment.wesl".into()
    }
    fn prepass_fragment_shader() -> ShaderRef {
        "embedded://openmars_engine/structures/shaders/prepass.wesl".into()
    }
    fn specialize(
        _: &MaterialExtensionPipeline,
        d: &mut RenderPipelineDescriptor,
        _: &MeshVertexBufferLayoutRef,
        key: MaterialExtensionKey<Self>,
    ) -> Result<(), SpecializedMeshPipelineError> {
        d.vertex.shader_defs.push(ShaderDefVal::Bool(
            "VERTEX_OUTPUT_INSTANCE_INDEX".into(),
            true,
        ));
        if let Some(f) = &mut d.fragment {
            f.shader_defs.push(ShaderDefVal::Bool(
                "VERTEX_OUTPUT_INSTANCE_INDEX".into(),
                true,
            ));
        }
        let prepass = d
            .vertex
            .shader_defs
            .iter()
            .any(|s| matches!(s,ShaderDefVal::Bool(name,true) if name=="PREPASS_PIPELINE"));
        if !prepass
            && key.mesh_key.contains(MeshPipelineKey::DEPTH_PREPASS)
            && !key.mesh_key.contains(MeshPipelineKey::BLEND_ALPHA)
            && let Some(depth) = &mut d.depth_stencil
        {
            depth.depth_write_enabled = Some(false);
            depth.depth_compare = Some(CompareFunction::Equal);
        }
        Ok(())
    }
}
pub struct StructureMaterialPlugin;
impl Plugin for StructureMaterialPlugin {
    fn build(&self, app: &mut App) {
        bevy::shader::load_shader_library!(app, "shaders/animation.wesl");
        bevy::shader::load_shader_library!(app, "shaders/io.wesl");
        bevy::asset::embedded_asset!(app, "shaders/vertex.wesl");
        bevy::asset::embedded_asset!(app, "shaders/fragment.wesl");
        bevy::asset::embedded_asset!(app, "shaders/prepass.wesl");
        app.add_plugins(MaterialPlugin::<StructureMaterial>::default());
    }
}
#[derive(Clone, Copy, Debug, PartialEq, Eq, PartialOrd, Ord)]
pub struct MaterialKey {
    pub index: usize,
    pub bright: bool,
}
/// Buffer classes: ordinary, glass, unfrosted, concrete bump, sand bump.
pub fn buffer_class(index: usize) -> usize {
    match index {
        10 => 1,
        11 | 14 | 15 => 2,
        13 => 3,
        16 => 4,
        _ => 0,
    }
}
pub fn parameters(class: usize, clock: Vec4) -> StructureUniform {
    StructureUniform {
        clock,
        surface: Vec4::new(
            if class == 2 { 0. } else { 1. },
            if class == 1 { 1. } else { 0. },
            1024.,
            16384.,
        ),
        bump: Vec4::new(
            match class {
                3 => 0.00025,
                4 => 0.0005,
                _ => 0.,
            },
            0.,
            0.,
            0.,
        ),
    }
}
pub fn surface(textures: &TextureLibrary, key: MaterialKey) -> Result<StandardMaterial, String> {
    let get = |slot| {
        textures
            .get(slot)
            .ok_or_else(|| format!("Structure texture {slot:?} absent"))
    };
    let color = |hex: u32| Color::srgb_u8((hex >> 16) as u8, (hex >> 8) as u8, hex as u8);
    let mut material = StandardMaterial {
        base_color: color(0xffffff),
        alpha_mode: AlphaMode::Mask(0.),
        perceptual_roughness: 1.,
        metallic: 0.,
        ..default()
    };
    if key.index < 3 {
        let family = key.index + if key.bright { 3 } else { 0 };
        material.base_color = color(0xeef2f2);
        material.base_color_texture = Some(get(Slot::Facade(family, 0))?);
        material.normal_map_texture = Some(get(Slot::Facade(family, 1))?);
        material.metallic_roughness_texture = Some(get(Slot::Facade(family, 2))?);
        material.emissive_texture = Some(get(Slot::Facade(family, 3))?);
        material.emissive = LinearRgba::WHITE * if key.bright { 1.4 } else { 0.6 };
        material.perceptual_roughness = 0.95;
        material.metallic = 0.08;
        return Ok(material);
    }
    let (base, rough, metal, emission) = match key.index {
        3 => (0x68777d, 0.85, 0.1, 0.22),
        4 => (0xa8a399, 0.9, 0., 0.22),
        5 => (0x344650, 0.7, 0.2, 0.22),
        6 => (0xb67c47, 0.5, 0.6, 0.22),
        7 => (0x496d38, 0.95, 0., 0.22),
        8 => (0x244b6d, 0.35, 0.5, 0.22),
        9 => (0x79b1bb, 0.25, 0.45, 0.25),
        10 => (0x8dced9, 0.15, 0.25, 0.2),
        11 => (0xffd899, 1., 0., 2.),
        12 => (0x1b252b, 0.9, 0., 0.15),
        13 => (0x697175, 0.98, 0., 0.25),
        14 => (0x000000, 1., 0., 0.),
        15 => (0xedaf59, 1., 0., 1.2),
        16 => (0x88694f, 1., 0., 0.2),
        17 => (0x99aab0, 0.38, 0.75, 0.),
        18 => (0xdce5e8, 0.3, 0.25, 0.),
        _ => return Err("Structure material channel".into()),
    };
    material.base_color = color(base);
    material.perceptual_roughness = rough;
    material.metallic = metal;
    material.emissive = color(match key.index {
        9 => 0x264754,
        10 => 0x488ba6,
        11 => 0xffc56c,
        _ => base,
    })
    .to_linear()
        * emission;
    if [3, 4, 5].contains(&key.index) {
        let family = (key.index + 2) % 3;
        material.base_color_texture = Some(get(Slot::Raw(family, 0))?);
        material.normal_map_texture = Some(get(Slot::Raw(family, 1))?);
        material.metallic_roughness_texture = Some(get(Slot::Raw(family, 2))?);
        material.emissive_texture = material.base_color_texture.clone();
    }
    material.base_color_texture = match key.index {
        7 => Some(get(Slot::Shared(1))?),
        13 => Some(get(Slot::Shared(0))?),
        16 => Some(get(Slot::Shared(2))?),
        14 | 15 => Some(get(Slot::Pool)?),
        _ => material.base_color_texture,
    };
    if [7, 13, 15, 16].contains(&key.index) {
        material.emissive_texture = material.base_color_texture.clone();
    }
    if [4, 12, 18].contains(&key.index) {
        material.cull_mode = None;
        material.double_sided = true;
    }
    if [10, 14, 15].contains(&key.index) {
        material.alpha_mode = AlphaMode::Blend;
        material.base_color =
            material
                .base_color
                .with_alpha(if key.index == 14 { 0.55 } else { 0.3 });
    }
    Ok(material)
}
