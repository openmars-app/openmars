import openmars_engine::atmosphere::common::source_pbr;
import bevy_pbr::render::{forward_io::{VertexOutput,FragmentOutput},mesh_functions,pbr_fragment::pbr_input_from_standard_material,pbr_functions};
@fragment fn fragment(in:VertexOutput,@builtin(front_facing) front:bool)->FragmentOutput {
 let tag=mesh_functions::get_tag(in.instance_index);
 if (tag&0x80000000u)!=0u {discard;}
 var pbr=pbr_input_from_standard_material(in,front);
 let tint=vec3<f32>(0.75,0.52,0.35)*bitcast<f32>(tag&0x7fffffffu);
 pbr.material.base_color=vec4<f32>(pbr.material.base_color.rgb*tint,pbr.material.base_color.a);
 pbr.material.base_color=pbr_functions::alpha_discard(pbr.material.flags,pbr.material.alpha_cutoff,pbr.material.base_color);
 var out:FragmentOutput;out.color=source_pbr(pbr,pbr_functions::apply_pbr_lighting(pbr));return out;
}
