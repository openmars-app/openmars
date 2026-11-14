//! Stable worksite bindings; two passes share one parameter buffer and mesh.
use crate::terrain::materials::HighlightUniform;
use bevy::{
    ecs::system::SystemParamItem,
    mesh::MeshVertexBufferLayoutRef,
    pbr::{MaterialPipeline, MaterialPipelineKey},
    prelude::*,
    render::{render_resource::*, renderer::RenderDevice},
    shader::{ShaderDefVal, ShaderRef},
};

#[derive(Clone, Copy, Debug, ShaderType)]
pub struct SiteUniform {
    pub geometry: Vec4,
    pub clock: Vec4,
    pub local_x: Vec4,
    pub local_y: Vec4,
    pub origin: Vec4,
    pub motion: Vec4,
    pub collapse_parts: [Vec4; 32],
    pub stages: Vec4,
    pub owner_color: Vec4,
    pub cart: Vec4,
}
#[derive(Asset, TypePath, AsBindGroup, Clone, Debug)]
#[bind_group_data(bool)]
pub struct Textures {
    pub ground: bool,
    #[dependency]
    #[texture(1)]
    #[sampler(2)]
    pub mid: Handle<Image>,
    #[dependency]
    #[texture(3)]
    #[sampler(4)]
    pub high: Handle<Image>,
    #[dependency]
    #[texture(17, filterable = false)]
    pub highlight_data: Option<Handle<Image>>,
}
impl From<&Textures> for bool {
    fn from(value: &Textures) -> Self {
        value.ground
    }
}
#[derive(Asset, TypePath, Clone, Debug)]
pub struct SiteMaterial {
    #[dependency]
    pub textures: Textures,
    pub parameters: Buffer,
    pub highlights: Buffer,
    pub atmosphere: bevy::render::render_resource::Buffer,
}
impl AsBindGroup for SiteMaterial {
    type Data = bool;
    type Param = <Textures as AsBindGroup>::Param;
    fn label() -> &'static str {
        "SiteMaterial"
    }
    fn bind_group_data(&self) -> bool {
        self.textures.ground
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
            (0, SiteUniform::min_size()),
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
impl Material for SiteMaterial {
    fn vertex_shader() -> ShaderRef {
        "embedded://openmars_engine/structures/sites/site.wesl".into()
    }
    fn fragment_shader() -> ShaderRef {
        Self::vertex_shader()
    }
    fn alpha_mode(&self) -> AlphaMode {
        AlphaMode::Blend
    }
    fn enable_prepass() -> bool {
        false
    }
    fn enable_shadows() -> bool {
        false
    }
    fn specialize(
        _: &MaterialPipeline,
        d: &mut RenderPipelineDescriptor,
        l: &MeshVertexBufferLayoutRef,
        key: MaterialPipelineKey<Self>,
    ) -> Result<(), SpecializedMeshPipelineError> {
        d.vertex.buffers = vec![l.0.get_layout(&[
            Mesh::ATTRIBUTE_POSITION.at_shader_location(0),
            Mesh::ATTRIBUTE_NORMAL.at_shader_location(1),
            Mesh::ATTRIBUTE_UV_0.at_shader_location(2),
            Mesh::ATTRIBUTE_UV_1.at_shader_location(3),
            Mesh::ATTRIBUTE_COLOR.at_shader_location(4),
        ])?];
        d.primitive.cull_mode = None;
        if let Some(depth) = &mut d.depth_stencil {
            depth.depth_write_enabled = Some(!key.bind_group_data);
            depth.depth_compare = Some(CompareFunction::GreaterEqual);
        }
        if let Some(fragment) = &mut d.fragment {
            fragment.shader_defs.push(ShaderDefVal::Bool(
                "SITE_GROUND".into(),
                key.bind_group_data,
            ));
        }
        Ok(())
    }
}
