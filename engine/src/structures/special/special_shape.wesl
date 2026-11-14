// Common source math for original special surfaces, with a defined reversed curve.
struct Parameters {geometry:vec4<f32>,seeds:array<vec4<f32>,4>,profile:vec4<f32>,clock:vec4<f32>,climate:vec4<f32>,sun:vec4<f32>,sun_color:vec4<f32>,local_x:vec4<f32>,local_y:vec4<f32>,origin:vec4<f32>,motion:vec4<f32>,collapse_parts:array<vec4<f32>,5>}
@group(3) @binding(0) var<uniform> params:Parameters;
fn curve(a:f32,b:f32,x:f32)->f32 {let t=clamp((x-a)/(b-a),0.0,1.0);return t*t*(3.0-2.0*t);}
fn hash(n:f32)->f32{return fract(sin(n)*43758.5453123);}
fn seedParam(i:f32)->f32{let n=u32(i);return params.seeds[n/4u][n%4u];}
fn mod289_v3(x:vec3<f32>)->vec3<f32> { return x - floor(x * (1.0 / 289.0)) * 289.0; }
fn mod289_v2(x:vec2<f32>)->vec2<f32> { return x - floor(x * (1.0 / 289.0)) * 289.0; }
fn permute_v(x:vec3<f32>)->vec3<f32> { return mod289_v3(((x * 34.0) + vec3<f32>(1.0)) * x); }

fn snoise(v:vec2<f32>)->f32 {
  let C= vec4<f32>(0.211324865405187, 0.366025403784439,
                      -0.577350269189626, 0.024390243902439);
  var i= floor(v + vec2<f32>(dot(v, C.yy)));
  var x0= v - i + vec2<f32>(dot(i, C.xx));
  var i1= select(vec2<f32>(0.0,1.0),vec2<f32>(1.0,0.0),x0.x>x0.y);
  var x12= x0.xyxy + C.xxzz;
  x12 = vec4<f32>(x12.xy - i1, x12.zw);
  i = mod289_v2(i);
  var p= permute_v(permute_v(vec3<f32>(i.y) + vec3<f32>(0.0, i1.y, 1.0)) + vec3<f32>(i.x) + vec3<f32>(0.0, i1.x, 1.0));
  var m= max(vec3<f32>(0.5) - vec3<f32>(dot(x0, x0), dot(x12.xy, x12.xy), dot(x12.zw, x12.zw)), vec3<f32>(0.0));
  m = m * m;
  m = m * m;
  var x_= 2.0 * fract(p * C.www) - vec3<f32>(1.0);
  var h= abs(x_) - vec3<f32>(0.5);
  var ox= floor(x_ + vec3<f32>(0.5));
  var a0= x_ - ox;
  m *= vec3<f32>(1.79284291400159) - 0.85373472095314 * (a0 * a0 + h * h);
  var g:vec3<f32>;
  g.x = a0.x * x0.x + h.x * x0.y;
  g = vec3<f32>(g.x, a0.yz * x12.xz + h.yz * x12.yw);
  return 130.0 * dot(m, g);
}

fn mohole_height(centered:vec2<f32>)->f32 {
 let dist=length(centered);let radius=params.profile.y*params.clock.w;
 let inside=curve(radius+0.02,radius,dist);var h=-inside*params.profile.x;
 if dist<radius {
  let u=atan2(centered.y,centered.x)/6.283+0.5;
  let v=clamp(inside*params.profile.x/(params.profile.x+0.0001),0.0,1.0);
  let c=fract(u*12.0);let r=fract(v*3.0);
  let horizontal=1.0-curve(0.0,0.05,c)*curve(0.0,0.05,1.0-c);
  let vertical=1.0-curve(0.0,0.05,r)*curve(0.0,0.05,1.0-r);
  h+=max(horizontal,vertical)*0.003*inside;
 }
 return h*params.clock.y;
}
fn rotate(v:vec3<f32>,axis:vec3<f32>,angle:f32)->vec3<f32>{let c=cos(angle);return v*c+cross(axis,v)*sin(angle)+axis*dot(axis,v)*(1.0-c);}
fn visible(pixel:vec2<f32>)->bool {
 if params.clock.z>=1.0{return true;}if params.clock.z<=0.0{return false;}
 let bayer=array<u32,16>(0u,8u,2u,10u,12u,4u,14u,6u,3u,11u,1u,9u,15u,7u,13u,5u);
 let p=vec2<u32>(floor(pixel))&vec2<u32>(3u);return params.clock.z>(f32(bayer[p.y*4u+p.x])+0.5)/16.0;
}

@group(3) @binding(1) var color_atlas:texture_2d_array<f32>;
@group(3) @binding(2) var atlas_sampler:sampler;
@group(3) @binding(3) var flow_texture:texture_2d<f32>;
fn atlas_color(uv:vec2<f32>,layer:i32)->vec4<f32>{let p=vec2<f32>(uv.x,-uv.y);return textureSampleGrad(color_atlas,atlas_sampler,p,layer,dpdx(p),dpdy(p));}
fn flow_load(p:vec2<i32>,d:vec2<i32>)->f32{return textureLoad(flow_texture,clamp(p,vec2<i32>(0),d-vec2<i32>(1)),0).r;}
// Exact R32F values with source linear interpolation; no optional float32
// filtering feature is required by the WebGL/low graphics path.
fn flow_value(uv:vec2<f32>)->f32 {
 let d=vec2<i32>(textureDimensions(flow_texture));let p=uv*vec2<f32>(d)-vec2<f32>(0.5);let a=vec2<i32>(floor(p));let f=fract(p);
 return mix(mix(flow_load(a,d),flow_load(a+vec2<i32>(1,0),d),f.x),mix(flow_load(a+vec2<i32>(0,1),d),flow_load(a+vec2<i32>(1,1),d),f.x),f.y);
}
