@if(!TERRAIN_BAKE_DIRECT)
import openmars_engine::atmosphere::common::source_pbr;
@if(!TERRAIN_BAKE_DIRECT)
import openmars_engine::terrain::shaders::highlight::apply_tile_highlight;
import openmars_engine::terrain::shaders::field::{params,terrain,field_sampler,field,field_uv,field_weights,field_layers,basin_shore,project,TerrainVertex};
@if(!TERRAIN_BAKE_DIRECT)
import openmars_engine::terrain::shaders::field::terrain_vertex;
@if(!TERRAIN_BAKE_DIRECT)
import bevy_pbr::render::{forward_io::{Vertex,decompress_vertex},mesh_functions,view_transformations::position_world_to_clip,pbr_types::{pbr_input_new,STANDARD_MATERIAL_FLAGS_ALPHA_MODE_BLEND},pbr_functions::{apply_pbr_lighting}};
@group(3) @binding(3) var surface_fields:texture_2d_array<f32>;
@group(3) @binding(5) var colors:texture_2d_array<f32>;
@group(3) @binding(6) var atlas_sampler:sampler;
@group(3) @binding(7) var details:texture_2d_array<f32>;
@group(3) @binding(8) var mars:texture_2d<f32>;
@group(3) @binding(9) var mars_sampler:sampler;
@group(3) @binding(10) var plant_shade:texture_2d<f32>;
@group(3) @binding(11) var shade_sampler:sampler;
@group(3) @binding(12) var surface_mask:texture_2d<f32>;
@group(3) @binding(13) var bake_albedo:texture_2d<f32>;
@group(3) @binding(14) var bake_surface:texture_2d<f32>;
@group(3) @binding(15) var bake_sampler:sampler;

