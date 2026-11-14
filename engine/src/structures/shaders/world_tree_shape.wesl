// Seeded height/noise arithmetic ported from original world-tree GLSL.
struct Parameters {geometry:vec4<f32>,clock:vec4<f32>,destruction:vec4<f32>,sun:vec4<f32>,sun_color:vec4<f32>,ground_destruction:vec4<f32>}
@group(3) @binding(0) var<uniform> params:Parameters;
// Explicitly defined reversed-edge curves preserve the intended original masks.
fn curve(a:f32,b:f32,x:f32)->f32 {let t=clamp((x-a)/(b-a),0.0,1.0);return t*t*(3.0-2.0*t);}
fn hash(n:f32)->f32 { return fract(sin(n) * 43758.5453123); }
fn seedParam(idx:f32)->f32 { return hash(params.geometry.z * 127.1 + idx * 311.7); }

fn mod289_v3(x:vec3<f32>)->vec3<f32> { return x - floor(x * (1.0 / 289.0)) * 289.0; }
fn mod289_v2(x:vec2<f32>)->vec2<f32> { return x - floor(x * (1.0 / 289.0)) * 289.0; }
fn permute_v(x:vec3<f32>)->vec3<f32> { return mod289_v3(((x * 34.0) + vec3<f32>(1.0)) * x); }

fn snoise(v:vec2<f32>)->f32 {
  let C= vec4<f32>(0.211324865405187, 0.366025403784439,
                      -0.577350269189626, 0.024390243902439);
  var i= floor(v + vec2<f32>(dot(v, C.yy)));
  var x0= v - i + vec2<f32>(dot(i, C.xx));
  var i1= select(vec2<f32>(0.0,1.0),vec2<f32>(1.0,0.0),x0.x>x0.y);
  var x12= x0.xyxy + C.xxzz;
  x12 = vec4<f32>(x12.xy - i1, x12.zw);
  i = mod289_v2(i);
  var p= permute_v(permute_v(vec3<f32>(i.y) + vec3<f32>(0.0, i1.y, 1.0)) + vec3<f32>(i.x) + vec3<f32>(0.0, i1.x, 1.0));
  var m= max(vec3<f32>(0.5) - vec3<f32>(dot(x0, x0), dot(x12.xy, x12.xy), dot(x12.zw, x12.zw)), vec3<f32>(0.0));
  m = m * m;
  m = m * m;
  var x_= 2.0 * fract(p * C.www) - vec3<f32>(1.0);
  var h= abs(x_) - vec3<f32>(0.5);
  var ox= floor(x_ + vec3<f32>(0.5));
  var a0= x_ - ox;
  m *= vec3<f32>(1.79284291400159) - 0.85373472095314 * (a0 * a0 + h * h);
  var g:vec3<f32>;
  g.x = a0.x * x0.x + h.x * x0.y;
  g = vec3<f32>(g.x, a0.yz * x12.xz + h.yz * x12.yw);
  return 130.0 * dot(m, g);
}

fn getHeight(centered:vec2<f32>)->f32 {
  var canopyH= params.geometry.y * (0.85 + seedParam(0.0) * 0.3);
  var canopyRad= 0.65 + seedParam(1.0) * 0.15;
  var centerOff= (vec2<f32>(seedParam(2.0), seedParam(3.0)) - vec2<f32>(0.5)) * 0.04;
  var seedOff= vec2<f32>(seedParam(5.0), seedParam(6.0)) * 100.0;

  var w= centered + centerOff;
  var dist= length(w);

  // === 1. Canopy dome — broad smooth hemisphere ===
  var canopyT= dist / canopyRad;
  var dome= max(0.0, 1.0 - canopyT * canopyT);
  dome = sqrt(dome);
  var h= dome * canopyH;

  // === 2. Leaf cluster bumps — organic canopy surface ===
  var leafN1= snoise(w * 8.0 + seedOff);
  var leafN2= snoise(w * 16.0 + seedOff + vec2<f32>(7.3));
  var leafN3= snoise(w * 28.0 + seedOff + vec2<f32>(14.1));
  var leafBumps= leafN1 * 0.5 + leafN2 * 0.3 + leafN3 * 0.2;
  var canopyMask= curve(canopyRad, canopyRad * 0.2, dist);
  h += leafBumps * canopyMask * 0.012;

  // === 3. Canopy edge irregularity — not a perfect circle ===
  var edgeNoise= snoise(w * 3.5 + seedOff + vec2<f32>(21.0));
  var edgeWobble= edgeNoise * 0.06;
  var edgeDist= dist - (canopyRad + edgeWobble);
  var edgeFalloff= curve(0.0, -0.15, edgeDist);
  h *= edgeFalloff;

  // === 4. Central peak — trunk crown pushing up ===
  var peakRad= 0.12 + seedParam(7.0) * 0.05;
  var peak= exp(-dist * dist / (peakRad * peakRad)) * canopyH * 0.25;
  h += peak * canopyMask;

  // === 5. Ground moss around canopy base ===
  var groundN= snoise(centered * 4.0 + seedOff + vec2<f32>(9.1));
  var groundMask= curve(canopyRad * 0.7, canopyRad * 1.2, dist)
                   * curve(1.0, canopyRad * 0.9, dist);
  h += groundN * groundMask * 0.002;

  return h;
}

