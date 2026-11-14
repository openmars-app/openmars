//! Fullscreen source sky. Background display processing is deliberately separate
//! from the untouched radiance maps used for material environment lighting.
use bevy::{
    asset::RenderAssetUsages,
    camera::visibility::NoFrustumCulling,
    ecs::system::SystemParamItem,
    mesh::{MeshVertexBufferLayoutRef, PrimitiveTopology},
    pbr::{MaterialPipeline, MaterialPipelineKey},
    prelude::*,
    render::render_resource::*,
    render::renderer::RenderDevice,
    shader::ShaderRef,
};
#[derive(Asset, TypePath, Clone, Debug, AsBindGroup)]
pub struct SkyTextures {
    #[texture(0, dimension = "cube")]
    #[sampler(1)]
    pub image: Handle<Image>,
}
#[derive(Asset, TypePath, Clone, Debug)]
pub struct SkyMaterial {
    #[dependency]
    pub textures: SkyTextures,
    pub atmosphere: Buffer,
}
impl AsBindGroup for SkyMaterial {
    type Data = ();
    type Param = <SkyTextures as AsBindGroup>::Param;
    fn label() -> &'static str {
        "SkyMaterial"
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
        crate::atmosphere::material::bind_haze(&self.atmosphere, out);
        Ok(())
    }
    fn bind_group_layout_entries(device: &RenderDevice, _: bool) -> Vec<BindGroupLayoutEntry> {
        let mut entries = SkyTextures::bind_group_layout_entries(device, true);
        entries.push(crate::atmosphere::material::haze_layout());
        entries
    }
}
impl Material for SkyMaterial {
    fn vertex_shader() -> ShaderRef {
        "embedded://openmars_engine/sky.wesl".into()
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
    fn specialize(
        _: &MaterialPipeline,
        d: &mut RenderPipelineDescriptor,
        _: &MeshVertexBufferLayoutRef,
        _: MaterialPipelineKey<Self>,
    ) -> Result<(), SpecializedMeshPipelineError> {
        d.primitive.cull_mode = None;
        if let Some(depth) = &mut d.depth_stencil {
            depth.depth_write_enabled = Some(false);
        }
        Ok(())
    }
}
pub struct SkyPlugin;
impl Plugin for SkyPlugin {
    fn build(&self, app: &mut App) {
        bevy::asset::embedded_asset!(app, "sky.wesl");
        app.add_plugins(MaterialPlugin::<SkyMaterial>::default());
    }
}
pub fn spawn(
    parent: &mut ChildSpawnerCommands,
    meshes: &mut Assets<Mesh>,
    materials: &mut Assets<SkyMaterial>,
    image: Handle<Image>,
    atmosphere: Buffer,
) {
    let mesh = mesh();
    parent.spawn((
        Mesh3d(meshes.add(mesh)),
        MeshMaterial3d(materials.add(SkyMaterial {
            textures: SkyTextures { image },
            atmosphere,
        })),
        Transform::default(),
        NoFrustumCulling,
    ));
}
pub fn mesh() -> Mesh {
    Mesh::new(
        PrimitiveTopology::TriangleList,
        RenderAssetUsages::RENDER_WORLD,
    )
    .with_inserted_attribute(
        Mesh::ATTRIBUTE_POSITION,
        vec![[-1., -1., 0.], [3., -1., 0.], [-1., 3., 0.]],
    )
}
