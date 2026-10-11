#include <beginnormal_vertex>
if (uStructureTime >= structureCollapseStart) {
objectNormal = structureCollapseRotation() * objectNormal;
#ifdef USE_TANGENT
  objectTangent = structureCollapseRotation() * objectTangent;
#endif

}
