import bevy_pbr::render::{mesh_functions,view_transformations::position_world_to_clip};
struct Parameters {surface:vec4<f32>,gouges:array<vec4<f32>,5>,entrance:vec4<f32>}
@group(3) @binding(0) var<uniform> params:Parameters;
struct Vertex {
 @builtin(instance_index) instance:u32,@location(0) position:vec3<f32>,@location(1) normal:vec3<f32>,
 @location(2) uv:vec2<f32>,@location(3) local:vec2<f32>,@location(4) coverage_apron:vec4<f32>,@location(5) direction:vec3<f32>,
}
struct Output {
 @builtin(position) position:vec4<f32>,@location(0) world_position:vec4<f32>,@location(1) world_normal:vec3<f32>,
 @location(2) uv:vec2<f32>,@location(3) local:vec2<f32>,@location(4) coverage_apron:vec2<f32>,@location(5) direction:vec3<f32>,@location(6) @interpolate(flat) instance:u32,
}
fn vertex(input:Vertex)->Output {
 var out:Output;let model=mesh_functions::get_world_from_local(input.instance);
 // Recover the original tangent plane before the Tile parent scale, then
 // reproject at the authored bowl/scorch height as the source shader does.
 let origin=params.entrance.xyz;let radial=normalize(origin);
 let plane=input.position*(dot(origin,radial)/dot(input.position,radial));
 let direction=normalize(origin+(plane-origin)*params.entrance.w);
 let projected=direction*length(input.position);
 out.world_position=mesh_functions::mesh_position_local_to_world(model,vec4<f32>(projected,1.0));out.position=position_world_to_clip(out.world_position.xyz);
 out.world_normal=mesh_functions::mesh_normal_local_to_world(input.normal,input.instance);out.uv=input.uv;out.local=input.local;out.coverage_apron=input.coverage_apron.xy;out.direction=input.direction;out.instance=input.instance;return out;
}
fn visible(pixel:vec2<f32>)->bool {
 if params.surface.z>=1.0{return true;}if params.surface.z<=0.0{return false;}
 let bayer=array<u32,16>(0u,8u,2u,10u,12u,4u,14u,6u,3u,11u,1u,9u,15u,7u,13u,5u);
 let p=vec2<u32>(floor(pixel))&vec2<u32>(3u);return params.surface.z>(f32(bayer[p.y*4u+p.x])+0.5)/16.0;
}
