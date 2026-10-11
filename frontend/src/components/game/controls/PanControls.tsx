import { useMemo, useRef, useEffect, useState } from "react";
import { useThree, useFrame } from "@react-three/fiber";
import * as THREE from "three";
import { useWorld3DSettings } from "../../../contexts/World3DSettingsContext";
import { usePlanetFocus } from "../../../contexts/PlanetFocusContext";
import {
  getPlanetCameraDefaultSpherical,
  getPlanetOrbit,
  getMarsOrbitalPosition,
} from "../board/solarSystemConfig";
import {
  MAX_POLAR_ANGLE,
  MIN_POLAR_ANGLE,
  computePlanetCenter,
  getBodyRadius,
  getOrbitLimits,
  getSurfaceDistance,
  sphericalToWorldOffset,
} from "./orbitFrame";
import { audioService } from "../../../services/audioService";
import { slopFor } from "./tapGesture";
import { useCameraFocusStore } from "@/stores/cameraFocusStore.ts";
import { useTileInspectStore } from "@/stores/tileInspectStore.ts";

export const panState = { isPanning: false, lastGestureWasDrag: false };

export function isDragClick(): boolean {
  return panState.lastGestureWasDrag;
}

const MOUSE_ORBIT_SPEED = 0.0003;
const WHEEL_ZOOM_SPEED = 0.005;
// Upper bound for a full-width one-finger swipe: roughly the width of the Mars board
const TOUCH_MAX_SWIPE_ARC = 1.5;
const MAJOR_RESIZE_AREA_CHANGE = 0.3;
const FOCUS_DURATION = 0.4;

interface FocusAnimation {
  start: number;
  fromTheta: number;
  fromPhi: number;
  fromRadius: number;
  toTheta: number;
  toPhi: number;
  toRadius: number;
}

function nearestEquivalentAngle(from: number, to: number): number {
  const turn = Math.PI * 2;
  const delta = ((((to - from) % turn) + turn * 1.5) % turn) - Math.PI;
  return from + delta;
}

interface TrackedPointer {
  x: number;
  y: number;
  originX: number;
  originY: number;
}

function measurePointers(pointers: Map<number, TrackedPointer>) {
  let x = 0;
  let y = 0;
  for (const pointer of pointers.values()) {
    x += pointer.x;
    y += pointer.y;
  }
  const count = Math.max(pointers.size, 1);
  x /= count;
  y /= count;
  let spread = 0;
  if (pointers.size >= 2) {
    for (const pointer of pointers.values()) {
      spread += Math.hypot(pointer.x - x, pointer.y - y);
    }
    spread /= pointers.size;
  }
  return { x, y, spread };
}

function isMajorResize(
  previous: { width: number; height: number },
  next: { width: number; height: number },
): boolean {
  const wasPortrait = previous.height > previous.width;
  const isPortrait = next.height > next.width;
  if (wasPortrait !== isPortrait) {
    return true;
  }
  const previousArea = previous.width * previous.height;
  if (previousArea <= 0) {
    return true;
  }
  return (
    Math.abs(next.width * next.height - previousArea) / previousArea > MAJOR_RESIZE_AREA_CHANGE
  );
}

function getVerticalFov(camera: THREE.Camera): number {
  return camera instanceof THREE.PerspectiveCamera ? camera.fov : 50;
}

