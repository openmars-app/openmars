//! Preserve authored GLB PBR properties while binding the shared scene haze.
use bevy::{
    asset::AssetId,
    ecs::system::{SystemParamItem, lifetimeless::SRes},
    mesh::MeshVertexBufferLayoutRef,
    pbr::{ExtendedMaterial, MaterialExtension, MaterialExtensionKey, MaterialExtensionPipeline},
    prelude::*,
    render::{
        render_asset::RenderAssets,
        render_resource::*,
        renderer::{RenderDevice, RenderQueue},
        texture::GpuImage,
    },
    shader::ShaderRef,
};
use std::collections::BTreeMap;
#[derive(Asset, TypePath, Clone, Debug)]
pub struct HazeExtension {
    pub atmosphere: Buffer,
    pub no_environment: bool,
    pub highlights: Buffer,
    #[dependency]
    pub highlight_data: Handle<Image>,
}
pub type HazeMaterial = ExtendedMaterial<StandardMaterial, HazeExtension>;
pub type SourceMaterials = BTreeMap<AssetId<StandardMaterial>, Handle<HazeMaterial>>;
impl AsBindGroup for HazeExtension {
    type Data = bool;
    type Param = SRes<RenderAssets<GpuImage>>;
    fn label() -> &'static str {
        "HazeExtension"
    }
    fn bind_group_data(&self) -> bool {
        self.no_environment
    }
    fn build_bind_group(
        &self,
        _: &BindGroupLayout,
        _: &RenderDevice,
        images: &mut SystemParamItem<'_, '_, Self::Param>,
        _: bool,
        out: &mut BindGroupBuilder,
    ) -> Result<(), AsBindGroupError> {
        let image = images
            .get(self.highlight_data.id())
            .ok_or(AsBindGroupError::RetryNextUpdate)?;
        super::material::bind_haze(&self.atmosphere, out);
        out.push((
            32,
            UnpreparedBindingResource::Buffer(self.highlights.clone()),
        ));
        out.push((
            33,
            UnpreparedBindingResource::TextureView(
                TextureViewDimension::D2,
                image.texture_view.clone(),
            ),
        ));
        Ok(())
    }
    fn bind_group_layout_entries(_: &RenderDevice, _: bool) -> Vec<BindGroupLayoutEntry> {
        vec![
            super::material::haze_layout(),
            BindGroupLayoutEntry {
                binding: 32,
                visibility: ShaderStages::FRAGMENT,
                ty: BindingType::Buffer {
                    ty: BufferBindingType::Uniform,
                    has_dynamic_offset: false,
                    min_binding_size: Some(crate::terrain::materials::HighlightUniform::min_size()),
                },
                count: None,
            },
            BindGroupLayoutEntry {
                binding: 33,
                visibility: ShaderStages::FRAGMENT,
                ty: BindingType::Texture {
                    sample_type: TextureSampleType::Float { filterable: false },
                    view_dimension: TextureViewDimension::D2,
                    multisampled: false,
                },
                count: None,
            },
        ]
    }
}
impl MaterialExtension for HazeExtension {
    fn specialize(
        _: &MaterialExtensionPipeline,
        descriptor: &mut RenderPipelineDescriptor,
        _: &MeshVertexBufferLayoutRef,
        key: MaterialExtensionKey<Self>,
    ) -> Result<(), SpecializedMeshPipelineError> {
        if let Some(fragment) = &mut descriptor.fragment {
            fragment.shader_defs.push(bevy::shader::ShaderDefVal::Bool(
                "STANDARD_HIGHLIGHTS".into(),
                true,
            ));
        }
        if key.bind_group_data
            && let Some(fragment) = &mut descriptor.fragment
        {
            fragment.shader_defs.push(bevy::shader::ShaderDefVal::Bool(
                "SOURCE_NO_ENVIRONMENT".into(),
                true,
            ));
        }
        Ok(())
    }
    fn fragment_shader() -> ShaderRef {
        "embedded://openmars_engine/atmosphere/standard.wesl".into()
    }
}
#[derive(Resource)]
pub struct DisabledHighlights {
    buffer: Buffer,
    data: Handle<Image>,
}
/// Allocate before the first ready frame, so every audience shares a bounded
/// disabled binding and the idle resource baseline includes this renderer cache.
pub fn initialize_highlights(world: &mut World) {
    if world.contains_resource::<DisabledHighlights>() {
        return;
    }
    let mut uniform: UniformBuffer<crate::terrain::materials::HighlightUniform> =
        crate::terrain::materials::HighlightUniform::default().into();
    uniform.set_label(Some("disabled planet tile highlights"));
    uniform.write_buffer(
        world.resource::<RenderDevice>(),
        world.resource::<RenderQueue>(),
    );
    let image = Image::new(
        Extent3d {
            width: 1,
            height: 1,
            depth_or_array_layers: 1,
        },
        TextureDimension::D2,
        vec![0; 16],
        TextureFormat::Rgba32Float,
        bevy::asset::RenderAssetUsages::RENDER_WORLD,
    );
    let data = world.resource_mut::<Assets<Image>>().add(image);
    world.insert_resource(DisabledHighlights {
        buffer: uniform.buffer().unwrap().clone(),
        data,
    });
}
pub fn disabled_highlights(world: &World) -> (Buffer, Handle<Image>) {
    let defaults = world.resource::<DisabledHighlights>();
    (defaults.buffer.clone(), defaults.data.clone())
}
pub fn material(world: &mut World, base: StandardMaterial) -> Result<Handle<HazeMaterial>, String> {
    let atmosphere = world.resource::<super::AtmosphereRuntime>().binding()?;
    initialize_highlights(world);
    let (highlights, highlight_data) = disabled_highlights(world);
    Ok(world
        .resource_mut::<Assets<HazeMaterial>>()
        .add(HazeMaterial {
            base,
            extension: HazeExtension {
                atmosphere,
                no_environment: false,
                highlights,
                highlight_data,
            },
        }))
}
/// Call only once the source WorldInstance is ready; replace its real mesh users.
pub fn wrap_sources(
    world: &mut World,
    entities: &[Entity],
    cache: &mut SourceMaterials,
) -> Result<(), String> {
    for &entity in entities {
        if world.get::<Mesh3d>(entity).is_none()
            || world.get::<MeshMaterial3d<HazeMaterial>>(entity).is_some()
        {
            continue;
        }
        let source = world
            .get::<MeshMaterial3d<StandardMaterial>>(entity)
            .ok_or("Source GLB mesh has no PBR material")?
            .0
            .clone();
        let handle = if let Some(handle) = cache.get(&source.id()) {
            handle.clone()
        } else {
            if cache.len() >= 64 {
                return Err("Source haze material bound exceeded".into());
            }
            let base = world
                .resource::<Assets<StandardMaterial>>()
                .get(&source)
                .ok_or("Source GLB material is not loaded")?
                .clone();
            let handle = material(world, base)?;
            cache.insert(source.id(), handle.clone());
            handle
        };
        world
            .entity_mut(entity)
            .remove::<MeshMaterial3d<StandardMaterial>>()
            .insert(MeshMaterial3d(handle));
    }
    Ok(())
}
pub struct HazeMaterialPlugin;
impl Plugin for HazeMaterialPlugin {
    fn build(&self, app: &mut App) {
        bevy::asset::embedded_asset!(app, "standard.wesl");
        app.add_plugins(MaterialPlugin::<HazeMaterial>::default());
    }
}
