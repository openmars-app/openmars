import { useRef, type ReactNode } from "react";
import { createPortal } from "react-dom";
import type { GameModeDto } from "@/types/generated/api-types.ts";
import { GameModal, useModal } from "../GameModal";
import CloseButton from "../buttons/CloseButton.tsx";
import { Z_INDEX } from "@/constants/zIndex.ts";
import { useBackDismiss } from "@/hooks/useBackDismiss.ts";
import { useLayoutMode } from "@/hooks/useLayoutMode.ts";
import { useRenderPause } from "@/stores/renderPauseStore.ts";
import CollapsibleSection from "../CollapsibleSection.tsx";
import GameIcon from "../display/GameIcon.tsx";
import { ModeSymbolLegend } from "../cards/CardStateVisuals.tsx";

export function modeText(text: string, parameters: Record<string, number>) {
  return text.replace(/\{\{?([\w-]+)\}?\}/g, (match, key: string) =>
    parameters[key] === undefined ? match : String(parameters[key]),
  );
}

interface Props {
  mode: GameModeDto;
  parameters: Record<string, number>;
  open: boolean;
  onClose: () => void;
  children?: ReactNode;
}

function CompactRules({
  title,
  onClose,
  children,
}: {
  title: string;
  onClose: () => void;
  children: ReactNode;
}) {
  const modalRef = useRef<HTMLDivElement>(null);
  useModal({ modalRef, isVisible: true, onClose, closeOnEscape: false });
  useBackDismiss(onClose);
  useRenderPause("game-mode-rules", true);

  return createPortal(
    <div
      ref={modalRef}
      role="dialog"
      aria-modal="true"
      aria-label={title}
      tabIndex={-1}
      className="fixed inset-0 flex flex-col bg-[rgba(3,3,4,0.98)] text-white text-left"
      style={{ zIndex: Z_INDEX.LOBBY_SETTINGS_MODAL }}
    >
      <header className="shrink-0 flex items-center gap-3 border-b border-white/15 pl-[calc(16px+var(--safe-left))] pr-[var(--safe-right)] pt-[var(--safe-top)]">
        <h2 className="min-w-0 flex-1 font-orbitron text-sm">{title}</h2>
        <CloseButton onClick={onClose} label="Close rules" />
      </header>
      <div className="flex-1 min-h-0 overflow-y-auto [scrollbar-gutter:stable_both-edges] overscroll-contain pt-4 pr-[calc(16px+var(--safe-right))] pb-[calc(16px+var(--safe-bottom))] pl-[calc(16px+var(--safe-left))]">
        {children}
      </div>
    </div>,
    document.body,
  );
}

export default function GameModeRules({ mode, parameters, open, onClose, children }: Props) {
  const { isCompact } = useLayoutMode();
  const content = (
    <>
      <p className="text-white/80 leading-relaxed mb-6">{modeText(mode.scenario, parameters)}</p>
      {children}
      <div className="grid grid-cols-2 gap-x-8 gap-y-6 mb-6 mx-auto max-w-3xl" data-mode-overview>
        {mode.rules
          .filter((rule) => !rule.advanced)
          .map((rule) => (
            <section key={rule.title} className="flex flex-col items-center gap-3 text-center">
              {rule.icon && <GameIcon iconType={rule.icon} size="large" className="shrink-0" />}
              <div className="min-w-0 max-w-xs">
                <h3 className="font-orbitron text-sm text-white mb-2">{rule.title}</h3>
                <p className="text-sm text-white/70 leading-relaxed">
                  {modeText(rule.text, parameters)}
                </p>
              </div>
            </section>
          ))}
      </div>
      {mode.rules.some((rule) => rule.advanced) && (
        <CollapsibleSection label="Detailed rules">
          <div className="space-y-4 pb-5 max-w-prose">
            {mode.rules
              .filter((rule) => rule.advanced)
              .map((rule) => (
                <section key={rule.title}>
                  <h3 className="font-orbitron text-sm text-white mb-1">{rule.title}</h3>
                  <p className="text-sm text-white/70 leading-relaxed">
                    {modeText(rule.text, parameters)}
                  </p>
                </section>
              ))}
          </div>
        </CollapsibleSection>
      )}
      {Object.keys(mode.stateVisuals ?? {}).length > 0 && (
        <CollapsibleSection label="Card symbols">
          <ModeSymbolLegend mode={mode} />
        </CollapsibleSection>
      )}
    </>
  );

  if (isCompact) {
    return open ? (
      <CompactRules title={`${mode.name} rules`} onClose={onClose}>
        {content}
      </CompactRules>
    ) : null;
  }

  return (
    <GameModal
      isVisible={open}
      onClose={onClose}
      theme="default"
      size="medium"
      animation="fadeIn"
      zIndex={Z_INDEX.LOBBY_SETTINGS_MODAL}
    >
      <div className="flex min-h-0 flex-col text-left text-white">
        <div className="shrink-0 flex items-center justify-between gap-4 px-6 pt-4 pb-3">
          <h2 className="font-orbitron text-xl">{mode.name} rules</h2>
          <CloseButton onClick={onClose} label="Close rules" />
        </div>
        <div className="min-h-0 overflow-y-auto [scrollbar-gutter:stable_both-edges] px-6 pb-6">
          {content}
        </div>
      </div>
    </GameModal>
  );
}
