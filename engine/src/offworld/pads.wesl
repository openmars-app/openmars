import openmars_engine::atmosphere::common::{source_pbr,source_display};
import bevy_pbr::render::{mesh_functions,mesh_view_bindings::view,view_transformations::position_world_to_clip,pbr_types,pbr_functions};
struct Parameters {mode:vec4<u32>}
@group(3) @binding(0) var<uniform> params:Parameters;
@group(3) @binding(1) var concrete:texture_2d<f32>;
@group(3) @binding(2) var concrete_sampler:sampler;
@group(3) @binding(3) var noise:texture_2d<f32>;
@group(3) @binding(4) var noise_sampler:sampler;
@group(3) @binding(5) var high_noise:texture_2d<f32>;
@group(3) @binding(6) var high_sampler:sampler;
struct Vertex {@builtin(instance_index) instance:u32,@location(0) position:vec3<f32>,@location(1) normal:vec3<f32>,@location(2) uv:vec2<f32>,@location(3) local:vec2<f32>,@location(4) color:vec4<f32>,@location(5) data:vec4<f32>}
struct Output {@builtin(position) position:vec4<f32>,@location(0) world:vec3<f32>,@location(1) normal:vec3<f32>,@location(2) uv:vec2<f32>,@location(3) local:vec2<f32>,@location(4) color:vec4<f32>,@location(5) offset:vec2<f32>}
@vertex fn vertex(input:Vertex)->Output {
 var out:Output;let model=mesh_functions::get_world_from_local(input.instance);
 out.world=mesh_functions::mesh_position_local_to_world(model,vec4<f32>(input.position,1.0)).xyz;
 out.position=position_world_to_clip(out.world);out.normal=normalize(mesh_functions::mesh_normal_local_to_world(input.normal,input.instance));
 out.uv=input.uv;out.local=input.local;out.color=input.color;out.offset=input.data.xy;return out;
}
@fragment fn fragment(input:Output,@builtin(front_facing) front:bool)->@location(0) vec4<f32> {
 if params.mode.x==1u {
  let base=textureSample(concrete,concrete_sampler,vec2<f32>(input.uv.x,-input.uv.y)*8.0).rgb;
  let angle=atan2(input.local.y,input.local.x);let stripe=step(0.0,sin(angle*24.0));
  let paint=mix(vec3<f32>(0.08,0.07,0.06),input.color.rgb,stripe);
  return vec4<f32>(source_display(base*0.3+paint*0.6,input.world,input.normal),1.0);
 }
 let p=input.local;let radius=0.096;
 let d=max(abs(p.x),max(abs(0.5*p.x+0.866025404*p.y),abs(-0.5*p.x+0.866025404*p.y)))-radius*0.866025404;
 let uv1=p*(1.5/radius)+input.offset;let uv2=p*(1.5/radius)*2.5+input.offset*1.7;
 let a=textureSample(noise,noise_sampler,vec2<f32>(uv1.x,-uv1.y)).r*2.0-1.0;
 let b=textureSample(high_noise,high_sampler,vec2<f32>(uv2.x,-uv2.y)).r*2.0-1.0;
 let alpha=pow(smoothstep(0.0,radius*0.35,-d+radius*0.25+(a*0.7+b*0.3)*radius*0.2),1.1);
 if alpha<0.01{discard;}
 let base=textureSample(concrete,concrete_sampler,vec2<f32>(input.uv.x,-input.uv.y)*2.0).rgb*vec3<f32>(0.3,0.28,0.26);
 var pbr=pbr_types::pbr_input_new();pbr.world_position=vec4<f32>(input.world,1.0);pbr.frag_coord=input.position;
 pbr.N=select(-input.normal,input.normal,front);pbr.world_normal=pbr.N;pbr.V=normalize(view.world_position-input.world);
 pbr.is_orthographic=view.clip_from_view[3].w==1.0;pbr.material.base_color=vec4<f32>(base,alpha);
 pbr.material.perceptual_roughness=0.95;pbr.material.metallic=0.05;
 pbr.material.flags=pbr_types::STANDARD_MATERIAL_FLAGS_ALPHA_MODE_BLEND|pbr_types::STANDARD_MATERIAL_FLAGS_DOUBLE_SIDED_BIT|pbr_types::STANDARD_MATERIAL_FLAGS_FOG_ENABLED_BIT;
 return source_pbr(pbr,pbr_functions::apply_pbr_lighting(pbr));
}
