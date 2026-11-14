use bevy::{
    ecs::system::SystemParamItem,
    mesh::MeshVertexBufferLayoutRef,
    pbr::{MaterialPipeline, MaterialPipelineKey},
    prelude::*,
    render::{render_resource::*, renderer::RenderDevice},
    shader::ShaderRef,
};
#[derive(Asset, TypePath, Clone, Debug)]
pub struct HaloMaterial {
    pub parameters: Buffer,
}
impl AsBindGroup for HaloMaterial {
    type Data = ();
    type Param = ();
    fn label() -> &'static str {
        "HaloMaterial"
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
            31,
            UnpreparedBindingResource::Buffer(self.parameters.clone()),
        ));
        Ok(())
    }
    fn bind_group_layout_entries(_: &RenderDevice, _: bool) -> Vec<BindGroupLayoutEntry> {
        vec![BindGroupLayoutEntry {
            binding: 31,
            visibility: ShaderStages::FRAGMENT,
            ty: BindingType::Buffer {
                ty: BufferBindingType::Uniform,
                has_dynamic_offset: false,
                min_binding_size: Some(super::AtmosphereUniform::min_size()),
            },
            count: None,
        }]
    }
}
impl Material for HaloMaterial {
    fn vertex_shader() -> ShaderRef {
        "embedded://openmars_engine/atmosphere/halo.wesl".into()
    }
    fn fragment_shader() -> ShaderRef {
        Self::vertex_shader()
    }
    fn alpha_mode(&self) -> AlphaMode {
        AlphaMode::Add
    }
    fn enable_prepass() -> bool {
        false
    }
    fn enable_shadows() -> bool {
        false
    }
    fn depth_bias(&self) -> f32 {
        -1_000_000.
    }
    fn specialize(
        _: &MaterialPipeline,
        d: &mut RenderPipelineDescriptor,
        _: &MeshVertexBufferLayoutRef,
        _: MaterialPipelineKey<Self>,
    ) -> Result<(), SpecializedMeshPipelineError> {
        d.primitive.cull_mode = Some(Face::Front);
        if let Some(depth) = &mut d.depth_stencil {
            depth.depth_write_enabled = Some(false);
        }
        Ok(())
    }
}
pub struct HaloMaterialPlugin;
impl Plugin for HaloMaterialPlugin {
    fn build(&self, app: &mut App) {
        bevy::shader::load_shader_library!(app, "common.wesl");
        bevy::asset::embedded_asset!(app, "halo.wesl");
        app.add_plugins(MaterialPlugin::<HaloMaterial>::default());
    }
}

/// Shared by each eligible material; source masks and effect sprites omit it.
pub fn haze_layout() -> BindGroupLayoutEntry {
    BindGroupLayoutEntry {
        binding: 31,
        visibility: ShaderStages::FRAGMENT,
        ty: BindingType::Buffer {
            ty: BufferBindingType::Uniform,
            has_dynamic_offset: false,
            min_binding_size: Some(super::AtmosphereUniform::min_size()),
        },
        count: None,
    }
}
pub fn bind_haze(buffer: &Buffer, output: &mut BindGroupBuilder) {
    output.push((31, UnpreparedBindingResource::Buffer(buffer.clone())));
}
