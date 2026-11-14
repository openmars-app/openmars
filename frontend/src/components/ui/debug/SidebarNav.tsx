import React from "react";
import { useSoundEffects } from "@/hooks/useSoundEffects.ts";

export type ActiveItem =
  | "game-state"
  | "player-resources"
  | "player-behavior"
  | "place-tile"
  | "game-commands"
  | "turn-commands"
  | "bots";

interface SidebarNavProps {
  activeItem: ActiveItem;
  onSelectItem: (item: ActiveItem) => void;
  developmentMode: boolean;
}

const SidebarNav: React.FC<SidebarNavProps> = ({ activeItem, onSelectItem, developmentMode }) => {
  const { playButtonClickSound } = useSoundEffects();
  const handleSelectItem = (item: ActiveItem) => {
    void playButtonClickSound();
    onSelectItem(item);
  };

  const itemStyle = (isActive: boolean) => ({
    padding: "6px 12px",
    fontSize: "11px",
    color: isActive ? "#3b82f6" : "#abb2bf",
    background: isActive ? "rgba(59, 130, 246, 0.2)" : "transparent",
    borderLeft: isActive ? "2px solid #3b82f6" : "2px solid transparent",
    cursor: "pointer" as const,
    transition: "all 0.15s ease",
    userSelect: "none" as const,
    textAlign: "left" as const,
  });

  const subItemStyle = (isActive: boolean) => ({
    ...itemStyle(isActive),
    paddingLeft: "24px",
    fontSize: "11px",
  });

  const headerStyle = {
    padding: "6px 12px",
    fontSize: "11px",
    color: "#666",
    borderLeft: "2px solid transparent",
    userSelect: "none" as const,
    textAlign: "left" as const,
    cursor: "default" as const,
  };

  return (
    <div
      style={{
        width: "180px",
        minWidth: "180px",
        background: "rgba(20, 20, 20, 0.5)",
        borderRight: "1px solid #333",
        overflow: "auto",
        display: "flex",
        flexDirection: "column" as const,
      }}
      onMouseDown={(e) => e.stopPropagation()}
    >
      <div
        onClick={() => handleSelectItem("game-state")}
        style={itemStyle(activeItem === "game-state")}
      >
        Game State
      </div>

      {developmentMode && (
        <>
          <div style={headerStyle}>Commands</div>

          <div
            onClick={() => handleSelectItem("player-resources")}
            style={subItemStyle(activeItem === "player-resources")}
          >
            Player Resources
          </div>
          <div
            onClick={() => handleSelectItem("player-behavior")}
            style={subItemStyle(activeItem === "player-behavior")}
          >
            Card & Corp
          </div>
          <div
            onClick={() => handleSelectItem("place-tile")}
            style={subItemStyle(activeItem === "place-tile")}
          >
            Place Tile
          </div>
          <div
            onClick={() => handleSelectItem("game-commands")}
            style={subItemStyle(activeItem === "game-commands")}
          >
            Game
          </div>
          <div
            onClick={() => handleSelectItem("turn-commands")}
            style={subItemStyle(activeItem === "turn-commands")}
          >
            Turn
          </div>

          <div style={headerStyle}>Bots</div>

          <div onClick={() => handleSelectItem("bots")} style={subItemStyle(activeItem === "bots")}>
            Inspector
          </div>
        </>
      )}
    </div>
  );
};

export default SidebarNav;
