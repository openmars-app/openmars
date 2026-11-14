//! Original PBR moss disc, including its two authored noise maps and soft hex edge.
use super::WorldTreeUniform;
use bevy::{
    ecs::system::SystemParamItem,
    mesh::MeshVertexBufferLayoutRef,
    pbr::{ExtendedMaterial, MaterialExtension, MaterialExtensionKey, MaterialExtensionPipeline},
    prelude::*,
    render::{render_resource::*, renderer::RenderDevice},
    shader::{ShaderDefVal, ShaderRef},
};
#[derive(Asset, TypePath, Clone, Debug, AsBindGroup)]
pub struct Textures {
    #[dependency]
    #[texture(101)]
    #[sampler(102)]
    pub mid: Handle<Image>,
    #[dependency]
    #[texture(103)]
    #[sampler(104)]
    pub high: Handle<Image>,
}
#[derive(Asset, TypePath, Clone, Debug)]
pub struct MossExtension {
    pub parameters: Buffer,
    #[dependency]
    pub textures: Textures,
    pub atmosphere: bevy::render::render_resource::Buffer,
}
pub type MossMaterial = ExtendedMaterial<StandardMaterial, MossExtension>;
impl AsBindGroup for MossExtension {
    type Data = <Textures as AsBindGroup>::Data;
    type Param = <Textures as AsBindGroup>::Param;
    fn label() -> &'static str {
        "World tree moss"
    }
    fn bind_group_data(&self) -> Self::Data {
        self.textures.bind_group_data()
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
        out.push((
            100,
            UnpreparedBindingResource::Buffer(self.parameters.clone()),
        ));
        crate::atmosphere::material::bind_haze(&self.atmosphere, out);
        Ok(())
    }
    fn bind_group_layout_entries(device: &RenderDevice, _: bool) -> Vec<BindGroupLayoutEntry> {
        let mut entries = Textures::bind_group_layout_entries(device, true);
        entries.push(BindGroupLayoutEntry {
            binding: 100,
            visibility: ShaderStages::VERTEX_FRAGMENT,
            ty: BindingType::Buffer {
                ty: BufferBindingType::Uniform,
                has_dynamic_offset: false,
                min_binding_size: Some(WorldTreeUniform::min_size()),
            },
            count: None,
        });
        entries.push(crate::atmosphere::material::haze_layout());
        entries.sort_by_key(|e| e.binding);
        entries
    }
}
impl MaterialExtension for MossExtension {
    fn vertex_shader() -> ShaderRef {
        "embedded://openmars_engine/structures/shaders/world_tree_ground.wesl".into()
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
        if let Some(depth) = &mut d.depth_stencil {
            depth.depth_write_enabled = Some(false);
            depth.depth_compare = Some(CompareFunction::GreaterEqual);
        }
        Ok(())
    }
}
pub fn material(
    parameters: Buffer,
    atmosphere: Buffer,
    mid: Handle<Image>,
    high: Handle<Image>,
) -> MossMaterial {
    ExtendedMaterial {
        base: StandardMaterial {
            base_color: Color::linear_rgb(0.12, 0.2, 0.06),
            perceptual_roughness: 0.92,
            metallic: 0.,
            double_sided: true,
            cull_mode: None,
            alpha_mode: AlphaMode::Blend,
            ..default()
        },
        extension: MossExtension {
            atmosphere,
            parameters,
            textures: Textures { mid, high },
        },
    }
}
