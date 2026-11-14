import bevy_pbr::render::{mesh_functions,view_transformations::position_world_to_clip};
import openmars_engine::structures::shaders::animation::{parameters,group_at,origin,local_vector,position,normal,opacity};
@if(PREPASS_PIPELINE)
import bevy_pbr::prepass::io::{Vertex,VertexOutput,decompress_vertex};
@else
import bevy_pbr::render::forward_io::{Vertex,decompress_vertex};
@if(!PREPASS_PIPELINE)
import openmars_engine::structures::shaders::io::Output;

@if(PREPASS_PIPELINE)
@vertex fn vertex(input:Vertex)->VertexOutput {
    let v=decompress_vertex(input,input.instance_index);let g=group_at(v.uv_b.y);
    let model=mesh_functions::get_world_from_local(input.instance_index);
    var out:VertexOutput;
    out.world_position=mesh_functions::mesh_position_local_to_world(model,vec4(position(g,v.position,parameters.clock.x),1.0));
    out.position=position_world_to_clip(out.world_position.xyz);
    @if(UNCLIPPED_DEPTH_ORTHO_EMULATION) {out.unclipped_depth=out.position.z;out.position.z=min(out.position.z,1.0);}
    @if(VERTEX_UVS_A) out.uv=v.uv;
    @if(VERTEX_UVS_B) out.uv_b=v.uv_b;
    @if(NORMAL_PREPASS_OR_DEFERRED_PREPASS) out.world_normal=mesh_functions::mesh_normal_local_to_world(normal(g,v.normal,parameters.clock.x),input.instance_index);
    @if(NORMAL_PREPASS_OR_DEFERRED_PREPASS && VERTEX_TANGENTS) out.world_tangent=mesh_functions::mesh_tangent_local_to_world(model,vec4(normal(g,v.tangent.xyz,parameters.clock.x),v.tangent.w),input.instance_index);
    @if(VERTEX_COLORS) out.color=v.color;
    @if(VERTEX_OUTPUT_INSTANCE_INDEX) out.instance_index=input.instance_index;
    @if(VISIBILITY_RANGE_DITHER) out.visibility_range_dither=mesh_functions::get_visibility_range_dither_level(input.instance_index,model[3]);
    @if(MOTION_VECTOR_PREPASS) out.previous_world_position=mesh_functions::mesh_position_local_to_world(mesh_functions::get_previous_world_from_local(input.instance_index),vec4(position(g,v.position,parameters.clock.y),1.0));
    return out;
}
@else
@vertex fn vertex(input:Vertex)->Output {
    let v=decompress_vertex(input,input.instance_index);let g=group_at(v.uv_b.y);
    let model=mesh_functions::get_world_from_local(input.instance_index);var out:Output;
    out.world_position=mesh_functions::mesh_position_local_to_world(model,vec4(position(g,v.position,parameters.clock.x),1.0));
    out.position=position_world_to_clip(out.world_position.xyz);
    out.world_normal=mesh_functions::mesh_normal_local_to_world(normal(g,v.normal,parameters.clock.x),input.instance_index);
    @if(VERTEX_UVS_A) out.uv=v.uv;
    @if(VERTEX_UVS_B) out.uv_b=v.uv_b;
    @if(VERTEX_TANGENTS) out.world_tangent=mesh_functions::mesh_tangent_local_to_world(model,vec4(normal(g,v.tangent.xyz,parameters.clock.x),v.tangent.w),input.instance_index);
    @if(VERTEX_COLORS) out.color=v.color;
    @if(VERTEX_OUTPUT_INSTANCE_INDEX) out.instance_index=input.instance_index;
    @if(VISIBILITY_RANGE_DITHER) out.visibility_range_dither=mesh_functions::get_visibility_range_dither_level(input.instance_index,model[3]);
    out.local_position=local_vector(g,v.position-origin(g));out.local_normal=local_vector(g,v.normal);out.opacity=opacity(g,parameters.clock.x);return out;
}
