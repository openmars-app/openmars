import openmars_engine::atmosphere::common::source_pbr;
import bevy_pbr::render::{forward_io::{Vertex,VertexOutput,FragmentOutput,decompress_vertex},mesh_functions,view_transformations::position_world_to_clip,pbr_fragment::pbr_input_from_standard_material,pbr_functions::{apply_pbr_lighting}};
struct Parameters {geometry:vec4<f32>,clock:vec4<f32>,destruction:vec4<f32>,sun:vec4<f32>,sun_color:vec4<f32>,ground_destruction:vec4<f32>}
@group(3) @binding(100) var<uniform> params:Parameters;
@group(3) @binding(101) var mid:texture_2d<f32>;
@group(3) @binding(102) var mid_sampler:sampler;
@group(3) @binding(103) var high:texture_2d<f32>;
@group(3) @binding(104) var high_sampler:sampler;
fn rotate(v:vec3<f32>,axis:vec3<f32>,angle:f32)->vec3<f32> {
    let c=cos(angle);return v*c+cross(axis,v)*sin(angle)+axis*dot(axis,v)*(1.0-c);
}
fn shake(model:mat4x4<f32>)->vec3<f32>{
    let elapsed=max(0.0,params.clock.x-params.geometry.w)*1000.0;
    let amplitude=0.006*(1.0-params.clock.y)*params.sun_color.w;
    return model[0].xyz*sin(elapsed*0.137+params.geometry.z)*amplitude+model[1].xyz*sin(elapsed*0.173+params.geometry.z*1.3)*amplitude;
}
@vertex fn vertex(input:Vertex)->VertexOutput {
    let v=decompress_vertex(input,input.instance_index);let model=mesh_functions::get_world_from_local(input.instance_index);
    let center=model[3].xyz-normalize(model[2].xyz)*params.geometry.x;
    let original=mesh_functions::mesh_position_local_to_world(model,vec4<f32>(v.position,1.0)).xyz+shake(model);
    var projected=center+normalize(original-center)*(params.geometry.x+0.003);
    var normal=mesh_functions::mesh_normal_local_to_world(v.normal,input.instance_index);
    if params.ground_destruction.z>0.0 {
        let axis=normalize(model[0].xyz*params.ground_destruction.x+model[1].xyz*params.ground_destruction.y);
        projected=model[3].xyz+rotate(projected-model[3].xyz,axis,params.ground_destruction.z)-normalize(model[2].xyz)*params.ground_destruction.w;
        normal=rotate(normal,axis,params.ground_destruction.z);
    }
    var out:VertexOutput;out.world_position=vec4<f32>(projected,1.0);out.position=position_world_to_clip(projected);out.world_normal=normal;
    out.uv=v.uv;out.instance_index=input.instance_index;
    @if(VISIBILITY_RANGE_DITHER) out.visibility_range_dither=mesh_functions::get_visibility_range_dither_level(input.instance_index,model[3]);
    return out;
}
@fragment fn fragment(in:VertexOutput,@builtin(front_facing) front:bool)->FragmentOutput {
    let model=mesh_functions::get_world_from_local(in.instance_index);
    // Original ground geometry rotates positions by pi/2, retaining its UVs.
    let p=(in.uv-vec2<f32>(0.5))*(0.166*1.8*2.0);let local=vec2<f32>(-p.y,p.x);
    let offset=(model[3].xyz+shake(model)).xy*37.0;
    let d=max(max(abs(local.x),abs(0.5*local.x+0.866025404*local.y)),abs(-0.5*local.x+0.866025404*local.y))-0.166*0.866025404;
    let scale=1.5/0.166;
    let n1=textureSample(mid,mid_sampler,local*scale+offset).r*2.0-1.0;
    let n2=textureSample(high,high_sampler,local*scale*2.5+offset*1.7).r*2.0-1.0;
    let alpha=pow(smoothstep(0.0,0.166*0.35,-d+0.166*0.25+(n1*0.7+n2*0.3)*(0.166*0.2)),1.1)*params.clock.w;
    if alpha<0.01 {discard;}
    var pbr=pbr_input_from_standard_material(in,front);pbr.material.base_color.a*=alpha;
    var out:FragmentOutput;out.color=source_pbr(pbr,apply_pbr_lighting(pbr));return out;
}
