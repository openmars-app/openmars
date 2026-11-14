import bevy_pbr::render::{mesh_functions,view_transformations::position_world_to_clip,pbr_types::{pbr_input_new,STANDARD_MATERIAL_FLAGS_ALPHA_MODE_BLEND},pbr_functions::main_pass_post_lighting_processing};
struct Parameters {center_radius:vec4<f32>}
@group(3) @binding(0) var<uniform> u:Parameters;
struct Vertex {@builtin(instance_index) instance:u32,@location(0) position:vec3<f32>}
struct Output {@builtin(position) position:vec4<f32>,@location(0) ground:vec2<f32>,@location(1) world:vec4<f32>,@location(2) @interpolate(flat) tag:u32}
@vertex fn vertex(in:Vertex)->Output {
 var out:Output;let model=mesh_functions::get_world_from_local(in.instance);
 let center=mesh_functions::mesh_position_local_to_world(model,vec4<f32>(u.center_radius.xyz,1.0));
 let base=mesh_functions::mesh_position_local_to_world(model,vec4<f32>(in.position.xy,0.0,1.0));
 out.world=vec4<f32>(center.xyz+normalize(base.xyz-center.xyz)*(u.center_radius.w+in.position.z),1.0);
 out.tag=mesh_functions::get_tag(in.instance);out.position=position_world_to_clip(out.world.xyz);out.ground=in.position.xy;return out;
}
fn hash(p:vec2<f32>)->f32 {return fract(sin(dot(p,vec2<f32>(127.1,311.7)))*43758.5453);}
fn noise(p:vec2<f32>)->f32 {
 let cell=floor(p);var f=fract(p);f=f*f*(3.0-2.0*f);
 return mix(mix(hash(cell),hash(cell+vec2<f32>(1,0)),f.x),mix(hash(cell+vec2<f32>(0,1)),hash(cell+vec2<f32>(1)),f.x),f.y);
}
@fragment fn fragment(in:Output)->@location(0) vec4<f32> {
 let age=f32(in.tag&1023u)*(0.8/60.0)-0.8;if (in.tag&0x80000000u)!=0u||age<0.0||age>=3.6 {discard;}
 let radius=length(in.ground);let travel=clamp(age/2.8,0.0,1.0);let front=0.98*(1.0-pow(1.0-travel,1.35));
 let width=mix(0.013,0.025,travel);let ring=1.0-smoothstep(width*0.3,width,abs(radius-front));
 let dust=exp(-square((radius-front*0.94)/mix(0.045,0.085,travel)));
 let wave_opacity=(ring*0.46+dust*0.15)*smoothstep(0.0,0.04,age)*(1.0-smoothstep(0.4,1.0,travel));
 let arrival=1.0-pow(1.0-clamp(radius/0.98,0.0,1.0),1.0/1.35);let dust_age=age-arrival*2.8;let dust_life=clamp(dust_age/0.8,0.0,1.0);
 let drift=in.ground/max(radius,0.001)*dust_life*0.025;let p=in.ground-drift;
 let clumps=noise(p*25.0)*0.65+noise(p*67.0+vec2<f32>(8.3,2.1))*0.35;
 let trail_opacity=0.16*smoothstep(0.28,0.72,clumps)*smoothstep(0.0,0.09,dust_life)*(1.0-smoothstep(0.15,1.0,dust_life))*smoothstep(0.025,0.1,radius)*(1.0-smoothstep(0.55,1.0,arrival));
 let opacity=wave_opacity+trail_opacity;if opacity<0.001 {discard;}
 let wave_color=mix(vec3<f32>(0.45,0.20,0.07),vec3<f32>(1.8,1.25,0.65),ring);
 let dust_color=mix(vec3<f32>(0.22,0.065,0.025),vec3<f32>(0.48,0.19,0.075),clumps);
 var pbr=pbr_input_new();pbr.material.flags=STANDARD_MATERIAL_FLAGS_ALPHA_MODE_BLEND;pbr.world_position=in.world;pbr.frag_coord=in.position;
 return main_pass_post_lighting_processing(pbr,vec4<f32>((wave_color*wave_opacity+dust_color*trail_opacity)/opacity,min(opacity,0.95)));
}

fn square(x:f32)->f32 {return x*x;}
