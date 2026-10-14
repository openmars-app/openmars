import { useIsolatedOverlay } from "@/contexts/OverlayEnvironment";
import React, { useEffect, useRef, useState } from "react";
import GameMenuModal from "./GameMenuModal.tsx";
import GameButton from "../buttons/GameButton.tsx";
import CloseButton from "../buttons/CloseButton.tsx";
import { APP_VERSION } from "@/config.ts";
import { Z_INDEX } from "@/constants/zIndex.ts";
import { useBackDismiss } from "@/hooks/useBackDismiss.ts";
import { useLayoutMode } from "@/hooks/useLayoutMode.ts";
import { apiService } from "@/services/apiService.ts";
import { useRenderPause } from "@/stores/renderPauseStore.ts";
import { useVersionStore } from "@/stores/versionStore.ts";
import type { ChangelogEntry, ChangelogSection } from "@/types/generated/api-types.ts";
import { compareVersions, isLocalBuild } from "@/utils/version.ts";

export type ChangelogState =
  | { status: "loading" }
  | { status: "error" }
  | { status: "ready"; entries: ChangelogEntry[] };

function InlineText({ text }: { text: string }) {
  return (
    <>
      {text.split("`").map((part, index) =>
        index % 2 === 1 ? (
          <code key={index} className="rounded-sm bg-white/10 px-1 font-mono text-[0.95em]">
            {part}
          </code>
        ) : (
          <React.Fragment key={index}>{part}</React.Fragment>
        ),
      )}
    </>
  );
}

function isNotInstalled(version: string): boolean {
  return !isLocalBuild(APP_VERSION) && compareVersions(version, APP_VERSION) > 0;
}

function BulletList({ items }: { items: string[] }) {
  return (
    <ul className="m-0 list-disc space-y-1 pl-4 text-sm text-white/85">
      {items.map((item, index) => (
        <li key={index}>
          <InlineText text={item} />
        </li>
      ))}
    </ul>
  );
}

function MajorUpdate({ version, section }: { version: string; section: ChangelogSection }) {
  return (
    <div className="max-w-[900px]">
      <h4 className="m-0 mb-1 font-orbitron text-base font-bold text-white">{section.title}</h4>
      <p className="m-0 mb-2 text-sm text-white/85">
        <InlineText text={section.intro} />
      </p>
      {section.image && (
        <img
          src={apiService.changelogImageUrl(version, section.image.file)}
          alt={section.image.alt}
          loading="lazy"
          className="mb-2 block h-auto max-h-[420px] w-auto max-w-full rounded-sm border border-white/10 compact:max-h-[220px]"
        />
      )}
      {section.items.length > 0 && <BulletList items={section.items} />}
    </div>
  );
}

const EntrySection = React.forwardRef<HTMLElement, { entry: ChangelogEntry; onUpdate: () => void }>(
  function EntrySection({ entry, onUpdate }, ref) {
    const notInstalled = isNotInstalled(entry.version);
    const majorSections = entry.sections.filter((section) => section.major);
    const otherSections = entry.sections.filter((section) => !section.major);
    return (
      <section
        ref={ref}
        className="grid scroll-mt-2 grid-cols-[120px_1fr] gap-x-6 border-t border-white/10 py-5 first:border-t-0 first:pt-0 compact:grid-cols-[72px_1fr] compact:gap-x-4 compact:py-4"
      >
        <div>
          <h3 className="m-0 font-orbitron text-lg font-bold text-white compact:text-base">
            {entry.version}
          </h3>
          {entry.version === APP_VERSION && (
            <span className="font-orbitron text-xs text-white/40">Installed</span>
          )}
        </div>
        <div className="min-w-0 space-y-4">
          {notInstalled && (
            <div className="flex items-center gap-4 text-sm text-white/70">
              <span>Not installed yet</span>
              <GameButton size="sm" height={36} onClick={onUpdate}>
                Update
              </GameButton>
            </div>
          )}
          {entry.intro && (
            <p className="m-0 text-sm text-white/70">
              <InlineText text={entry.intro} />
            </p>
          )}
          {majorSections.map((section) => (
            <MajorUpdate key={section.title} version={entry.version} section={section} />
          ))}
          {otherSections.length > 0 && (
            <div className="grid grid-cols-[repeat(auto-fit,minmax(240px,1fr))] gap-x-8 gap-y-4">
              {otherSections.map((section) => (
                <div key={section.title}>
                  <h4 className="m-0 mb-1 font-orbitron text-xs uppercase tracking-wider text-white/50">
                    {section.title}
                  </h4>
                  <BulletList items={section.items} />
                </div>
              ))}
            </div>
          )}
        </div>
      </section>
    );
  },
);

