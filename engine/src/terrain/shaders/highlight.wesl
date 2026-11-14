// The same body-local lookup is used on terrain, globe, water and grid.
// Five RGBA32F texels/cell preserve the frozen scalar presentation layout.
struct HighlightUniform {body_inverse:mat4x4<f32>,grid:vec4<f32>,projection:vec4<f32>}
@if(STANDARD_HIGHLIGHTS)
@group(3) @binding(32) var<uniform> highlights:HighlightUniform;
@else
@group(3) @binding(16) var<uniform> highlights:HighlightUniform;
@if(STANDARD_HIGHLIGHTS)
@group(3) @binding(33) var highlight_data:texture_2d<f32>;
@else
@group(3) @binding(17) var highlight_data:texture_2d<f32>;
fn tile_texel(cell:vec2<i32>,offset:i32)->vec4<f32> {return textureLoad(highlight_data,vec2<i32>(cell.x*5+offset,cell.y),0);}
fn highlight_coordinate(direction:vec3<f32>)->vec2<f32> {
    let d=length(direction.xy);let board=direction.xy*(acos(clamp(direction.z,-1.0,1.0))*highlights.projection.x/max(d,0.0000001));
    let r=board.y*highlights.projection.y/0.28274333882;let q=board.x/0.32648388556-r*0.5;
    let cube=vec3<f32>(q,r,-q-r);var rounded=floor(cube+0.5);let error=abs(rounded-cube);
    if error.x>error.y&&error.x>error.z {rounded.x=-rounded.y-rounded.z;}else if error.y>error.z {rounded.y=-rounded.x-rounded.z;}
    return rounded.xy-highlights.grid.xy;
}
fn apply_tile_highlight(color:vec3<f32>,world_position:vec3<f32>)->vec3<f32> {
    if highlights.projection.w<0.5 {return color;}
    let local=(highlights.body_inverse*vec4<f32>(world_position,1.0)).xyz;let direction=normalize(local);
    if direction.z<=0.0 {return color;}
    let cell=highlight_coordinate(direction);if any(cell<vec2<f32>(0.0))||any(cell>=highlights.grid.zw){return color;}
    let vp=tile_texel(vec2<i32>(cell),1);if vp.a<0.5{return color;}
    let state=tile_texel(vec2<i32>(cell),0);if state.a<=0.002||state.r+state.g+state.b<=0.0{return color;}
    let normal=tile_texel(vec2<i32>(cell),2);let facing=dot(direction,normal.xyz);if facing<=0.0{return color;}
    let delta=direction*(normal.w/facing)-normal.xyz*normal.w;
    let p=vec2<f32>(dot(delta,tile_texel(vec2<i32>(cell),3).xyz),dot(delta,tile_texel(vec2<i32>(cell),4).xyz));
    let edge=max(abs(p.x),max(abs(0.5*p.x+0.86602540378*p.y),abs(-0.5*p.x+0.86602540378*p.y)));
    let mask=1.0-smoothstep(0.14176021702,0.14376021702,edge);let gradient=0.25+0.75*smoothstep(0.15,0.45,length(p)/0.332);
    let selected=step(1.5,state.g);let time=highlights.projection.z;
    let pulse=mix(0.5+0.3*sin(time*2.0),0.8+0.2*sin(time*6.0),selected);
    let placement=min(state.g,1.0)*pulse*mix(0.6,1.0,selected);let strength=state.r+placement+state.b;
    let placement_tint=mix(vec3<f32>(0.4,1.0,0.4),vec3<f32>(0.8,1.0,0.8),selected);
    let tint=(vec3<f32>(0.95,0.95,1.0)*state.r+placement_tint*placement+vp.rgb*state.b)/max(strength,0.00001);
    return mix(color,tint,min(mix(0.55,0.75,selected),strength*gradient)*mask*state.a);
}
