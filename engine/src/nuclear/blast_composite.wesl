struct Parameters {inverse_projection:mat4x4<f32>,camera_world:mat4x4<f32>,world_to_local:mat4x4<f32>,values:vec4<f32>}
@group(0) @binding(0) var<uniform> u:Parameters;
@if(WEBGL_DEPTH)
@group(0) @binding(1) var scene_depth:texture_2d<f32>;
@if(!WEBGL_DEPTH)
@group(0) @binding(1) var scene_depth:texture_depth_2d;
@group(0) @binding(2) var cloud_texture:texture_2d<f32>;
@group(0) @binding(3) var linear_sampler:sampler;
@group(0) @binding(4) var scene_texture:texture_2d<f32>;
struct Screen {@builtin(position) position:vec4<f32>,@location(0) uv:vec2<f32>}
@vertex fn vertex(@builtin(vertex_index) i:u32)->Screen {
 var out:Screen;let uv=vec2<f32>(f32((i<<1u)&2u),f32(i&2u));out.position=vec4<f32>(uv*2.0-1.0,0.0,1.0);out.uv=vec2<f32>(uv.x,1.0-uv.y);return out;
}
fn distance_at(uv:vec2<f32>)->f32 {
 let extent=textureDimensions(scene_depth);let pixel=clamp(vec2<i32>(uv*vec2<f32>(extent)),vec2<i32>(0),vec2<i32>(extent)-1);
@if(WEBGL_DEPTH)
 let depth=textureLoad(scene_depth,pixel,0).r;
@if(!WEBGL_DEPTH)
 let depth=textureLoad(scene_depth,pixel,0);
 // Finite sentinel makes sky/sky weight1 rather than exp(NaN).
 if depth<=0.0 {return 1000000.0;}
 let p=u.inverse_projection*vec4<f32>(uv.x*2.0-1.0,1.0-uv.y*2.0,depth,1.0);return length(p.xyz/p.w);
}
fn aces(color:vec3<f32>)->vec3<f32> {
 // Three's original ACES fitted matrices, exposure1 and1/0.6 input scale.
 let input=mat3x3<f32>(vec3<f32>(0.59719,0.07600,0.02840),vec3<f32>(0.35458,0.90834,0.13383),vec3<f32>(0.04823,0.01566,0.83777));
 let output=mat3x3<f32>(vec3<f32>(1.60475,-0.10208,-0.00327),vec3<f32>(-0.53108,1.10813,-0.07276),vec3<f32>(-0.07367,-0.00605,1.07602));
 let c=input*(color/0.6);let a=c*(c+0.0245786)-0.000090537;let b=c*(0.983729*c+0.4329510)+0.238081;
 return clamp(output*(a/b),vec3<f32>(0.0),vec3<f32>(1.0));
}
@fragment fn fragment(in:Screen)->@location(0) vec4<f32> {
 let center=distance_at(in.uv);var cloud=vec4<f32>(0.0);var total=0.0;
 for(var x=0;x<2;x++) {for(var y=0;y<2;y++) {
  let uv=in.uv+(vec2<f32>(f32(x),f32(y))-0.5)/u.values.zw;
  let weight=exp(-abs(distance_at(uv)-center)*100.0)+0.0001;
  cloud+=textureSampleLevel(cloud_texture,linear_sampler,uv,0.0)*weight;total+=weight;
 }}
 cloud/=total;let scene=textureSampleLevel(scene_texture,linear_sampler,in.uv,0.0);
 if cloud.a<0.001 {return scene;}
 // Attachment sRGB encoding happens exactly once after this linear blend.
 let alpha=clamp(cloud.a,0.0,1.0);let color=aces(cloud.rgb/max(cloud.a,0.00001));
 return vec4<f32>(color*alpha+scene.rgb*(1.0-alpha),alpha+scene.a*(1.0-alpha));
}
