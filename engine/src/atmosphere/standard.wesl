import openmars_engine::terrain::shaders::highlight::apply_tile_highlight;
import openmars_engine::atmosphere::common::source_pbr;
import bevy_pbr::render::{forward_io::{VertexOutput,FragmentOutput},pbr_fragment::pbr_input_from_standard_material,pbr_functions::{alpha_discard,apply_pbr_lighting}};
@fragment fn fragment(in:VertexOutput,@builtin(front_facing) front:bool)->FragmentOutput {
 var pbr=pbr_input_from_standard_material(in,front);
 pbr.material.base_color=vec4<f32>(apply_tile_highlight(pbr.material.base_color.rgb,pbr.world_position.xyz),pbr.material.base_color.a);
 pbr.material.base_color=alpha_discard(pbr.material.flags,pbr.material.alpha_cutoff,pbr.material.base_color);
 @if(SOURCE_NO_ENVIRONMENT)
 let lit=vec4<f32>(pbr.material.emissive.rgb,pbr.material.base_color.a);
 @else
 let lit=apply_pbr_lighting(pbr);
 var out:FragmentOutput;out.color=source_pbr(pbr,lit);return out;
}
