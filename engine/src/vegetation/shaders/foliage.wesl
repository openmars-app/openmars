import openmars_engine::atmosphere::common::source_lambert;
import bevy_pbr::render::{forward_io::{VertexOutput,FragmentOutput},mesh_functions,pbr_fragment::pbr_input_from_standard_material,pbr_functions::{alpha_discard}};
import openmars_engine::vegetation::shaders::foliage_coverage::visible;
struct Parameters{frost:vec4<f32>}
@group(3) @binding(100) var<uniform> parameters:Parameters;
@fragment fn fragment(in:VertexOutput,@builtin(front_facing) front:bool)->FragmentOutput {
    // The equal-depth shading pass only visits cutouts already admitted by
    // the matching prepass. Keep it discard-free for early depth rejection.
    @if(!DEPTH_PREPASS) { if !visible(in.position.xy,mesh_functions::get_tag(in.instance_index)){discard;} }
    var pbr=pbr_input_from_standard_material(in,front);
    // Standard inverse-transpose normals are transformed back with M^T; this
    // recovers the actual source normal even under nonuniform growth scaling.
    let m=mesh_functions::get_world_from_local(in.instance_index);
    let local_normal=normalize(transpose(mat3x3(m[0].xyz,m[1].xyz,m[2].xyz))*in.world_normal);
    let frost=parameters.frost.x*smoothstep(0.1,0.7,local_normal.z)*0.85;
    let tint=0.65+f32(mesh_functions::get_tag(in.instance_index)&255u)/400.0;
    pbr.material.base_color=vec4(mix(pbr.material.base_color.rgb,vec3(0.86,0.9,0.95),frost)*tint,pbr.material.base_color.a);
    pbr.material.emissive=vec4(pbr.material.emissive.rgb*(1.0-parameters.frost.x*0.6),pbr.material.emissive.a);
    @if(!DEPTH_PREPASS) { pbr.material.base_color=alpha_discard(pbr.material.flags,pbr.material.alpha_cutoff,pbr.material.base_color); }
    @else { pbr.material.base_color.a=1.0; }
    var out:FragmentOutput;out.color=source_lambert(pbr);return out;
}
