import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "path";
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
  plugins: [react(), assetWatcher()],
  server: {
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
            {
              name: "three-vendor",
              test: /node_modules\/(three|@react-three\/[^/]+)\//,
              priority: 10,
            },
          ],
        },
      },
    },
  },
  worker: {
    format: "es",
  },
  optimizeDeps: {
    include: ["three", "@react-three/fiber", "@react-three/drei"],
  },
});
