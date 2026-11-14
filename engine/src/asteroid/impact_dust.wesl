import bevy_pbr::render::{mesh_functions,view_transformations::position_world_to_clip,pbr_types::{pbr_input_new,STANDARD_MATERIAL_FLAGS_ALPHA_MODE_BLEND},pbr_functions::main_pass_post_lighting_processing};
@group(3) @binding(0) var smoke:texture_2d<f32>;
@group(3) @binding(1) var smoke_sampler:sampler;
struct Vertex {@builtin(instance_index) instance:u32,@location(0) position:vec3<f32>,@location(1) uv:vec2<f32>}
struct Output {@builtin(position) position:vec4<f32>,@location(0) uv:vec2<f32>,@location(1) world:vec4<f32>,@location(2) @interpolate(flat) tag:u32}
@vertex fn vertex(in:Vertex)->Output {
 var out:Output;out.world=mesh_functions::mesh_position_local_to_world(mesh_functions::get_world_from_local(in.instance),vec4<f32>(in.position,1.0));
 out.position=position_world_to_clip(out.world.xyz);out.uv=in.uv;out.tag=mesh_functions::get_tag(in.instance);return out;
}
@fragment fn fragment(in:Output)->@location(0) vec4<f32> {
 if (in.tag&0x80000000u)!=0u {discard;}
 let value=textureSample(smoke,smoke_sampler,in.uv);let alpha=value.a*f32(in.tag&65535u)/65535.0;if alpha<0.0001{discard;}
 var pbr=pbr_input_new();pbr.material.flags=STANDARD_MATERIAL_FLAGS_ALPHA_MODE_BLEND;pbr.world_position=in.world;pbr.frag_coord=in.position;
 return main_pass_post_lighting_processing(pbr,vec4<f32>(value.rgb*vec3<f32>(0.8,0.4,0.15),alpha));
}