@if(!TERRAIN_BAKE_DIRECT)
@vertex fn vertex(input:Vertex)->TerrainVertex {
    @if(TERRAIN_GLOBE) {
        let v=decompress_vertex(input,input.instance_index);var out:TerrainVertex;
        out.world_position=mesh_functions::mesh_position_local_to_world(mesh_functions::get_world_from_local(input.instance_index),vec4<f32>(v.position,1.0));
        out.world_normal=mesh_functions::mesh_normal_local_to_world(v.normal,input.instance_index);
        out.position=position_world_to_clip(out.world_position.xyz);out.local=v.uv;out.board=vec2<f32>(0.0);out.layer_birth=vec3<f32>(0.0);return out;
    }
    @if(!TERRAIN_GLOBE) {
        var out=terrain_vertex(input,false);
        @if(TERRAIN_BAKE_ALBEDO || TERRAIN_BAKE_SURFACE) {
            let uv=(out.board-params.bake_bounds.xy)/params.bake_bounds.zw;
            let stripe_y=(uv.y-params.bake.z)/params.bake.w;
            out.position=vec4<f32>(uv.x*2.0-1.0,1.0-stripe_y*2.0,0.5,1.0);
        }
        return out;
    }
}
fn color_layer(uv:vec2<f32>,layer:i32,dx:vec2<f32>,dy:vec2<f32>)->vec4<f32> {
    return textureSampleGrad(colors,atlas_sampler,vec2<f32>(uv.x,-uv.y),layer,vec2<f32>(dx.x,-dx.y),vec2<f32>(dy.x,-dy.y));
}
fn detail_layer(uv:vec2<f32>,layer:i32,dx:vec2<f32>,dy:vec2<f32>)->vec4<f32> {
    return textureSampleGrad(details,atlas_sampler,vec2<f32>(uv.x,-uv.y),layer,vec2<f32>(dx.x,-dx.y),vec2<f32>(dy.x,-dy.y));
}
fn noise(uv:vec2<f32>,dx:vec2<f32>,dy:vec2<f32>)->f32 {return detail_layer(uv,7,dx,dy).r;}
fn channel_field(local:vec2<f32>,lb:vec3<f32>,detail_channel:bool)->vec4<f32> {
    let source_uv=field_uv(local);
    // Original RGBA8 material/detail planes occupy adjacent, equally sized
    // rectangles. Their existing two-texel borders keep filtering independent.
    let uv=vec2<f32>(source_uv.x*0.5+select(0.0,0.5,detail_channel),source_uv.y);
    let layers=field_layers(lb);let w=field_weights(local,lb);
    if w.y>=1.0 {return textureSampleLevel(surface_fields,field_sampler,uv,layers.z,0.0);}
    var middle=textureSampleLevel(surface_fields,field_sampler,uv,layers.y,0.0);
    if w.x<1.0 {middle=mix(textureSampleLevel(surface_fields,field_sampler,uv,layers.x,0.0),middle,w.x);}
    if w.y>0.0 {middle=mix(middle,textureSampleLevel(surface_fields,field_sampler,uv,layers.z,0.0),w.y);}
    return middle;
}
struct Surface {base:vec3<f32>,alpha:f32,slope:vec2<f32>,roughness:f32,ao:f32,paving_gradient:vec2<f32>}
fn compose(in:TerrainVertex,uvdx:vec2<f32>,uvdy:vec2<f32>)->Surface {
    var out:Surface;let f=field(in.local,in.layer_birth);let shore=f.y;let foundation=f.w;
    let paving=color_layer(in.board*9.0,8,uvdx*9.0,uvdy*9.0).rgb;
    let paving_height=dot(paving,vec3<f32>(0.2126,0.7152,0.0722))*0.00004*foundation;
    out.paving_gradient=vec2<f32>(dpdx(paving_height),dpdy(paving_height));
    let radial=normalize(project(in.board,0.0));
    // Bevy stores decoded rows top-first. The legacy 2D WebGL sampler flipped V;
    // cancel that flip here, matching the relief globe and scalar sampler.
    let mars_uv=vec2<f32>(atan2(radial.z,-radial.x)/6.283185307179586,acos(clamp(radial.y,-1.0,1.0))/3.141592653589793);
    let mars_dx=dpdx(mars_uv);let mars_dy=dpdy(mars_uv);
    let bake_uv=(in.board-params.bake_bounds.xy)/params.bake_bounds.zw;
    @if(!TERRAIN_BAKE_ALBEDO && !TERRAIN_BAKE_SURFACE) {
        if params.bake.x>0.5 && all(bake_uv>vec2<f32>(0.0005)) && all(bake_uv<vec2<f32>(0.9995)) {
            let albedo=textureSampleGrad(bake_albedo,bake_sampler,bake_uv,uvdx/params.bake_bounds.zw,uvdy/params.bake_bounds.zw);
            let surface=textureSampleGrad(bake_surface,bake_sampler,bake_uv,uvdx/params.bake_bounds.zw,uvdy/params.bake_bounds.zw)/max(albedo.a,0.0001);
            out.base=albedo.rgb/max(albedo.a,0.0001);out.alpha=albedo.a;out.slope=surface.xy*2.0-1.0;out.roughness=surface.z;out.ao=surface.w;return out;
        }
    }
    let weights=channel_field(in.local,in.layer_birth,false);let df=channel_field(in.local,in.layer_birth,true);
    let mars_color=textureSampleGrad(mars,mars_sampler,mars_uv,mars_dx,mars_dy).rgb;
    var base=mix(mars_color,mars_color*vec3<f32>(0.9,0.96,1.08),params.water_extra.w*0.6);
    let relief=pow(clamp(mars_color.r/0.65,0.0,1.0),1.15);let natural_shade=mix(0.30,1.0,relief);
    let coverage=clamp(f.z,0.0,1.0);let growth=mix(coverage,1.0-pow(1.0-coverage,2.2),smoothstep(0.2,0.7,coverage));
    let bank_width=mix(0.075,0.05,clamp(weights.y+weights.z,0.0,1.0));let bank=1.0-smoothstep(bank_width*0.55,bank_width,max(shore,0.0));
    let ground_coverage=clamp(foundation+growth,0.0,1.0);let nature_weight=growth*(1.0-foundation*0.85);let nature_ratio=nature_weight/max(foundation+nature_weight,0.0001);
    let turn=mat2x2<f32>(0.8,-0.6,0.6,0.8);let material_uv=in.board*18.0;let lush_uv=turn*material_uv*0.83;
    let lush_dx=turn*uvdx*14.94;let lush_dy=turn*uvdy*14.94;
    let present=step(vec4<f32>(0.001),weights);
    var lush_detail=vec4<f32>(0.5,0.5,0.9,0.5);var litter_detail=lush_detail;var soil_detail=lush_detail;
    if present.y>0.0 {lush_detail=detail_layer(lush_uv,0,lush_dx,lush_dy);}
    if present.z>0.0 {litter_detail=detail_layer(material_uv*0.7,1,uvdx*12.6,uvdy*12.6);}
    if present.w>0.0 {soil_detail=detail_layer(material_uv*0.65,2,uvdx*11.7,uvdy*11.7);}
    let heights=vec4<f32>(0.5,lush_detail.a,litter_detail.a,soil_detail.a);let lifted=(weights+heights*0.5)*present;
    let crest=max(max(lifted.x,lifted.y),max(lifted.z,lifted.w))-0.2;var mw=max(lifted-vec4<f32>(crest),vec4<f32>(0.0))*present;mw/=max(0.0001,dot(mw,vec4<f32>(1.0)));
    var mud=vec3<f32>(0.0);if present.w>0.0 {mud=color_layer(material_uv*0.65,2,uvdx*11.7,uvdy*11.7).rgb*vec3<f32>(0.65,0.7,0.6);}
    var nature=vec3<f32>(0.0);var macro_noise=0.5;var plant_shadow=0.0;
    if nature_weight>0.0 {
        var dry=vec3<f32>(0.0);var lush=dry;var litter=dry;
        if mw.x>0.001 {dry=color_layer(in.board*80.0,5,uvdx*80.0,uvdy*80.0).rgb*vec3<f32>(0.48,0.66,0.32);}
        if present.y>0.0 {lush=color_layer(lush_uv,0,lush_dx,lush_dy).rgb*vec3<f32>(0.65,1.1,0.6);}
        if present.z>0.0 {litter=color_layer(material_uv*0.7,1,uvdx*12.6,uvdy*12.6).rgb*vec3<f32>(0.48,0.58,0.4);}
        nature=dry*mw.x+lush*mw.y+litter*mw.z+mud*mw.w;
        let green_patch=noise(in.board*3.7+vec2<f32>(0.11),uvdx*3.7,uvdy*3.7)*0.6+lush_detail.a*0.4;
        let stage=clamp(params.climate.z*4.0+(green_patch-0.55)*0.8+weights.w*0.3,0.0,4.0);
        var tone=mix(vec3<f32>(0.36,0.37,0.38),vec3<f32>(0.3,0.23,0.15),clamp(stage,0.0,1.0));
        tone=mix(tone,vec3<f32>(0.22,0.22,0.12),clamp(stage-1.0,0.0,1.0));tone=mix(tone,vec3<f32>(0.11,0.19,0.08),clamp(stage-2.0,0.0,1.0));
        let tone_detail=0.12+0.16*dot(heights.yw,vec2<f32>(0.6,0.4));nature=mix(tone*(0.55+tone_detail*1.8),nature,smoothstep(3.0,4.0,stage));
        macro_noise=noise(in.board*2.4,uvdx*2.4,uvdy*2.4);nature*=mix(0.84,1.12,macro_noise);nature=mix(nature,nature*vec3<f32>(1.04,1.0,0.9),smoothstep(0.55,0.8,macro_noise)*0.25);
        let hill=clamp((df.g-0.25)/0.45,0.0,1.0);nature*=mix(1.0,mix(0.84,1.06,hill),df.b)*natural_shade;
        let shade_uv=(in.board-params.shade_bounds.xy)/params.shade_bounds.zw;
        plant_shadow=textureSampleGrad(plant_shade,shade_sampler,shade_uv,uvdx/params.shade_bounds.zw,uvdy/params.shade_bounds.zw).r;
    }
    var bank_color=vec3<f32>(0.0);let cold_meadow=df.r*(1.0-params.climate.w);
    if bank>0.001 || cold_meadow>0.001 {
        let sand=color_layer(in.board*48.0,6,uvdx*48.0,uvdy*48.0).rgb;let rock=color_layer(in.board*29.6,7,uvdx*29.6,uvdy*29.6).rgb;
        bank_color=mix(sand,rock,clamp(weights.z*0.7,0.0,0.3))*mix(0.65,1.0,relief);
    }
    nature=mix(nature,bank_color,cold_meadow);let prepared=paving*vec3<f32>(0.18,0.195,0.2);
    base=mix(base,mix(prepared,nature,nature_ratio),ground_coverage);base=mix(base,bank_color,bank);base=mix(base,mud*natural_shade,weights.w*0.55);
    var frost_amount=0.0;var frost_detail=vec4<f32>(0.5,0.5,1.0,0.5);
    if params.climate.x>0.0 {
        let frost_uv=material_uv*0.45;let fdx=uvdx*8.1;let fdy=uvdy*8.1;let frost_turn=mat2x2<f32>(0.6,-0.8,0.8,0.6);
        frost_detail=detail_layer(frost_uv,4,fdx,fdy);let hollow_b=detail_layer(frost_turn*material_uv*0.29+vec2<f32>(0.37),4,frost_turn*uvdx*5.22,frost_turn*uvdy*5.22).a;
        let hollow=1.0-(frost_detail.a*0.55+hollow_b*0.45);let threshold=1.0-params.climate.x*0.8;
        let cover=smoothstep(0.0,0.8,clamp(growth+bank,0.0,1.0)+(hollow_b-0.5)*0.3);
        frost_amount=smoothstep(threshold,threshold+0.3,hollow)*(1.0-foundation)*cover;
        let drift=params.climate.y*(1.0-smoothstep(0.0,0.03,max(shore,0.0)))*smoothstep(0.3,0.6,hollow);frost_amount=max(frost_amount,drift*(1.0-foundation));
        if frost_amount>0.0 {
            let solid=dot(color_layer(frost_uv,3,fdx,fdy).rgb,vec3<f32>(0.3333));let snow=vec3<f32>(0.9,0.94,1.0)*(0.92+0.08*solid);base=mix(base,snow,frost_amount);
            frost_detail=mix(frost_detail,detail_layer(frost_uv,3,fdx,fdy),smoothstep(0.5,0.9,params.climate.x));
        }
    }
    let visible=max(ground_coverage,1.0-smoothstep(bank_width,bank_width+0.012,shore));out.alpha=smoothstep(0.001,0.02,visible);
    let lush_slope=mat2x2<f32>(0.8,0.6,-0.6,0.8)*(lush_detail.rg*2.0-1.0);
    out.slope=mw.y*lush_slope+mw.z*(litter_detail.rg*2.0-1.0)+mw.w*(soil_detail.rg*2.0-1.0);out.slope=mix(out.slope,frost_detail.rg*2.0-1.0,frost_amount);
    let nature_surface=ground_coverage*nature_ratio*(1.0-bank);out.slope*=nature_surface;
    var roughness=dot(mw,vec4<f32>(0.93,lush_detail.b,litter_detail.b,soil_detail.b));roughness=mix(roughness,frost_detail.b,frost_amount);
    out.roughness=mix(0.9,clamp(roughness+(macro_noise-0.5)*0.1,0.65,1.0),nature_surface);
    let cavity=mix(dot(mw,heights),frost_detail.a,frost_amount);let cavity_ao=mix(1.0,clamp(cavity*1.3,0.0,1.0),0.45*nature_surface);
    out.ao=cavity_ao*(1.0-plant_shadow*0.55*nature_surface)*(1.0-weights.z*0.25);out.base=base;return out;
}
@if(!TERRAIN_BAKE_DIRECT)
@fragment fn fragment(in:TerrainVertex)->@location(0) vec4<f32> {
    let dx=dpdx(in.world_position.xyz);let dy=dpdy(in.world_position.xyz);let uvdx=dpdx(in.board);let uvdy=dpdy(in.board);
    @if(TERRAIN_GLOBE) {
        let color=textureSample(mars,mars_sampler,in.local).rgb;
        let mask=textureLoad(surface_mask,vec2<i32>(in.position.xy),0).rgb;if any(mask>vec3<f32>(0.5)){discard;}
        var p=pbr_input_new();p.world_position=in.world_position;p.world_normal=normalize(in.world_normal);p.N=p.world_normal;p.V=normalize(params.camera.xyz-in.world_position.xyz);p.frag_coord=in.position;
        p.material.base_color=vec4<f32>(apply_tile_highlight(mix(color,color*vec3<f32>(0.9,0.96,1.08),params.water_extra.w*0.6),in.world_position.xyz),1.0);p.material.perceptual_roughness=1.0;
        let lit=apply_pbr_lighting(p);
        return source_pbr(p,lit);
    }
    @if(!TERRAIN_GLOBE) {
        let surface=compose(in,uvdx,uvdy);let shore=basin_shore(in.local,in.layer_birth);
        let footprint=length(abs(uvdx*18.0)+abs(uvdy*18.0));var normal=normalize(in.world_normal);
        let px=cross(dy,normal);let py=cross(normal,dx);let tangent=px*uvdx.x+py*uvdy.x;let bitangent=px*uvdx.y+py*uvdy.y;
        let scale=inverseSqrt(max(max(dot(tangent,tangent),dot(bitangent,bitangent)),1e-20));let slope=surface.slope*mix(0.55,0.3,smoothstep(0.15,0.6,footprint));
        normal=normalize(normal*sqrt(max(0.0,1.0-dot(slope,slope)))+(tangent*slope.x+bitangent*slope.y)*scale);
        let paving_x=cross(dy,normal);let paving_y=cross(normal,dx);let determinant=dot(dx,paving_x);normal=normalize(normal-(surface.paving_gradient.x*paving_x+surface.paving_gradient.y*paving_y)*sign(determinant)/max(abs(determinant),1e-12));
        // Derivatives and all explicit gradients precede ownership discards.
        @if(TERRAIN_BASIN) {if shore>=0.115 {discard;}}
        @if(!TERRAIN_BASIN && !TERRAIN_BAKE_ALBEDO && !TERRAIN_BAKE_SURFACE) {if shore<0.115 {discard;}}
        @if(TERRAIN_OPAQUE) {if surface.alpha<1.0 {discard;}}
        @if(TERRAIN_EDGE) {if surface.alpha>=1.0||surface.alpha<0.001 {discard;}}
        @if(TERRAIN_BAKE_ALBEDO) {return vec4<f32>(surface.base*surface.alpha,surface.alpha);}
        @if(TERRAIN_BAKE_SURFACE) {return vec4<f32>(surface.slope*0.5+0.5,surface.roughness,surface.ao)*surface.alpha;}
        @if(!TERRAIN_BAKE_ALBEDO && !TERRAIN_BAKE_SURFACE) {
            let mask=textureLoad(surface_mask,vec2<i32>(in.position.xy),0).rgb;if mask.r>0.5||mask.b>0.5 {discard;}
            var p=pbr_input_new();p.world_position=in.world_position;p.world_normal=normalize(in.world_normal);p.N=normal;p.V=normalize(params.camera.xyz-in.world_position.xyz);p.frag_coord=in.position;
            var alpha=surface.alpha;@if(TERRAIN_BASIN){alpha=1.0;}
            p.material.base_color=vec4<f32>(apply_tile_highlight(surface.base,in.world_position.xyz),alpha);p.material.perceptual_roughness=surface.roughness;p.diffuse_occlusion=vec3<f32>(surface.ao);
            let nv=clamp(dot(normal,p.V),0.0,1.0);p.specular_occlusion=clamp(pow(nv+surface.ao,exp2(-16.0*surface.roughness-1.0))-1.0+surface.ao,0.0,1.0);
            let micro=clamp(dot(normal,normalize(params.sun_direction.xyz))+2.0*surface.ao*surface.ao-1.0,0.0,1.0);p.directional_shadow_factor=mix(1.0,micro,0.75);
            @if(TERRAIN_EDGE){p.material.flags=STANDARD_MATERIAL_FLAGS_ALPHA_MODE_BLEND;}
            let lit=apply_pbr_lighting(p);
        return source_pbr(p,lit);
        }
    }
}

