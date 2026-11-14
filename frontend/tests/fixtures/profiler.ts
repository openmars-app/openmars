import { createElement, Fragment, useState } from "react";
import { createRoot, type Root } from "react-dom/client";
import PerformanceWindow from "../../src/components/ui/debug/PerformanceWindow";
import { Z_INDEX } from "../../src/constants/zIndex";
import { WindowManagerProvider } from "../../src/components/ui/debug/WindowManager";

let root: Root | null = null;
let host: HTMLDivElement | null = null;

function ProfilerFixture() {
  const [visible, setVisible] = useState(true);
  return createElement(WindowManagerProvider, {
    children: createElement(
      Fragment,
      null,
      !visible &&
        createElement(
          "button",
          {
            type: "button",
            className: "game-button fixed top-2 left-2 !min-h-11",
            style: { zIndex: Z_INDEX.DEBUG_WINDOWS },
            onClick: () => setVisible(true),
          },
          "Open performance",
        ),
      createElement(PerformanceWindow, { isVisible: visible, onClose: () => setVisible(false) }),
    ),
  });
}

/** Mount the unchanged production panel over /dev/engine's real production canvas. */
export function mountProfilerFixture() {
  unmountProfilerFixture();
  host = document.createElement("div");
  host.dataset.testProfilerHost = "";
  document.body.append(host);
  root = createRoot(host);
  root.render(createElement(ProfilerFixture));
}

export function unmountProfilerFixture() {
  root?.unmount();
  host?.remove();
  root = null;
  host = null;
}
