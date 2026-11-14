import openmars_engine::atmosphere::common::{source_light,source_finish,source_display};
import openmars_engine::terrain::shaders::highlight::apply_tile_highlight;
import bevy_pbr::render::{mesh_functions,mesh_bindings::mesh,mesh_view_bindings::view,view_transformations::position_world_to_clip,pbr_types,pbr_functions};
struct Base {geometry:vec4<f32>,seeds:array<vec4<f32>,4>,profile:vec4<f32>,clock:vec4<f32>,climate:vec4<f32>,sun:vec4<f32>,sun_color:vec4<f32>,local_x:vec4<f32>,local_y:vec4<f32>,origin:vec4<f32>,motion:vec4<f32>,collapse_parts:array<vec4<f32>,5>}
struct Parameters {base:Base,stages:vec4<f32>,fence_surface:vec4<f32>}
@group(3) @binding(0) var<uniform> params:Parameters;
@group(3) @binding(1) var colors:texture_2d_array<f32>;
@group(3) @binding(2) var color_sampler:sampler;
@group(3) @binding(3) var mid_noise:texture_2d<f32>;
@group(3) @binding(4) var mid_sampler:sampler;
@group(3) @binding(5) var high_noise:texture_2d<f32>;
@group(3) @binding(6) var high_sampler:sampler;
@group(3) @binding(7) var fence:texture_2d<f32>;
@group(3) @binding(8) var fence_sampler:sampler;
struct Vertex {@builtin(instance_index) instance:u32,@location(0) position:vec3<f32>,@location(1) normal:vec3<f32>,@location(2) uv:vec2<f32>,@location(3) kind:vec2<f32>,@location(4) color:vec4<f32>,@location(5) growth:vec4<f32>}
struct Output {@builtin(position) position:vec4<f32>,@location(0) world_position:vec3<f32>,@location(1) normal:vec3<f32>,@location(2) uv:vec2<f32>,@location(3) local:vec3<f32>,@location(4) color:vec4<f32>,@location(5) @interpolate(flat) kind:u32,@location(6) @interpolate(flat) instance:u32,@location(7) depth:f32}
fn rotate(v:vec3<f32>,axis:vec3<f32>,angle:f32)->vec3<f32>{let c=cos(angle);return v*c+cross(axis,v)*sin(angle)+axis*dot(axis,v)*(1.0-c);}
@vertex fn vertex(input:Vertex)->Output {
 let b=params.base;let kind=u32(input.kind.x+0.5);let model=mesh_functions::get_world_from_local(input.instance);
 var p=input.position;var n=input.normal;
 let radial=normalize(cross(b.local_x.xyz,b.local_y.xyz));
 let delta=input.position-b.origin.xyz;let local=vec3<f32>(dot(delta,b.local_x.xyz),dot(delta,b.local_y.xyz),dot(delta,radial));
 if kind==3u {p-=input.growth.xyz*(1.0-params.stages.z);n=normalize(n+radial*dot(n,radial)*(1.0/max(params.stages.z,0.0001)-1.0));}
 if kind!=3u {p+=b.local_x.xyz*b.motion.x+b.local_y.xyz*b.motion.y;}
 if kind<2u {p=normalize(mesh_functions::mesh_position_local_to_world(model,vec4<f32>(p,1.0)).xyz)*(b.geometry.x+select(0.006,0.007,kind==1u));}
 var world=mesh_functions::mesh_position_local_to_world(model,vec4<f32>(p,1.0)).xyz;
 if kind<2u {world=p;}
 n=normalize(mesh_functions::mesh_normal_local_to_world(n,input.instance));
 let collapse=b.collapse_parts[select(kind+2u,0u,kind==3u)];
 if collapse.z!=0.0||collapse.w!=0.0 {
  let axis=normalize(mesh_functions::mesh_normal_local_to_world(b.local_x.xyz*collapse.x+b.local_y.xyz*collapse.y,input.instance));
  let origin=mesh_functions::mesh_position_local_to_world(model,vec4<f32>(b.origin.xyz,1.0)).xyz;
  let radial=normalize(mesh_functions::mesh_normal_local_to_world(normalize(b.origin.xyz),input.instance));
  world=origin+rotate(world-origin,axis,collapse.z)-radial*collapse.w;n=rotate(n,axis,collapse.z);
 }
 var out:Output;out.position=position_world_to_clip(world);out.world_position=world;out.normal=n;out.uv=input.uv;out.local=local;out.color=input.color;out.kind=kind;out.instance=input.instance;out.depth=length(input.position)-b.geometry.x;return out;
}
fn color_sample(uv:vec2<f32>,dx:vec2<f32>,dy:vec2<f32>)->vec3<f32>{let p=vec2<f32>(uv.x,-uv.y);return textureSampleGrad(colors,color_sampler,p,8,dx*vec2<f32>(1.0,-1.0),dy*vec2<f32>(1.0,-1.0)).rgb;}
@fragment fn fragment(in:Output,@builtin(front_facing) front:bool)->@location(0) vec4<f32> {
 let du=dpdx(in.uv);let dv=dpdy(in.uv);
 let edge_radius=0.098;
 let noise_uv=in.local.xy*(1.5/edge_radius)+params.base.origin.xy*37.0;let noise_uv2=in.local.xy*(1.5/edge_radius)*2.5+params.base.origin.xy*37.0*1.7;
 let ndx=dpdx(noise_uv)*vec2<f32>(1.0,-1.0);let ndy=dpdy(noise_uv)*vec2<f32>(1.0,-1.0);let ndx2=dpdx(noise_uv2)*vec2<f32>(1.0,-1.0);let ndy2=dpdy(noise_uv2)*vec2<f32>(1.0,-1.0);
 let is_ground=in.kind==0u;
 @if(SITE_GROUND) {if !is_ground {discard;}}
 @if(!SITE_GROUND) {if is_ground {discard;}}
 var pbr=pbr_types::pbr_input_new();var color=in.color.rgb;var alpha=params.base.clock.z;var rough=0.95;var metal=0.0;
 if is_ground {
  if in.kind==0u {color=color_sample(in.uv*2.0,du*2.0,dv*2.0)*vec3<f32>(0.3,0.28,0.26);alpha*=params.stages.x;metal=0.05;}
  let r=edge_radius;let p=in.local.xy;let d=max(abs(p.x),max(abs(p.x*0.5+p.y*0.866025404),abs(-p.x*0.5+p.y*0.866025404)))-r*0.866025404;
  let uv=noise_uv;let uv2=noise_uv2;
  let a=textureSampleGrad(mid_noise,mid_sampler,vec2<f32>(uv.x,-uv.y),ndx,ndy).r*2.0-1.0;
  let b=textureSampleGrad(high_noise,high_sampler,vec2<f32>(uv2.x,-uv2.y),ndx2,ndy2).r*2.0-1.0;
  alpha*=pow(smoothstep(0.0,r*0.35,-d+r*0.25+(a*0.7+b*0.3)*r*0.2),1.1);
 } else if in.kind==1u {
  let stripe=step(0.0,sin(atan2(in.local.y,in.local.x)*24.0));color=color_sample(in.uv*8.0,du*8.0,dv*8.0)*0.3+mix(vec3<f32>(0.08,0.07,0.06),vec3<f32>(0.75,0.45,0.05),stripe)*0.6;alpha*=params.stages.x;
 } else if in.kind==2u {color=vec3<f32>(0.6583748,0.3515326,0.0085681);rough=0.4;metal=0.5;alpha*=params.stages.y;}
 else if in.kind==3u {
  let sample=textureSampleGrad(fence,fence_sampler,in.uv,du,dv);color=sample.rgb*in.color.rgb;alpha*=sample.a;rough=params.fence_surface.x;metal=params.fence_surface.y;
  if sample.a<0.5||params.stages.z<=0.0 {discard;}
 }
 if alpha<0.01 {discard;}
 pbr.frag_coord=in.position;pbr.world_position=vec4<f32>(in.world_position,1.0);pbr.flags=mesh[in.instance].flags;
 let logical=pbr_functions::winding_corrected_front_facing(pbr.flags,front);
 pbr.world_normal=pbr_functions::prepare_world_normal(in.normal,true,logical);pbr.N=pbr.world_normal;
 pbr.is_orthographic=view.clip_from_view[3].w==1.0;pbr.V=pbr_functions::calculate_view(pbr.world_position,pbr.is_orthographic);
 pbr.material.base_color=vec4<f32>(color,alpha);pbr.material.perceptual_roughness=rough;pbr.material.metallic=metal;
 pbr.material.flags=pbr_types::STANDARD_MATERIAL_FLAGS_ALPHA_MODE_BLEND|pbr_types::STANDARD_MATERIAL_FLAGS_DOUBLE_SIDED_BIT|pbr_types::STANDARD_MATERIAL_FLAGS_FOG_ENABLED_BIT;
 var lit=pbr_functions::apply_pbr_lighting(pbr);
 if in.kind!=1u {lit=source_light(pbr,lit);}
 if in.kind==1u {
  // Original warning stripe is a display-RGB custom ShaderMaterial.
  lit=vec4<f32>(source_display(color,in.world_position,pbr.N),alpha);
 }
 if in.kind==2u {let v=clamp((0.005-in.depth)/0.045,0.0,1.0);lit=vec4<f32>(lit.rgb*(1.0-v*v),lit.a);}
 lit=vec4<f32>(apply_tile_highlight(lit.rgb,in.world_position),lit.a);
 if in.kind==1u {return pbr_functions::main_pass_post_lighting_processing(pbr,lit);}
 return source_finish(pbr,lit);
}
