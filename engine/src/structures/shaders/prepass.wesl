import bevy_pbr::{prepass::io,render::{pbr_prepass_functions,pbr_bindings,mesh_bindings::mesh,mesh_view_bindings::view,pbr_functions,pbr_types}};
import openmars_engine::structures::shaders::animation::{parameters,group_at,opacity,visible};
@if(PREPASS_FRAGMENT)
@fragment fn fragment(in:io::VertexOutput,@builtin(front_facing) front:bool)->io::FragmentOutput {
    if !visible(in.position.xy,opacity(group_at(in.uv_b.y),parameters.clock.x)){discard;}
    pbr_prepass_functions::prepass_sample_color_and_alpha_discard(in);
    var out:io::FragmentOutput;
    @if(UNCLIPPED_DEPTH_ORTHO_EMULATION) out.frag_depth=in.unclipped_depth;
    @if(NORMAL_PREPASS) {
        let double_sided=(pbr_bindings::material.flags&pbr_types::STANDARD_MATERIAL_FLAGS_DOUBLE_SIDED_BIT)!=0u;
        let logically_front=pbr_functions::winding_corrected_front_facing(mesh[in.instance_index].flags,front);
        var normal=pbr_functions::prepare_world_normal(in.world_normal,double_sided,logically_front);
        @if(VERTEX_UVS && VERTEX_TANGENTS && STANDARD_MATERIAL_NORMAL_MAP) {
            @if(STANDARD_MATERIAL_NORMAL_MAP_UV_B)
            let uv=(pbr_bindings::material.uv_transform*vec3(in.uv_b,1.0)).xy;
            @else
            let uv=(pbr_bindings::material.uv_transform*vec3(in.uv,1.0)).xy;
            let sample=textureSampleBias(pbr_bindings::normal_map_texture,pbr_bindings::normal_map_sampler,uv,view.mip_bias).rgb;
            normal=pbr_functions::apply_normal_mapping(pbr_bindings::material.flags,pbr_functions::calculate_tbn_mikktspace(normal,in.world_tangent),double_sided,logically_front,sample);
        }
        out.normal=vec4(normal*0.5+0.5,1.0);
    }
    @if(MOTION_VECTOR_PREPASS) out.motion_vector=pbr_prepass_functions::calculate_motion_vector(in.world_position,in.previous_world_position);
    return out;
}
@else
@fragment fn fragment(in:io::VertexOutput) {
    if !visible(in.position.xy,opacity(group_at(in.uv_b.y),parameters.clock.x)){discard;}
    pbr_prepass_functions::prepass_sample_color_and_alpha_discard(in);
}
