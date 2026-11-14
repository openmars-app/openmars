import bevy_pbr::render::{mesh_functions,view_transformations::{position_world_to_view,position_view_to_clip}};
struct Palette {colors:array<vec4<f32>,16>}
@group(3) @binding(0) var<uniform> palette:Palette;
@group(3) @binding(1) var smoke:texture_2d<f32>;
@group(3) @binding(2) var smoke_sampler:sampler;
struct Vertex {@builtin(instance_index) instance:u32,@location(0) position:vec3<f32>,@location(1) uv:vec2<f32>}
struct Output {@builtin(position) position:vec4<f32>,@location(0) uv:vec2<f32>,@location(1) @interpolate(flat) tag:u32}
@vertex fn vertex(in:Vertex)->Output {
 var out:Output;let model=mesh_functions::get_world_from_local(in.instance);let tag=mesh_functions::get_tag(in.instance);
 let center=mesh_functions::mesh_position_local_to_world(model,vec4<f32>(0.0,0.0,0.0,1.0));
 let scale=length(model[0]);let rot=f32((tag>>12u)&16383u)/16383.0*6.28318530718;
 let cr=cos(rot);let sr=sin(rot);let p=in.position.xy;
 let offset=vec2<f32>(p.x*cr-p.y*sr,p.x*sr+p.y*cr)*scale;
 let view=position_world_to_view(center.xyz);out.position=position_view_to_clip(view+vec3<f32>(offset,0.0));out.uv=in.uv;out.tag=tag;return out;
}
@fragment fn fragment(in:Output)->@location(0) vec4<f32> {
 if (in.tag&0x80000000u)!=0u {discard;}
 let tex=textureSample(smoke,smoke_sampler,in.uv);let opacity=f32(in.tag&4095u)/4095.0;
 let kind=(in.tag>>26u)&15u;
 var color=tex.rgb*palette.colors[kind].rgb;
 // MoholeSmoke is the original display-RGB custom shader; other pools
 // use SpriteMaterial's linear-light output conversion.
 if kind==3u {let c=max(color,vec3<f32>(0.0));color=select(c/12.92,pow((c+vec3<f32>(0.055))/1.055,vec3<f32>(2.4)),c>vec3<f32>(0.04045));}
 return vec4<f32>(color,tex.a*opacity);
}
