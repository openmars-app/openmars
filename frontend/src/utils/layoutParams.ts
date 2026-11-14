import { useLayoutEffect } from "react";
import { useLocation } from "react-router-dom";

/** Keep the explicit compact-layout override across in-app navigation. */
export const COMPACT_PARAM = "compact";

const bootParams = new URLSearchParams(window.location.search);

const TESTING_PARAMS = [COMPACT_PARAM]
  .map((name) => ({ name, value: bootParams.get(name) }))
  .filter((param): param is { name: string; value: string } => param.value !== null);

export const FORCE_COMPACT = bootParams.has(COMPACT_PARAM);

/**
 * Re-appends every testing flag present at boot whenever an in-app navigation drops it.
 * Rewrites the address bar directly instead of navigating, so the router sees no new
 * location and route effects (game bootstrapping keyed on `location.state`) don't run a
 * second time.
 */
export function useKeepLayoutParams() {
  const location = useLocation();

  useLayoutEffect(() => {
    const params = new URLSearchParams(location.search);
    const missing = TESTING_PARAMS.filter((param) => !params.has(param.name));
    if (missing.length === 0) {
      return;
    }
    // URLSearchParams would write a bare flag as "quick=", so append flags by hand.
    const flags = missing.map((param) =>
      param.value ? `${param.name}=${encodeURIComponent(param.value)}` : param.name,
    );
    const rest = params.toString();
    const search = [rest, ...flags].filter(Boolean).join("&");
    window.history.replaceState(
      window.history.state,
      "",
      `${location.pathname}?${search}${location.hash}`,
    );
  }, [location]);
}
