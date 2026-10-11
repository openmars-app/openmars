export const SPHERE_RADIUS = 2.02;
export const STRUCTURE_EMERGENCE_DURATION = 1.12;
export const CHROME_Z_BASE = 0.0156;
export const MOHOLE_STENCIL_BIT = 1;
export const LAKE_STENCIL_BIT = 2;
export const NUCLEAR_STENCIL_BIT = 4;

export const VENUS_RADIUS = 2.02;
export const VENUS_POSITION = [-45, 25, -148] as const;

export const ORBITAL_STATION_ORBIT_RADIUS = 3.0;
export const ORBITAL_STATION_ORBIT_SPEED = 0.08;
export const ORBITAL_STATION_TILT = 0.3;

export function easeOutCubic(t: number): number {
  return 1 - (1 - t) * (1 - t) * (1 - t);
}
