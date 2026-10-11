import React from "react";
import ReactDOM from "react-dom/client";
import "./index.css";
import { initInstallDetection } from "./utils/installApp.ts";

const inspecting = import.meta.env.DEV && window.location.pathname === "/dev/board";
const { default: App } = inspecting
  ? await import("./dev/inspection/InspectionApp.tsx")
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
