//! One shared rock PBR material; linear per-instance tint and hidden warmup tag.
use bevy::{
    ecs::system::SystemParamItem,
    mesh::MeshVertexBufferLayoutRef,
    pbr::{ExtendedMaterial, MaterialExtension, MaterialExtensionKey, MaterialExtensionPipeline},
    prelude::*,
    render::{render_resource::*, renderer::RenderDevice},
    shader::{ShaderDefVal, ShaderRef},
};
#[derive(Asset, TypePath, Clone, Debug)]
pub struct DebrisExtension {
    pub atmosphere: Buffer,
}
impl AsBindGroup for DebrisExtension {
    type Data = ();
    type Param = ();
    fn label() -> &'static str {
        "DebrisExtension"
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
        crate::atmosphere::material::bind_haze(&self.atmosphere, out);
        Ok(())
    }
    fn bind_group_layout_entries(_: &RenderDevice, _: bool) -> Vec<BindGroupLayoutEntry> {
        vec![crate::atmosphere::material::haze_layout()]
    }
}
pub type DebrisMaterial = ExtendedMaterial<StandardMaterial, DebrisExtension>;
impl MaterialExtension for DebrisExtension {
    fn fragment_shader() -> ShaderRef {
        "embedded://openmars_engine/nuclear/debris.wesl".into()
    }
    // Hidden instance tags require fragment evaluation. Bevy omits an opaque
    // depth-only prepass fragment stage, so let the forward pass own their depth.
    fn enable_prepass() -> bool {
        false
    }
    fn enable_shadows() -> bool {
        false
    }
    fn specialize(
        _: &MaterialExtensionPipeline,
        d: &mut RenderPipelineDescriptor,
        _: &MeshVertexBufferLayoutRef,
        _: MaterialExtensionKey<Self>,
    ) -> Result<(), SpecializedMeshPipelineError> {
        d.vertex.shader_defs.push(ShaderDefVal::Bool(
            "VERTEX_OUTPUT_INSTANCE_INDEX".into(),
            true,
        ));
        if let Some(f) = &mut d.fragment {
            f.shader_defs.push(ShaderDefVal::Bool(
                "VERTEX_OUTPUT_INSTANCE_INDEX".into(),
                true,
            ));
        }
        Ok(())
    }
}
pub struct DebrisMaterialPlugin;
impl Plugin for DebrisMaterialPlugin {
    fn build(&self, app: &mut App) {
        bevy::asset::embedded_asset!(app, "debris.wesl");
        app.add_plugins(MaterialPlugin::<DebrisMaterial>::default());
    }
}
