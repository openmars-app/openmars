import { engineProfiler } from "./engine/profiler";
import React from "react";
import ReactDOM from "react-dom/client";
import "./index.css";
import { initInstallDetection } from "./utils/installApp.ts";
import { Z_INDEX } from "./constants/zIndex.ts";

document.documentElement.style.setProperty("--surface-layer", String(Z_INDEX.CONTROL_DECORATION));

engineProfiler.mark("app.bootstrap");

const inspecting = import.meta.env.DEV && window.location.pathname === "/dev/engine";
const { default: App } = inspecting
  ? await import("./engine/EnginePreview.tsx")
  : await import("./App");

if (!inspecting) {
  initInstallDetection();
}

const root = ReactDOM.createRoot(document.getElementById("root"));
root.render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
