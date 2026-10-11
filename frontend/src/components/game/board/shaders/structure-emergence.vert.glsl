attribute float structureBirth;
attribute float structureEmergenceDepth;
attribute float structureFrostExposure;
varying vec3 vStructureFrostPosition;
varying vec3 vStructureFrostNormal;
varying float vStructureFrostExposure;
uniform float uStructureTime;
uniform float uStructureDuration;
uniform float uNuclearPlaybackRate;
attribute float structureCollapseStart;
attribute vec3 structureCollapsePivot;
attribute vec4 structureCollapseMotion;
float structureCollapseProgress() {
  float t = clamp(((uStructureTime-structureCollapseStart)*uNuclearPlaybackRate-structureCollapseMotion.x)/0.75,0.0,1.0);
  return t*t;
}
mat3 structureCollapseRotation() {
  vec2 tilt = structureCollapseMotion.yz * structureCollapseProgress();
  float cx=cos(tilt.x), sx=sin(tilt.x), cy=cos(tilt.y), sy=sin(tilt.y);
  return mat3(cy,0.0,-sy, sy*sx,cx,cy*sx, sy*cx,-sx,cy*cx);
}
//#pragma body
#include <begin_vertex>
vStructureFrostPosition=position;
vStructureFrostNormal=normal;
vStructureFrostExposure=structureFrostExposure;
float structureProgress=clamp((uStructureTime-structureBirth)/uStructureDuration,0.0,1.0);
transformed.z-=structureEmergenceDepth*pow(1.0-structureProgress,3.0);
if (uStructureTime >= structureCollapseStart) {
  transformed = structureCollapsePivot + structureCollapseRotation() * (transformed-structureCollapsePivot);
  transformed.z -= structureCollapseMotion.w * structureCollapseProgress();
}
