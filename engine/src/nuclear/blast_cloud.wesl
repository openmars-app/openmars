// Original nuclear-cloud volume. WGPU uses top-left UV and reverse-Z [0,1].
// There is no OpenGL depth remap and no proxy/billboard substitute for the field.
struct Parameters {
 inverse_projection:mat4x4<f32>, camera_world:mat4x4<f32>, world_to_local:mat4x4<f32>, values:vec4<f32>,
}
@group(0) @binding(0) var<uniform> u:Parameters;
@if(WEBGL_DEPTH)
@group(0) @binding(1) var scene_depth:texture_2d<f32>;
@if(!WEBGL_DEPTH)
@group(0) @binding(1) var scene_depth:texture_depth_2d;
struct Screen {@builtin(position) position:vec4<f32>,@location(0) uv:vec2<f32>}
@vertex fn vertex(@builtin(vertex_index) i:u32)->Screen {
 var out:Screen;let uv=vec2<f32>(f32((i<<1u)&2u),f32(i&2u));
 out.position=vec4<f32>(uv*2.0-1.0,0.0,1.0);out.uv=vec2<f32>(uv.x,1.0-uv.y);return out;
}
fn hash(input:vec3<f32>)->f32 {
 var p=fract(input*0.1031);p+=vec3<f32>(dot(p,p.yzx+33.33));return fract((p.x+p.y)*p.z);
}
fn noise(p:vec3<f32>)->f32 {
 let i=floor(p);var f=fract(p);f=f*f*(3.0-2.0*f);
 return mix(mix(mix(hash(i),hash(i+vec3<f32>(1,0,0)),f.x),mix(hash(i+vec3<f32>(0,1,0)),hash(i+vec3<f32>(1,1,0)),f.x),f.y),mix(mix(hash(i+vec3<f32>(0,0,1)),hash(i+vec3<f32>(1,0,1)),f.x),mix(hash(i+vec3<f32>(0,1,1)),hash(i+vec3<f32>(1,1,1)),f.x),f.y),f.z);
}
fn turbulence(p:vec3<f32>)->f32 {return noise(p)*0.57+noise(p*2.07+4.1)*0.28+noise(p*4.13-7.3)*0.15;}
fn ellipsoid(p:vec3<f32>,center:vec3<f32>,radii:vec3<f32>)->f32 {return (length((p-center)/radii)-1.0)*min(radii.x,min(radii.y,radii.z));}
fn field(p:vec3<f32>)->vec3<f32> {
 let t=max(u.values.x,0.0);let rise=smoothstep(0.12,2.1,t);let spread=smoothstep(0.18,1.35,t);
 let height=mix(0.055,0.21,rise);let width=mix(0.045,0.15,spread);let cap_depth=mix(0.030,0.063,spread);
 let angle=atan2(p.y,p.x);let radial=length(p.xy);var flow=p;let roll=atan2(p.z-height,radial-width*0.72);
 flow=vec3<f32>(flow.xy*(1.0+0.08*sin(roll-t*1.6)),flow.z-t*0.035);
 let seed=u.values.y;let offset=vec3<f32>(seed-floor(seed/71.0)*71.0,seed-floor(seed/127.0)*127.0,seed-floor(seed/31.0)*31.0);
 let n=turbulence(flow*36.0+offset);let folds=sin(angle*7.0+t*0.5)*0.006+sin(angle*11.0-t*0.3)*0.004;
 let crown_lift=(sin(p.x*68.0+sin(p.y*43.0)-t*0.4)*0.008+cos(p.y*75.0-t*0.3)*0.006)*spread;
 var crown=ellipsoid(p,vec3<f32>(0,0,height+crown_lift),vec3<f32>(width+folds,width,cap_depth));crown=max(crown,height-0.014-p.z);
 let rim_section=vec2<f32>((radial-width*0.76)/(width*0.27),(p.z-height+0.006)/mix(0.014,0.023,spread));
 let rim=(length(rim_section)-1.0)*0.023;let cap=min(crown,rim)+(n-0.48)*0.026;
 let foot=0.013*(1.0-smoothstep(0.0,0.045,p.z));let collar=0.008*smoothstep(height-0.045,height-0.010,p.z);
 let billows=(sin(p.z*91.0-t*1.8+sin(angle*3.0))*0.008+sin(p.z*143.0-t*1.1+angle*5.0)*0.004)*rise;
 let stem_width=mix(0.025,0.035,smoothstep(0.12,0.6,t))+foot+collar+billows;
 let stem_offset=vec2<f32>(sin(p.z*39.0-t*1.3),cos(p.z*53.0-t*0.9))*0.012*rise;
 var stem=max(length(p.xy-stem_offset)-stem_width,max(-0.012-p.z,p.z-height));stem+=(n-0.48)*0.024;
 var fireball=length(p-vec3<f32>(0,0,0.045))-mix(0.022,0.075,smoothstep(0.0,0.22,t));fireball+=smoothstep(0.25,0.65,t)*0.3;
 let shape=min(min(cap,stem),fireball+(n-0.48)*0.016);var density=1.0-smoothstep(-0.010,0.012,shape);
 density*=smoothstep(-0.018,-0.004,p.z);density*=1.0-smoothstep(3.6,6.2,t);
 let breakup=smoothstep(4.0,6.2,t);density*=1.0-smoothstep(0.75-breakup*0.6,0.9-breakup*0.6,1.0-n);
 let hot_life=1.0-smoothstep(2.5,4.7,t);var heat=hot_life*smoothstep(0.38,0.76,n);
 heat*=mix(1.0,0.45,smoothstep(height-0.005,height+0.04,p.z));
 let underside=exp(-square((p.z-height+0.012)/0.020))*(1.0-smoothstep(width*0.65,width,radial));heat=max(heat,underside*0.65*hot_life);
 let incandescent=1.0-smoothstep(0.65,2.0,t);heat=mix(heat,0.94+0.06*n,incandescent);
 let ground_z=p.z+0.01+radial*radial/4.04;let ground_radius=mix(0.035,0.235,smoothstep(0.0,1.6,t));
 var bed=(length(vec3<f32>(p.xy/ground_radius,(ground_z-0.016)/0.024))-1.0)*0.024;bed+=(n-0.48)*0.014;
 var ground=(1.0-smoothstep(-0.010,0.012,bed))*smoothstep(-0.008,0.002,ground_z);ground*=1.0-smoothstep(3.0,6.2,t);
 let ground_share=ground/max(density+ground,0.001);heat*=1.0-ground_share*0.75*smoothstep(0.25,0.8,t);density=max(density,ground*0.85);
 let halo_radius=0.07+t*0.11;let halo=exp(-square((radial-halo_radius)/0.018)-square((p.z-height*0.8)/0.007));
 let halo_life=smoothstep(0.3,0.55,t)*(1.0-smoothstep(0.7,1.3,t));density=max(density,halo*halo_life*0.3);
 return vec3<f32>(density,heat,halo*halo_life);
}
fn depth_at(uv:vec2<f32>)->f32 {
 let extent=textureDimensions(scene_depth);let pixel=clamp(vec2<i32>(uv*vec2<f32>(extent)),vec2<i32>(0),vec2<i32>(extent)-1);
@if(WEBGL_DEPTH)
 return textureLoad(scene_depth,pixel,0).r;
@if(!WEBGL_DEPTH)
 return textureLoad(scene_depth,pixel,0);
}
@fragment fn fragment(in:Screen)->@location(0) vec4<f32> {
 let age=u.values.x;if age<0.0||age>=6.2 {discard;}
 let ndc=vec2<f32>(in.uv.x*2.0-1.0,1.0-in.uv.y*2.0);
 // Reverse-Z infinite perspective: near is1, not the singular far plane0.
 let view_near=u.inverse_projection*vec4<f32>(ndc,1.0,1.0);
 let ray_world=normalize((u.camera_world*vec4<f32>(view_near.xyz/view_near.w,0.0)).xyz);
 let ro=(u.world_to_local*vec4<f32>(u.camera_world[3].xyz,1.0)).xyz;
 let rd=normalize((u.world_to_local*vec4<f32>(ray_world,0.0)).xyz);
 let inv=1.0/(rd+vec3<f32>(0.0000001));let a=(vec3<f32>(-0.36,-0.36,-0.06)-ro)*inv;let b=(vec3<f32>(0.36,0.36,0.50)-ro)*inv;
 let lo=min(a,b);let hi=max(a,b);let entry=max(0.0,max(lo.x,max(lo.y,lo.z)));var leave=min(hi.x,min(hi.y,hi.z));
 let depth=depth_at(in.uv);
 if depth>0.0 {
  let scene=u.inverse_projection*vec4<f32>(ndc,depth,1.0);leave=min(leave,length(scene.xyz/scene.w));
 }
 if leave<=entry {discard;}
 let step_size=(leave-entry)/48.0;var distance=entry+step_size*0.5;
 let white_heat=1.0-smoothstep(0.65,1.75,age);var result=vec4<f32>(0.0);
 for(var i=0;i<48;i++) {
  let p=ro+rd*distance;let f=field(p);
  if f.x>0.005 {
   let alpha=1.0-exp(-f.x*step_size*60.0);let upper=smoothstep(0.0,0.42,p.z);
   let smoke=mix(vec3<f32>(0.050,0.009,0.003),vec3<f32>(0.115,0.029,0.011),upper);
   let light=clamp(0.4+(f.x-field(p+vec3<f32>(-0.016,0.012,0.024)).x)*1.7,0.18,1.0);
   var fire=mix(vec3<f32>(2.8,0.22,0.005),vec3<f32>(5.0,1.7,0.22),smoothstep(0.25,0.95,f.y));
   fire=mix(fire,vec3<f32>(8.0,6.6,4.3),white_heat*smoothstep(0.65,0.92,f.y));
   var color=smoke*light+fire*f.y*f.y;color=mix(color,vec3<f32>(0.75,0.70,0.60),clamp(f.z*2.0,0.0,1.0));
   result=vec4<f32>(result.rgb+(1.0-result.a)*color*alpha,result.a+(1.0-result.a)*alpha);
   if result.a>0.985 {break;}
  }
  distance+=step_size;
 }
 return result;
}

fn square(x:f32)->f32 {return x*x;}
