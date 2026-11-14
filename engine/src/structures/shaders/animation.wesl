struct Parameters { clock:vec4<f32>, surface:vec4<f32>, bump:vec4<f32> }
@group(3) @binding(100) var<uniform> parameters:Parameters;
@group(3) @binding(101) var group_data:texture_2d<f32>;
struct Group { pivot:vec4<f32>, motion:vec4<f32>, timing:vec4<f32>, up:vec4<f32>, right:vec4<f32>, forward:vec4<f32> }
fn group_at(id:f32)->Group {
    let base=u32(round(id))*6u;
    let width=u32(parameters.surface.z);
    var data:array<vec4<f32>,6>;
    for(var i=0u;i<6u;i++){let n=base+i;data[i]=textureLoad(group_data,vec2<i32>(i32(n%width),i32(n/width)),0);}
    return Group(data[0],data[1],data[2],data[3],data[4],data[5]);
}
fn origin(g:Group)->vec3<f32>{return vec3(g.up.w,g.right.w,g.forward.w);}
fn local_vector(g:Group,value:vec3<f32>)->vec3<f32>{return vec3(dot(value,g.right.xyz),dot(value,g.forward.xyz),dot(value,g.up.xyz));}
fn body_vector(g:Group,value:vec3<f32>)->vec3<f32>{return g.right.xyz*value.x+g.forward.xyz*value.y+g.up.xyz*value.z;}
fn collapse(g:Group,seconds:f32)->f32 {
    if g.timing.y>seconds{return 0.0;}
    let t=clamp(((seconds-g.timing.y)*0.8-g.pivot.w)/0.75,0.0,1.0);return t*t;
}
fn rotation(g:Group,seconds:f32)->mat3x3<f32>{
    let tilt=g.motion.xy*collapse(g,seconds);let cx=cos(tilt.x);let sx=sin(tilt.x);let cy=cos(tilt.y);let sy=sin(tilt.y);
    return mat3x3<f32>(vec3(cy,0.0,-sy),vec3(sy*sx,cx,cy*sx),vec3(sy*cx,-sx,cy*cx));
}
fn birth(g:Group,seconds:f32)->f32 {
    // Normal motion starts at the first visible roof height, skipping buried
    // dead time. Reduced motion keeps the original 150 ms opacity crossfade.
    let lead=select(g.timing.w,0.0,parameters.clock.w>0.5);
    return clamp(lead+(seconds-g.timing.x)/select(67.0/60.0,9.0/60.0,parameters.clock.w>0.5),0.0,1.0);
}
fn position(g:Group,value:vec3<f32>,seconds:f32)->vec3<f32>{
    var next=value;
    if parameters.clock.w<0.5{next-=g.up.xyz*g.motion.w*pow(1.0-birth(g,seconds),3.0);}
    if seconds>=g.timing.y {
        if parameters.clock.w>0.5{return next;}
        next=g.pivot.xyz+body_vector(g,rotation(g,seconds)*local_vector(g,next-g.pivot.xyz));
        next-=g.up.xyz*g.motion.z*collapse(g,seconds);
    }
    return next;
}
fn normal(g:Group,value:vec3<f32>,seconds:f32)->vec3<f32>{
    if parameters.clock.w>0.5{return value;}
    return body_vector(g,rotation(g,seconds)*local_vector(g,value));
}
fn opacity(g:Group,seconds:f32)->f32 {
    var alpha=g.timing.z;
    if parameters.clock.w>0.5{
        alpha*=birth(g,seconds);
        if seconds>=g.timing.y{alpha*=1.0-clamp((seconds-g.timing.y)/(9.0/60.0),0.0,1.0);}
    }
    return alpha;
}
// Stable integer pixel hash, shared by depth/shadow/forward. No animated noise
// and no double-sided depth mismatch during reduced-motion crossfades.
fn visible(pixel:vec2<f32>,alpha:f32)->bool {
    if alpha>=1.0{return true;}if alpha<=0.0{return false;}
    let xy=vec2<u32>(max(pixel,vec2(0.0)));var n=(xy.x*0x9e3779b9u)^(xy.y*0x85ebca6bu);n^=n>>16u;n*=0x7feb352du;n^=n>>15u;
    return f32(n&0xffffu)/65535.0<alpha;
}
