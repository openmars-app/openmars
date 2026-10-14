import { useOverlayEventTarget } from "@/contexts/OverlayEnvironment";
import { useEffect, useMemo, useState } from "react";
import BlurredOverlay from "./BlurredOverlay.tsx";
import { Z_INDEX } from "@/constants/zIndex.ts";
import CorporationCard from "../cards/CorporationCard.tsx";
import { PlayerDto, OtherPlayerDto, CardDto } from "@/types/generated/api-types.ts";
import { getCorporationBorderColor } from "@/utils/corporationColors.ts";

interface CorporationOverlayProps {
  visible: boolean;
  onClose: () => void;
  currentPlayer: PlayerDto | null;
  otherPlayers: OtherPlayerDto[];
}

interface CorpEntry {
  name: string;
  color: string;
  corporation: CardDto;
}

export default function CorporationOverlay({
  visible,
  onClose,
  currentPlayer,
  otherPlayers,
}: CorporationOverlayProps) {
  const eventTarget = useOverlayEventTarget();
  const [windowWidth, setWindowWidth] = useState(window.innerWidth);

  useEffect(() => {
    const handleResize = () => setWindowWidth(window.innerWidth);
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);

  useEffect(() => {
    if (!visible) {
      return;
    }
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.stopImmediatePropagation();
        onClose();
      }
    };
    eventTarget.addEventListener("keydown", handleKeyDown);
    return () => eventTarget.removeEventListener("keydown", handleKeyDown);
  }, [visible, onClose, eventTarget]);

  const entries = useMemo(() => {
    const result: CorpEntry[] = [];
    if (currentPlayer?.corporation) {
      result.push({
        name: currentPlayer.name,
        color: currentPlayer.color,
        corporation: currentPlayer.corporation,
      });
    }
    for (const player of otherPlayers) {
      if (player.corporation) {
        result.push({
          name: player.name,
          color: player.color,
          corporation: player.corporation,
        });
      }
    }
    return result;
  }, [currentPlayer, otherPlayers]);

  const scale = useMemo(() => {
    if (entries.length === 0) {
      return 1;
    }
    return Math.min(1, (windowWidth - 80) / (entries.length * 440));
  }, [entries.length, windowWidth]);

  return (
    <BlurredOverlay visible={visible} onClose={onClose} zIndex={Z_INDEX.CORPORATION_OVERLAY}>
      <div
        className="flex items-center justify-center h-full w-full gap-8"
        style={{
          pointerEvents: visible ? "auto" : "none",
          transform: `scale(${scale})`,
          transformOrigin: "center center",
        }}
      >
        {entries.map((entry) => (
          <div key={entry.name} className="flex flex-col items-center gap-3">
            <span
              className="font-orbitron font-bold text-sm tracking-wide"
              style={{ color: entry.color }}
            >
              {entry.name}
            </span>
            <CorporationCard
              card={entry.corporation}
              isSelected={false}
              onSelect={() => {}}
              disableInteraction={true}
              borderColor={getCorporationBorderColor(entry.corporation.name)}
            />
          </div>
        ))}
      </div>
    </BlurredOverlay>
  );
}
