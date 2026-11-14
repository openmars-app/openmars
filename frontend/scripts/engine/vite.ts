import { spawn } from "node:child_process";
import path from "node:path";
import type { Plugin } from "vite";

export function engineWatcher(): Plugin {
  return {
    name: "bevy-engine-watcher",
    apply: "serve",
    configureServer(server) {
      const engine = path.resolve(server.config.root, "../engine");
      const sources = ["src", "scene/src", "preparation/src"].map(
        (dir) => path.join(engine, dir) + path.sep,
      );
      const manifests = [
        "Cargo.toml",
        "Cargo.lock",
        "rust-toolchain.toml",
        "build.ts",
        "scene/Cargo.toml",
        "preparation/Cargo.toml",
      ].map((file) => path.join(engine, file));
      server.watcher.add([...sources, ...manifests]);
      let timer: ReturnType<typeof setTimeout>;
      let pending = Promise.resolve();
      let build: ReturnType<typeof spawn> | undefined;
      let closed = false;
      const changed = (file: string) => {
        if (!sources.some((source) => file.startsWith(source)) && !manifests.includes(file)) return;
        clearTimeout(timer);
        timer = setTimeout(() => {
          pending = pending.then(
            () =>
              new Promise<void>((resolve) => {
                if (closed) return resolve();
                build = spawn("bun", [path.join(engine, "build.ts")], {
                  stdio: "inherit",
                  detached: process.platform !== "win32",
                });
                build.on("error", (error) => {
                  server.config.logger.error(error.message);
                  resolve();
                });
                build.on("exit", (code) => {
                  if (code === 0 && !closed) server.ws.send({ type: "full-reload" });
                  else if (!closed)
                    server.config.logger.error("Bevy build failed; see the compiler output.");
                  resolve();
                });
              }),
          );
        }, 150);
      };
      server.watcher.on("add", changed).on("change", changed).on("unlink", changed);
      server.httpServer?.once("close", () => {
        closed = true;
        clearTimeout(timer);
        server.watcher.off("add", changed).off("change", changed).off("unlink", changed);
        if (build?.pid && build.exitCode === null) {
          if (process.platform === "win32") {
            build.kill();
          } else {
            try {
              process.kill(-build.pid, "SIGTERM");
            } catch (error) {
              if ((error as NodeJS.ErrnoException).code !== "ESRCH") throw error;
            }
          }
        }
      });
    },
  };
}
