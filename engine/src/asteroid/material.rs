//! One source-colored/depth-independent dust material shared by an impact pool.
use bevy::{
    mesh::MeshVertexBufferLayoutRef,
    pbr::{MaterialPipeline, MaterialPipelineKey},
    prelude::*,
    render::render_resource::*,
    shader::ShaderRef,
};
#[derive(Asset, TypePath, AsBindGroup, Clone, Debug)]
pub struct AsteroidDustMaterial {
    #[dependency]
    #[texture(0)]
    #[sampler(1)]
    pub image: Handle<Image>,
}
impl Material for AsteroidDustMaterial {
    fn vertex_shader() -> ShaderRef {
        "embedded://openmars_engine/asteroid/impact_dust.wesl".into()
    }
    fn fragment_shader() -> ShaderRef {
        Self::vertex_shader()
    }
    fn alpha_mode(&self) -> AlphaMode {
        AlphaMode::Blend
    }
    fn depth_bias(&self) -> f32 {
        1_000_000.
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
pub struct AsteroidDustMaterialPlugin;
impl Plugin for AsteroidDustMaterialPlugin {
    fn build(&self, app: &mut App) {
        bevy::asset::embedded_asset!(app, "impact_dust.wesl");
        app.add_plugins(MaterialPlugin::<AsteroidDustMaterial>::default());
    }
}
