import bevy_pbr::render::{pbr_types::PbrInput,mesh_view_bindings::view,pbr_lighting as lighting,pbr_functions::{main_pass_post_lighting_processing,calculate_diffuse_color,calculate_F0_dielectric}};
struct AtmosphereUniform {
 bodies:array<vec4<f32>,16>,colors:array<vec4<f32>,16>,shadows:array<vec4<f32>,16>,profiles:array<vec4<f32>,16>,
 sun:vec4<f32>,sun_color:vec4<f32>,lighting:vec4<f32>,metadata:vec4<u32>
}
@group(3) @binding(31) var<uniform> atmosphere:AtmosphereUniform;
fn source_aces(color:vec3<f32>)->vec3<f32> {
 let input_matrix=mat3x3<f32>(vec3<f32>(0.59719,0.07600,0.02840),vec3<f32>(0.35458,0.90834,0.13383),vec3<f32>(0.04823,0.01566,0.83777));
 let output_matrix=mat3x3<f32>(vec3<f32>(1.60475,-0.10208,-0.00327),vec3<f32>(-0.53108,1.10813,-0.07276),vec3<f32>(-0.07367,-0.00605,1.07602));
 let v=input_matrix*(color/0.6);let a=v*(v+vec3<f32>(0.0245786))-0.000090537;let b=v*(v*0.983729+0.4329510)+0.238081;
 return clamp(output_matrix*(a/b),vec3<f32>(0.0),vec3<f32>(1.0));
}
fn source_srgb(c:vec3<f32>)->vec3<f32> {return select(c*12.92,1.055*pow(max(c,vec3<f32>(0.0)),vec3<f32>(0.41666))-0.055,c>=vec3<f32>(0.0031308));}
fn source_linear(c:vec3<f32>)->vec3<f32> {return select(c*0.0773993808,pow(max(c*0.9478672986+0.0521327014,vec3<f32>(0.0)),vec3<f32>(2.4)),c>=vec3<f32>(0.04045));}
// Profiles contain actual light/visibility-scaled intensity. Colors are linear.
fn apply_haze(color:vec3<f32>,world:vec3<f32>,surface_normal:vec3<f32>,camera:vec3<f32>,linear_color:bool)->vec3<f32> {
 var result=color;
 for(var i=0u;i<min(atmosphere.metadata.x,16u);i++) {
  let body=atmosphere.bodies[i];let profile=atmosphere.profiles[i];
  if profile.y<=0.0||atmosphere.colors[i].w<0.5 {continue;}
  let offset=world-body.xyz;var height=length(offset)/body.w-1.0;
  if height>profile.x||height< -0.5 {continue;}
  var normal=normalize(offset);
  if profile.w>0.5&&linear_color {normal=normalize(surface_normal);height=0.0;}
  let sun=normalize(atmosphere.sun.xyz-body.xyz);let key=normalize(sun*0.45+vec3<f32>(0.0,0.9,0.0));
  let warmth=smoothstep(-0.35,0.65,dot(normal,key));let daylight=0.28+0.72*smoothstep(-0.25,0.65,dot(normal,sun));
  let facing=clamp(dot(normal,normalize(camera-world)),0.0,1.0);let path=0.65*pow(1.0-facing,2.2)+0.01*warmth*warmth;
  let altitude=max(height,0.0)/profile.x;let fade=exp(-2.0*altitude)*(1.0-smoothstep(0.6,1.0,altitude));
  let transmission=exp(-path*daylight*profile.y*fade);
  var radiance=mix(atmosphere.shadows[i].rgb*0.3,atmosphere.colors[i].rgb*atmosphere.sun_color.rgb*1.2,warmth);
  if !linear_color {radiance=source_srgb(source_aces(radiance));}
  result=result*transmission+radiance*(1.0-transmission);
 }
 return result;
}

