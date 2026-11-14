import openmars_engine::structures::special::special_shape::{params,curve,snoise,seedParam,atlas_color,flow_value};
fn mohole_color(uv:vec2<f32>,extra:vec4<f32>,world_normal:vec3<f32>,world_position:vec3<f32>,body_center:vec3<f32>)->vec4<f32> {
 var centered=(uv-vec2<f32>(0.5))*2.0;

  var dist= extra.x;
  var seedOff= vec2<f32>(seedParam(0.0), seedParam(1.0)) * 100.0;

  // Extend past hole edge to overlap with flat rim (avoids gap from sphere projection)
  var effectiveRadius= params.profile.y * params.clock.w;


  var n= normalize(world_normal);
  var sunDir= normalize(params.sun.xyz);
  var maxDepth= params.profile.x * extra.z;

  // Visual depth based on distance from hole edge (for texturing, not geometry)
  var depthRatio= clamp(1.0 - dist / effectiveRadius, 0.0, 1.0);

  // === Hole walls — concrete building blocks ===
  var NdotL= dot(n, sunDir);
  var wrapDiffuse= NdotL * 0.5 + 0.5;
  wrapDiffuse *= wrapDiffuse;
  var lighting= vec3<f32>(0.2) + params.sun_color.rgb * wrapDiffuse * 0.7 * params.sun.w;

  // Cylindrical unwrap: angle for horizontal, vertex height for vertical
  var angle= atan2(centered.y, centered.x);
  var u= angle / 6.283 + 0.5;
  var v= clamp(-extra.y / (maxDepth + 0.0001), 0.0, 1.0);

  // Block grid — rows and columns of concrete panels
  var blockCols= 12.0;
  var blockRows= 3.0;
  var colCell= fract(u * blockCols);
  var rowCell= fract(v * blockRows);

  // Mortar lines (thin dark gaps between blocks)
  var mortarW= 0.05;
  var hMortar= curve(0.0, mortarW, colCell) * curve(0.0, mortarW, 1.0 - colCell);
  var vMortar= curve(0.0, mortarW, rowCell) * curve(0.0, mortarW, 1.0 - rowCell);
  var mortar= hMortar * vMortar;

  // Concrete texture per block
  var wallUV= vec2<f32>(u * blockCols, v * blockRows);
  var concreteSample= atlas_color(wallUV * 0.5 + seedOff * 0.01, 8).rgb;

  // Concrete texture — match ground darkness
  var blockColor= concreteSample * mix(vec3<f32>(0.6, 0.56, 0.52), vec3<f32>(0.45, 0.42, 0.39), v * 0.5);
  var mortarColor= vec3<f32>(0.3, 0.28, 0.26);

  var wallColor= mix(mortarColor, blockColor, mortar);
  wallColor *= lighting;

  // AO: fade to black deeper in the shaft
  wallColor *= mix(1.0, 0.0, pow(v, 1.5));

  // Subtle heat haze near top
  var shimmer= sin(params.clock.x * 2.0 + snoise(centered * 4.0 + seedOff) * 3.0) * 0.5 + 0.5;
  var shimmerMask= (1.0 - depthRatio) * 0.3;
  var emissiveFade= curve(0.3, 0.8, extra.z);
  wallColor += vec3<f32>(0.3, 0.12, 0.03) * shimmer * shimmerMask * 0.06 * emissiveFade;

  // Faint orange glow from the center depths, randomly breathing
  var deepMask= curve(0.6, 1.0, depthRatio);
  var breath1= snoise(vec2<f32>(params.clock.x * 0.6, params.geometry.y * 3.0));
  var breath2= snoise(vec2<f32>(params.clock.x * 0.3, params.geometry.y * 7.0 + 50.0));
  var breath= 0.5 + 0.3 * breath1 + 0.2 * breath2;
  var glowRadius= effectiveRadius * (0.25 + 0.25 * breath);
  var centerMask= 1.0 - curve(0.0, glowRadius, dist);
  var glowIntensity= centerMask * deepMask * (0.6 + 0.4 * breath) * emissiveFade;
  wallColor += vec3<f32>(1.0, 0.4, 0.08) * glowIntensity;

  return vec4<f32>(wallColor, select(0.0,1.0,dist<effectiveRadius+0.1));

}
