// Sample actual source textures, including hardware-decoded compressed variants.
// Rendering into sRGB targets performs the matching encode; alpha stays straight.
@group(0) @binding(0) var source:texture_2d<f32>;
@group(0) @binding(1) var source_sampler:sampler;
struct Output {@builtin(position) position:vec4<f32>,@location(0) uv:vec2<f32>};
@vertex fn vertex(@builtin(vertex_index) index:u32)->Output {
    let uv=vec2<f32>(f32((index<<1u)&2u),f32(index&2u));var out:Output;
    out.position=vec4<f32>(uv.x*2.0-1.0,1.0-uv.y*2.0,0.0,1.0);out.uv=uv;return out;
}
@fragment fn fragment(in:Output)->@location(0) vec4<f32> {
    return textureSampleLevel(source,source_sampler,in.uv,0.0);
}
