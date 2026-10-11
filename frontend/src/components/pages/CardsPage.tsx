import { MainMenuDrawerButton } from "../ui/buttons/MainMenuHamburger.tsx";
import { useCallback, useEffect, useRef, useState } from "react";
import { useLocation, useNavigate, useSearchParams } from "react-router-dom";
import CardBrowser from "../ui/CardBrowser.tsx";
import { parseFamily, parseSize, parseSort } from "../ui/cardBrowser/cardCatalog.ts";
import type { CardBrowserView } from "../ui/cardBrowser/cardCatalog.ts";
import { afterNextPaint } from "@/utils/scheduling.ts";

const LIST_PARAMS = {
  ids: "cId",
  tags: "tag",
  types: "type",
} as const;
type ListKey = keyof typeof LIST_PARAMS;
const LIST_KEYS = Object.keys(LIST_PARAMS) as ListKey[];

function parseView(params: URLSearchParams): CardBrowserView {
  return {
    query: params.get("q") ?? "",
    family: parseFamily(params.get("family")),
    sort: parseSort(params.get("sort")),
    size: parseSize(params.get("size")),
    ids: params.getAll(LIST_PARAMS.ids),
    tags: params.getAll(LIST_PARAMS.tags),
    types: params.getAll(LIST_PARAMS.types),
    gameModeId: params.get("mode") ?? "",
    preview: params.get("preview") ?? "connected",
  };
}

function serializeView(view: CardBrowserView) {
  const params = new URLSearchParams();
  if (view.gameModeId) {
    params.set("mode", view.gameModeId);
    params.set("preview", view.preview);
  }
  if (view.query) {
    params.set("q", view.query);
  }
  if (view.family) {
    params.set("family", view.family);
  }
  if (view.sort !== "id") {
    params.set("sort", view.sort);
  }
  if (view.size !== "small") {
    params.set("size", view.size);
  }
  for (const key of LIST_KEYS) {
    view[key].forEach((value) => params.append(LIST_PARAMS[key], value));
  }
  return params;
}

export default function CardsPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const [view, setView] = useState(() => parseView(searchParams));
  const latest = useRef(view);
  const pendingWrite = useRef<(() => void) | null>(null);
  const writtenSearch = useRef<string | null>(null);

  useEffect(() => {
    const params = new URLSearchParams(location.search);
    if (writtenSearch.current === params.toString()) {
      writtenSearch.current = null;
      return;
    }
    if (serializeView(latest.current).toString() === params.toString()) {
      return;
    }
    pendingWrite.current?.();
    pendingWrite.current = null;
    const next = parseView(params);
    latest.current = next;
    setView(next);
  }, [location.key, location.search]);

  useEffect(() => () => pendingWrite.current?.(), []);

  const handleViewChange = useCallback(
    (patch: Partial<CardBrowserView>) => {
      pendingWrite.current?.();
      const next = { ...latest.current, ...patch };
      latest.current = next;
      setView(next);
      const write = () => {
        pendingWrite.current = null;
        const params = serializeView(latest.current);
        writtenSearch.current = params.toString();
        setSearchParams(params, { replace: true });
      };
      if (Object.keys(patch).every((key) => key === "query")) {
        const timeout = setTimeout(write, 300);
        pendingWrite.current = () => clearTimeout(timeout);
      } else {
        pendingWrite.current = afterNextPaint(write);
      }
    },
    [setSearchParams],
  );
  const handleBack = useCallback(() => navigate("/"), [navigate]);

  return (
    <CardBrowser
      onBack={handleBack}
      view={view}
      onViewChange={handleViewChange}
      headerStart={<MainMenuDrawerButton />}
    />
  );
}
