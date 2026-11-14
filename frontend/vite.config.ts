import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "path";
import { engineWatcher } from "./scripts/engine/vite.ts";
import { assetWatcher } from "./scripts/assets/vite.ts";

const backendProxy = {
  "/api": {
    target: process.env.OPENMARS_BACKEND_ORIGIN ?? "http://localhost:3001",
    changeOrigin: true,
    secure: false,
  },
  "/ws": {
    target: process.env.OPENMARS_BACKEND_ORIGIN ?? "http://localhost:3001",
    ws: true,
    changeOrigin: true,
  },
};

export default defineConfig({
  plugins: [react(), assetWatcher(), engineWatcher()],
  server: {
    // Inspector scenarios retain a renderer across long-lived GPU sequences.
    // Independent builds may refresh generated files without changing their contents.
    hmr: process.env.E2E_DISABLE_HMR === "1" ? false : undefined,
    watch: { ignored: ["**/playwright-report/**", "**/test-results/**"] },
    fs: { allow: [import.meta.dirname, path.resolve(import.meta.dirname, "../engine/pkg")] },
    host: true,
    port: 3000,
    // Behind the dev gateway the page is on another origin, so CSS url()s need this one
    origin: process.env.OPENMARS_DEV_ORIGIN,
    proxy: backendProxy,
  },
  preview: {
    host: true,
    proxy: backendProxy,
  },
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
  build: {
    outDir: "build",
    sourcemap: true,
    rolldownOptions: {
      output: {
        codeSplitting: {
          groups: [
            {
              name: "react-vendor",
              test: /node_modules\/(react|react-dom|scheduler)\//,
              priority: 20,
            },
          ],
        },
      },
    },
  },
  worker: {
    format: "es",
  },
});
