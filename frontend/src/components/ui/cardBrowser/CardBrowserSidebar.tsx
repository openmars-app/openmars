import { useOverlayEventTarget } from "@/contexts/OverlayEnvironment";
import type { GameModeDto } from "@/types/generated/api-types.ts";
import { memo, useEffect, useRef, useState, type ReactNode } from "react";
import { useSoundEffects } from "@/hooks/useSoundEffects.ts";
import { useHoverSound } from "@/hooks/useHoverSound.ts";
import { Z_INDEX } from "@/constants/zIndex.ts";
import GameIcon from "../display/GameIcon.tsx";
import BackButton from "../buttons/BackButton.tsx";
import GameButton from "../buttons/GameButton.tsx";
import GameSelect from "../GameSelect.tsx";
import CopyLinkButton from "../buttons/CopyLinkButton.tsx";
import { CARD_FAMILIES, FAMILY_LABELS, labelFor } from "./cardCatalog.ts";
import type { CardDisplaySize, CardFamily, CardSort } from "./cardCatalog.ts";

export type FilterKind = "tags" | "types";
export type BrowserFilters = Record<FilterKind, ReadonlySet<string>>;

interface SidebarProps {
  family: CardFamily;
  query: string;
  sort: CardSort;
  cardSize: CardDisplaySize;
  onCardSize: (size: CardDisplaySize) => void;
  filters: BrowserFilters;
  tags: string[];
  modes: GameModeDto[];
  gameModeId: string;
  preview: string;
  onMode: (id: string) => void;
  onPreview: (preview: string) => void;
  counts: Record<CardFamily, number>;
  selectedCounts: Record<CardFamily, number>;
  selectedCount: number;
  shareUrl: string;
  shared: boolean;
  open: boolean;
  persistent: boolean;
  headerStart?: ReactNode;
  backLabel: string;
  onBack: () => void;
  onClose: () => void;
  onFamily: (family: CardFamily) => void;
  onQuery: (query: string) => void;
  onSort: (sort: CardSort) => void;
  onToggle: (kind: FilterKind, value: string) => void;
  onClearFilters: () => void;
  onClearSelection: () => void;
  onExitShared: () => void;
}

const fieldClass =
  "w-full rounded-none border border-space-blue-500 bg-black px-3 py-2.5 text-sm text-white outline-none focus-visible:border-blue-400 focus-visible:ring-1 focus-visible:ring-blue-400";

