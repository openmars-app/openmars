import openmars_engine::atmosphere::common::{atmosphere,source_aces};
import bevy_pbr::render::{forward_io::{Vertex,decompress_vertex},mesh_functions,view_transformations::position_world_to_clip,mesh_view_bindings::view};
struct Output {@builtin(position) position:vec4<f32>,@location(0) world:vec3<f32>,@location(1) normal:vec3<f32>,@location(2) @interpolate(flat) body:u32}
@vertex fn vertex(input:Vertex)->Output {
 let v=decompress_vertex(input,input.instance_index);var out:Output;
 out.world=mesh_functions::mesh_position_local_to_world(mesh_functions::get_world_from_local(input.instance_index),vec4<f32>(v.position,1.0)).xyz;
 out.normal=mesh_functions::mesh_normal_local_to_world(v.normal,input.instance_index);
 out.position=position_world_to_clip(out.world);out.body=mesh_functions::get_tag(input.instance_index);return out;
}
@fragment fn fragment(in:Output)->@location(0) vec4<f32> {
 if in.body>=atmosphere.metadata.x {discard;}
 let profile=atmosphere.profiles[in.body];let normal=normalize(in.normal);let view_direction=normalize(view.world_position-in.world);
 let facing=dot(normal,view_direction);let ray_radius=(1.0+profile.x)*sqrt(max(1.0-facing*facing,0.0));
 let altitude=max((ray_radius-1.0)/profile.x,0.0);var density=0.3*exp(-4.0*altitude)*(1.0-smoothstep(0.6,1.0,altitude));
 var opacity=1.0;if profile.w>0.5 {density=0.3;opacity=1.0/16.0;}
 let tangent=normal-facing*view_direction;let n=tangent/max(length(tangent),0.0001);
 let sun=normalize(atmosphere.sun.xyz-atmosphere.bodies[in.body].xyz);let key=normalize(sun*0.45+vec3<f32>(0.0,0.9,0.0));
 let warmth=smoothstep(-0.35,0.65,dot(n,key));let daylight=0.28+0.72*smoothstep(-0.25,0.65,dot(n,sun));
 let scattering=mix(atmosphere.shadows[in.body].rgb*0.9,atmosphere.colors[in.body].rgb*atmosphere.sun_color.rgb*1.5,warmth);
 // Bevy Add shares the premultiplied blend state: preserve destination alpha.
 return vec4<f32>(source_aces(scattering*density*daylight*profile.y*profile.z)*opacity,0.0);
}
