import openmars_engine::atmosphere::common::{source_pbr,source_linear};
import bevy_pbr::render::{forward_io::{Vertex,decompress_vertex},mesh_functions,view_transformations::position_world_to_clip};
import bevy_pbr::render::{mesh_view_bindings::view,pbr_types::{pbr_input_new,STANDARD_MATERIAL_FLAGS_ALPHA_MODE_BLEND},pbr_functions::{apply_pbr_lighting}};
import openmars_engine::terrain::shaders::highlight::{highlights,tile_texel,apply_tile_highlight};
@group(3) @binding(18) var noise_texture:texture_2d_array<f32>;
@group(3) @binding(19) var noise_sampler:sampler;
struct Output {
    @builtin(position) position:vec4<f32>,@location(0) world_position:vec3<f32>,
    @location(1) uv:vec2<f32>,@location(2) @interpolate(flat) cell:vec2<f32>,
    @location(3) color:vec4<f32>,@location(4) world_uv:vec2<f32>,
    @location(5) world_normal:vec3<f32>,@location(6) border_coverage:f32,
}
@vertex fn vertex(input:Vertex)->Output {
    let v=decompress_vertex(input,input.instance_index);var out:Output;
    let cell=v.uv_b-highlights.grid.xy;let scale=tile_texel(vec2<i32>(cell),3).w;
    let center=tile_texel(vec2<i32>(cell),2).xyz*length(v.position);
    let position=normalize(center+(v.position-center)*scale)*length(v.position);
    out.world_position=mesh_functions::mesh_position_local_to_world(mesh_functions::get_world_from_local(input.instance_index),vec4<f32>(position,1.0)).xyz;
    out.position=position_world_to_clip(out.world_position);out.uv=v.uv;out.cell=v.uv_b-highlights.grid.xy;out.color=v.color;out.border_coverage=1.0;
    // Keep board outlines continuous when the render scale makes their world
    // width subpixel. Expand around the authored midline, preserving its depth
    // and normal thickness at close range. Inner/outer ring vertices encode
    // their edge in the otherwise-unused border alpha lane.
    if v.color.a<0.5 && out.position.w>0.0 {
        let radius=select(0.156,0.166,v.color.a>0.125);
        let mid_local=normalize(center+(v.position-center)*(0.161/radius)*scale)*length(v.position);
        let mid_world=mesh_functions::mesh_position_local_to_world(mesh_functions::get_world_from_local(input.instance_index),vec4<f32>(mid_local,1.0)).xyz;
        let mid=position_world_to_clip(mid_world);
        if mid.w>0.0 {
            let delta=(out.position.xy/out.position.w-mid.xy/mid.w)*view.viewport.zw*0.5;
            let pixels=length(delta);
            if pixels>0.00001 && pixels<0.65 {
                let offset=delta/pixels*0.65/view.viewport.zw*2.0;
                out.position=vec4<f32>((mid.xy/mid.w+offset)*out.position.w,out.position.zw);
                // Conserve the authored stroke coverage: a wider rasterization
                // footprint must not make distant or low-resolution lines bold.
                out.border_coverage=pixels/0.65;
            }
        }
    }
    out.world_normal=mesh_functions::mesh_normal_local_to_world(v.normal,input.instance_index);
    out.world_uv=(highlights.body_inverse*vec4<f32>(out.world_position,1.0)).xy*3.0;return out;
}
@fragment fn fragment(in:Output)->@location(0) vec4<f32> {
    let noise=textureSample(noise_texture,noise_sampler,in.world_uv,7).r;
    let state=tile_texel(vec2<i32>(in.cell),0);let mode=in.color.a;var color=in.color.rgb;var alpha=0.0;
    if mode<0.5 && tile_texel(vec2<i32>(in.cell),4).w>0.5 {color=vec3<f32>(1.0);}
    @if(GRID_OCCLUDED) {if mode>0.5{discard;}alpha=0.9*0.4*mix(0.92,1.0,noise);}
    @if(!GRID_OCCLUDED) {
        if mode<0.5{alpha=0.9*mix(0.92,1.0,noise);}
        else if mode<2.5{let radial=smoothstep(0.2,0.45,length(in.uv-0.5));alpha=radial*select(0.32,0.4,mode>1.5);}
        else if mode<3.5{alpha=1.0;}
        else{alpha=mode-4.0;color=apply_tile_highlight(color,in.world_position);}
    }
    if mode<0.5 {alpha*=in.border_coverage;}
    // Border opacity is independent of entrance scale; highlights retain its
    // visibility multiplier. The projected geometry itself shrinks per tile.
    let entrance=tile_texel(vec2<i32>(in.cell),3).w;
    if entrance<=0.0001 {discard;}
    alpha*=state.a/entrance;if alpha<=0.001{discard;}
    @if(!GRID_OCCLUDED){if mode>=3.5{
        var p=pbr_input_new();p.world_position=vec4<f32>(in.world_position,1.0);p.world_normal=normalize(in.world_normal);p.N=p.world_normal;p.V=normalize(view.world_position-in.world_position);p.frag_coord=in.position;
        p.material.flags=STANDARD_MATERIAL_FLAGS_ALPHA_MODE_BLEND;p.material.base_color=vec4<f32>(color,alpha);p.material.perceptual_roughness=0.7;p.material.metallic=0.1;
        return source_pbr(p,apply_pbr_lighting(p));
    }}
    // The original unlit border shader wrote its linear uniform directly to
    // the display. Compensate for this surface's automatic sRGB conversion.
    if mode<0.5 {return vec4<f32>(source_linear(color),alpha);}
    return vec4<f32>(color,alpha);
}
