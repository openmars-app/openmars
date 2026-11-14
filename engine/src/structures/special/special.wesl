import openmars_engine::atmosphere::common::source_display;
import openmars_engine::structures::special::special_shape::{params,curve,mohole_height,rotate,visible};
import openmars_engine::structures::special::volcano_color::volcano_color;
import openmars_engine::structures::special::mohole_color::mohole_color;
import openmars_engine::terrain::shaders::highlight::apply_tile_highlight;
import bevy_pbr::render::{mesh_functions,view_transformations::position_world_to_clip,pbr_types::pbr_input_new,pbr_functions::main_pass_post_lighting_processing};
struct Vertex {@builtin(instance_index) instance:u32,@location(0) position:vec3<f32>,@location(1) metadata:vec3<f32>,@location(2) uv:vec2<f32>}
struct Output {@builtin(position) position:vec4<f32>,@location(0) world_position:vec3<f32>,@location(1) world_normal:vec3<f32>,@location(2) uv:vec2<f32>,@location(3) extra:vec4<f32>,@location(4) center:vec3<f32>}
@vertex fn vertex(input:Vertex)->Output {
 let model=mesh_functions::get_world_from_local(input.instance);
 let plane=input.position+params.local_x.xyz*params.motion.x+params.local_y.xyz*params.motion.y;
 let projected_plane=mesh_functions::mesh_position_local_to_world(model,vec4<f32>(plane,1.0)).xyz;
 let direction=normalize(projected_plane);let up=normalize(mesh_functions::mesh_normal_local_to_world(direction,input.instance));
 let centered=(input.uv-vec2<f32>(0.5))*2.0;var h=input.metadata.x*params.clock.y;var dx=input.metadata.y*params.clock.y;var dy=input.metadata.z*params.clock.y;var offset=0.003;
 if params.geometry.w>0.5 {
  h=mohole_height(centered);offset=0.005;
  dx=(mohole_height(centered+vec2<f32>(0.008,0.0))-mohole_height(centered-vec2<f32>(0.008,0.0)))/0.016;
  dy=(mohole_height(centered+vec2<f32>(0.0,0.008))-mohole_height(centered-vec2<f32>(0.0,0.008)))/0.016;
 }
 @if(SPECIAL_MASK) {h=0.0;dx=0.0;dy=0.0;}
 var right=normalize(cross(vec3<f32>(0.0,1.0,0.0),up));if abs(up.y)>=0.99{right=normalize(cross(vec3<f32>(1.0,0.0,0.0),up));}
 let forward=cross(up,right);var normal=normalize(up*0.15-right*dx-forward*dy);
 var world_position=direction*(params.geometry.x+offset+h);
 let collapse=params.collapse_parts[select(0u,1u,params.geometry.w>0.5)];
 if collapse.z!=0.0||collapse.w!=0.0 {
  let axis=normalize(mesh_functions::mesh_normal_local_to_world(params.local_x.xyz*collapse.x+params.local_y.xyz*collapse.y,input.instance));
  let origin=mesh_functions::mesh_position_local_to_world(model,vec4<f32>(params.origin.xyz,1.0)).xyz;
  let radial=normalize(mesh_functions::mesh_normal_local_to_world(normalize(params.origin.xyz),input.instance));
  world_position=origin+rotate(world_position-origin,axis,collapse.z)-radial*collapse.w;normal=rotate(normal,axis,collapse.z);
 }
 var out:Output;out.world_position=world_position;out.position=position_world_to_clip(world_position);out.world_normal=normal;out.uv=input.uv;out.extra=vec4<f32>(length(centered),h,params.clock.y,params.clock.z);out.center=vec3<f32>(0.0);return out;
}
@fragment fn fragment(in:Output)->@location(0) vec4<f32> {
 @if(SPECIAL_MASK) {
  if params.motion.w>0.5&&params.motion.z>=0.0{discard;}
  if !visible(in.position.xy)||in.extra.x>=params.profile.y*params.clock.w {discard;}
  return vec4<f32>(1.0,0.0,0.0,1.0);
 }
 @if(!SPECIAL_MASK) {
  var color:vec4<f32>;
  if params.geometry.w<0.5 {color=volcano_color(in.uv,in.extra,in.world_normal,in.world_position,in.center);}
  else {color=mohole_color(in.uv,in.extra,in.world_normal,in.world_position,in.center);}
  if !visible(in.position.xy)||color.a<0.002{discard;}
  @if(SPECIAL_EDGE) {if color.a>=0.999{discard;}}
  @if(!SPECIAL_EDGE) {if color.a<0.999{discard;}}
  // Original custom ShaderMaterial returns display RGB without color-space chunks.
  // Decode once before Bevy writes the sRGB attachment, preserving authored output.
  color=vec4<f32>(source_display(color.rgb,in.world_position,in.world_normal),color.a);
  color=vec4<f32>(apply_tile_highlight(color.rgb,in.world_position),color.a);
  var pbr=pbr_input_new();pbr.frag_coord=in.position;return main_pass_post_lighting_processing(pbr,color);
 }
}