// Dedicated cache entry points use the exact live compose() implementation and
// existing patch UV/layer data. No camera, PBR view state, HDR or depth target.
@if(TERRAIN_BAKE_DIRECT)
struct BakeVertex { @location(0) uv:vec2<f32>, @location(1) color:vec4<f32>, @location(2) clocks:vec2<f32> }
@if(TERRAIN_BAKE_DIRECT)
@vertex fn bake_vertex(input:BakeVertex)->TerrainVertex {
    var out:TerrainVertex;
    out.local=input.uv;out.board=input.color.xy+input.uv*params.geometry.y;out.layer_birth=vec3<f32>(input.color.zw,input.clocks.x);
    let h=field(out.local,out.layer_birth).x;
    out.world_position=vec4<f32>(project(out.board,h),1.0);out.world_normal=normalize(project(out.board,0.0));
    let uv=(out.board-params.bake_bounds.xy)/params.bake_bounds.zw;
    let stripe_y=(uv.y-params.bake.z)/params.bake.w;
    out.position=vec4<f32>(uv.x*2.0-1.0,1.0-stripe_y*2.0,0.5,1.0);
    return out;
}
@if(TERRAIN_BAKE_DIRECT)
@fragment fn bake_fragment(in:TerrainVertex)->@location(0) vec4<f32> {
    let surface=compose(in,dpdx(in.board),dpdy(in.board));
    @if(TERRAIN_BAKE_ALBEDO) {return vec4<f32>(surface.base*surface.alpha,surface.alpha);}
    @if(TERRAIN_BAKE_SURFACE) {return vec4<f32>(surface.slope*0.5+0.5,surface.roughness,surface.ao)*surface.alpha;}
}
