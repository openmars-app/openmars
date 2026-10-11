import { createContext, useCallback, useContext, useRef } from "react";

/** Tracks asynchronous preparation, separately from the animation clock. */
export class ScenePreparation {
  readonly pending = new Map<symbol, string>();
  begin(label: string) {
    const token = Symbol(label);
    this.pending.set(token, label);
    return () => {
      this.pending.delete(token);
    };
  }
}
export const ScenePreparationContext = createContext<ScenePreparation | null>(null);
export function useScenePreparation(label: string) {
  const preparation = useContext(ScenePreparationContext);
  const release = useRef<(() => void) | undefined>(undefined);
  const finish = useCallback(() => {
    release.current?.();
    release.current = undefined;
  }, []);
  const begin = useCallback(() => {
    finish();
    release.current = preparation?.begin(label);
    return finish;
  }, [preparation, label, finish]);
  return { begin, finish };
}
