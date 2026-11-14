@if(!TERRAIN_BAKE_DIRECT)
import bevy_pbr::render::{forward_io::{Vertex,decompress_vertex},mesh_functions,view_transformations::position_world_to_clip};
struct TerrainUniform {
    geometry:vec4<f32>,climate:vec4<f32>,water:vec4<f32>,water_extra:vec4<f32>,
    camera:vec4<f32>,sun_direction:vec4<f32>,sun_color:vec4<f32>,
    shade_bounds:vec4<f32>,bake_bounds:vec4<f32>,bake:vec4<f32>,
}
@group(3) @binding(0) var<uniform> params:TerrainUniform;
@group(3) @binding(1) var terrain:texture_2d_array<f32>;
@group(3) @binding(2) var field_sampler:sampler;
struct TerrainVertex {
    @builtin(position) position:vec4<f32>,
    @location(0) world_position:vec4<f32>,@location(1) world_normal:vec3<f32>,
    @location(2) local:vec2<f32>,@location(3) board:vec2<f32>,
    @location(4) @interpolate(flat) layer_birth:vec3<f32>,
}
fn field_uv(local:vec2<f32>)->vec2<f32> {return (local*127.0+2.5)/132.0;}
fn elapsed(birth:f32)->f32 {
    if params.bake.y<0.0 {return 1.0;}
    if birth<=-2.5 {return 0.0;}
    if birth<=-1.5 {return 1.0;}
    let start=select(birth,params.bake.y,birth<0.0);
    return clamp((params.geometry.w-start)/0.6,0.0,1.0);
}
fn weight(previous:f32,next:f32,t:f32)->f32 {
    if next>=previous {return t*t*(3.0-2.0*t);}
    let radius=0.17*0.6283185307179586;let front=-radius+t*(0.2+radius);let settle=max(next,-radius);
    return max(smoothstep(settle,settle+0.1,front),smoothstep(0.85,1.0,t));
}
fn field_layers(lb:vec3<f32>)->vec3<i32> {
    let bits=u32(lb.x);
    return vec3<i32>(i32(bits&255u),i32((bits>>8u)&255u),i32((bits>>16u)&255u));
}
fn field(local:vec2<f32>,lb:vec3<f32>)->vec4<f32> {
    let uv=field_uv(local);let layers=field_layers(lb);let a=elapsed(lb.y);let b=elapsed(lb.z);
    if b>=1.0 {return textureSampleLevel(terrain,field_sampler,uv,layers.z,0.0);}
    var middle=textureSampleLevel(terrain,field_sampler,uv,layers.y,0.0);
    if a<1.0 {let previous=textureSampleLevel(terrain,field_sampler,uv,layers.x,0.0);middle=mix(previous,middle,weight(previous.y,middle.y,a));}
    if b>0.0 {let next=textureSampleLevel(terrain,field_sampler,uv,layers.z,0.0);middle=mix(middle,next,weight(middle.y,next.y,b));}
    return middle;
}
fn field_weights(local:vec2<f32>,lb:vec3<f32>)->vec2<f32> {
    let a=elapsed(lb.y);let b=elapsed(lb.z);
    if b>=1.0 {return vec2<f32>(1.0,1.0);}
    let uv=field_uv(local);let layers=field_layers(lb);
    var middle=textureSampleLevel(terrain,field_sampler,uv,layers.y,0.0).y;var first=1.0;
    if a<1.0 {let previous=textureSampleLevel(terrain,field_sampler,uv,layers.x,0.0).y;first=weight(previous,middle,a);middle=mix(previous,middle,first);}
    var second=0.0;
    if b>0.0 {let next=textureSampleLevel(terrain,field_sampler,uv,layers.z,0.0).y;second=weight(middle,next,b);}
    return vec2<f32>(first,second);
}
fn basin_shore(local:vec2<f32>,lb:vec3<f32>)->f32 {
    let uv=field_uv(local);let layers=field_layers(lb);let a=elapsed(lb.y);let b=elapsed(lb.z);
    if b>=1.0 {return textureSampleLevel(terrain,field_sampler,uv,layers.z,0.0).y;}
    var middle=textureSampleLevel(terrain,field_sampler,uv,layers.y,0.0).y;var basin=middle;
    if a<1.0 {
        let previous=textureSampleLevel(terrain,field_sampler,uv,layers.x,0.0).y;let w=weight(previous,middle,a);
        basin=select(min(previous,middle),previous,a<=0.0||(middle<previous&&w<0.02));middle=mix(previous,middle,w);
    }
    if b<=0.0 {return basin;}
    let next=textureSampleLevel(terrain,field_sampler,uv,layers.z,0.0).y;
    if next<middle && weight(middle,next,b)<0.02 {return basin;}
    return min(basin,next);
}
fn project(p:vec2<f32>,height:f32)->vec3<f32> {
    let d=length(p);let radius=params.geometry.x;
    if d<0.000001 {return vec3<f32>(0.0,0.0,radius+height);}
    // GPU trigonometric approximations need not yield an exactly unit vector.
    // Preserve the requested radius: a small inward error can hide a0.0003
    // flattened city pad behind the independently prepared CPU relief globe.
    let direction=vec3<f32>(p*sin(d/radius)/d,cos(d/radius));
    return normalize(direction)*(radius+height);
}
@if(!TERRAIN_BAKE_DIRECT)
fn terrain_vertex(input:Vertex,water:bool)->TerrainVertex {
    let vertex=decompress_vertex(input,input.instance_index);var out:TerrainVertex;
    out.local=vertex.uv;out.board=vertex.color.xy+out.local*params.geometry.y;out.layer_birth=vec3<f32>(vertex.color.zw,-3.0);
    @if(VERTEX_UVS_B) {out.layer_birth.z=vertex.uv_b.x;}
    let h=select(field(out.local,out.layer_birth).x,-0.008,water);let point=project(out.board,h);
    let e=1.0/127.0;let step_size=params.geometry.y*e;
    var normal=normalize(project(out.board,0.0));
    if !water {
        let x0=project(out.board-vec2<f32>(step_size,0.0),field(out.local-vec2<f32>(e,0.0),out.layer_birth).x);
        let x1=project(out.board+vec2<f32>(step_size,0.0),field(out.local+vec2<f32>(e,0.0),out.layer_birth).x);
        let y0=project(out.board-vec2<f32>(0.0,step_size),field(out.local-vec2<f32>(0.0,e),out.layer_birth).x);
        let y1=project(out.board+vec2<f32>(0.0,step_size),field(out.local+vec2<f32>(0.0,e),out.layer_birth).x);
        normal=normalize(cross(x1-x0,y1-y0));
    }
    out.world_position=mesh_functions::mesh_position_local_to_world(mesh_functions::get_world_from_local(input.instance_index),vec4<f32>(point,1.0));
    out.world_normal=mesh_functions::mesh_normal_local_to_world(normal,input.instance_index);
    out.position=position_world_to_clip(out.world_position.xyz);return out;
}
