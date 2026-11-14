use crate::terrain::materials::HighlightUniform;
use bevy::{
    ecs::system::SystemParamItem,
    mesh::MeshVertexBufferLayoutRef,
    pbr::{MaterialPipeline, MaterialPipelineKey},
    prelude::*,
    render::{render_resource::*, renderer::RenderDevice},
    shader::{ShaderDefVal, ShaderRef},
};
#[derive(Clone, Copy, Debug, ShaderType)]
pub struct SpecialUniform {
    /// Body radius, f32 source seed, unused, kind (0 volcano,1 mohole).
    pub geometry: Vec4,
    pub seeds: [Vec4; 4],
    /// Height/depth, normalized crater/hole radius, crater depth, disc radius.
    pub profile: Vec4,
    /// Decorative clock seconds, emergence, visible coverage, hole-radius scale.
    pub clock: Vec4,
    pub climate: Vec4,
    pub sun: Vec4,
    pub sun_color: Vec4,
    pub local_x: Vec4,
    pub local_y: Vec4,
    pub origin: Vec4,
    /// Original local shakeXY, visual age since impact and full-motion destruction flag.
    pub motion: Vec4,
    pub collapse_parts: [Vec4; 5],
}
#[derive(Clone, Copy, Debug, Hash, PartialEq, Eq)]
pub enum SpecialPass {
    Core,
    Fringe,
    MoholeMask,
}
#[derive(Asset, TypePath, AsBindGroup, Clone, Debug)]
#[bind_group_data(SpecialPass)]
pub struct Textures {
    pub pass: SpecialPass,
    #[dependency]
    #[texture(1, dimension = "2d_array")]
    #[sampler(2)]
    pub colors: Handle<Image>,
    #[dependency]
    #[texture(3, filterable = false)]
    pub flow: Handle<Image>,
    #[dependency]
    #[texture(17, filterable = false)]
    pub highlight_data: Option<Handle<Image>>,
}
impl From<&Textures> for SpecialPass {
    fn from(t: &Textures) -> Self {
        t.pass
    }
}
#[derive(Asset, TypePath, Clone, Debug)]
pub struct SpecialMaterial {
    #[dependency]
    pub textures: Textures,
    pub parameters: Buffer,
    pub highlights: Buffer,
    pub atmosphere: bevy::render::render_resource::Buffer,
}
impl AsBindGroup for SpecialMaterial {
    type Data = SpecialPass;
    type Param = <Textures as AsBindGroup>::Param;
    fn label() -> &'static str {
        "SpecialMaterial"
    }
    fn bind_group_data(&self) -> SpecialPass {
        self.textures.pass
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
            0,
            UnpreparedBindingResource::Buffer(self.parameters.clone()),
        ));
        out.push((
            16,
            UnpreparedBindingResource::Buffer(self.highlights.clone()),
        ));
        crate::atmosphere::material::bind_haze(&self.atmosphere, out);
        Ok(())
    }
    fn bind_group_layout_entries(device: &RenderDevice, _: bool) -> Vec<BindGroupLayoutEntry> {
        let mut entries = Textures::bind_group_layout_entries(device, true);
        for (binding, size) in [
            (0, SpecialUniform::min_size()),
            (16, HighlightUniform::min_size()),
        ] {
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
        entries.push(crate::atmosphere::material::haze_layout());
        entries.sort_by_key(|e| e.binding);
        entries
    }
}
impl Material for SpecialMaterial {
    fn vertex_shader() -> ShaderRef {
        "embedded://openmars_engine/structures/special/special.wesl".into()
    }
    fn fragment_shader() -> ShaderRef {
        Self::vertex_shader()
    }
    fn alpha_mode(&self) -> AlphaMode {
        if self.textures.pass == SpecialPass::Fringe {
            AlphaMode::Blend
        } else {
            AlphaMode::Opaque
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
        ])?];
        d.primitive.cull_mode = None;
        let mask = key.bind_group_data == SpecialPass::MoholeMask;
        let edge = key.bind_group_data == SpecialPass::Fringe;
        for (name, value) in [("SPECIAL_MASK", mask), ("SPECIAL_EDGE", edge)] {
            d.vertex
                .shader_defs
                .push(ShaderDefVal::Bool(name.into(), value));
            if let Some(f) = &mut d.fragment {
                f.shader_defs.push(ShaderDefVal::Bool(name.into(), value));
            }
        }
        if let Some(depth) = &mut d.depth_stencil {
            depth.depth_write_enabled = Some(!mask && !edge);
            depth.depth_compare = Some(if mask {
                CompareFunction::Always
            } else {
                CompareFunction::GreaterEqual
            });
        }
        if mask && let Some(f) = &mut d.fragment {
            for target in f.targets.iter_mut().flatten() {
                target.write_mask = ColorWrites::RED;
                target.blend = None;
            }
        }
        Ok(())
    }
}
pub struct SpecialMaterialPlugin;
impl Plugin for SpecialMaterialPlugin {
    fn build(&self, app: &mut App) {
        bevy::shader::load_shader_library!(app, "special_shape.wesl");
        bevy::shader::load_shader_library!(app, "volcano_color.wesl");
        bevy::shader::load_shader_library!(app, "mohole_color.wesl");
        bevy::asset::embedded_asset!(app, "special.wesl");
        app.add_plugins(MaterialPlugin::<SpecialMaterial>::default());
    }
}
