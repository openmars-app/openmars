struct Out { @builtin(position) position:vec4<f32>, @location(0) uv:vec2<f32>, @location(1) strength:f32, @location(2) taper:f32 }
@vertex fn vertex(@builtin(vertex_index) i:u32, @location(0) pose:vec4<f32>, @location(1) form:vec4<f32>)->Out {
    let corners=array<vec2<f32>,6>(vec2(0.,0.),vec2(1.,0.),vec2(0.,1.),vec2(0.,1.),vec2(1.,0.),vec2(1.,1.));
    let uv=corners[i]; let local=(uv-vec2(0.5))*form.xy;
    let p=pose.xy+vec2(pose.z*local.x-pose.w*local.y,pose.w*local.x+pose.z*local.y);
    var out:Out; out.position=vec4(p.x*2.-1.,1.-p.y*2.,0.,1.);out.uv=uv;out.strength=form.z;out.taper=form.w;return out;
}
@fragment fn fragment(in:Out)->@location(0) vec4<f32> {
    let falloff=1.-smoothstep(0.,1.,length(in.uv*2.-1.));
    let fade=1.-in.taper*smoothstep(0.15,1.,in.uv.x);
    return vec4(vec3(in.strength*falloff*falloff*fade),1.);
}