export default memo(function CardBrowserSidebar(props: SidebarProps) {
  const eventTarget = useOverlayEventTarget();
  const { playButtonClickSound } = useSoundEffects();
  const panel = useRef<HTMLElement>(null);
  const [copyError, setCopyError] = useState(false);
  const { open, onClose, persistent } = props;
  const searchInput = useRef<HTMLInputElement>(null);
  const clearSound = useHoverSound();
  useEffect(() => {
    if (!open) {
      return;
    }
    const previouslyFocused = document.activeElement as HTMLElement | null;
    panel.current?.querySelector<HTMLElement>("[data-drawer-close]")?.focus();
    const keydown = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        onClose();
      } else if (event.key === "Tab") {
        const focusable = Array.from(
          panel.current?.querySelectorAll<HTMLElement>(
            'button:not(:disabled), input, select, a[href], summary, [tabindex="0"]',
          ) ?? [],
        ).filter((element) => element.getClientRects().length > 0);
        const first = focusable[0];
        const last = focusable[focusable.length - 1];
        if (event.shiftKey && document.activeElement === first) {
          event.preventDefault();
          last?.focus();
        } else if (!event.shiftKey && document.activeElement === last) {
          event.preventDefault();
          first?.focus();
        }
      }
    };
    eventTarget.addEventListener("keydown", keydown, true);
    return () => {
      eventTarget.removeEventListener("keydown", keydown, true);
      previouslyFocused?.focus();
    };
  }, [open, onClose, eventTarget]);

  return (
    <>
      {open && (
        <div
          className="fixed inset-0 bg-black/75 lg:hidden"
          style={{ zIndex: Z_INDEX.LEFT_SIDEBAR }}
          onClick={onClose}
        />
      )}
      <aside
        ref={panel}
        aria-label="Card browser controls"
        role={open ? "dialog" : undefined}
        aria-modal={open ? true : undefined}
        className={
          persistent
            ? "static flex w-[calc(232px+var(--safe-left))] shrink-0 flex-col overflow-y-auto overscroll-contain pb-[var(--safe-bottom)] pl-[var(--safe-left)] border-r border-space-blue-400 bg-space-black-darker text-left"
            : `${open ? "flex" : "hidden lg:flex"} fixed inset-y-0 left-0 w-[calc(304px+var(--safe-left))] max-w-[calc(100vw-32px)] shrink-0 flex-col pt-[var(--safe-top)] pb-[var(--safe-bottom)] pl-[var(--safe-left)] border-r border-space-blue-400 bg-space-black-darker text-left shadow-[12px_0_32px_rgba(10,25,65,0.45)] lg:static lg:max-w-none`
        }
        style={{ zIndex: Z_INDEX.LEFT_SIDEBAR }}
      >
        {persistent && (
          <div className="sticky top-0 shrink-0 flex h-[calc(2.75rem+var(--safe-top))] items-center gap-1 border-b border-white/10 bg-space-black-darker pt-[var(--safe-top)] pr-3">
            {props.headerStart ?? <BackButton onClick={props.onBack}>{props.backLabel}</BackButton>}
            <h1 className="m-0 font-orbitron text-sm font-semibold uppercase tracking-wider text-white">
              Cards
            </h1>
          </div>
        )}
        <div
          className={persistent ? "shrink-0 space-y-4 px-3 py-3" : "shrink-0 space-y-5 p-5 pb-4"}
        >
          <div className={persistent ? "hidden" : "flex items-center justify-between gap-2"}>
            <BackButton onClick={props.onBack}>{props.backLabel}</BackButton>
            <GameButton
              emphasis="quiet"
              data-drawer-close
              type="button"
              onClick={onClose}
              aria-label="Close card controls"
              className="cursor-pointer rounded-none p-2 text-white/70 hover:text-white focus-visible:outline-2 focus-visible:outline-blue-400 lg:hidden"
            >
              ✕
            </GameButton>
          </div>
          {!persistent && (
            <h1 className="m-0 pb-3 font-orbitron text-xl font-semibold leading-relaxed text-white">
              Card browser
            </h1>
          )}
          <div className="space-y-2">
            <span className="font-orbitron text-xs text-white/70">Game mode</span>
            <GameSelect
              id="browser-mode"
              label="Game mode"
              value={props.gameModeId}
              options={[{ id: "", name: "All cards" }, ...props.modes]}
              onChange={props.onMode}
              className="w-full"
            />
          </div>
          {props.modes
            .find((mode) => mode.id === props.gameModeId)
            ?.statuses.some((status) => status.kind === "connection") ? (
            <div className="space-y-2">
              <span className="font-orbitron text-xs text-white/70">Preview state</span>
              <GameSelect
                id="browser-preview"
                label="Preview state"
                value={props.preview}
                options={[
                  { id: "connected", name: "Connected" },
                  { id: "disconnected", name: "Disconnected" },
                  { id: "restored", name: "Contact restored" },
                ]}
                onChange={props.onPreview}
                className="w-full"
              />
            </div>
          ) : null}
          <fieldset className="m-0 space-y-1 border-0 p-0">
            <legend className="sr-only">Card family</legend>
            {CARD_FAMILIES.map((family) => (
              <label
                key={family}
                className={`flex cursor-pointer items-center gap-2 rounded-none border px-3 py-2.5 text-sm transition-colors has-focus-visible:ring-2 has-focus-visible:ring-blue-400 ${props.family === family ? "border-space-blue-600 bg-space-blue-400 text-white" : "border-transparent text-white/60 hover:bg-white/5 hover:text-white"}`}
              >
                <input
                  className="sr-only"
                  type="radio"
                  name="card-family"
                  value={family}
                  checked={props.family === family}
                  onChange={() => {
                    void playButtonClickSound();
                    props.onFamily(family);
                  }}
                />
                <span className="flex-1 font-orbitron text-xs">{FAMILY_LABELS[family]}</span>
                {props.selectedCounts[family] > 0 && (
                  <span className="text-[10px] text-blue-200">
                    {props.selectedCounts[family]} selected
                  </span>
                )}
                <span className="font-orbitron text-xs tabular-nums">{props.counts[family]}</span>
              </label>
            ))}
          </fieldset>
          <label className="block space-y-2">
            <span className="font-orbitron text-xs text-white/70">Search cards</span>
            <span className="relative block">
              <input
                ref={searchInput}
                type="search"
                value={props.query}
                onChange={(event) => props.onQuery(event.target.value)}
                placeholder="ID, name, text, or tag"
                className={`${fieldClass} cursor-text pr-9 [&::-webkit-search-cancel-button]:appearance-none`}
              />
              {props.query && (
                <button
                  type="button"
                  aria-label="Clear search"
                  onMouseEnter={clearSound.onMouseEnter}
                  onClick={() => {
                    clearSound.onClick?.();
                    props.onQuery("");
                    searchInput.current?.focus();
                  }}
                  className="absolute inset-y-0 right-0 flex w-9 cursor-pointer items-center justify-center text-sm text-blue-300/80 hover:text-white focus-visible:outline-2 focus-visible:outline-blue-400"
                >
                  ✕
                </button>
              )}
            </span>
          </label>
        </div>
        <div
          className={
            persistent
              ? "space-y-5 border-y border-white/10 px-3 py-4"
              : "min-h-0 flex-1 space-y-5 overflow-y-auto overscroll-contain border-y border-white/10 px-5 py-4"
          }
        >
          {props.shared && (
            <div className="space-y-2 text-sm text-blue-100">
              <p className="m-0">Viewing a shared selection</p>
              <GameButton size="sm" emphasis="secondary" onClick={props.onExitShared}>
                Exit shared selection
              </GameButton>
            </div>
          )}
          {props.family !== "corporation" && (
            <fieldset className="m-0 space-y-2 border-0 p-0">
              <legend className="font-orbitron text-xs text-white/70">Card size</legend>
              <div className="flex">
                {(["small", "large"] as const).map((size) => (
                  <GameButton
                    key={size}
                    size="sm"
                    shape="toolbar"
                    emphasis="secondary"
                    selected={props.cardSize === size}
                    aria-pressed={props.cardSize === size}
                    className="flex-1 compact:min-h-11"
                    onClick={() => props.onCardSize(size)}
                  >
                    {size === "small" ? "Small" : "Large"}
                  </GameButton>
                ))}
              </div>
            </fieldset>
          )}
          <div className="space-y-2">
            <label
              htmlFor="card-browser-sort"
              className="block font-orbitron text-xs text-white/70"
            >
              Sort by
            </label>
            <GameSelect
              id="card-browser-sort"
              label="Sort by"
              className="w-full"
              value={
                props.family !== "project" && props.sort.startsWith("type") ? "id" : props.sort
              }
              options={[
                { id: "id", name: props.shared ? "Link order" : "Card ID" },
                { id: "name-asc", name: "Name (A–Z)" },
                { id: "name-desc", name: "Name (Z–A)" },
                ...(props.family === "project"
                  ? [
                      { id: "type-asc", name: "Type (A–Z)" },
                      { id: "type-desc", name: "Type (Z–A)" },
                    ]
                  : []),
              ]}
              onChange={(value) => props.onSort(value as CardSort)}
            />
          </div>
          {props.family === "project" && (
            <FilterGroup
              label="Project type"
              kind="types"
              values={["automated", "active", "event"]}
              selected={props.filters.types}
              onToggle={props.onToggle}
            />
          )}
          <FilterGroup
            label="Tags"
            kind="tags"
            values={props.tags}
            selected={props.filters.tags}
            onToggle={props.onToggle}
          />
          <GameButton emphasis="secondary" size="sm" onClick={props.onClearFilters}>
            Clear filters
          </GameButton>
        </div>
        <div className="shrink-0 space-y-3 p-5">
          <p aria-live="polite" className="m-0 text-xs text-white/60">
            {props.counts[props.family]} results · {props.selectedCount} selected
          </p>
          {props.selectedCount > 0 && (
            <div className="space-y-2">
              <CopyLinkButton
                textToCopy={props.shareUrl}
                defaultText={`Copy link (${props.selectedCount})`}
                size="sm"
                className="w-full"
                onCopyError={() => setCopyError(true)}
                onCopySuccess={() => setCopyError(false)}
              />
              <GameButton emphasis="quiet" size="sm" onClick={props.onClearSelection}>
                Clear selection
              </GameButton>
              {copyError && (
                <p role="alert" className="text-xs text-error-red">
                  Could not copy the link. Please try again.
                </p>
              )}
            </div>
          )}
        </div>
      </aside>
    </>
  );
});

