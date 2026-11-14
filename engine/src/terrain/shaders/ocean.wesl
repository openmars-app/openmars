import openmars_engine::atmosphere::common::{apply_haze,source_linear};
import openmars_engine::terrain::shaders::highlight::apply_tile_highlight;
import openmars_engine::terrain::shaders::field::{params,field,terrain_vertex,TerrainVertex};
import bevy_pbr::render::forward_io::Vertex;
@group(3) @binding(5) var colors:texture_2d_array<f32>;
@group(3) @binding(6) var atlas_sampler:sampler;
@group(3) @binding(7) var details:texture_2d_array<f32>;
@group(3) @binding(12) var surface_mask:texture_2d<f32>;
@vertex fn vertex(input:Vertex)->TerrainVertex {return terrain_vertex(input,true);}
var<private> board_dx:vec2<f32>;
var<private> board_dy:vec2<f32>;
fn detail_at(uv:vec2<f32>,layer:i32,dx:vec2<f32>,dy:vec2<f32>)->vec4<f32> {return textureSampleGrad(details,atlas_sampler,vec2<f32>(uv.x,-uv.y),layer,vec2<f32>(dx.x,-dx.y),vec2<f32>(dy.x,-dy.y));}
fn noise(uv:vec2<f32>,scale:f32)->f32 {return detail_at(uv,7,board_dx*scale,board_dy*scale).r;}
fn clarity(murky:vec3<f32>,teal:vec3<f32>,blue:vec3<f32>)->vec3<f32> {return mix(mix(murky,teal,smoothstep(0.0,0.55,params.water.y)),blue,smoothstep(0.45,1.0,params.water.y));}
fn ice_detail(uv:vec2<f32>,dx:vec2<f32>,dy:vec2<f32>)->vec4<f32> {return mix(detail_at(uv,5,dx,dy),detail_at(uv,6,dx,dy),params.water.x);}
fn ice_color(uv:vec2<f32>,dx:vec2<f32>,dy:vec2<f32>)->vec4<f32> {
    return mix(textureSampleGrad(colors,atlas_sampler,vec2<f32>(uv.x,-uv.y),9,vec2<f32>(dx.x,-dx.y),vec2<f32>(dy.x,-dy.y)),textureSampleGrad(colors,atlas_sampler,vec2<f32>(uv.x,-uv.y),10,vec2<f32>(dx.x,-dx.y),vec2<f32>(dy.x,-dy.y)),params.water.x);
}
fn foam_at(board:vec2<f32>,distance:f32)->f32 {
    let time=params.camera.w;let drift=vec2<f32>(time*0.006,-time*0.004);let variation=smoothstep(0.35,0.65,noise(board*1.7,1.7));
    let breakup=noise(board*12.0+drift,12.0);let bubbles=noise(board*65.0-drift*1.3,65.0);let arrival=dot(board,vec2<f32>(2.1,2.9));
    let wave=distance+(variation-0.5)*0.012+arrival*0.075;let phase=(wave+time*0.005625)/0.075;
    let crest_distance=(fract(phase+0.5)-0.5)*0.075+(breakup-0.5)*0.003;let aa=max(fwidth(wave),0.00025);
    let crest=1.0-smoothstep(0.0006,0.00125+aa,abs(crest_distance));let wake=smoothstep(0.0,0.0015+aa,crest_distance)*(1.0-smoothstep(0.002,0.006,crest_distance));
    let age=clamp(crest_distance/0.006,0.0,1.0);let porous=smoothstep(0.44+age*0.18,0.62+age*0.18,breakup*0.65+bubbles*0.35);let broken=smoothstep(0.46,0.62,breakup);
    let offshore=1.0-smoothstep(0.012,0.035,distance);let resolved=1.0-smoothstep(0.003,0.012,fwidth(wave));
    let activity=mix(0.65,1.0,params.water.w)*mix(0.65,1.0,params.water_extra.x);let thawed=smoothstep(0.0,0.05,1.0-params.climate.y);
    return clamp((crest*broken+wake*porous*0.3)*offshore*resolved*activity*thawed*(0.65+variation*0.35)*0.35,0.0,0.3);
}
@fragment fn fragment(in:TerrainVertex)->@location(0) vec4<f32> {
    board_dx=dpdx(in.board);board_dy=dpdy(in.board);
    let f=field(in.local,in.layer_birth);let aa=max(fwidth(f.y),0.00025);let depth=max(0.0,-0.008-f.x);
    var water=(1.0-smoothstep(-0.009-aa,0.001+aa,f.y))*smoothstep(0.0,0.0003,depth);
    let foam=foam_at(in.board,max(-f.y,0.0));
    let mask=textureLoad(surface_mask,vec2<i32>(in.position.xy),0).rgb;if water<0.001||mask.r>0.5||mask.b>0.5 {discard;}
    let time=params.camera.w;let uv=in.board*38.0;let n1=detail_at(uv+vec2<f32>(time*0.025,time*0.017),8,board_dx*38.0,board_dy*38.0).xyz*2.0-1.0;let n2=detail_at(uv*0.63-vec2<f32>(time*0.016,time*0.023),8,board_dx*23.94,board_dy*23.94).xyz*2.0-1.0;
    let radial=normalize(in.world_normal);let axis=select(vec3<f32>(0.0,1.0,0.0),vec3<f32>(1.0,0.0,0.0),abs(radial.y)>0.99);let tangent=normalize(cross(axis,radial));let bitangent=cross(radial,tangent);
    let amplitude=mix(0.025,0.085,params.water.w);let normal=normalize(radial+tangent*(n1.x+n2.x)*amplitude+bitangent*(n1.y+n2.y)*amplitude);let view=normalize(params.camera.xyz-in.world_position.xyz);
    let daylight=0.3+0.7*max(radial.z,0.0);let shallow=1.0-smoothstep(0.0003,0.014,depth);
    let deep=clarity(vec3<f32>(0.12,0.09,0.05),vec3<f32>(0.04,0.09,0.08),vec3<f32>(0.05,0.09,0.1))+vec3<f32>(0.025,0.06,0.07);let shallow_color=clarity(vec3<f32>(0.26,0.21,0.13),vec3<f32>(0.13,0.27,0.3),vec3<f32>(0.12,0.3,0.4));
    var color=mix(deep,shallow_color,shallow*0.65)*params.sun_color.rgb*daylight;
    let bloom=smoothstep(0.72,0.9,detail_at(in.board*2.5+vec2<f32>(time*0.002),8,board_dx*2.5,board_dy*2.5).x*0.5+0.5);color=mix(color,vec3<f32>(0.1,0.2,0.08)*daylight,params.water.z*shallow*bloom*0.3);
    let fres=0.02+0.98*pow(1.0-max(dot(normal,view),0.0),5.0);color=mix(color,vec3<f32>(0.12,0.2,0.26)*daylight,fres*0.65);
    let sun=normalize(params.sun_direction.xyz);let spec=pow(max(dot(reflect(-sun,normal),view),0.0),100.0);color+=params.sun_color.rgb*spec*params.sun_direction.w*0.35*(1.0+params.water_extra.y*2.0);
    color=mix(color,vec3<f32>(0.92,0.95,0.91)*params.sun_color.rgb*daylight,foam);
    if params.climate.y>0.0 {
        let ice_uv=in.board*12.0;let turn=mat2x2<f32>(0.6,-0.8,0.8,0.6);let ice_uv_b=turn*in.board*7.3+vec2<f32>(0.41);let detail=mix(ice_detail(ice_uv,board_dx*12.0,board_dy*12.0),ice_detail(ice_uv_b,turn*board_dx*7.3,turn*board_dy*7.3),0.4);
        let inland=max(-f.y,0.0)+(detail.a-0.5)*0.004;let edge=(1.0-params.climate.y)*params.water_extra.z;let ice=smoothstep(edge,edge+0.003,inland)*smoothstep(0.0,0.15,params.climate.y);let thickness=smoothstep(0.0,0.02,inland-edge);
        let slope=(detail.rg*2.0-1.0)*0.12;let ice_normal=normalize(radial+tangent*slope.x+bitangent*slope.y);let plates=dot(mix(ice_color(ice_uv,board_dx*12.0,board_dy*12.0),ice_color(ice_uv_b,turn*board_dx*7.3,turn*board_dy*7.3),0.4).rgb,vec3<f32>(0.3333));
        let solid=vec3<f32>(0.8,0.87,0.92);let thin=mix(deep,vec3<f32>(0.55,0.68,0.75),0.6);let base=mix(thin,mix(solid,thin,params.water.x*0.75),thickness)*(0.9+0.2*plates);
        let diffuse=0.55+0.45*max(dot(ice_normal,sun),0.0);var ice_color_value=base*diffuse*params.sun_color.rgb*daylight;
        let ice_fresnel=pow(1.0-max(dot(ice_normal,view),0.0),4.0);ice_color_value=mix(ice_color_value,vec3<f32>(0.7,0.8,0.9)*daylight,ice_fresnel*0.4);
        let roughness=mix(0.25,0.6,detail.b);let glint=pow(max(dot(reflect(-sun,ice_normal),view),0.0),mix(220.0,30.0,roughness));ice_color_value+=params.sun_color.rgb*glint*params.sun_direction.w*0.4*(1.0-roughness);
        color=mix(color,ice_color_value,ice);water*=mix(1.0,mix(0.55,1.0,thickness),ice);
    }
    // Source water ShaderMaterial emits display RGB; haze mixes in that space.
    let hazed=apply_haze(color,in.world_position.xyz,normal,params.camera.xyz,false);
    return vec4<f32>(apply_tile_highlight(source_linear(hazed),in.world_position.xyz),water);
}
