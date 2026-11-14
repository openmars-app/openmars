import bevy_pbr::render::{mesh_functions,view_transformations::position_world_to_clip};
import openmars_engine::atmosphere::common::source_aces;
struct Parameters { data:vec4<f32> }
@group(3) @binding(0) var<uniform> params:Parameters;
@group(3) @binding(1) var image:texture_2d<f32>;
@group(3) @binding(2) var image_sampler:sampler;
struct Vertex { @builtin(instance_index) instance:u32,@location(0) position:vec3<f32>,@location(1) uv:vec2<f32> }
struct Output { @builtin(position) position:vec4<f32>,@location(0) uv:vec2<f32>,@location(1) @interpolate(flat) opacity:f32 }
@vertex fn vertex(input:Vertex)->Output {
 var out:Output;
 let model=mesh_functions::get_world_from_local(input.instance);
 out.position=position_world_to_clip(mesh_functions::mesh_position_local_to_world(model,vec4<f32>(input.position,1.0)).xyz);
 out.uv=input.uv;
 out.opacity=f32(mesh_functions::get_tag(input.instance)&65535u)/65535.0;
 return out;
}
@fragment fn fragment(input:Output)->@location(0) vec4<f32> {
 let sample=textureSample(image,image_sampler,input.uv);
 if params.data.x>1.5 {
  let alpha=sample.a*input.opacity;
  if alpha<0.1 {discard;}
  // Original MeshBasicMaterial icons are sRGB textures, linear .7 tint and
  // toneMapped=false. The sRGB attachment performs the final output conversion.
  return vec4<f32>(sample.rgb*0.7,alpha);
 }
 let distance=(sample.r-0.5)*2.0*params.data.y;
 let aa=max(fwidth(distance)*0.5,0.03125);
 let fill=smoothstep(-aa,aa,distance);
 let edge=smoothstep(-aa,aa,distance+params.data.z);
 let alpha=edge*input.opacity;
 if alpha<=0.0 {discard;}
 var color=vec3<f32>(0.0);
 if params.data.x<0.5 {color=source_aces(vec3<f32>(1.0))*fill/max(edge,0.000001);}
 return vec4<f32>(color,alpha);
}
