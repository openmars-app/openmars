import path from "node:path";
import type { Plugin } from "vite";
import { buildAssets } from "./pipeline.ts";

export function assetWatcher(): Plugin {
  return {
    name: "game-asset-watcher",
    apply: "serve",
    configureServer(server) {
      const root = path.resolve(server.config.root, "..");
      const sources = ["assets", "engine/asset-pipeline"].map(
        (directory) => path.join(root, directory) + path.sep,
      );
      const manifests = [
        "engine/Cargo.toml",
        "engine/Cargo.lock",
        "engine/rust-toolchain.toml",
      ].map((file) => path.join(root, file));
      server.watcher.add([...sources, ...manifests]);
      let timer: ReturnType<typeof setTimeout> | undefined;
      let pending = Promise.resolve();
      const changed = (file: string) => {
        if (!sources.some((source) => file.startsWith(source)) && !manifests.includes(file)) {
          return;
        }
        clearTimeout(timer);
        timer = setTimeout(() => {
          pending = pending.then(async () => {
            try {
              const result = await buildAssets(root);
              server.config.logger.info(`Assets: ${result.built} regenerated.`);
              server.ws.send({ type: "full-reload" });
            } catch (error) {
              const message = error instanceof Error ? error.message : String(error);
              server.config.logger.error(message);
              server.ws.send({
                type: "error",
                err: { message, stack: "", plugin: "game-asset-watcher" },
              });
            }
          });
        }, 150);
      };
      server.watcher.on("add", changed).on("change", changed).on("unlink", changed);
      server.httpServer?.once("close", () => {
        clearTimeout(timer);
        server.watcher.off("add", changed).off("change", changed).off("unlink", changed);
      });
    },
  };
}
