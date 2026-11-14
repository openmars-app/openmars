fn visible(pixel:vec2<f32>,tag:u32)->bool {
    let alpha=f32((tag>>8u)&255u)/255.0;
    if alpha>=1.0{return true;}if alpha<=0.0{return false;}
    let xy=vec2<u32>(max(pixel,vec2(0.0)));var n=(xy.x*0x9e3779b9u)^(xy.y*0x85ebca6bu);n^=n>>16u;n*=0x7feb352du;n^=n>>15u;
    return f32(n&0xffffu)/65535.0<alpha;
}
