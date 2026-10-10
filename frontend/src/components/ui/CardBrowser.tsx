import type { GameModeDto } from "@/types/generated/api-types.ts";
import { apiService } from "@/services/apiService.ts";
import type { ReactNode } from "react";
import { useLayoutMode } from "@/hooks/useLayoutMode.ts";
import { useCallback, useDeferredValue, useEffect, useMemo, useRef, useState } from "react";
import { Z_INDEX } from "@/constants/zIndex.ts";
import { afterNextPaint, whenIdle } from "@/utils/scheduling.ts";
import GameButton from "./buttons/GameButton.tsx";
import CardBrowserSidebar from "./cardBrowser/CardBrowserSidebar.tsx";
import type { BrowserFilters, FilterKind } from "./cardBrowser/CardBrowserSidebar.tsx";
import CachedCardFamily from "./cardBrowser/VirtualCardGrid.tsx";
import {
  CARD_FAMILIES,
  EMPTY_VIEW,
  filterCatalog,
  loadCardCatalog,
  useCardCatalog,
} from "./cardBrowser/cardCatalog.ts";
import type {
  CardBrowserView,
  CardDisplaySize,
  CardFamily,
  CardSort,
} from "./cardBrowser/cardCatalog.ts";
import { serverLink } from "@/utils/gateway.ts";

interface CardBrowserProps {
  onBack: () => void;
  backLabel?: string;
  view?: CardBrowserView;
  onViewChange?: (patch: Partial<CardBrowserView>) => void;
  headerStart?: ReactNode;
}

type ContentView = Pick<CardBrowserView, "sort" | "size" | "tags" | "types"> & {
  family: CardFamily;
};

function sameContent(a: ContentView, b: ContentView) {
  return (
    a.family === b.family &&
    a.sort === b.sort &&
    a.size === b.size &&
    a.tags === b.tags &&
    a.types === b.types
  );
}

function toFilters(tags: readonly string[], types: readonly string[]): BrowserFilters {
  return { tags: new Set(tags), types: new Set(types) };
}

