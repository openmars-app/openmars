import openmars_engine::atmosphere::common::source_pbr;
import bevy_pbr::render::{forward_io::FragmentOutput,pbr_fragment::pbr_input_from_standard_material,pbr_bindings,pbr_functions::{alpha_discard,apply_pbr_lighting}};
import openmars_engine::structures::shaders::{io::{Output,standard},animation::{parameters,visible}};
@group(3) @binding(102) var noise_texture:texture_2d<f32>;
@group(3) @binding(103) var noise_sampler:sampler;
@fragment fn fragment(in:Output,@builtin(front_facing) front:bool)->FragmentOutput {
    if !visible(in.position.xy,in.opacity){discard;}
    var pbr=pbr_input_from_standard_material(standard(in),front);
    // Original concrete/sand bump maps use the same source red channel as
    // their base map. Preserve Mikkelsen screen-space forward differences.
    @if(VERTEX_UVS_A) {
        if parameters.bump.x>0.0 {
            let uv=(pbr_bindings::material.uv_transform*vec3(in.uv,1.0)).xy;
            let du=dpdx(uv);let dv=dpdy(uv);
            let h=textureSample(pbr_bindings::base_color_texture,pbr_bindings::base_color_sampler,uv).r;
            let dh=parameters.bump.x*vec2(
                textureSample(pbr_bindings::base_color_texture,pbr_bindings::base_color_sampler,uv+du).r-h,
                textureSample(pbr_bindings::base_color_texture,pbr_bindings::base_color_sampler,uv+dv).r-h);
            let sx=normalize(dpdx(in.world_position.xyz));let sy=normalize(dpdy(in.world_position.xyz));
            let r1=cross(sy,pbr.N);let r2=cross(pbr.N,sx);
            let det=dot(sx,r1)*select(-1.0,1.0,front);
            pbr.N=normalize(abs(det)*pbr.N-sign(det)*(dh.x*r1+dh.y*r2));
        }
    }
    if parameters.surface.x>0.5&&parameters.clock.z>0.001 {
        let n=normalize(in.local_normal);var weights=pow(abs(n),vec3(4.0));weights/=max(dot(weights,vec3(1.0)),0.0001);
        let uv=in.local_position*45.0;
        let grain=textureSample(noise_texture,noise_sampler,uv.yz).r*weights.x+textureSample(noise_texture,noise_sampler,uv.xz).r*weights.y+textureSample(noise_texture,noise_sampler,uv.xy).r*weights.z;
        let crystals=textureSample(noise_texture,noise_sampler,(in.local_position.xy+in.local_position.z*vec2(0.63,0.81))*220.0).r;
        let up=smoothstep(-0.1,0.8,n.z);let rim=1.0-smoothstep(0.02,0.045,in.local_position.z);
        let exposure=mix(0.3+0.7*up,0.6+0.25*up+0.15*rim,parameters.surface.y);
        let frost=parameters.clock.z*in.uv_b.x*exposure*(0.2+0.8*smoothstep(0.35,0.65,grain))*0.6;
        let color=mix(vec3(0.55,0.66,0.73),vec3(0.8,0.86,0.9),smoothstep(0.4,0.62,crystals));
        pbr.material.base_color=vec4(mix(pbr.material.base_color.rgb,color,frost),mix(pbr.material.base_color.a,0.65,frost*parameters.surface.y));
        pbr.material.perceptual_roughness=mix(pbr.material.perceptual_roughness,0.9,frost);
        pbr.material.metallic*=1.0-frost*0.85;
        pbr.material.emissive=vec4(pbr.material.emissive.rgb*(1.0-frost*0.35),pbr.material.emissive.a);
    }
    pbr.material.base_color=alpha_discard(pbr.material.flags,pbr.material.alpha_cutoff,pbr.material.base_color);
    var out:FragmentOutput;out.color=source_pbr(pbr,apply_pbr_lighting(pbr));return out;
}
