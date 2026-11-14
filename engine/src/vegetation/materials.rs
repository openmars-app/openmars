//! Foliage uses the exact standard vertex path in forward, depth and shadow
//! passes. Per-plant world transforms are shared through Bevy's instance data.
use bevy::{
    mesh::MeshVertexBufferLayoutRef,
    pbr::{
        ExtendedMaterial, MaterialExtension, MaterialExtensionKey, MaterialExtensionPipeline,
        MeshPipelineKey,
    },
    prelude::*,
    render::render_resource::{
        AsBindGroup, CompareFunction, RenderPipelineDescriptor, ShaderType,
        SpecializedMeshPipelineError,
    },
    shader::{ShaderDefVal, ShaderRef},
};
#[derive(Clone, Copy, Debug, ShaderType)]
pub struct FoliageUniform {
    pub frost: Vec4,
}
#[derive(Asset, TypePath, Clone, Debug)]
pub struct FoliageExtension {
    pub parameters: bevy::render::render_resource::Buffer,
    pub atmosphere: bevy::render::render_resource::Buffer,
}
impl AsBindGroup for FoliageExtension {
    fn label() -> &'static str {
        "FoliageExtension"
    }
    type Data = ();
    type Param = ();
    fn bind_group_data(&self) {}
    fn build_bind_group(
        &self,
        _: &bevy::render::render_resource::BindGroupLayout,
        _: &bevy::render::renderer::RenderDevice,
        _: &mut bevy::ecs::system::SystemParamItem<'_, '_, Self::Param>,
        _: bool,
        out: &mut bevy::render::render_resource::BindGroupBuilder,
    ) -> Result<(), bevy::render::render_resource::AsBindGroupError> {
        out.push((
            100,
            bevy::render::render_resource::UnpreparedBindingResource::Buffer(
                self.parameters.clone(),
            ),
        ));
        crate::atmosphere::material::bind_haze(&self.atmosphere, out);
        Ok(())
    }
    fn bind_group_layout_entries(
        _: &bevy::render::renderer::RenderDevice,
        _: bool,
    ) -> Vec<bevy::render::render_resource::BindGroupLayoutEntry> {
        use bevy::render::render_resource::*;
        vec![
            BindGroupLayoutEntry {
                binding: 100,
                visibility: ShaderStages::FRAGMENT,
                ty: BindingType::Buffer {
                    ty: BufferBindingType::Uniform,
                    has_dynamic_offset: false,
                    min_binding_size: Some(FoliageUniform::min_size()),
                },
                count: None,
            },
            crate::atmosphere::material::haze_layout(),
        ]
    }
}
pub type FoliageMaterial = ExtendedMaterial<StandardMaterial, FoliageExtension>;
impl MaterialExtension for FoliageExtension {
    fn fragment_shader() -> ShaderRef {
        "embedded://openmars_engine/vegetation/shaders/foliage.wesl".into()
    }
    fn prepass_fragment_shader() -> ShaderRef {
        "embedded://openmars_engine/vegetation/shaders/foliage_prepass.wesl".into()
    }
    fn specialize(
        _: &MaterialExtensionPipeline,
        d: &mut RenderPipelineDescriptor,
        _: &MeshVertexBufferLayoutRef,
        key: MaterialExtensionKey<Self>,
    ) -> Result<(), SpecializedMeshPipelineError> {
        // Both stock vertex paths must carry the instance used by coverage,
        // per-plant tint and local-normal frost into their custom fragment.
        d.vertex.shader_defs.push(ShaderDefVal::Bool(
            "VERTEX_OUTPUT_INSTANCE_INDEX".into(),
            true,
        ));
        if let Some(fragment) = &mut d.fragment {
            fragment.shader_defs.push(ShaderDefVal::Bool(
                "VERTEX_OUTPUT_INSTANCE_INDEX".into(),
                true,
            ));
        }
        let prepass = d
            .vertex
            .shader_defs
            .iter()
            .any(|s| matches!(s,ShaderDefVal::Bool(name,true) if name=="PREPASS_PIPELINE"));
        if !prepass
            && key.mesh_key.contains(MeshPipelineKey::DEPTH_PREPASS)
            && let Some(depth) = &mut d.depth_stencil
        {
            depth.depth_write_enabled = Some(false);
            depth.depth_compare = Some(CompareFunction::Equal);
        }
        Ok(())
    }
}
pub struct FoliageMaterialPlugin;
impl Plugin for FoliageMaterialPlugin {
    fn build(&self, app: &mut App) {
        bevy::shader::load_shader_library!(app, "shaders/foliage_coverage.wesl");
        bevy::asset::embedded_asset!(app, "shaders/foliage.wesl");
        bevy::asset::embedded_asset!(app, "shaders/foliage_prepass.wesl");
        app.add_plugins(MaterialPlugin::<FoliageMaterial>::default());
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::terrain::materials::{
        GridMaterial, GroundMaterial, LakeMaskMaterial, OceanMaterial,
    };
    #[test]
    fn camera_prepass_accepts_stock_foliage_positions_and_excludes_custom_surfaces() {
        assert!(FoliageMaterial::enable_prepass());
        assert!(!GroundMaterial::enable_prepass());
        assert!(!OceanMaterial::enable_prepass());
        assert!(!LakeMaskMaterial::enable_prepass());
        assert!(!GridMaterial::enable_prepass());
    }
}
