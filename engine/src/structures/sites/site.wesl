import openmars_engine::atmosphere::common::{source_light,source_finish};
import openmars_engine::terrain::shaders::highlight::apply_tile_highlight;
import bevy_pbr::render::{mesh_functions,mesh_bindings::mesh,mesh_view_bindings::view,view_transformations::position_world_to_clip,pbr_types,pbr_functions};
struct Parameters {geometry:vec4<f32>,clock:vec4<f32>,local_x:vec4<f32>,local_y:vec4<f32>,origin:vec4<f32>,motion:vec4<f32>,collapse_parts:array<vec4<f32>,32>,stages:vec4<f32>,owner_color:vec4<f32>,cart:vec4<f32>}
@group(3) @binding(0) var<uniform> params:Parameters;
@group(3) @binding(1) var mid:texture_2d<f32>;
@group(3) @binding(2) var mid_sampler:sampler;
@group(3) @binding(3) var high:texture_2d<f32>;
@group(3) @binding(4) var high_sampler:sampler;
struct Vertex {@builtin(instance_index) instance:u32,@location(0) position:vec3<f32>,@location(1) normal:vec3<f32>,@location(2) uv:vec2<f32>,@location(3) kind:vec2<f32>,@location(4) color:vec4<f32>}
struct Output {@builtin(position) position:vec4<f32>,@location(0) world_position:vec3<f32>,@location(1) normal:vec3<f32>,@location(2) uv:vec2<f32>,@location(3) local:vec3<f32>,@location(4) color:vec4<f32>,@location(5) @interpolate(flat) kind:u32,@location(6) @interpolate(flat) instance:u32,@location(7) depth:f32}
fn rotate(v:vec3<f32>,axis:vec3<f32>,angle:f32)->vec3<f32>{let c=cos(angle);return v*c+cross(axis,v)*sin(angle)+axis*dot(axis,v)*(1.0-c);}
@vertex fn vertex(input:Vertex)->Output {
 let b=params;let kind=u32(input.kind.x+0.5);let model=mesh_functions::get_world_from_local(input.instance);
 var p=input.position;var n=input.normal;
 let radial=normalize(cross(b.local_x.xyz,b.local_y.xyz));
 let delta=input.position-b.origin.xyz;let local=vec3<f32>(dot(delta,b.local_x.xyz),dot(delta,b.local_y.xyz),dot(delta,radial));
 if kind==5u||kind==6u {
  let progress=select(params.stages.x,params.stages.y,kind==6u);
  p-=radial*local.z*(1.0-progress);n=normalize(n+radial*dot(n,radial)*(1.0/max(progress,0.0001)-1.0));
 }
 if kind>=13u&&kind<=15u {
  let dump=params.cart.z;
  let angle=b.geometry.z;let along=b.local_x.xyz*(-sin(angle))+b.local_y.xyz*cos(angle);
  let y=params.cart.x;let z=params.cart.y;
  p=b.origin.xyz+rotate(delta,along,dump)*b.clock.y+along*y+radial*z;n=rotate(n,along,dump);
 }
 if kind!=3u {p+=b.local_x.xyz*b.motion.x+b.local_y.xyz*b.motion.y;}
 if kind==4u||kind==7u {
  let projected=mesh_functions::mesh_position_local_to_world(model,vec4<f32>(p,1.0)).xyz;
  let world=normalize(projected)*(b.geometry.x+0.003);
  p=world;
 }
 var world=mesh_functions::mesh_position_local_to_world(model,vec4<f32>(p,1.0)).xyz;
 if kind==4u||kind==7u {world=p;}
 n=normalize(mesh_functions::mesh_normal_local_to_world(n,input.instance));
 let collapse=b.collapse_parts[u32(input.kind.y+0.5)];
 if collapse.z!=0.0||collapse.w!=0.0 {
  let axis=normalize(mesh_functions::mesh_normal_local_to_world(b.local_x.xyz*collapse.x+b.local_y.xyz*collapse.y,input.instance));
  let origin=mesh_functions::mesh_position_local_to_world(model,vec4<f32>(b.origin.xyz,1.0)).xyz;
  let radial=normalize(mesh_functions::mesh_normal_local_to_world(normalize(b.origin.xyz),input.instance));
  world=origin+rotate(world-origin,axis,collapse.z)-radial*collapse.w;n=rotate(n,axis,collapse.z);
 }
 var out:Output;out.position=position_world_to_clip(world);out.world_position=world;out.normal=n;out.uv=input.uv;out.local=local;out.color=input.color;out.kind=kind;out.instance=input.instance;out.depth=length(input.position)-b.geometry.x;return out;
}
@fragment fn fragment(in:Output,@builtin(front_facing) front:bool)->@location(0) vec4<f32> {
 let edge_radius=select(0.166,0.136,in.kind==4u);
 let noise_uv=in.local.xy*(1.5/edge_radius)+params.origin.xy*37.0;let noise_uv2=in.local.xy*(1.5/edge_radius)*2.5+params.origin.xy*37.0*1.7;
 let ndx=dpdx(noise_uv)*vec2<f32>(1.0,-1.0);let ndy=dpdy(noise_uv)*vec2<f32>(1.0,-1.0);let ndx2=dpdx(noise_uv2)*vec2<f32>(1.0,-1.0);let ndy2=dpdy(noise_uv2)*vec2<f32>(1.0,-1.0);
 let is_ground=in.kind==4u||in.kind==7u;
 @if(SITE_GROUND) {if !is_ground {discard;}}
 @if(!SITE_GROUND) {if is_ground {discard;}}
 var pbr=pbr_types::pbr_input_new();var color=in.color.rgb;var alpha=params.clock.z;var rough=0.95;var metal=0.0;
 if is_ground {
  if in.kind==4u {rough=0.9;}
  let r=edge_radius;let p=in.local.xy;let d=max(abs(p.x),max(abs(p.x*0.5+p.y*0.866025404),abs(-p.x*0.5+p.y*0.866025404)))-r*0.866025404;
  let uv=noise_uv;let uv2=noise_uv2;
  let a=textureSampleGrad(mid,mid_sampler,vec2<f32>(uv.x,-uv.y),ndx,ndy).r*2.0-1.0;
  let b=textureSampleGrad(high,high_sampler,vec2<f32>(uv2.x,-uv2.y),ndx2,ndy2).r*2.0-1.0;
  alpha*=pow(smoothstep(0.0,r*0.35,-d+r*0.25+(a*0.7+b*0.3)*r*0.2),1.1);
 }
 if in.kind==5u||in.kind==6u {color*=params.owner_color.rgb;rough=0.4;metal=0.3;let progress=select(params.stages.x,params.stages.y,in.kind==6u);if progress<=0.0 {discard;}}
 if in.kind==9u {rough=0.9;metal=0.05;}
 if in.kind==11u {rough=0.9;}
 if in.kind==12u {rough=0.3;metal=0.7;}
 if in.kind==13u||in.kind==14u {rough=0.5;metal=0.5;}
 if in.kind==15u {rough=0.8;metal=0.2;if params.cart.w<0.5 {discard;}}
 if in.kind==16u {metal=0.1;}
 if in.kind==17u {rough=1.0;}
 if in.kind>=13u&&in.kind<=15u&&params.clock.y<=0.0 {discard;}
 if alpha<0.01 {discard;}
 pbr.frag_coord=in.position;pbr.world_position=vec4<f32>(in.world_position,1.0);pbr.flags=mesh[in.instance].flags;
 let logical=pbr_functions::winding_corrected_front_facing(pbr.flags,front);
 pbr.world_normal=pbr_functions::prepare_world_normal(in.normal,true,logical);pbr.N=pbr.world_normal;
 pbr.is_orthographic=view.clip_from_view[3].w==1.0;pbr.V=pbr_functions::calculate_view(pbr.world_position,pbr.is_orthographic);
 pbr.material.base_color=vec4<f32>(color,alpha);pbr.material.perceptual_roughness=rough;pbr.material.metallic=metal;
 pbr.material.flags=pbr_types::STANDARD_MATERIAL_FLAGS_ALPHA_MODE_BLEND|pbr_types::STANDARD_MATERIAL_FLAGS_DOUBLE_SIDED_BIT|pbr_types::STANDARD_MATERIAL_FLAGS_FOG_ENABLED_BIT;
 var lit=source_light(pbr,pbr_functions::apply_pbr_lighting(pbr));
 lit=vec4<f32>(apply_tile_highlight(lit.rgb,in.world_position),lit.a);
 return source_finish(pbr,lit);
}
