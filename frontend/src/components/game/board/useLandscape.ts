import { useScenePreparation } from "../../../contexts/ScenePreparationContext";
import { useEffect, useRef, useState } from "react";
import landscapeWorkerUrl from "./landscape.worker.ts?worker&url";
import { serverUrl } from "@/config.ts";
import {
  EMPTY_LANDSCAPE,
  type LandscapeInput,
  type LandscapeState,
  type LandscapeRequest,
  type LandscapeDelta,
} from "./landscapeTypes";
export function useLandscape(input: LandscapeInput, signature: string) {
  const { begin, finish } = useScenePreparation("Landscape worker");
  const worker = useRef<Worker | null>(null);
  const generation = useRef(0);
  const accepted = useRef(EMPTY_LANDSCAPE);
  const [result, setResult] = useState<LandscapeState>(EMPTY_LANDSCAPE);
  useEffect(() => {
    const instance = createLandscapeWorker();
    worker.current = instance;
    accepted.current = EMPTY_LANDSCAPE;
    instance.onmessage = ({ data }: MessageEvent<LandscapeDelta>) => {
      if (data.id !== generation.current) {
        return;
      }
      const patches = new Map(accepted.current.patches);
      for (const key of data.removed) {
        patches.delete(key);
      }
      for (const patch of data.patches) {
        patches.set(patch.key, patch);
      }
      const state = {
        id: data.id,
        seed: data.seed,
        relief: data.relief,
        sources: data.sources,
        patches,
        plants: [...patches.values()].flatMap((p) => p.plants),
      };
      accepted.current = state;
      setResult(state);
      finish();
    };
    instance.onerror = (event) => {
      console.error("Landscape worker failed", event.message);
    };
    return () => {
      instance.terminate();
      worker.current = null;
      finish();
    };
  }, [finish]);
  useEffect(() => {
    begin();
    worker.current!.postMessage({
      ...input,
      id: ++generation.current,
      known: Object.fromEntries(
        [...accepted.current.patches].map(([key, p]) => [key, p.signature]),
      ),
    } satisfies LandscapeRequest);
  }, [signature, begin]);
  return result;
}

/**
 * Browsers refuse to start a worker from another origin, which happens when a
 * gateway boots this bundle. A same-origin blob that imports the real script
 * is allowed, because module imports only need CORS.
 */
function createLandscapeWorker(): Worker {
  const url = serverUrl(landscapeWorkerUrl);
  if (new URL(url).origin === window.location.origin) {
    return new Worker(url, { type: "module" });
  }
  const bootstrap = URL.createObjectURL(
    new Blob([`import ${JSON.stringify(url)};`], { type: "text/javascript" }),
  );
  const worker = new Worker(bootstrap, { type: "module" });
  URL.revokeObjectURL(bootstrap);
  return worker;
}