function useChangelog(): ChangelogState {
  const [state, setState] = useState<ChangelogState>({ status: "loading" });
  useEffect(() => {
    const controller = new AbortController();
    apiService.getChangelog(controller.signal).then(
      (changelog) => setState({ status: "ready", entries: changelog.entries }),
      () => {
        if (!controller.signal.aborted) {
          setState({ status: "error" });
        }
      },
    );
    return () => controller.abort();
  }, []);
  return state;
}

function ChangelogBody({
  state,
  focus,
  onUpdate,
}: {
  state: ChangelogState;
  focus: string | null;
  onUpdate: () => void;
}) {
  const focusRef = useRef<HTMLElement>(null);

  useEffect(() => {
    if (state.status === "ready") {
      focusRef.current?.scrollIntoView({ block: "start" });
    }
  }, [state.status]);

  if (state.status === "loading") {
    return <p className="m-0 text-center text-sm text-white/60">Loading changelog…</p>;
  }
  if (state.status === "error") {
    return <p className="m-0 text-center text-sm text-white/60">Could not load the changelog.</p>;
  }
  if (state.entries.length === 0) {
    return <p className="m-0 text-center text-sm text-white/60">No release notes yet.</p>;
  }
  return (
    <div className="text-left">
      {state.entries.map((entry) => (
        <EntrySection
          key={entry.version}
          ref={entry.version === focus ? focusRef : undefined}
          entry={entry}
          onUpdate={onUpdate}
        />
      ))}
    </div>
  );
}

function ChangelogScreen({
  onClose,
  children,
}: {
  onClose: () => void;
  children: React.ReactNode;
}) {
  const isolated = useIsolatedOverlay();
  useRenderPause("changelog", !isolated);
  return (
    <div
      className="fixed inset-0 flex flex-col bg-black text-white"
      style={{ zIndex: Z_INDEX.APP_OVERLAY }}
    >
      <header className="flex shrink-0 items-center gap-3 border-b border-white/10 bg-black pt-[calc(6px+var(--safe-top))] pr-[calc(8px+var(--safe-right))] pb-[6px] pl-[calc(16px+var(--safe-left))]">
        <h1 className="m-0 flex-1 font-orbitron text-base font-bold text-shadow-glow-strong">
          Changelog
        </h1>
        <CloseButton onClick={onClose} label="Close changelog" />
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain pt-4 pr-[calc(16px+var(--safe-right))] pb-[calc(16px+var(--safe-bottom))] pl-[calc(16px+var(--safe-left))]">
        {children}
      </div>
    </div>
  );
}

function Changelog() {
  const focus = useVersionStore((s) => s.changelogFocus);
  const closeChangelog = useVersionStore((s) => s.closeChangelog);
  const state = useChangelog();
  useBackDismiss(closeChangelog);
  return (
    <ChangelogModalView
      state={state}
      focus={focus}
      onClose={closeChangelog}
      onUpdate={() => window.location.reload()}
    />
  );
}

export function ChangelogModalView({
  state,
  focus = null,
  onClose: closeChangelog,
  onUpdate,
}: {
  state: ChangelogState;
  focus?: string | null;
  onClose: () => void;
  onUpdate: () => void;
}) {
  const { isCompact } = useLayoutMode();
  const body = <ChangelogBody state={state} focus={focus} onUpdate={onUpdate} />;
  if (isCompact) {
    return <ChangelogScreen onClose={closeChangelog}>{body}</ChangelogScreen>;
  }
  return (
    <GameMenuModal
      layout="wide"
      title="Changelog"
      showBackdrop={true}
      zIndex={Z_INDEX.APP_OVERLAY}
      onClose={closeChangelog}
      showCloseButton={true}
    >
      {body}
    </GameMenuModal>
  );
}

const ChangelogModal: React.FC = () => {
  const open = useVersionStore((s) => s.changelogOpen);
  return open ? <Changelog /> : null;
};

export default ChangelogModal;
