import type { ReactNode } from "react";
import type { GameModeDto } from "@/types/generated/api-types.ts";
import { GameModal } from "../GameModal";
import CloseButton from "../buttons/CloseButton.tsx";
import { Z_INDEX } from "@/constants/zIndex.ts";

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

export default function GameModeRules({ mode, parameters, open, onClose, children }: Props) {
  return (
    <GameModal
      isVisible={open}
      onClose={onClose}
      theme="default"
      size="medium"
      animation="fadeIn"
      zIndex={Z_INDEX.LOBBY_SETTINGS_MODAL}
    >
      <div className="p-6 compact:p-4 overflow-y-auto max-h-[85dvh] text-left text-white">
        <div className="flex items-center justify-between gap-4 mb-5">
          <h2 className="font-orbitron text-xl">{mode.name} rules</h2>
          <CloseButton onClick={onClose} label="Close rules" />
        </div>
        <p className="text-white/80 leading-relaxed mb-6">{modeText(mode.scenario, parameters)}</p>
        {children}
        <div className="space-y-5">
          {mode.rules.map((rule) => (
            <section key={rule.title}>
              <h3 className="font-orbitron text-sm text-white mb-2">{rule.title}</h3>
              <p className="text-sm text-white/70 whitespace-pre-line leading-relaxed">
                {modeText(rule.text, parameters)}
              </p>
            </section>
          ))}
        </div>
      </div>
    </GameModal>
  );
}
