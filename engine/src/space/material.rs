//! Constant material bindings; all decorative time comes from the shared frame.
use bevy::{
    ecs::system::SystemParamItem,
    mesh::MeshVertexBufferLayoutRef,
    pbr::{MaterialPipeline, MaterialPipelineKey},
    prelude::*,
    render::{render_resource::*, renderer::RenderDevice},
    shader::ShaderRef,
};
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum Mode {
    Sun,
    Corona,
    Prominence,
    Orbit,
}
#[derive(Asset, TypePath, AsBindGroup, Clone, Debug)]
#[bind_group_data(Mode)]
pub struct Textures {
    #[texture(0)]
    #[sampler(1)]
    pub surface: Option<Handle<Image>>,
    #[uniform(2)]
    pub parameters: Vec4,
    pub mode: Mode,
}
impl From<&Textures> for Mode {
    fn from(t: &Textures) -> Self {
        t.mode
    }
}
#[derive(Asset, TypePath, Clone, Debug)]
pub struct SpaceMaterial {
    #[dependency]
    pub textures: Textures,
    pub atmosphere: Buffer,
}
impl AsBindGroup for SpaceMaterial {
    type Data = Mode;
    type Param = <Textures as AsBindGroup>::Param;
    fn label() -> &'static str {
        "SpaceMaterial"
    }
    fn bind_group_data(&self) -> Mode {
        self.textures.mode
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
        let mut frame = crate::atmosphere::material::haze_layout();
        frame.visibility = ShaderStages::VERTEX_FRAGMENT;
        entries.push(frame);
        entries
    }
}
impl Material for SpaceMaterial {
    fn vertex_shader() -> ShaderRef {
        "embedded://openmars_engine/space/space.wesl".into()
    }
    fn fragment_shader() -> ShaderRef {
        Self::vertex_shader()
    }
    fn enable_prepass() -> bool {
        false
    }
    fn enable_shadows() -> bool {
        false
    }
    fn alpha_mode(&self) -> AlphaMode {
        match self.textures.mode {
            Mode::Sun => AlphaMode::Opaque,
            Mode::Corona => AlphaMode::Add,
            Mode::Prominence | Mode::Orbit => AlphaMode::Blend,
        }
    }
    fn specialize(
        _: &MaterialPipeline,
        d: &mut RenderPipelineDescriptor,
        _: &MeshVertexBufferLayoutRef,
        key: MaterialPipelineKey<Self>,
    ) -> Result<(), SpecializedMeshPipelineError> {
        d.primitive.cull_mode = match key.bind_group_data {
            Mode::Sun => Some(Face::Back),
            Mode::Corona => Some(Face::Front),
            _ => None,
        };
        if key.bind_group_data != Mode::Sun
            && let Some(depth) = &mut d.depth_stencil
        {
            depth.depth_write_enabled = Some(false);
        }
        Ok(())
    }
}
pub struct SpaceMaterialPlugin;
impl Plugin for SpaceMaterialPlugin {
    fn build(&self, app: &mut App) {
        bevy::asset::embedded_asset!(app, "space.wesl");
        app.add_plugins(MaterialPlugin::<SpaceMaterial>::default());
    }
}
