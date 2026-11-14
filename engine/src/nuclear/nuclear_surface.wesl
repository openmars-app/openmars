import openmars_engine::atmosphere::common::source_pbr;
import openmars_engine::nuclear::nuclear_io::{Vertex,Output,vertex as project,params,visible};
import openmars_engine::terrain::shaders::highlight::apply_tile_highlight;
import bevy_pbr::render::{pbr_types::{pbr_input_new,STANDARD_MATERIAL_FLAGS_ALPHA_MODE_BLEND},pbr_functions::{apply_pbr_lighting,calculate_view,winding_corrected_front_facing},mesh_bindings::mesh,mesh_view_bindings::view};
@group(3) @binding(1) var mars_texture:texture_2d<f32>;
@group(3) @binding(2) var mars_sampler:sampler;
@vertex fn vertex(input:Vertex)->Output{return project(input);}
fn hash(p:vec2<f32>)->f32{return fract(sin(dot(p,vec2<f32>(127.1,311.7)))*43758.5453);}
fn noise(p:vec2<f32>)->f32 {
 let i=floor(p);var f=fract(p);f=f*f*(3.0-2.0*f);
 return mix(mix(hash(i),hash(i+vec2<f32>(1.0,0.0)),f.x),mix(hash(i+vec2<f32>(0.0,1.0)),hash(i+vec2<f32>(1.0,1.0)),f.x),f.y);
}
@fragment fn fragment(in:Output,@builtin(front_facing) front:bool)->@location(0) vec4<f32>{
 let radial=normalize(in.direction);let mars_uv=vec2<f32>(atan2(radial.z,-radial.x)/6.28318530718,1.0-acos(clamp(radial.y,-1.0,1.0))/3.14159265359);
 let sampled=textureSampleGrad(mars_texture,mars_sampler,mars_uv,dpdx(mars_uv),dpdy(mars_uv)).rgb;
 let mars=mix(sampled,sampled*vec3<f32>(0.9,0.96,1.08),params.surface.y*0.6);
 let r=in.uv.y;let p=in.local/0.1;let seed=vec2<f32>((params.surface.x%997.0)*0.037,params.surface.x%113.0);
 let warped=p+vec2<f32>(noise(p*3.1+seed),noise(p*3.7-seed))*0.12;
 let soil=noise(warped*4.0+seed)*0.6+noise(warped*10.0-seed)*0.4;
 let grain_visibility=1.0-smoothstep(0.35,1.0,length(fwidth(p*48.0)));
 let grain=mix(0.5,noise(p*24.0+seed)*0.65+noise(p*48.0-seed)*0.35,grain_visibility);
 let center=1.0-smoothstep(0.28,0.78,r+(soil-0.5)*0.08);
 let soot=smoothstep(0.36,0.72,noise(warped*3.5-seed))*(1.0-smoothstep(0.6,1.1,r));
 var rock=mix(vec3<f32>(0.105,0.037,0.018),vec3<f32>(0.22,0.082,0.037),soil)*mix(0.96,1.04,grain);
 rock=mix(rock,vec3<f32>(0.055,0.023,0.014),soot*0.35*(1.0-center*0.65));
 var grooves=0.0;
 for(var i=0u;i<5u;i++){
  let a=params.gouges[i].xy/0.1;let b=params.gouges[i].zw/0.1;let ab=b-a;
  let t=clamp(dot(warped-a,ab)/dot(ab,ab),0.0,1.0);let distance=length(warped-a-ab*t);let width=0.025+0.014*f32(i%3u);
  let broken=smoothstep(0.32,0.65,noise(warped*17.0+seed+f32(i)));
  grooves=max(grooves,(1.0-smoothstep(width*0.2,width,distance))*broken*smoothstep(0.0,0.2,t)*(1.0-smoothstep(0.72,1.0,t)));
 }
 rock*=1.0-grooves*0.22;let floor_color=vec3<f32>(0.065,0.025,0.013)*mix(0.97,1.03,grain);rock=mix(rock,floor_color,center*0.88);
 rock=mix(rock,mars*mix(0.92,1.0,soil),smoothstep(0.65,1.12,r));var alpha=1.0;
 @if(NUCLEAR_SCORCH){
  let t=in.coverage_apron.y;let soil_patch=noise(warped*6.0+seed)*0.65+noise(warped*19.0-seed)*0.35;rock=mix(rock,mars,smoothstep(0.0,0.65,t));
  let fade=t+(soil_patch-0.5)*0.48*smoothstep(0.0,0.25,t);
  alpha=(1.0-smoothstep(0.0,1.0,fade))*mix(1.0,0.20+soil_patch*0.38,smoothstep(0.0,0.7,t))*(1.0-smoothstep(0.85,1.0,t))*in.coverage_apron.x;
 }
 var flat_normal=normalize(cross(dpdx(in.world_position.xyz),dpdy(in.world_position.xyz)));
 if dot(flat_normal,in.world_normal)<0.0{flat_normal=-flat_normal;}if !winding_corrected_front_facing(mesh[in.instance].flags,front){flat_normal=-flat_normal;}
 if !visible(in.position.xy)||alpha<0.002{discard;}
 let ao=mix(0.74,1.0,smoothstep(0.28,1.0,r));var pbr=pbr_input_new();pbr.world_position=in.world_position;pbr.world_normal=flat_normal;pbr.N=flat_normal;
 pbr.flags=mesh[in.instance].flags;pbr.is_orthographic=view.clip_from_view[3].w==1.0;pbr.V=calculate_view(in.world_position,pbr.is_orthographic);pbr.frag_coord=in.position;
 pbr.material.base_color=vec4<f32>(apply_tile_highlight(rock*ao,in.world_position.xyz),alpha);pbr.material.perceptual_roughness=0.8;
 @if(NUCLEAR_SCORCH){pbr.material.flags=STANDARD_MATERIAL_FLAGS_ALPHA_MODE_BLEND;}
 return source_pbr(pbr,apply_pbr_lighting(pbr));
}
