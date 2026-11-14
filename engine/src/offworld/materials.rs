use bevy::{
    ecs::system::SystemParamItem,
    mesh::MeshVertexBufferLayoutRef,
    pbr::{MaterialPipeline, MaterialPipelineKey},
    prelude::*,
    render::{render_resource::*, renderer::RenderDevice},
    shader::ShaderRef,
};
#[derive(Clone, Copy, Debug, ShaderType)]
pub struct Parameters {
    pub mode: UVec4,
}
#[derive(Asset, TypePath, AsBindGroup, Clone, Debug)]
#[bind_group_data(bool)]
pub struct Textures {
    pub warning: bool,
    #[uniform(0)]
    pub parameters: Parameters,
    #[dependency]
    #[texture(1)]
    #[sampler(2)]
    pub concrete: Handle<Image>,
    #[dependency]
    #[texture(3)]
    #[sampler(4)]
    pub noise: Handle<Image>,
    #[dependency]
    #[texture(5)]
    #[sampler(6)]
    pub high_noise: Handle<Image>,
}
impl From<&Textures> for bool {
    fn from(m: &Textures) -> Self {
        m.warning
    }
}
#[derive(Asset, TypePath, Clone, Debug)]
pub struct ShipPadMaterial {
    #[dependency]
    pub textures: Textures,
    pub atmosphere: Buffer,
}
impl AsBindGroup for ShipPadMaterial {
    type Data = bool;
    type Param = <Textures as AsBindGroup>::Param;
    fn label() -> &'static str {
        "ShipPadMaterial"
    }
    fn bind_group_data(&self) -> bool {
        self.textures.warning
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
        crate::atmosphere::material::bind_haze(&self.atmosphere, out);
        Ok(())
    }
    fn bind_group_layout_entries(device: &RenderDevice, _: bool) -> Vec<BindGroupLayoutEntry> {
        let mut entries = Textures::bind_group_layout_entries(device, true);
        entries.push(crate::atmosphere::material::haze_layout());
        entries.sort_by_key(|e| e.binding);
        entries
    }
}
impl Material for ShipPadMaterial {
    fn vertex_shader() -> ShaderRef {
        "embedded://openmars_engine/offworld/pads.wesl".into()
    }
    fn fragment_shader() -> ShaderRef {
        Self::vertex_shader()
    }
    fn alpha_mode(&self) -> AlphaMode {
        if self.textures.warning {
            AlphaMode::Opaque
        } else {
            AlphaMode::Blend
        }
    }
    fn depth_bias(&self) -> f32 {
        if self.textures.warning {
            -6_000_000.
        } else {
            -5_000_000.
        }
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
            Mesh::ATTRIBUTE_TANGENT.at_shader_location(5),
        ])?];
        d.primitive.cull_mode = None;
        if let Some(depth) = &mut d.depth_stencil {
            depth.depth_write_enabled = Some(key.bind_group_data);
        }
        Ok(())
    }
}
#[derive(Asset, TypePath, AsBindGroup, Clone, Debug)]
#[bind_group_data(u32)]
pub struct ShipParticleMaterial {
    #[uniform(0)]
    pub parameters: Parameters,
    #[dependency]
    #[texture(1)]
    #[sampler(2)]
    pub image: Handle<Image>,
}
impl From<&ShipParticleMaterial> for u32 {
    fn from(m: &ShipParticleMaterial) -> Self {
        m.parameters.mode.x
    }
}
impl Material for ShipParticleMaterial {
    fn vertex_shader() -> ShaderRef {
        "embedded://openmars_engine/offworld/particles.wesl".into()
    }
    fn fragment_shader() -> ShaderRef {
        Self::vertex_shader()
    }
    fn alpha_mode(&self) -> AlphaMode {
        if matches!(self.parameters.mode.x, 2 | 3) {
            AlphaMode::Add
        } else {
            AlphaMode::Blend
        }
    }
    fn depth_bias(&self) -> f32 {
        -(if self.parameters.mode.x == 4 {
            20.
        } else {
            101. + self.parameters.mode.x as f32
        }) * 1_000_000.
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
            Mesh::ATTRIBUTE_UV_0.at_shader_location(1),
        ])?];
        d.primitive.cull_mode = None;
        if let Some(depth) = &mut d.depth_stencil {
            depth.depth_write_enabled = Some(false);
            if key.bind_group_data == 4 {
                depth.depth_compare = Some(CompareFunction::Always);
            }
        }
        Ok(())
    }
}

pub struct OffworldMaterialPlugin;
impl Plugin for OffworldMaterialPlugin {
    fn build(&self, app: &mut App) {
        bevy::asset::embedded_asset!(app, "pads.wesl");
        bevy::asset::embedded_asset!(app, "particles.wesl");
        app.add_plugins(MaterialPlugin::<ShipParticleMaterial>::default());
        app.add_plugins(MaterialPlugin::<ShipPadMaterial>::default());
    }
}
