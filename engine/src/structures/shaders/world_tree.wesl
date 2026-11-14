import openmars_engine::atmosphere::common::source_display;
import bevy_pbr::render::{forward_io::{Vertex,decompress_vertex},mesh_functions,view_transformations::position_world_to_clip,pbr_types::{pbr_input_new,STANDARD_MATERIAL_FLAGS_ALPHA_MODE_BLEND},pbr_functions::main_pass_post_lighting_processing};
import openmars_engine::structures::shaders::world_tree_shape::{params,curve,seedParam,snoise,getHeight};
struct TreeVertex {
 @builtin(position) position:vec4<f32>,@location(0) world_position:vec4<f32>,
 @location(1) world_normal:vec3<f32>,@location(2) uv:vec2<f32>,
 @location(3) extra:vec4<f32>,@location(4) center:vec3<f32>
}
fn rotate(v:vec3<f32>,axis:vec3<f32>,angle:f32)->vec3<f32> {
 let c=cos(angle);return v*c+cross(axis,v)*sin(angle)+axis*dot(axis,v)*(1.0-c);
}
@vertex fn vertex(input:Vertex)->TreeVertex {
 let v=decompress_vertex(input,input.instance_index);let model=mesh_functions::get_world_from_local(input.instance_index);
 let center=model[3].xyz-normalize(model[2].xyz)*params.geometry.x;
 var world_position=mesh_functions::mesh_position_local_to_world(model,vec4<f32>(v.position,1.0)).xyz;
 let elapsed=max(0.0,params.clock.x-params.geometry.w)*1000.0;
 let amplitude=0.006*(1.0-params.clock.y)*params.sun_color.w;
 world_position+=model[0].xyz*sin(elapsed*0.137+params.geometry.z)*amplitude+model[1].xyz*sin(elapsed*0.173+params.geometry.z*1.3)*amplitude;
 let sphere_dir=normalize(world_position-center);let centered=(v.uv-vec2<f32>(0.5))*2.0;
 let emergence=params.clock.y;let h=getHeight(centered)*emergence;
 let eps=0.008;
 let dhdx=(getHeight(centered+vec2<f32>(eps,0.0))-getHeight(centered-vec2<f32>(eps,0.0)))*emergence/(2.0*eps);
 let dhdy=(getHeight(centered+vec2<f32>(0.0,eps))-getHeight(centered-vec2<f32>(0.0,eps)))*emergence/(2.0*eps);
 var right=normalize(cross(vec3<f32>(0.0,1.0,0.0),sphere_dir));
 if abs(sphere_dir.y)>=0.99 {right=normalize(cross(vec3<f32>(1.0,0.0,0.0),sphere_dir));}
 let fwd=cross(sphere_dir,right);var out:TreeVertex;
 out.uv=v.uv;out.extra=vec4<f32>(length(centered),h,params.clock.y,params.clock.w);out.center=center;
 out.world_normal=normalize(sphere_dir*0.15-right*dhdx-fwd*dhdy);
 var projected=center+sphere_dir*(params.geometry.x+0.003+h);
 if params.clock.z>0.0 {
   let axis=normalize(model[0].xyz*params.destruction.x+model[1].xyz*params.destruction.y);
   projected=model[3].xyz+rotate(projected-model[3].xyz,axis,params.destruction.z)-normalize(model[2].xyz)*params.destruction.w;
   out.world_normal=rotate(out.world_normal,axis,params.destruction.z);
 }
 out.world_position=vec4<f32>(projected,1.0);
 out.position=position_world_to_clip(out.world_position.xyz);return out;
}
@fragment fn fragment(in:TreeVertex)->@location(0) vec4<f32> {
  if in.extra.w<=0.0 {discard;}

  var centered= (in.uv - vec2<f32>(0.5)) * 2.0;
  var rawDist= in.extra.x;

  // === Per-tile parameters (must match vertex shader) ===
  var canopyH= params.geometry.y * (0.85 + seedParam(0.0) * 0.3);
  var canopyRad= 0.65 + seedParam(1.0) * 0.15;
  var centerOff= (vec2<f32>(seedParam(2.0), seedParam(3.0)) - vec2<f32>(0.5)) * 0.04;
  var seedOff= vec2<f32>(seedParam(5.0), seedParam(6.0)) * 100.0;

  var w= centered + centerOff;
  var dist= length(w);
  var normHeight= clamp(in.extra.y / canopyH, 0.0, 1.0);

  // === Canopy edge (must match vertex shader) ===
  var edgeNoise= snoise(w * 3.5 + seedOff + vec2<f32>(21.0));
  var edgeWobble= edgeNoise * 0.06;
  var edgeDist= dist - (canopyRad + edgeWobble);
  var canopyMask= curve(0.0, -0.15, edgeDist);

  // === Slope ===
  var n= normalize(in.world_normal);
  var up= normalize(in.world_position.xyz-in.center);
  var slope= 1.0 - abs(dot(n, up));

  // === Foliage colors — rich greens with variation ===
  var deepGreen= vec3<f32>(0.05, 0.20, 0.02);
  var brightGreen= vec3<f32>(0.15, 0.45, 0.05);
  var sunlitGreen= vec3<f32>(0.30, 0.55, 0.08);
  var shadowGreen= vec3<f32>(0.03, 0.12, 0.02);

  // Leaf cluster noise at multiple scales
  var leafN1= snoise(w * 8.0 + seedOff) * 0.5 + 0.5;
  var leafN2= snoise(w * 16.0 + seedOff + vec2<f32>(7.3)) * 0.5 + 0.5;
  var leafN3= snoise(w * 32.0 + seedOff + vec2<f32>(14.1)) * 0.5 + 0.5;
  var leafPattern= leafN1 * 0.5 + leafN2 * 0.3 + leafN3 * 0.2;

  // Blend greens based on leaf pattern and height
  var foliageColor= mix(deepGreen, brightGreen, leafPattern);
  foliageColor = mix(foliageColor, sunlitGreen, normHeight * 0.6);
  foliageColor = mix(foliageColor, shadowGreen, (1.0 - leafPattern) * 0.3);

  // === Branch structure visible through canopy (subtle dark lines) ===
  var angle= atan2(w.y,w.x);
  var branchFreq= 5.0 + seedParam(4.0) * 3.0;
  var branchLine= abs(sin(angle * branchFreq + seedParam(8.0) * 6.28));
  branchLine = pow(branchLine, 6.0);
  var branchRadial= curve(0.05, 0.15, dist) * curve(canopyRad * 0.9, 0.2, dist);
  var branchMask= branchLine * branchRadial;

  // Secondary smaller branches
  var subBranch= abs(sin(angle * branchFreq * 2.5 + seedParam(9.0) * 6.28 + dist * 8.0));
  subBranch = pow(subBranch, 8.0);
  var subBranchMask= subBranch * curve(0.2, 0.35, dist) * curve(canopyRad * 0.85, 0.3, dist);
  branchMask = max(branchMask * 0.35, subBranchMask * 0.15);

  // Darken foliage where branches are visible underneath
  var branchColor= vec3<f32>(0.08, 0.14, 0.03);
  foliageColor = mix(foliageColor, branchColor, branchMask);

  // === Trunk peek — tiny brown center visible through canopy gap ===
  var trunkPeek= curve(0.08, 0.02, dist);
  var trunkColor= vec3<f32>(0.25, 0.15, 0.06);
  foliageColor = mix(foliageColor, trunkColor, trunkPeek * 0.4);

  // === Ground/moss around canopy edge ===
  var mossColor= vec3<f32>(0.10, 0.25, 0.05);
  var groundColor= vec3<f32>(0.15, 0.12, 0.07);
  var mossN= snoise(centered * 10.0 + seedOff + vec2<f32>(15.7)) * 0.5 + 0.5;
  var outerColor= mix(groundColor, mossColor, mossN);
  var outerMask= curve(-0.05, 0.1, edgeDist);

  // Final surface = canopy inside, ground outside
  var surfaceColor= mix(foliageColor, outerColor, outerMask);

  // === Magical glow — energy pulsing through the canopy ===
  var veinN1= snoise(w * 5.0 + seedOff * 0.1 + vec2<f32>(params.clock.x * 0.06, -params.clock.x * 0.04));
  var veinN2= snoise(w * 10.0 + seedOff * 0.1 + vec2<f32>(-params.clock.x * 0.05, params.clock.x * 0.07) + vec2<f32>(13.7));
  var vein= curve(0.35, 0.65, veinN1 * 0.6 + veinN2 * 0.4);
  vein *= canopyMask;

  // Radial energy lines following branch structure
  var energyLine= pow(branchLine, 3.0) * branchRadial;
  var glowMask= max(vein * 0.3, energyLine * 0.5);

  // Crown glow at top
  var crownGlow= curve(0.25, 0.03, dist) * normHeight;

  glowMask += crownGlow * 0.4;

  // Pulsing animation
  var pulse= 0.6 + 0.4 * sin(params.clock.x * 0.7 + dist * 5.0);
  var pulse2= 0.7 + 0.3 * sin(params.clock.x * 1.1 + angle * 2.0 + 1.8);
  var combinedPulse= pulse * 0.6 + pulse2 * 0.4;

  // Glow color: golden-green energy
  var glowInner= vec3<f32>(0.5, 1.0, 0.3);
  var glowOuter= vec3<f32>(0.9, 0.8, 0.2);
  var glowBlend= curve(0.1, 0.5, dist);
  var glowColor= mix(glowInner, glowOuter, glowBlend);

  // === Lighting ===
  var sunDir= normalize(params.sun.xyz);
  var NdotL= dot(n, sunDir);
  var wrapDiffuse= NdotL * 0.5 + 0.5;
  wrapDiffuse *= wrapDiffuse;
  var lighting= vec3<f32>(0.35) + params.sun_color.xyz * wrapDiffuse * 0.65 * params.sun.w;

  // Subsurface scattering approximation for leaves
  var sss= max(0.0, -dot(n, sunDir)) * 0.15;
  lighting += vec3<f32>(0.1, 0.3, 0.05) * sss;

  // AO: darken canopy underside/edges
  var canopyAO= mix(0.6, 1.0, normHeight);
  var leafAO= 0.85 + leafPattern * 0.15;
  var ao= canopyAO * leafAO;

  // === Combine ===
  var color= surfaceColor * lighting * ao;

  // Emissive glow
  var emissiveFade= curve(0.3, 0.8, in.extra.z);
  var glowIntensity= glowMask * combinedPulse * emissiveFade;
  color += glowColor * glowIntensity * 0.45;

  // Canopy rim light — bright green edge catch
  var rimAngle= 1.0 - max(0.0, dot(n, sunDir));
  var rimMask= pow(rimAngle, 3.0) * canopyMask * normHeight;
  color += vec3<f32>(0.1, 0.3, 0.05) * rimMask * 0.4 * emissiveFade;

  // Edge fade
  var edgeAlpha= curve(1.0, 0.85, rawDist);

  if edgeAlpha<=0.001 {discard;}
  var pbr=pbr_input_new();pbr.frag_coord=in.position;pbr.world_position=in.world_position;pbr.world_normal=n;
  pbr.material.flags=STANDARD_MATERIAL_FLAGS_ALPHA_MODE_BLEND;
  // Legacy ShaderMaterial explicitly emits display colours (see addPlanetHaze
  // linearColor=false). Decode once before Bevy's sRGB attachment encodes them.
  let linear=source_display(color,in.world_position.xyz,n);
  return main_pass_post_lighting_processing(pbr,vec4<f32>(linear,edgeAlpha*in.extra.w));

}
