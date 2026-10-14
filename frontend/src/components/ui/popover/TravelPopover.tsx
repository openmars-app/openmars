import GameButton from "@/components/ui/buttons/GameButton.tsx";
import React from "react";
import GamePopover from "../GamePopover/GamePopover";
import { usePlanetFocus, type PlanetTarget } from "../../../contexts/PlanetFocusContext";

interface TravelDestination {
  id: PlanetTarget;
  name: string;
}

const DESTINATIONS: TravelDestination[] = [
  { id: "mars", name: "Mars" },
  { id: "solar-system", name: "Solar System" },
  { id: "mercury", name: "Mercury" },
  { id: "venus", name: "Venus" },
  { id: "earth", name: "Earth" },
  { id: "ceres", name: "Ceres" },
  { id: "jupiter", name: "Jupiter" },
  { id: "saturn", name: "Saturn" },
  { id: "uranus", name: "Uranus" },
  { id: "neptune", name: "Neptune" },
];

interface TravelPopoverProps {
  isVisible: boolean;
  onClose: () => void;
  anchorRef: React.RefObject<HTMLElement | null>;
}

export default function TravelPopover({ isVisible, onClose, anchorRef }: TravelPopoverProps) {
  const { activePlanet, setActivePlanet } = usePlanetFocus();

  return (
    <TravelPopoverView
      isVisible={isVisible}
      onClose={onClose}
      anchorRef={anchorRef}
      activePlanet={activePlanet}
      onTravel={setActivePlanet}
    />
  );
}

export function TravelPopoverView({
  isVisible,
  onClose,
  anchorRef,
  activePlanet,
  onTravel,
}: TravelPopoverProps & { activePlanet: PlanetTarget; onTravel: (planet: PlanetTarget) => void }) {
  return (
    <GamePopover
      isVisible={isVisible}
      onClose={onClose}
      position={{ type: "anchor", anchorRef, placement: "below" }}
      theme="menu"
      excludeRef={anchorRef}
      width={220}
      maxHeight={500}
      animation="slideDown"
      className="game-popover-list"
    >
      <div className="popover-list p-2 flex flex-col gap-1">
        {DESTINATIONS.map((dest) => {
          const isCurrent = dest.id === activePlanet;
          return (
            <GameButton
              emphasis="quiet"
              key={dest.id}
              style={
                {
                  "--item-border": "#ffffff30",
                  "--item-surface": isCurrent ? "#141416" : "#0c0c0e",
                } as React.CSSProperties
              }
              onClick={() => {
                if (!isCurrent) {
                  onTravel(dest.id);
                }
                onClose();
              }}
              className={`popover-list-item flex items-center justify-start text-left px-3 py-2 rounded-none border transition-all duration-150 ${
                isCurrent
                  ? "border-white/30 bg-white/10 opacity-50 cursor-default"
                  : "border-white/10 bg-white/5 hover:border-white/30 hover:bg-white/15 cursor-pointer"
              }`}
            >
              <span className="font-orbitron text-sm text-white/90">{dest.name}</span>
            </GameButton>
          );
        })}
      </div>
    </GamePopover>
  );
}
