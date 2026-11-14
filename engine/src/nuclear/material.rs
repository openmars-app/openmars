//! Original crater PBR and independent blue mask with stable shared buffers.
use crate::terrain::materials::HighlightUniform;
use bevy::{
    ecs::system::SystemParamItem,
    mesh::{MeshVertexAttribute, MeshVertexBufferLayoutRef},
    pbr::{MaterialPipeline, MaterialPipelineKey},
    prelude::*,
    render::{render_resource::*, renderer::RenderDevice},
    shader::{ShaderDefVal, ShaderRef},
};
pub const DIRECTION: MeshVertexAttribute = MeshVertexAttribute::new(
    "NuclearPlanetDirection",
    0x4f4d4e55,
    VertexFormat::Float32x3,
);
#[derive(Clone, Copy, Debug, ShaderType)]
pub struct NuclearUniform {
    /// Original seed (GPU f32), climate chill, visible coverage, scorch flag.
    pub surface: Vec4,
    pub gouges: [Vec4; 5],
    /// Tangent tile origin and source entrance scale. Debris is a separate layer.
    pub entrance: Vec4,
}
#[derive(Clone, Copy, Debug, Hash, PartialEq, Eq)]
pub enum NuclearPass {
    Bowl,
    Scorch,
}
#[derive(Asset, TypePath, AsBindGroup, Clone, Debug)]
#[bind_group_data(NuclearPass)]
pub struct Textures {
    pub pass: NuclearPass,
    #[dependency]
    #[texture(1)]
    #[sampler(2)]
    pub mars: Handle<Image>,
    #[dependency]
    #[texture(17, filterable = false)]
    pub highlight_data: Option<Handle<Image>>,
}
impl From<&Textures> for NuclearPass {
    fn from(t: &Textures) -> Self {
        t.pass
    }
}
#[derive(Asset, TypePath, Clone, Debug)]
pub struct NuclearMaterial {
    #[dependency]
    pub textures: Textures,
    pub parameters: Buffer,
    pub highlights: Buffer,
    pub atmosphere: bevy::render::render_resource::Buffer,
}
impl AsBindGroup for NuclearMaterial {
    type Data = NuclearPass;
    type Param = <Textures as AsBindGroup>::Param;
    fn label() -> &'static str {
        "NuclearMaterial"
    }
    fn bind_group_data(&self) -> NuclearPass {
        self.textures.pass
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
            0,
            UnpreparedBindingResource::Buffer(self.parameters.clone()),
        ));
        out.push((
            16,
            UnpreparedBindingResource::Buffer(self.highlights.clone()),
        ));
        crate::atmosphere::material::bind_haze(&self.atmosphere, out);
        Ok(())
    }
    fn bind_group_layout_entries(device: &RenderDevice, _: bool) -> Vec<BindGroupLayoutEntry> {
        let mut entries = Textures::bind_group_layout_entries(device, true);
        for (binding, size) in [
            (0, NuclearUniform::min_size()),
            (16, HighlightUniform::min_size()),
        ] {
            entries.push(BindGroupLayoutEntry {
                binding,
                visibility: ShaderStages::VERTEX_FRAGMENT,
                ty: BindingType::Buffer {
                    ty: BufferBindingType::Uniform,
                    has_dynamic_offset: false,
                    min_binding_size: Some(size),
                },
                count: None,
            });
        }
        entries.push(crate::atmosphere::material::haze_layout());
        entries.sort_by_key(|e| e.binding);
        entries
    }
}
fn layout(
    d: &mut RenderPipelineDescriptor,
    l: &MeshVertexBufferLayoutRef,
) -> Result<(), SpecializedMeshPipelineError> {
    d.vertex.buffers = vec![l.0.get_layout(&[
        Mesh::ATTRIBUTE_POSITION.at_shader_location(0),
        Mesh::ATTRIBUTE_NORMAL.at_shader_location(1),
        Mesh::ATTRIBUTE_UV_0.at_shader_location(2),
        Mesh::ATTRIBUTE_UV_1.at_shader_location(3),
        Mesh::ATTRIBUTE_COLOR.at_shader_location(4),
        DIRECTION.at_shader_location(5),
    ])?];
    Ok(())
}
impl Material for NuclearMaterial {
    fn enable_prepass() -> bool {
        false
    }
    fn enable_shadows() -> bool {
        false
    }
    fn vertex_shader() -> ShaderRef {
        "embedded://openmars_engine/nuclear/nuclear_surface.wesl".into()
    }
    fn fragment_shader() -> ShaderRef {
        Self::vertex_shader()
    }
    fn alpha_mode(&self) -> AlphaMode {
        match self.textures.pass {
            NuclearPass::Bowl => AlphaMode::Opaque,
            NuclearPass::Scorch => AlphaMode::Blend,
        }
    }
    fn specialize(
        _: &MaterialPipeline,
        d: &mut RenderPipelineDescriptor,
        l: &MeshVertexBufferLayoutRef,
        key: MaterialPipelineKey<Self>,
    ) -> Result<(), SpecializedMeshPipelineError> {
        layout(d, l)?;
        if let Some(f) = &mut d.fragment {
            f.shader_defs.push(ShaderDefVal::Bool(
                "NUCLEAR_SCORCH".into(),
                key.bind_group_data == NuclearPass::Scorch,
            ));
        }
        d.primitive.cull_mode = None;
        if let Some(depth) = &mut d.depth_stencil {
            depth.depth_write_enabled = Some(key.bind_group_data == NuclearPass::Bowl);
            depth.depth_compare = Some(CompareFunction::GreaterEqual);
        }
        Ok(())
    }
}
#[derive(Asset, TypePath, Clone, Debug)]
pub struct NuclearMaskMaterial {
    pub parameters: Buffer,
}
impl AsBindGroup for NuclearMaskMaterial {
    type Data = ();
    type Param = ();
    fn label() -> &'static str {
        "NuclearMaskMaterial"
    }
    fn bind_group_data(&self) {}
    fn build_bind_group(
        &self,
        _: &BindGroupLayout,
        _: &RenderDevice,
        _: &mut SystemParamItem<'_, '_, Self::Param>,
        _: bool,
        out: &mut BindGroupBuilder,
    ) -> Result<(), AsBindGroupError> {
        out.push((
            0,
            UnpreparedBindingResource::Buffer(self.parameters.clone()),
        ));
        Ok(())
    }
    fn bind_group_layout_entries(_: &RenderDevice, _: bool) -> Vec<BindGroupLayoutEntry> {
        vec![BindGroupLayoutEntry {
            binding: 0,
            visibility: ShaderStages::VERTEX_FRAGMENT,
            ty: BindingType::Buffer {
                ty: BufferBindingType::Uniform,
                has_dynamic_offset: false,
                min_binding_size: Some(NuclearUniform::min_size()),
            },
            count: None,
        }]
    }
}
impl Material for NuclearMaskMaterial {
    fn enable_prepass() -> bool {
        false
    }
    fn enable_shadows() -> bool {
        false
    }
    fn vertex_shader() -> ShaderRef {
        "embedded://openmars_engine/nuclear/nuclear_mask.wesl".into()
    }
    fn fragment_shader() -> ShaderRef {
        Self::vertex_shader()
    }
    fn specialize(
        _: &MaterialPipeline,
        d: &mut RenderPipelineDescriptor,
        l: &MeshVertexBufferLayoutRef,
        _: MaterialPipelineKey<Self>,
    ) -> Result<(), SpecializedMeshPipelineError> {
        layout(d, l)?;
        d.primitive.cull_mode = Some(bevy::render::render_resource::Face::Back);
        if let Some(depth) = &mut d.depth_stencil {
            depth.depth_write_enabled = Some(false);
            depth.depth_compare = Some(CompareFunction::Always);
        }
        if let Some(fragment) = &mut d.fragment {
            for target in fragment.targets.iter_mut().flatten() {
                target.write_mask = ColorWrites::BLUE;
                target.blend = None;
            }
        }
        Ok(())
    }
}
pub struct NuclearMaterialPlugin;
impl Plugin for NuclearMaterialPlugin {
    fn build(&self, app: &mut App) {
        bevy::shader::load_shader_library!(app, "nuclear_io.wesl");
        bevy::asset::embedded_asset!(app, "nuclear_surface.wesl");
        bevy::asset::embedded_asset!(app, "nuclear_mask.wesl");
        app.add_plugins((
            MaterialPlugin::<NuclearMaterial>::default(),
            MaterialPlugin::<NuclearMaskMaterial>::default(),
        ));
    }
}
