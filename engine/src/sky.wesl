import openmars_engine::atmosphere::common::{source_aces,source_linear,source_srgb,atmosphere};
import bevy_pbr::render::{forward_io::{Vertex,decompress_vertex},mesh_view_bindings::view,utils::coords_to_viewport_uv};
@group(3) @binding(0) var sky:texture_cube<f32>;
@group(3) @binding(1) var sky_sampler:sampler;
struct Output {@builtin(position) position:vec4<f32>}
@vertex fn vertex(input:Vertex)->Output {
 let v=decompress_vertex(input,input.instance_index);
 // Reverse-Z far plane; depth is never written. Geometry always wins.
 return Output(vec4<f32>(v.position.xy,0.0,1.0));
}
@fragment fn fragment(in:Output)->@location(0) vec4<f32> {
 let uv=coords_to_viewport_uv(in.position.xy,view.main_pass_viewport);
 let near=view.view_from_clip*vec4<f32>(uv*vec2<f32>(2.0,-2.0)+vec2<f32>(-1.0,1.0),1.0,1.0);
 let ray=normalize((view.world_from_view*vec4<f32>(near.xyz/near.w,0.0)).xyz);
 let angle=atmosphere.lighting.y;
 let rotated=vec3<f32>(cos(angle)*ray.x+sin(angle)*ray.z,ray.y,-sin(angle)*ray.x+cos(angle)*ray.z);
 let color=max(textureSample(sky,sky_sampler,rotated*vec3<f32>(1.0,1.0,-1.0)).rgb,vec3<f32>(0.0));
 let luminance=dot(color,vec3<f32>(0.2126,0.7152,0.0722));
 let lift=1.0+1.2/(1.0+luminance/0.008);
 // A display-space dark overlay, independent of the radiance used by materials.
 let display=source_srgb(source_aces(color*lift*0.35))*atmosphere.lighting.z;
 return vec4<f32>(source_linear(display),1.0);
}
