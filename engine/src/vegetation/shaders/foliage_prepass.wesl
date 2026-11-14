import bevy_pbr::{prepass::io,render::{mesh_functions,pbr_prepass_functions,pbr_bindings,mesh_bindings::mesh,pbr_functions,pbr_types}};
import openmars_engine::vegetation::shaders::foliage_coverage::visible;
@if(PREPASS_FRAGMENT)
@fragment fn fragment(in:io::VertexOutput,@builtin(front_facing) front:bool)->io::FragmentOutput {
    if !visible(in.position.xy,mesh_functions::get_tag(in.instance_index)){discard;}
    pbr_prepass_functions::prepass_sample_color_and_alpha_discard(in);
    var out:io::FragmentOutput;
    @if(UNCLIPPED_DEPTH_ORTHO_EMULATION) out.frag_depth=in.unclipped_depth;
    @if(NORMAL_PREPASS) {
        let double_sided=(pbr_bindings::material.flags&pbr_types::STANDARD_MATERIAL_FLAGS_DOUBLE_SIDED_BIT)!=0u;
        let logically_front=pbr_functions::winding_corrected_front_facing(mesh[in.instance_index].flags,front);
        let normal=pbr_functions::prepare_world_normal(in.world_normal,double_sided,logically_front);
        out.normal=vec4(normal*0.5+0.5,1.0);
    }
    @if(MOTION_VECTOR_PREPASS) out.motion_vector=pbr_prepass_functions::calculate_motion_vector(in.world_position,in.previous_world_position);
    return out;
}
@else
@fragment fn fragment(in:io::VertexOutput) {
    if !visible(in.position.xy,mesh_functions::get_tag(in.instance_index)){discard;}
    pbr_prepass_functions::prepass_sample_color_and_alpha_discard(in);
}
