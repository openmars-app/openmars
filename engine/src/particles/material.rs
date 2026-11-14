use bevy::{
    ecs::system::SystemParamItem,
    mesh::MeshVertexBufferLayoutRef,
    pbr::{MaterialPipeline, MaterialPipelineKey},
    prelude::*,
    render::{render_resource::*, renderer::RenderDevice},
    shader::ShaderRef,
};
#[derive(Clone, Copy, Debug, ShaderType)]
pub struct Palette {
    pub colors: [Vec4; 16],
}
#[derive(Asset, TypePath, AsBindGroup, Clone, Debug)]
pub struct Textures {
    #[dependency]
    #[texture(1)]
    #[sampler(2)]
    pub image: Handle<Image>,
}
#[derive(Asset, TypePath, Clone, Debug)]
pub struct ParticleMaterial {
    #[dependency]
    pub textures: Textures,
    pub parameters: Buffer,
}
impl AsBindGroup for ParticleMaterial {
    type Data = ();
    type Param = <Textures as AsBindGroup>::Param;
    fn label() -> &'static str {
        "ParticleMaterial"
    }
    fn bind_group_data(&self) {}
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
        Ok(())
    }
    fn bind_group_layout_entries(device: &RenderDevice, _: bool) -> Vec<BindGroupLayoutEntry> {
        let mut entries = Textures::bind_group_layout_entries(device, true);
        {
            let (binding, size) = (0, Palette::min_size());
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
        entries.sort_by_key(|e| e.binding);
        entries
    }
}
impl Material for ParticleMaterial {
    fn vertex_shader() -> ShaderRef {
        "embedded://openmars_engine/particles/particle.wesl".into()
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
        }
        Ok(())
    }
}
pub struct ParticleMaterialPlugin;
impl Plugin for ParticleMaterialPlugin {
    fn build(&self, app: &mut App) {
        bevy::asset::embedded_asset!(app, "particle.wesl");
        app.add_plugins(MaterialPlugin::<ParticleMaterial>::default());
    }
}
