import type { RootState } from "@react-three/fiber";
import { ScenePreparation } from "../../contexts/ScenePreparationContext";
import { useCameraFocusStore } from "../../stores/cameraFocusStore";
import { boardCenter, projectBoardPoint } from "../../components/game/board/landscapeGeometry";
import type { InspectionView } from "./scene";

const yieldTask = () =>
  new Promise<void>((resolve) => {
    const channel = new MessageChannel();
    channel.port1.onmessage = () => {
      channel.port1.close();
      channel.port2.close();
      resolve();
    };
    channel.port2.postMessage(null);
  });
const browserFrame = () => new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
export const FPS = 60;
export const PREPARATION_FRAMES = 300;
export class InspectionDriver {
  root: RootState | null = null;
  preparation = new ScenePreparation();
  skyboxReady = false;
  gpuReady = false;
  disposed = false;
  seconds = 0;
  frame = 0;
  ready = false;
  view: InspectionView = "gameplay";
  async draw() {
    if (this.disposed) {
      throw new Error("Inspection session replaced");
    }
    this.root?.advance(this.seconds, false);
    await yieldTask();
  }
  async drain(initial = false) {
    const deadline = performance.now() + 180_000;
    let stable = 0;
    while (stable < 3) {
      await this.draw();
      const loaded = this.root && (!initial || (this.skyboxReady && this.gpuReady));
      stable = loaded && this.preparation.pending.size === 0 ? stable + 1 : 0;
      if (performance.now() > deadline) {
        throw new Error(
          `Scene preparation timed out: ${[...this.preparation.pending.values()].join(", ") || "textures or GPU"}`,
        );
      }
      await browserFrame();
    }
  }
  focus(view: InspectionView, scene = "comparison") {
    this.view = view;
    const coordinate = scene === "structures" ? { q: 0, r: 0, s: 0 } : { q: 0.6, r: 1, s: -1.6 };
    const center = boardCenter(coordinate);
    const direction = projectBoardPoint(center.x, center.y).normalize();
    useCameraFocusStore.getState().requestFocus({
      direction: [direction.x, direction.y, direction.z],
      distance: view === "close" ? 2.9 : 4.1,
      instant: true,
    });
  }
  async prepare(view: InspectionView, scene = "comparison") {
    this.focus(view, scene);
    await this.drain(true);
    for (let i = 1; i <= PREPARATION_FRAMES; i++) {
      this.seconds = i / FPS;
      await this.draw();
      if (this.preparation.pending.size) {
        await this.drain();
      }
    }
    await this.drain();
  }
  async forward(frame: number, settle = true) {
    for (let i = this.frame + 1; i <= frame; i++) {
      this.seconds = (PREPARATION_FRAMES + i) / FPS;
      await this.draw();
      if (this.preparation.pending.size) {
        await this.drain();
      }
      this.frame = i;
    }
    if (settle) {
      await this.drain();
    }
  }
  diagnostics() {
    const gl = this.root?.gl.getContext();
    const extension = gl?.getExtension("WEBGL_debug_renderer_info");
    return {
      frame: this.frame,
      seconds: this.seconds,
      ready: this.ready,
      pending: [...this.preparation.pending.values()],
      renderer: gl ? gl.getParameter(extension?.UNMASKED_RENDERER_WEBGL ?? gl.RENDERER) : null,
      drawCalls: this.root?.gl.info.render.calls,
      triangles: this.root?.gl.info.render.triangles,
    };
  }
}
