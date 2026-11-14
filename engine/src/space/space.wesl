import openmars_engine::atmosphere::common::{atmosphere,source_aces};
import bevy_pbr::render::{forward_io::{Vertex,decompress_vertex},mesh_functions,view_transformations::position_world_to_clip,mesh_view_bindings::view};
@group(3) @binding(0) var surface:texture_2d<f32>;
@group(3) @binding(1) var surface_sampler:sampler;
// Mode; orbit initial angle/period/Mars flag. These never change per frame.
@group(3) @binding(2) var<uniform> parameters:vec4<f32>;
struct Output {
 @builtin(position) position:vec4<f32>,
 @location(0) world_position:vec3<f32>,
 @location(1) world_normal:vec3<f32>,
 @location(2) uv:vec2<f32>,
 @location(3) local_position:vec3<f32>,
 @location(4) orbit_color:vec3<f32>,
}
@vertex fn vertex(input:Vertex)->Output {
 let v=decompress_vertex(input,input.instance_index);let transform=mesh_functions::get_world_from_local(input.instance_index);
 var out:Output;out.world_position=mesh_functions::mesh_position_local_to_world(transform,vec4<f32>(v.position,1.0)).xyz;
 out.world_normal=mesh_functions::mesh_normal_local_to_world(v.normal,input.instance_index);
 out.position=position_world_to_clip(out.world_position);out.uv=v.uv;out.local_position=v.position;out.orbit_color=orbit_color(v.uv.x);return out;
}
fn unit(v:vec3<f32>)->vec3<f32>{return v/max(length(v),0.000001);}
fn hash(p:vec3<f32>)->f32 {let v=fract(p*0.3183099+vec3<f32>(0.11,0.27,0.43))*17.0;return fract(v.x*v.y*v.z*(v.x+v.y+v.z));}
fn noise(p:vec3<f32>)->f32 {
 let i=floor(p);let a=fract(p);let f=a*a*(3.0-2.0*a);
 return mix(mix(mix(hash(i),hash(i+vec3<f32>(1.,0.,0.)),f.x),mix(hash(i+vec3<f32>(0.,1.,0.)),hash(i+vec3<f32>(1.,1.,0.)),f.x),f.y),mix(mix(hash(i+vec3<f32>(0.,0.,1.)),hash(i+vec3<f32>(1.,0.,1.)),f.x),mix(hash(i+vec3<f32>(0.,1.,1.)),hash(i+vec3<f32>(1.,1.,1.)),f.x),f.y),f.z);
}
fn orbit_color(phase:f32)->vec3<f32> {
 let angle=parameters.y+atmosphere.lighting.w*6.28318530718/max(parameters.z,0.0001);
 let delta=((angle-phase)%6.28318530718+6.28318530718)%6.28318530718;
 var brightness=0.08;
 if delta>0.005 && delta<2.51327412287 {let t=1.0-(delta-0.005)/(2.51327412287-0.005);brightness+=0.92*t*t;}
 let base=mix(vec3<f32>(0.25,0.35,0.5),vec3<f32>(0.8,0.3,0.2),parameters.w);
 return base*brightness+(vec3<f32>(1.0)-base)*max(0.0,brightness-0.08);
}
@fragment fn fragment(in:Output)->@location(0) vec4<f32> {
 let time=atmosphere.lighting.w;
 if parameters.x>2.5 {return vec4<f32>(source_aces(in.orbit_color),1.0);}
 let normal=unit(in.world_normal);let direction=unit(view.world_position-in.world_position);let facing=dot(normal,direction);
 if parameters.x<0.5 {
  let p=unit(in.local_position);let t=time*0.055;
  let flow=vec3<f32>(noise(p*7.0+vec3<f32>(t)),noise(p*7.0-vec3<f32>(t)+vec3<f32>(12.0)),noise(p*7.0+vec3<f32>(26.0)));
  let drift=(flow.xy-vec2<f32>(0.5))*0.009;
  let source=textureSample(surface,surface_sampler,vec2<f32>(fract(in.uv.x+time*0.0006+drift.x),clamp(in.uv.y-drift.y,0.002,0.998))).rgb;
  let heat=clamp(dot(source,vec3<f32>(0.5,0.35,0.15))*1.5,0.0,1.0);
  let granules=noise(p*120.0+flow*1.8+vec3<f32>(t));
  let color=mix(vec3<f32>(0.9,0.06,0.003),vec3<f32>(2.4,0.8,0.12),heat)*(0.68+0.65*granules)*(0.55+0.45*pow(max(facing,0.0),0.35));
  return vec4<f32>(source_aces(color),1.0);
 }
 if parameters.x<1.5 {
  let altitude=max(1.5*sqrt(max(1.0-facing*facing,0.0))-1.0,0.0);let tangent=unit(normal-facing*direction);let t=time*0.12;
  let inner=sin(dot(tangent,vec3<f32>(19.,31.,47.))+t);
  let filaments=sin(dot(tangent,vec3<f32>(83.,127.,61.))+inner*2.5);
  let streamers=pow(0.5+0.5*filaments,4.0);let length_scale=9.0+8.0*(0.5+0.5*sin(dot(tangent,vec3<f32>(7.,11.,5.))-t));
  let density=(exp(-altitude*length_scale)*(0.14+0.11*streamers)+0.4*exp(-altitude*65.0))*(1.0-smoothstep(0.28,0.5,altitude));
  // Bevy's additive mode uses premultiplied blending. Alpha zero preserves
  // the destination sky; alpha one would replace it with a black sphere.
  let color=mix(vec3<f32>(1.,0.5,0.14),vec3<f32>(1.,0.13,0.018),smoothstep(0.,0.2,altitude));return vec4<f32>(source_aces(color*density),0.0);
 }
 let flow=0.7+0.3*sin(in.uv.x*55.0-time*1.4+sin(in.uv.x*19.0+time*0.4));let f=abs(facing);
 return vec4<f32>(source_aces(mix(vec3<f32>(0.45,0.018,0.002),vec3<f32>(1.2,0.16,0.012),f)*flow),pow(f,1.5)*0.65);
}
