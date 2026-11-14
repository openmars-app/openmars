//! Shared immutable wave parameters; per-event age is a mesh tag.
use bevy::{
    mesh::MeshVertexBufferLayoutRef,
    pbr::{MaterialPipeline, MaterialPipelineKey},
    prelude::*,
    render::render_resource::*,
    shader::ShaderRef,
};
#[derive(Clone, Copy, Debug, ShaderType)]
pub struct WaveUniform {
    pub center_radius: Vec4,
}
#[derive(Asset, TypePath, AsBindGroup, Clone, Debug)]
pub struct WaveMaterial {
    #[uniform(0)]
    pub parameters: WaveUniform,
}
impl Material for WaveMaterial {
    fn vertex_shader() -> ShaderRef {
        "embedded://openmars_engine/nuclear/blast_wave.wesl".into()
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
        d.vertex.buffers = vec![l.0.get_layout(&[Mesh::ATTRIBUTE_POSITION.at_shader_location(0)])?];
        d.primitive.cull_mode = None;
        if let Some(depth) = &mut d.depth_stencil {
            depth.depth_write_enabled = Some(false);
        }
        Ok(())
    }
}
