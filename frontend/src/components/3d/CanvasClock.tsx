import { useRef } from "react";
import { useFrame } from "@react-three/fiber";

export default function CanvasClock() {
  const elapsed = useRef(0);
  useFrame((state, delta) => {
    // R3F resets its clock when frameloop changes; retain animation time across pauses.
    if (state.frameloop !== "never") {
      elapsed.current += Math.min(delta, 0.05);
      state.clock.elapsedTime = elapsed.current;
    } else {
      elapsed.current = state.clock.elapsedTime;
    }
  }, -Infinity);
  return null;
}