export function PanControls() {
  const { camera, gl, size } = useThree();
  const {
    settings,
    storedCameraState,
    setStoredCameraState,
    cameraStateRef,
    pendingCameraTransformRef,
  } = useWorld3DSettings();
  const { activePlanet } = usePlanetFocus();
  const previousSize = useRef({ width: size.width, height: size.height });

  const marsInitPos = getMarsOrbitalPosition(0);
  const marsOrbit = getPlanetOrbit("mars");

  const [spherical] = useState(() => {
    return new THREE.Spherical(marsOrbit.defaultRadius, Math.PI / 2, 0);
  });
  const targetSpherical = useRef(new THREE.Spherical(marsOrbit.defaultRadius, Math.PI / 2, 0));

  const orbitCenter = useRef(new THREE.Vector3(marsInitPos[0], marsInitPos[1], marsInitPos[2]));
  const lastPlanetRef = useRef(activePlanet);
  const activePlanetRef = useRef(activePlanet);
  activePlanetRef.current = activePlanet;

  const wasFreeCameraEnabled = useRef(settings.freeCameraEnabled);
  const focusAnimation = useRef<FocusAnimation | null>(null);
  const focusSpherical = useMemo(() => new THREE.Spherical(), []);
  const focusDirection = useMemo(() => new THREE.Vector3(), []);

  // Reusable temp vectors
  const _toSun = useMemo(() => new THREE.Vector3(), []);
  const _quat = useMemo(() => new THREE.Quaternion(), []);
  const _zAxis = useMemo(() => new THREE.Vector3(0, 0, 1), []);
  const _offset = useMemo(() => new THREE.Vector3(), []);
  const _camOffset = useMemo(() => new THREE.Vector3(), []);

  useFrame((state) => {
    // --- Free camera toggle ---
    if (settings.freeCameraEnabled && !wasFreeCameraEnabled.current) {
      setStoredCameraState({
        position: { x: camera.position.x, y: camera.position.y, z: camera.position.z },
        spherical: { radius: spherical.radius, phi: spherical.phi, theta: spherical.theta },
      });
      wasFreeCameraEnabled.current = true;
    } else if (!settings.freeCameraEnabled && wasFreeCameraEnabled.current) {
      if (storedCameraState) {
        spherical.radius = storedCameraState.spherical.radius;
        spherical.phi = storedCameraState.spherical.phi;
        spherical.theta = storedCameraState.spherical.theta;
        targetSpherical.current.radius = storedCameraState.spherical.radius;
        targetSpherical.current.phi = storedCameraState.spherical.phi;
        targetSpherical.current.theta = storedCameraState.spherical.theta;
      }
      wasFreeCameraEnabled.current = false;
    }

    if (settings.freeCameraEnabled) {
      cameraStateRef.current = {
        position: { x: camera.position.x, y: camera.position.y, z: camera.position.z },
        rotation: { x: camera.rotation.x, y: camera.rotation.y, z: camera.rotation.z },
      };
      return;
    }

    // --- Detect planet change → instant teleport ---
    if (activePlanet !== lastPlanetRef.current) {
      void audioService.playTravelSound();
      lastPlanetRef.current = activePlanet;

      const orbit = getOrbitLimits(activePlanet);
      const defaultSpherical = getPlanetCameraDefaultSpherical(activePlanet);
      targetSpherical.current.radius = defaultSpherical?.radius ?? orbit.defaultRadius;
      targetSpherical.current.phi =
        activePlanet === "solar-system" ? 1.281 : (defaultSpherical?.phi ?? Math.PI / 2);
      targetSpherical.current.theta = defaultSpherical?.theta ?? 0;

      // Snap spherical immediately
      spherical.radius = targetSpherical.current.radius;
      spherical.phi = targetSpherical.current.phi;
      spherical.theta = targetSpherical.current.theta;

      // Snap orbit center
      const center = computePlanetCenter(
        activePlanet,
        state.clock.elapsedTime,
        _toSun,
        _quat,
        _zAxis,
        _camOffset,
      );
      orbitCenter.current.copy(center);

      // Snap camera position
      sphericalToWorldOffset(spherical, activePlanet, center, _toSun, _quat, _zAxis, _offset);
      camera.position.copy(center).add(_offset);
      camera.lookAt(center);
    }

    // --- Steady state: follow orbiting planet + apply pan/zoom ---
    const targetCenter = computePlanetCenter(
      activePlanet,
      state.clock.elapsedTime,
      _toSun,
      _quat,
      _zAxis,
      _camOffset,
    );
    orbitCenter.current.copy(targetCenter);

    if (size.width !== previousSize.current.width || size.height !== previousSize.current.height) {
      if (isMajorResize(previousSize.current, size)) {
        targetSpherical.current.theta = 0;
        targetSpherical.current.phi = Math.PI / 2;
      } else {
        const limits = getOrbitLimits(activePlanet);
        targetSpherical.current.phi = THREE.MathUtils.clamp(
          targetSpherical.current.phi,
          MIN_POLAR_ANGLE,
          MAX_POLAR_ANGLE,
        );
        targetSpherical.current.radius = THREE.MathUtils.clamp(
          targetSpherical.current.radius,
          limits.minDistance,
          limits.maxDistance,
        );
      }
      previousSize.current = { width: size.width, height: size.height };
    }

    const focusRequest = useCameraFocusStore.getState().consume();
    if (focusRequest) {
      const [x, y, z] = focusRequest.direction;
      focusSpherical.setFromVector3(focusDirection.set(x, y, z));
      focusAnimation.current = {
        start: state.clock.elapsedTime - (focusRequest.instant ? FOCUS_DURATION : 0),
        fromTheta: spherical.theta,
        fromPhi: spherical.phi,
        fromRadius: spherical.radius,
        toTheta: nearestEquivalentAngle(spherical.theta, focusSpherical.theta),
        toPhi: THREE.MathUtils.clamp(focusSpherical.phi, MIN_POLAR_ANGLE, MAX_POLAR_ANGLE),
        toRadius: focusRequest.distance,
      };
    }

    const focus = focusAnimation.current;
    if (focus) {
      const t = Math.min((state.clock.elapsedTime - focus.start) / FOCUS_DURATION, 1);
      const eased = 1 - (1 - t) ** 3;
      targetSpherical.current.theta = THREE.MathUtils.lerp(focus.fromTheta, focus.toTheta, eased);
      targetSpherical.current.phi = THREE.MathUtils.lerp(focus.fromPhi, focus.toPhi, eased);
      targetSpherical.current.radius = THREE.MathUtils.lerp(
        focus.fromRadius,
        focus.toRadius,
        eased,
      );
      spherical.theta = targetSpherical.current.theta;
      spherical.phi = targetSpherical.current.phi;
      spherical.radius = targetSpherical.current.radius;
      if (t >= 1) {
        focusAnimation.current = null;
      }
    } else {
      const radiusDelta = Math.abs(targetSpherical.current.radius - spherical.radius);
      const panLerp = radiusDelta > 50 ? 0.02 : 0.1;

      spherical.theta += (targetSpherical.current.theta - spherical.theta) * panLerp;
      spherical.phi += (targetSpherical.current.phi - spherical.phi) * panLerp;
      spherical.radius += (targetSpherical.current.radius - spherical.radius) * panLerp;
    }

    const pending = pendingCameraTransformRef.current;
    if (pending) {
      if (pending.position) {
        camera.position.set(pending.position.x, pending.position.y, pending.position.z);
        const newSpherical = new THREE.Spherical().setFromVector3(
          camera.position.clone().sub(orbitCenter.current),
        );
        spherical.radius = newSpherical.radius;
        spherical.phi = newSpherical.phi;
        spherical.theta = newSpherical.theta;
        targetSpherical.current.radius = newSpherical.radius;
        targetSpherical.current.phi = newSpherical.phi;
        targetSpherical.current.theta = newSpherical.theta;
      }
      pendingCameraTransformRef.current = null;
    }

    sphericalToWorldOffset(
      spherical,
      activePlanet,
      orbitCenter.current,
      _toSun,
      _quat,
      _zAxis,
      _offset,
    );
    camera.position.copy(orbitCenter.current).add(_offset);
    camera.lookAt(orbitCenter.current);

    cameraStateRef.current = {
      position: { x: camera.position.x, y: camera.position.y, z: camera.position.z },
      rotation: { x: camera.rotation.x, y: camera.rotation.y, z: camera.rotation.z },
    };
  });

  useEffect(() => {
    if (!settings.freeCameraEnabled) {
      const offset = new THREE.Vector3().setFromSpherical(spherical);
      camera.position.copy(orbitCenter.current).add(offset);
      camera.lookAt(orbitCenter.current);
    }

    const domElement = gl.domElement;
    const pointers = new Map<number, TrackedPointer>();
    const gesture = {
      pointerType: "mouse",
      maxPointers: 0,
      moved: false,
      centroidX: 0,
      centroidY: 0,
      spread: 0,
    };

    const rebaseline = () => {
      const { x, y, spread } = measurePointers(pointers);
      gesture.centroidX = x;
      gesture.centroidY = y;
      gesture.spread = spread;
    };

    const rotateBy = (radiansX: number, radiansY: number) => {
      targetSpherical.current.theta -= radiansX;
      targetSpherical.current.phi = THREE.MathUtils.clamp(
        targetSpherical.current.phi - radiansY,
        MIN_POLAR_ANGLE,
        MAX_POLAR_ANGLE,
      );
    };

    const zoomTo = (radius: number, anchor: { x: number; y: number } | null) => {
      const planet = activePlanetRef.current;
      if (planet === "solar-system") {
        return;
      }
      const limits = getOrbitLimits(planet);
      const previous = targetSpherical.current.radius;
      const next = THREE.MathUtils.clamp(radius, limits.minDistance, limits.maxDistance);
      targetSpherical.current.radius = next;

      const bodyRadius = getBodyRadius(planet);
      if (!anchor || bodyRadius <= 0) {
        return;
      }
      const rect = domElement.getBoundingClientRect();
      if (rect.height <= 0) {
        return;
      }
      const halfFovTan = Math.tan(THREE.MathUtils.degToRad(getVerticalFov(camera)) / 2);
      const surfaceShift =
        getSurfaceDistance(previous, bodyRadius) - getSurfaceDistance(next, bodyRadius);
      const radiansPerPixel = (2 * halfFovTan * surfaceShift) / (rect.height * bodyRadius);
      const view = camera instanceof THREE.PerspectiveCamera ? camera.view : null;
      const viewShiftX = view?.enabled ? view.offsetX : 0;
      const viewShiftY = view?.enabled ? view.offsetY : 0;
      const offsetX = anchor.x - (rect.left + rect.width / 2 - viewShiftX);
      const offsetY = anchor.y - (rect.top + rect.height / 2 - viewShiftY);
      rotateBy(-offsetX * radiansPerPixel, -offsetY * radiansPerPixel);
    };

    const radiansPerPixel = () => {
      if (gesture.pointerType !== "touch") {
        return MOUSE_ORBIT_SPEED;
      }
      const planet = activePlanetRef.current;
      const radius = targetSpherical.current.radius;
      const width = Math.max(domElement.clientWidth, 1);
      const height = Math.max(domElement.clientHeight, 1);
      const swipeCap = TOUCH_MAX_SWIPE_ARC / width;
      const bodyRadius = getBodyRadius(planet);
      if (bodyRadius <= 0) {
        return (swipeCap * radius) / getOrbitLimits(planet).defaultRadius;
      }
      const halfFovTan = Math.tan(THREE.MathUtils.degToRad(getVerticalFov(camera)) / 2);
      const worldPerPixel = (2 * getSurfaceDistance(radius, bodyRadius) * halfFovTan) / height;
      return Math.min(worldPerPixel / bodyRadius, swipeCap);
    };

    const handlePointerDown = (event: PointerEvent) => {
      if (settings.freeCameraEnabled) {
        panState.lastGestureWasDrag = false;
        return;
      }
      if (pointers.size === 0) {
        focusAnimation.current = null;
        gesture.pointerType = event.pointerType;
        gesture.maxPointers = 0;
        gesture.moved = false;
        panState.isPanning = true;
        domElement.style.cursor = "grabbing";
      }
      pointers.set(event.pointerId, {
        x: event.clientX,
        y: event.clientY,
        originX: event.clientX,
        originY: event.clientY,
      });
      gesture.maxPointers = Math.max(gesture.maxPointers, pointers.size);
      rebaseline();
    };

    const handlePointerMove = (event: PointerEvent) => {
      const pointer = pointers.get(event.pointerId);
      if (!pointer) {
        return;
      }
      pointer.x = event.clientX;
      pointer.y = event.clientY;

      if (!gesture.moved) {
        const dx = pointer.x - pointer.originX;
        const dy = pointer.y - pointer.originY;
        const slop = slopFor(gesture.pointerType);
        if (dx * dx + dy * dy > slop * slop) {
          if (!gesture.moved) {
            useTileInspectStore.getState().clear();
          }
          gesture.moved = true;
        }
      }

      const { x, y, spread } = measurePointers(pointers);
      const speed = radiansPerPixel();
      rotateBy((x - gesture.centroidX) * speed, (y - gesture.centroidY) * speed);

      if (pointers.size >= 2 && gesture.spread > 0 && spread > 0) {
        const radius = targetSpherical.current.radius;
        const bodyRadius = getBodyRadius(activePlanetRef.current);
        const surfaceDistance = getSurfaceDistance(radius, bodyRadius);
        const nextSurfaceDistance = (surfaceDistance * gesture.spread) / spread;
        zoomTo(radius + nextSurfaceDistance - surfaceDistance, { x, y });
      }

      gesture.centroidX = x;
      gesture.centroidY = y;
      gesture.spread = spread;
    };

    const handlePointerEnd = (event: PointerEvent) => {
      if (!pointers.delete(event.pointerId)) {
        return;
      }
      panState.lastGestureWasDrag = gesture.moved || gesture.maxPointers > 1;
      if (pointers.size > 0) {
        rebaseline();
        return;
      }
      panState.isPanning = false;
      domElement.style.cursor = "grab";
    };

    const handleWheel = (event: WheelEvent) => {
      if (settings.freeCameraEnabled) {
        return;
      }
      event.preventDefault();
      focusAnimation.current = null;
      zoomTo(targetSpherical.current.radius + event.deltaY * WHEEL_ZOOM_SPEED, null);
    };

    if (!settings.freeCameraEnabled) {
      domElement.style.cursor = "grab";
    }
    domElement.style.touchAction = "none";

    domElement.addEventListener("pointerdown", handlePointerDown);
    domElement.addEventListener("lostpointercapture", handlePointerEnd);
    domElement.addEventListener("wheel", handleWheel, { passive: false });
    document.addEventListener("pointermove", handlePointerMove);
    document.addEventListener("pointerup", handlePointerEnd);
    document.addEventListener("pointercancel", handlePointerEnd);

    return () => {
      domElement.removeEventListener("pointerdown", handlePointerDown);
      domElement.removeEventListener("lostpointercapture", handlePointerEnd);
      domElement.removeEventListener("wheel", handleWheel);
      document.removeEventListener("pointermove", handlePointerMove);
      document.removeEventListener("pointerup", handlePointerEnd);
      document.removeEventListener("pointercancel", handlePointerEnd);
      pointers.clear();
      panState.isPanning = false;
    };
  }, [camera, gl, spherical, settings.freeCameraEnabled]);

  return null;
}