// Source ambient and hemisphere irradiance use linear colors, Y-up and no
// exposure conversion. Unlike scene IBL, these lights also illuminate Lambert.
fn source_ambient(normal:vec3<f32>)->vec3<f32> {
 let ambient=vec3<f32>(atmosphere.lighting.x);
 let sky=source_linear(vec3<f32>(98.0,117.0,139.0)/255.0);
 let ground=source_linear(vec3<f32>(10.0,36.0,66.0)/255.0);
 return ambient+mix(ground,sky,normal.y*0.5+0.5)*(atmosphere.sun.w*0.35);
}
fn source_finish(pbr:PbrInput,lit:vec4<f32>)->vec4<f32> {
 let color=source_aces(apply_haze(lit.rgb,pbr.world_position.xyz,pbr.N,view.world_position,true));
 return main_pass_post_lighting_processing(pbr,vec4<f32>(color,lit.a));
}
// Build the same microfacet input as Bevy's physical material path. Keeping
// direct evaluation independent of its view-light array avoids the WebGL2
// one-directional-light limit, while retaining metal, roughness and clearcoat.
fn source_lighting_input(pbr:PbrInput)->lighting::LightingInput {
 var input:lighting::LightingInput;
 let material=pbr.material;let nv=max(dot(pbr.N,pbr.V),0.0001);
 input.layers[0].N=pbr.N;input.layers[0].R=reflect(-pbr.V,pbr.N);input.layers[0].NdotV=nv;
 input.layers[0].perceptual_roughness=material.perceptual_roughness;
 input.layers[0].roughness=lighting::perceptualRoughnessToRoughness(material.perceptual_roughness);
 input.P=pbr.world_position.xyz;input.V=pbr.V;
 input.diffuse_color=calculate_diffuse_color(material.base_color.rgb,material.metallic,material.specular_transmission,material.diffuse_transmission);
 input.metallic=material.metallic;input.F0_dielectric=calculate_F0_dielectric(material.reflectance);input.F0_metallic=material.base_color.rgb;
 input.F_ab=lighting::F_AB(material.perceptual_roughness,nv);
 @if(STANDARD_MATERIAL_CLEARCOAT) {
  input.layers[1].N=pbr.clearcoat_N;input.layers[1].R=reflect(-pbr.V,pbr.clearcoat_N);
  input.layers[1].NdotV=max(dot(pbr.clearcoat_N,pbr.V),0.0001);
  input.layers[1].perceptual_roughness=material.clearcoat_perceptual_roughness;
  input.layers[1].roughness=lighting::perceptualRoughnessToRoughness(material.clearcoat_perceptual_roughness);
  input.clearcoat_strength=material.clearcoat;
 }
 @if(STANDARD_MATERIAL_ANISOTROPY) {
  input.anisotropy=pbr.anisotropy_strength;input.Ta=pbr.anisotropy_T;input.Ba=pbr.anisotropy_B;
 }
 return input;
}
fn source_directional(input:ptr<function,lighting::LightingInput>,direction:vec3<f32>)->vec3<f32> {
 var derived=lighting::derive_lighting_input((*input).layers[0].N,(*input).V,direction);
 let roughness=(*input).layers[0].roughness;
 let diffuse=(*input).diffuse_color*lighting::Fd_Burley(input,&derived);
 @if(STANDARD_MATERIAL_ANISOTROPY)
 let specular=lighting::specular_anisotropy(input,&derived,direction,roughness,1.0);
 @else
 let specular=lighting::specular(input,&derived,roughness,1.0);
 @if(STANDARD_MATERIAL_CLEARCOAT) {
  var coat=lighting::derive_lighting_input((*input).layers[1].N,(*input).V,direction);
  let c=lighting::specular_clearcoat(input,&coat,(*input).clearcoat_strength,(*input).layers[1].roughness,1.0);
  let inverse=1.0-c.r;
  return (diffuse+specular*inverse)*inverse*derived.NdotL+c.g*coat.NdotL;
 }
 @if(!STANDARD_MATERIAL_CLEARCOAT) {return (diffuse+specular)*derived.NdotL;}
}
fn source_direct(pbr:PbrInput)->vec3<f32> {
 var input=source_lighting_input(pbr);
 // Actual point direction per fragment, without inverse-square attenuation.
 let sun=normalize(atmosphere.sun.xyz-pbr.world_position.xyz);
 let primary=source_directional(&input,sun)*atmosphere.sun_color.rgb;
 let fill=source_directional(&input,vec3<f32>(0.0,1.0,0.0))*source_linear(vec3<f32>(255.0,241.0,212.0)/255.0)*3.2;
 return (primary+fill)*atmosphere.sun.w*pbr.directional_shadow_factor;
}
// Native lighting provides IBL and emissive; direct lights and the source
// ambient/hemisphere irradiance are evaluated once in source radiance units.
fn source_light(pbr:PbrInput,lit:vec4<f32>)->vec4<f32> {
 let diffuse=calculate_diffuse_color(pbr.material.base_color.rgb,pbr.material.metallic,pbr.material.specular_transmission,pbr.material.diffuse_transmission)*pbr.diffuse_occlusion/3.141592653589793;
 return vec4<f32>(lit.rgb+source_direct(pbr)+diffuse*source_ambient(pbr.N),lit.a);
}
fn source_pbr(pbr:PbrInput,lit:vec4<f32>)->vec4<f32> {return source_finish(pbr,source_light(pbr,lit));}
// The source foliage is MeshLambertMaterial: exact nonattenuating point light,
// directional fill, hemisphere, ambient and mapped emissive, without PBR IBL.
fn source_lambert(pbr:PbrInput)->vec4<f32> {
 let sun=normalize(atmosphere.sun.xyz-pbr.world_position.xyz);
 let direct=atmosphere.sun_color.rgb*max(dot(pbr.N,sun),0.0)*atmosphere.sun.w;
 let fill=source_linear(vec3<f32>(255.0,241.0,212.0)/255.0)*max(pbr.N.y,0.0)*atmosphere.sun.w*3.2;
 let diffuse=pbr.material.base_color.rgb/3.141592653589793;
 return source_finish(pbr,vec4<f32>(diffuse*(direct+fill+source_ambient(pbr.N))*pbr.diffuse_occlusion+pbr.material.emissive.rgb,pbr.material.base_color.a));
}
// Custom source ShaderMaterials already emit display RGB; preserve that path.
fn source_display(color:vec3<f32>,world:vec3<f32>,normal:vec3<f32>)->vec3<f32> {
 return source_linear(apply_haze(color,world,normal,view.world_position,false));
}
