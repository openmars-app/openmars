import openmars_engine::structures::special::special_shape::{params,curve,snoise,seedParam,atlas_color,flow_value};
fn volcano_color(uv:vec2<f32>,extra:vec4<f32>,world_normal:vec3<f32>,world_position:vec3<f32>,body_center:vec3<f32>)->vec4<f32> {
  var centered= (uv - vec2<f32>(0.5)) * 2.0;
  var rawDist= extra.x;

  // === Same per-tile parameters as vertex shader ===
  var coneHeight= params.profile.x * (0.85 + seedParam(0.0) * 0.3);
  var craterRad= params.profile.y * (0.85 + seedParam(1.0) * 0.3);
  var craterOff= (vec2<f32>(seedParam(5.0), seedParam(6.0)) - vec2<f32>(0.5)) * 0.05;
  var ellipX= 0.9 + seedParam(7.0) * 0.2;
  var ellipY= 0.9 + seedParam(8.0) * 0.2;
  var coneOff= (vec2<f32>(seedParam(9.0), seedParam(10.0)) - vec2<f32>(0.5)) * 0.08;
  var gullyFreq= 5.0 + seedParam(11.0) * 4.0;
  var seedOff= vec2<f32>(seedParam(12.0), seedParam(13.0)) * 100.0;

  // === Domain warp (must match vertex shader) ===
  var wN1= snoise(centered * 1.8 + seedOff);
  var wN2= snoise(centered * 1.8 + seedOff + vec2<f32>(50.0, 50.0));
  var rawDistF= length(centered);
  var warpMask= curve(0.12, 0.35, rawDistF);
  var w= centered + coneOff + vec2<f32>(wN1, wN2) * (0.12 * warpMask);

  // === Shape field (must match vertex shader) ===
  var s1= snoise(w * 0.7 + seedOff * 0.05);
  var s2= snoise(w * 0.35 + seedOff * 0.05 + vec2<f32>(19.1));
  var shape= s1 * 0.7 + s2 * 0.3;
  var dist= length(w) * (1.0 + shape * 0.18);

  // === Slope and height for material blending ===
  var n= normalize(world_normal);
  var up= normalize((world_position-body_center));
  var slope= 1.0 - abs(dot(n, up));
  var normHeight= clamp(extra.y / coneHeight, 0.0, 1.0);

  // === Gully pattern (must match vertex shader — XY space, no angle) ===
  var g1= snoise(w * gullyFreq + seedOff * 0.05);
  var g2= snoise(w * (gullyFreq * 2.0) + seedOff * 0.05 + vec2<f32>(31.7));
  var gridge= 1.0 - abs(g1 * 0.8 + g2 * 0.2);
  gridge = pow(gridge, 2.0);
  var gs= curve(0.18, 0.32, dist) * curve(0.88, 0.58, dist);
  var gullyField= gridge * gs;

  // === Rock material: 3 layers by height and slope ===
  var basalt= vec3<f32>(0.22, 0.08, 0.05);
  var ashRock= vec3<f32>(0.40, 0.16, 0.10);
  var dustRock= vec3<f32>(0.52, 0.25, 0.14);

  var rockN1= snoise(centered * 10.0 + seedOff + vec2<f32>(5.3)) * 0.5 + 0.5;
  var rockN2= snoise(centered * 20.0 + seedOff + vec2<f32>(9.7)) * 0.5 + 0.5;
  var rockNoise= rockN1 * 0.65 + rockN2 * 0.35;

  var rockColor= mix(basalt, ashRock, curve(0.0, 0.5, normHeight));
  rockColor = mix(rockColor, dustRock, curve(0.5, 0.9, normHeight));
  rockColor *= mix(0.75, 1.0, 1.0 - slope * 0.5);
  rockColor *= 0.85 + rockNoise * 0.3;
  rockColor *= 1.0 - gullyField * 0.3;

  // === Crater geometry ===
  var cp= (w - craterOff) * vec2<f32>(ellipX, ellipY);
  var craterDist= length(cp);

  // === Flow map: valley-seeking lava from CPU computation ===
  var flowUV= uv;
  var flowVal= flow_value(flowUV);

  // Threshold flow into lava mask (hot core + crust edge)
  var lavaMask= curve(0.45, 0.80, flowVal);
  var crustMask= curve(0.30, 0.50, flowVal) * (1.0 - lavaMask);

  // Lava only outside crater bowl
  var outsideCrater= curve(craterRad * 0.7, craterRad * 1.1, craterDist);
  lavaMask *= outsideCrater;
  crustMask *= outsideCrater;

  // Subtle turbulence inside lava (no zebra stripes)
  var lavaTurb= snoise(centered * 8.0 + vec2<f32>(params.clock.x * 0.2, -params.clock.x * 0.15) + seedOff) * 0.15;
  lavaMask = clamp(lavaMask + lavaTurb * lavaMask, 0.0, 1.0);

  // Lava colors
  var lavaPulse= 0.85 + 0.15 * sin(params.clock.x * 1.5 + dist * 3.0);
  var hotLava= mix(vec3<f32>(1.0, 0.3, 0.0), vec3<f32>(1.0, 0.7, 0.1), lavaPulse * 0.5) * 2.0;
  var crustColor= vec3<f32>(0.15, 0.08, 0.05);

  // === Crater magma ===
  var craterMask= 1.0 - curve(0.0, craterRad * 0.8, craterDist);
  craterMask *= craterMask;
  var magmaPulse= 0.6 + 0.4 * sin(params.clock.x * 1.2);
  var magmaDetail= snoise(centered * 6.0 + vec2<f32>(params.clock.x * 0.3, -params.clock.x * 0.2) + seedOff) * 0.5 + 0.5;
  var magmaColor= mix(vec3<f32>(1.0, 0.25, 0.0), vec3<f32>(1.0, 0.8, 0.15), magmaDetail * magmaPulse);

  // === Grass at base/apron only ===
  var grassColor= atlas_color(uv * 3.0, 5).rgb * vec3<f32>(0.55, 0.62, 0.48);
  var grassEdge= snoise(centered * 6.0 + seedOff + vec2<f32>(7.7)) * 0.08;
  var grassMask= curve(0.55, 0.85 + grassEdge, rawDist) * params.climate.x;

  // === Snow creeping up from the base: patchy like the ground frost, gone towards the hot peak ===
  var snowPatch= snoise(centered * 9.0 + seedOff + vec2<f32>(3.1)) * 0.5 + 0.5;
  var snowLine= 0.22 + snoise(centered * 4.0 + seedOff + vec2<f32>(11.3)) * 0.06;
  var snowThreshold= 1.0 - params.climate.y * 0.8;
  var snowMask= curve(snowThreshold, snowThreshold + 0.3, snowPatch)
                 * (1.0 - curve(snowLine - 0.1, snowLine, normHeight))
                 * (1.0 - grassMask);
  var snowColor= vec3<f32>(0.9, 0.94, 1.0);

  // === Lighting: wrap diffuse + AO ===
  var sunDir= normalize(params.sun.xyz);
  var NdotL= dot(n, sunDir);
  var wrapDiffuse= NdotL * 0.5 + 0.5;
  wrapDiffuse *= wrapDiffuse;
  var lighting= vec3<f32>(0.3) + params.sun_color.rgb * wrapDiffuse * 0.7 * params.sun.w;

  // AO: darken crater interior, gully concavities, and ridge crests brightened
  var craterAO= mix(0.4, 1.0, curve(0.0, craterRad * 1.2, craterDist));
  var gullyAO= 1.0 - gullyField * 0.25;
  var slopeAO= 1.0 + slope * 0.15;  // slightly brighten steep ridges
  var ao= craterAO * gullyAO * slopeAO;

  // === Combine ===
  var surfaceColor= mix(rockColor, grassColor, grassMask);
  surfaceColor = mix(surfaceColor, snowColor, snowMask);
  var color= surfaceColor * lighting * ao;

  // Scale emissive effects by emergence (lava fades in as volcano rises)
  var emissiveFade= curve(0.3, 0.8, extra.z);

  // Lava crust (dark cooled edges)
  color = mix(color, crustColor, crustMask * 0.8 * emissiveFade);

  // Hot lava (emissive, ignores lighting)
  color = mix(color, hotLava, lavaMask * 0.9 * emissiveFade);

  // Crater magma (strongest emissive)
  color = mix(color, magmaColor * 2.5, craterMask * emissiveFade);

  // Rim glow
  var rimGlow= curve(craterRad * 1.6, craterRad, craterDist)
                * (1.0 - curve(0.0, craterRad * 0.8, craterDist));
  color += vec3<f32>(1.0, 0.3, 0.0) * rimGlow * 0.3 * magmaPulse * emissiveFade;

  // Edge fade for soft blending with ground
  var edgeAlpha= curve(1.0, 0.85, rawDist);

  // === Debug views ===
  if (params.climate.z == 1.0) {
    // Height (grayscale)
    var hNorm= clamp(extra.y / (params.profile.x * 1.2), -0.2, 1.0) * 0.5 + 0.5;
    return vec4<f32>(vec3<f32>(hNorm), edgeAlpha);

  }
  if (params.climate.z == 2.0) {
    // Slope
    return vec4<f32>(vec3<f32>(slope), edgeAlpha);

  }
  if (params.climate.z == 3.0) {
    // Gully field
    return vec4<f32>(vec3<f32>(gullyField), edgeAlpha);

  }
  if (params.climate.z == 4.0) {
    // Flow map
    return vec4<f32>(vec3<f32>(flowVal), edgeAlpha);

  }

  return vec4<f32>(color, edgeAlpha);

}