function FilterGroup({
  label,
  kind,
  values,
  selected,
  onToggle,
}: {
  label: string;
  kind: FilterKind;
  values: string[];
  selected: ReadonlySet<string>;
  onToggle: SidebarProps["onToggle"];
}) {
  const { playButtonClickSound } = useSoundEffects();
  const [expanded, setExpanded] = useState(() => selected.size > 0);
  return (
    <details
      className="group"
      open={expanded}
      onToggle={(event) => setExpanded(event.currentTarget.open)}
    >
      <summary
        onClick={() => void playButtonClickSound()}
        className="cursor-pointer font-orbitron text-xs text-white/80 focus-visible:outline-blue-400"
      >
        {label}
        {selected.size > 0 ? ` (${selected.size})` : ""}
      </summary>
      <div className="mt-3 flex flex-wrap gap-2">
        {values.map((value) => (
          <label
            key={value}
            className={`flex cursor-pointer items-center gap-1.5 rounded-none border px-2 py-1.5 text-xs has-focus-visible:ring-2 has-focus-visible:ring-blue-400 ${selected.has(value) ? "border-blue-400/70 bg-blue-950 text-white" : "border-white/15 text-white/60 hover:border-white/40 hover:text-white"}`}
          >
            <input
              type="checkbox"
              className="sr-only"
              checked={selected.has(value)}
              onChange={() => {
                void playButtonClickSound();
                onToggle(kind, value);
              }}
            />
            {kind === "tags" && <GameIcon iconType={`${value}-tag`} size="small" />}
            {labelFor(value)}
          </label>
        ))}
      </div>
    </details>
  );
}