export default function CardBrowser({
  onBack,
  backLabel = "Back to Home",
  view: controlledView,
  onViewChange,
  headerStart,
}: CardBrowserProps) {
  const { isCompact } = useLayoutMode();
  const [localView, setLocalView] = useState(EMPTY_VIEW);
  const view = controlledView ?? localView;
  const updateView = useCallback(
    (patch: Partial<CardBrowserView>) => {
      if (onViewChange) {
        onViewChange(patch);
      } else {
        setLocalView((previous) => ({ ...previous, ...patch }));
      }
    },
    [onViewChange],
  );
  const { entries, status, error } = useCardCatalog();
  const [modes, setModes] = useState<GameModeDto[]>([]);
  const [modeError, setModeError] = useState("");
  const filters = useMemo(() => toFilters(view.tags, view.types), [view.tags, view.types]);
  const [selected, setSelected] = useState<Set<string>>(() => new Set());
  const [visitedFamilies, setVisitedFamilies] = useState<ReadonlySet<CardFamily>>(() => new Set());
  const [drawerOpen, setDrawerOpen] = useState(false);
  const query = useDeferredValue(view.query);
  const closeDrawer = useCallback(() => setDrawerOpen(false), []);
  useEffect(() => {
    void loadCardCatalog(view.gameModeId, view.preview);
  }, [view.gameModeId, view.preview]);
  useEffect(() => {
    const controller = new AbortController();
    void apiService
      .getGameModes(controller.signal)
      .then((data) => {
        if (!controller.signal.aborted) {
          setModes(data);
        }
      })
      .catch((reason: Error) => {
        if (!controller.signal.aborted) {
          setModeError(reason.message);
        }
      });
    return () => controller.abort();
  }, []);
  useEffect(() => {
    const breakpoint = window.matchMedia("(min-width: 1024px)");
    const closeOnDesktop = () => {
      if (breakpoint.matches) {
        closeDrawer();
      }
    };
    breakpoint.addEventListener("change", closeOnDesktop);
    return () => breakpoint.removeEventListener("change", closeOnDesktop);
  }, [closeDrawer]);

  const baseFamily = useMemo(() => {
    if (!view.ids.length) {
      return "project";
    }
    const ids = new Set(view.ids);
    return (
      CARD_FAMILIES.find((family) =>
        entries.some((entry) => entry.family === family && ids.has(entry.card.id)),
      ) ?? "project"
    );
  }, [entries, view.ids]);
  const family = view.family ?? baseFamily;
  const content = useMemo<ContentView>(
    () => ({
      family,
      sort: view.sort,
      size: view.size,
      tags: view.tags,
      types: view.types,
    }),
    [family, view.sort, view.size, view.tags, view.types],
  );
  const [shown, setShown] = useState(content);
  useEffect(() => {
    if (sameContent(shown, content)) {
      return;
    }
    // Let the sidebar controls paint on their own before the heavier grid update.
    return afterNextPaint(() => setShown(content));
  }, [content, shown]);
  const shownFamily = shown.family;
  const shownFilters = useMemo(() => toFilters(shown.tags, shown.types), [shown.tags, shown.types]);
  useEffect(() => {
    setVisitedFamilies((previous) => {
      if (previous.has(shownFamily)) {
        return previous;
      }
      return new Set([...previous, shownFamily]);
    });
  }, [shownFamily]);
  useEffect(() => {
    if (status !== "ready") {
      return;
    }
    const missing = CARD_FAMILIES.find((item) => !visitedFamilies.has(item));
    if (!missing) {
      return;
    }
    return whenIdle(() => setVisitedFamilies((previous) => new Set([...previous, missing])));
  }, [status, visitedFamilies]);
  const groups = useMemo(
    () => filterCatalog(entries, { query, ids: view.ids, ...shownFilters, sort: shown.sort }),
    [entries, query, view.ids, shownFilters, shown.sort],
  );
  const counts = useMemo(
    () => ({
      project: groups.project.length,
      prelude: groups.prelude.length,
      corporation: groups.corporation.length,
    }),
    [groups],
  );
  const { tags } = useMemo(
    () => ({
      tags: [...new Set(entries.flatMap(({ card }) => card.tags ?? []))].sort(),
    }),
    [entries],
  );
  const selectedCounts = useMemo(() => {
    const counts = { project: 0, prelude: 0, corporation: 0 };
    for (const entry of entries) {
      if (selected.has(entry.card.id)) {
        counts[entry.family]++;
      }
    }
    return counts;
  }, [entries, selected]);
  const shareUrl = useMemo(() => {
    const params = new URLSearchParams();
    const sharedFamily =
      selectedCounts[family] > 0
        ? family
        : (CARD_FAMILIES.find((item) => selectedCounts[item] > 0) ?? "project");
    params.set("family", sharedFamily);
    if (view.gameModeId) {
      params.set("mode", view.gameModeId);
      params.set("preview", view.preview);
    }
    [...selected].sort().forEach((id) => params.append("cId", id));
    return serverLink(`/cards?${params}`);
  }, [selected, selectedCounts, family, view.gameModeId, view.preview]);
  const toggleSelection = useCallback((id: string) => {
    setSelected((previous) => {
      const next = new Set(previous);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);
  const currentView = useRef(view);
  currentView.current = view;
  const toggleFilter = useCallback(
    (kind: FilterKind, value: string) => {
      const values = currentView.current[kind];
      updateView({
        [kind]: values.includes(value)
          ? values.filter((item) => item !== value)
          : [...values, value],
      });
    },
    [updateView],
  );
  const clearFilters = useCallback(
    () => updateView({ query: "", tags: [], types: [] }),
    [updateView],
  );
  const changeCardSize = useCallback((size: CardDisplaySize) => updateView({ size }), [updateView]);
  const changeFamily = useCallback(
    (next: CardFamily) => updateView({ family: next }),
    [updateView],
  );
  const changeQuery = useCallback((next: string) => updateView({ query: next }), [updateView]);
  const changeSort = useCallback((next: CardSort) => updateView({ sort: next }), [updateView]);
  const clearSelection = useCallback(() => setSelected(new Set()), []);
  const exitShared = useCallback(() => updateView({ ids: [] }), [updateView]);

  return (
    <div
      className="relative flex h-dvh min-h-0 w-full overflow-hidden bg-black text-white"
      style={{ zIndex: Z_INDEX.UI_BASE }}
      onKeyDown={(event) => {
        if (event.key !== "Escape") {
          event.stopPropagation();
        }
      }}
    >
      <CardBrowserSidebar
        family={family}
        query={view.query}
        sort={view.sort}
        cardSize={view.size}
        onCardSize={changeCardSize}
        filters={filters}
        tags={tags}
        modes={modes}
        gameModeId={view.gameModeId}
        preview={view.preview}
        onMode={(gameModeId) => updateView({ gameModeId, preview: "connected" })}
        onPreview={(preview) => updateView({ preview })}
        counts={counts}
        selectedCounts={selectedCounts}
        selectedCount={selected.size}
        shareUrl={shareUrl}
        shared={view.ids.length > 0}
        open={drawerOpen && !isCompact}
        persistent={isCompact}
        headerStart={headerStart}
        backLabel={backLabel}
        onBack={onBack}
        onClose={closeDrawer}
        onFamily={changeFamily}
        onQuery={changeQuery}
        onSort={changeSort}
        onToggle={toggleFilter}
        onClearFilters={clearFilters}
        onClearSelection={clearSelection}
        onExitShared={exitShared}
      />
      <main
        className={
          isCompact
            ? "relative min-h-0 min-w-0 flex-1 pt-[var(--safe-top)] pr-[var(--safe-right)] pb-[var(--safe-bottom)]"
            : "relative min-h-0 min-w-0 flex-1 pt-[calc(4rem+var(--safe-top))] pr-[var(--safe-right)] pb-[var(--safe-bottom)] max-lg:pl-[var(--safe-left)] lg:pt-[var(--safe-top)]"
        }
        inert={drawerOpen && !isCompact}
      >
        {!isCompact && (
          <div className="absolute left-[calc(1rem+var(--safe-left))] top-[calc(0.75rem+var(--safe-top))] lg:hidden">
            <GameButton size="sm" emphasis="secondary" onClick={() => setDrawerOpen(true)}>
              Browse cards
            </GameButton>
          </div>
        )}
        {modeError && (
          <p role="alert" className="p-4 text-amber-200">
            {modeError}
          </p>
        )}
        {(status === "idle" || status === "loading") && (
          <div role="status" className="p-8 text-left text-white/60">
            Loading cards…
          </div>
        )}
        {status === "error" && (
          <div role="alert" className="space-y-4 p-8 text-left">
            <p>{error}</p>
            <GameButton
              emphasis="secondary"
              onClick={() => void loadCardCatalog(view.gameModeId, view.preview)}
            >
              Retry
            </GameButton>
          </div>
        )}
        {status === "ready" && groups[shownFamily].length === 0 && (
          <div className="space-y-4 p-8 text-left">
            <h2 className="font-orbitron text-lg">No matching cards</h2>
            <p className="text-white/60">Try another family or clear your search and filters.</p>
            <GameButton emphasis="secondary" onClick={clearFilters}>
              Clear filters
            </GameButton>
            {view.ids.length > 0 && (
              <GameButton emphasis="quiet" onClick={() => updateView({ ids: [] })}>
                Exit shared selection
              </GameButton>
            )}
          </div>
        )}
        {status === "ready" && (
          <div className="relative h-full min-h-0" hidden={groups[shownFamily].length === 0}>
            {CARD_FAMILIES.filter((item) => item === shownFamily || visitedFamilies.has(item)).map(
              (item) => (
                <CachedCardFamily
                  key={item}
                  active={item === shownFamily}
                  cards={groups[item]}
                  family={item}
                  size={shown.size}
                  selected={selected}
                  onSelect={toggleSelection}
                />
              ),
            )}
          </div>
        )}
      </main>
    </div>
  );
}
