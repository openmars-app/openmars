import bevy_pbr::render::forward_io::VertexOutput;
struct Output {
    @builtin(position) position:vec4<f32>,
    @location(0) world_position:vec4<f32>,@location(1) world_normal:vec3<f32>,
    @if(VERTEX_UVS_A) @location(2) uv:vec2<f32>,
    @if(VERTEX_UVS_B) @location(3) uv_b:vec2<f32>,
    @if(VERTEX_TANGENTS) @location(4) world_tangent:vec4<f32>,
    @if(VERTEX_COLORS) @location(5) color:vec4<f32>,
    @if(VERTEX_OUTPUT_INSTANCE_INDEX) @location(6) @interpolate(flat) instance_index:u32,
    @if(VISIBILITY_RANGE_DITHER) @location(7) @interpolate(flat) visibility_range_dither:i32,
    @location(8) local_position:vec3<f32>,@location(9) local_normal:vec3<f32>,
    @location(10) opacity:f32,
}
fn standard(in:Output)->VertexOutput {
    var out:VertexOutput;out.position=in.position;out.world_position=in.world_position;out.world_normal=in.world_normal;
    @if(VERTEX_UVS_A) out.uv=in.uv;
    @if(VERTEX_UVS_B) out.uv_b=in.uv_b;
    @if(VERTEX_TANGENTS) out.world_tangent=in.world_tangent;
    @if(VERTEX_COLORS) out.color=in.color;
    @if(VERTEX_OUTPUT_INSTANCE_INDEX) out.instance_index=in.instance_index;
    @if(VISIBILITY_RANGE_DITHER) out.visibility_range_dither=in.visibility_range_dither;
    return out;
}
