import openmars_engine::atmosphere::common::{source_linear,source_aces};
import bevy_pbr::render::{mesh_functions,view_transformations::{position_world_to_view,position_view_to_clip,position_world_to_clip}};
struct Parameters {mode:vec4<u32>}
@group(3) @binding(0) var<uniform> params:Parameters;
@group(3) @binding(1) var smoke:texture_2d<f32>;
@group(3) @binding(2) var smoke_sampler:sampler;
struct Vertex {@builtin(instance_index) instance:u32,@location(0) position:vec3<f32>,@location(1) uv:vec2<f32>}
struct Output {@builtin(position) position:vec4<f32>,@location(0) uv:vec2<f32>,@location(1) @interpolate(flat) tag:u32}
@vertex fn vertex(input:Vertex)->Output {
 var out:Output;let model=mesh_functions::get_world_from_local(input.instance);let tag=mesh_functions::get_tag(input.instance);
 let center=mesh_functions::mesh_position_local_to_world(model,vec4<f32>(0.0,0.0,0.0,1.0));
 if params.mode.x==4u {
  out.position=position_world_to_clip(mesh_functions::mesh_position_local_to_world(model,vec4<f32>(input.position,1.0)).xyz);
 } else {
  let scale=length(model[0]);let angle=f32((tag>>10u)&1023u)/1023.0*6.28318530718;
  let c=cos(angle);let s=sin(angle);let p=input.position.xy;
  let offset=vec2<f32>(p.x*c-p.y*s,p.x*s+p.y*c)*scale;
  out.position=position_view_to_clip(position_world_to_view(center.xyz)+vec3<f32>(offset,0.0));
 }
 out.uv=input.uv;out.tag=tag;return out;
}
@fragment fn fragment(input:Output)->@location(0) vec4<f32>{
 let opacity=f32(input.tag&1023u)/1023.0;if opacity<=0.0{discard;}
 let tex=textureSample(smoke,smoke_sampler,input.uv);let life=f32(input.tag>>20u)/4095.0;
 var color=vec3<f32>(0.0);
 if params.mode.x==0u {let base=vec3<f32>(0.95,0.93,0.9);color=mix(base,vec3<f32>(1.0),0.4)*tex.rgb+base*0.2;}
 if params.mode.x==1u {color=vec3<f32>(0.72,0.55,0.35)*tex.rgb;}
 if params.mode.x==2u {
  if life<0.15{color=mix(vec3<f32>(0.7,0.85,1.0),vec3<f32>(1.0,0.95,0.4),life/0.15);}
  else if life<0.4{color=mix(vec3<f32>(1.0,0.95,0.4),vec3<f32>(1.0,0.6,0.1),(life-0.15)/0.25);}
  else{color=mix(vec3<f32>(1.0,0.6,0.1),vec3<f32>(0.8,0.15,0.0),(life-0.4)/0.6);}
  color=(color+vec3<f32>(0.3)*(1.0-life))*tex.rgb;
 }
 if params.mode.x==3u {color=vec3<f32>(0.7,0.85,1.0)*tex.rgb;}
 if params.mode.x==4u {color=vec3<f32>(0.9,0.85,0.7)*tex.rgb;}
 // Source raw ShaderMaterials emit display RGB. Sprite/Basic core/exhaust
 // instead run the source ACES tone map and output conversion once.
 if params.mode.x==3u||params.mode.x==4u {color=source_aces(color);}
 else {color=source_linear(color);}
 let alpha=tex.a*opacity;
 // Bevy Add shares the premultiplied blend state. Multiplying RGB by the
 // source alpha and returning alpha0 reproduces Three SrcAlpha+One, without
 // darkening the destination or exposing transparent texture rectangles.
 if params.mode.x==2u||params.mode.x==3u {return vec4<f32>(color*alpha,0.0);}
 return vec4<f32>(color,alpha);
}
