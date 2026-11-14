//! Native, unlit typography and prepared bonus icons with source depth rules.
use bevy::{
    mesh::MeshVertexBufferLayoutRef,
    pbr::{MaterialPipeline, MaterialPipelineKey},
    prelude::*,
    render::render_resource::*,
    shader::ShaderRef,
};
#[derive(Clone, Copy, Debug, ShaderType)]
pub struct Parameters {
    /// 0: outlined white name, 1: black credit amount, 2: prepared bonus icon.
    pub data: Vec4,
}
#[derive(Asset, TypePath, AsBindGroup, Clone, Debug)]
pub struct LabelMaterial {
    #[uniform(0)]
    pub parameters: Parameters,
    #[dependency]
    #[texture(1)]
    #[sampler(2)]
    pub image: Handle<Image>,
}
impl Material for LabelMaterial {
    fn vertex_shader() -> ShaderRef {
        "embedded://openmars_engine/labels/label.wesl".into()
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
        _: MaterialPipelineKey<Self>,
    ) -> Result<(), SpecializedMeshPipelineError> {
        d.vertex.buffers = vec![l.0.get_layout(&[
            Mesh::ATTRIBUTE_POSITION.at_shader_location(0),
            Mesh::ATTRIBUTE_UV_0.at_shader_location(1),
        ])?];
        d.primitive.cull_mode = None;
        if let Some(depth) = &mut d.depth_stencil {
            depth.depth_write_enabled = Some(false);
            depth.depth_compare = Some(CompareFunction::Always);
        }
        Ok(())
    }
}
pub struct LabelMaterialPlugin;
impl Plugin for LabelMaterialPlugin {
    fn build(&self, app: &mut App) {
        bevy::asset::embedded_asset!(app, "label.wesl");
        app.add_plugins(MaterialPlugin::<LabelMaterial>::default());
    }
}
