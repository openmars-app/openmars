import React, { useMemo } from "react";
import { PlayerDto, OtherPlayerDto, GlobalParameterBonusDto } from "@/types/generated/api-types.ts";
import GameIcon from "../../ui/display/GameIcon.tsx";
import { Z_INDEX, getZIndex } from "@/constants/zIndex.ts";
import { useTapReveal } from "@/hooks/useTapReveal.ts";

interface GlobalParameters {
  temperature: number;
  oxygen: number;
  oceans: number;
  maxOceans: number;
  venus: number;
  bonuses?: GlobalParameterBonusDto[];
}

interface RightSidebarProps {
  globalParameters?: GlobalParameters;
  generation?: number;
  currentPlayer?: PlayerDto | OtherPlayerDto | null;
  showVenus?: boolean;
}

const ANGLE_INDENT = 20;
const BORDER_COLOR = "rgba(60,60,70,0.7)";
const THICK_BORDER_COLOR = "rgba(80,80,90,0.9)";
const BACKGROUND_COLOR = "black";
const SIDEBAR_WIDTH = 65;
const GAUGE_GAP = 2;

interface GenerationPanelProps {
  generation: number;
  width: number;
  height: number;
}

const GenerationPanel: React.FC<GenerationPanelProps> = ({ generation, width, height }) => {
  const fillPoints = `0,${ANGLE_INDENT} ${width},0 ${width},${height - ANGLE_INDENT} 0,${height}`;

  return (
    <div
      className="relative pointer-events-auto transition-[width] duration-300 ease-out"
      style={{ width, height }}
    >
      <svg
        className="absolute inset-0 w-full h-full"
        viewBox={`0 0 ${width} ${height}`}
        preserveAspectRatio="none"
      >
        <polygon points={fillPoints} fill={BACKGROUND_COLOR} />
        <line x1={0} y1={ANGLE_INDENT} x2={width} y2={0} stroke={BORDER_COLOR} strokeWidth="2" />
        <line
          x1={width}
          y1={0}
          x2={width}
          y2={height - ANGLE_INDENT}
          stroke={BORDER_COLOR}
          strokeWidth="2"
        />
        <line
          x1={width}
          y1={height - ANGLE_INDENT}
          x2={0}
          y2={height}
          stroke={THICK_BORDER_COLOR}
          strokeWidth="4"
        />
        <line x1={0} y1={height} x2={0} y2={ANGLE_INDENT} stroke={BORDER_COLOR} strokeWidth="2" />
      </svg>
      <div
        className="relative h-full flex flex-col items-center justify-center"
        style={{ zIndex: getZIndex("LOCAL", 10) }}
      >
        <div className="text-[10px] font-orbitron font-bold text-white/70 uppercase tracking-[1px]">
          GEN
        </div>
        <div className="text-xl font-orbitron font-bold text-white [text-shadow:0_0_8px_rgba(255,255,255,0.3)]">
          {generation}
        </div>
      </div>
    </div>
  );
};

const REWARD_ICON_MAP: Record<string, string> = {
  "heat-production": "heat-production",
  "ocean-placement": "ocean",
  temperature: "temperature",
  "card-draw": "card-draw",
  tr: "tr",
};

interface BonusIconProps {
  rewardType: string;
  isHovered: boolean;
}

const BonusIcon: React.FC<BonusIconProps> = ({ rewardType, isHovered }) => {
  const iconType = REWARD_ICON_MAP[rewardType] ?? rewardType;
  const isProduction = iconType.endsWith("-production");
  const baseScale = isProduction ? 0.5 : 0.65;
  const hoverScale = isProduction ? 0.6 : 0.75;

  return (
    <div
      className="transition-all duration-300"
      style={{ transform: `scale(${isHovered ? hoverScale : baseScale})` }}
    >
      <GameIcon iconType={iconType} size="small" />
    </div>
  );
};

interface GaugesSectionProps {
  oxygen: number;
  temperature: number;
  width: number;
  isHovered: boolean;
  bonuses?: GlobalParameterBonusDto[];
}

const GaugesSection: React.FC<GaugesSectionProps> = ({
  oxygen,
  temperature,
  width,
  isHovered,
  bonuses,
}) => {
  const oxygenPercent = Math.max(0, (oxygen / 14) * 100);
  const temperaturePercent = Math.max(0, ((temperature + 30) / 38) * 100);

  const gaugeWidth = (width - GAUGE_GAP) / 2;

  const oxygenSteps = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14];
  const tempSteps = [
    -30, -28, -26, -24, -22, -20, -18, -16, -14, -12, -10, -8, -6, -4, -2, 0, 2, 4, 6, 8,
  ];

  const oxygenBonusMap = useMemo(() => {
    const map = new Map<number, GlobalParameterBonusDto>();
    if (bonuses) {
      for (const b of bonuses) {
        if (b.parameter === "oxygen") {
          map.set(b.threshold, b);
        }
      }
    }
    return map;
  }, [bonuses]);

  const tempBonusMap = useMemo(() => {
    const map = new Map<number, GlobalParameterBonusDto>();
    if (bonuses) {
      for (const b of bonuses) {
        if (b.parameter === "temperature") {
          map.set(b.threshold, b);
        }
      }
    }
    return map;
  }, [bonuses]);

  return (
    <div
      className="relative pointer-events-auto flex-1 transition-[width] duration-300 ease-out"
      style={{ width }}
    >
      <svg
        className="absolute inset-0 w-full h-full transition-all duration-300 ease-out"
        preserveAspectRatio="none"
      >
        <line
          x1={0}
          y1={ANGLE_INDENT}
          x2={width}
          y2={0}
          stroke={THICK_BORDER_COLOR}
          strokeWidth="4"
        />
        <line x1={width} y1={0} x2={width} y2="100%" stroke={BORDER_COLOR} strokeWidth="2" />
        <line
          x1={width}
          y1="100%"
          x2={0}
          y2={`calc(100% - ${ANGLE_INDENT}px)`}
          stroke={BORDER_COLOR}
          strokeWidth="2"
        />
        <line
          x1={0}
          y1={`calc(100% - ${ANGLE_INDENT}px)`}
          x2={0}
          y2={ANGLE_INDENT}
          stroke={BORDER_COLOR}
          strokeWidth="2"
        />
      </svg>

      <div className="relative h-full flex" style={{ zIndex: getZIndex("LOCAL", 10) }}>
        {/* Oxygen Gauge */}
        <div
          className="relative h-full bg-black overflow-hidden transition-[width] duration-300 ease-out"
          style={{ width: gaugeWidth, borderLeft: `${GAUGE_GAP}px solid ${BORDER_COLOR}` }}
        >
          <div
            className="absolute bottom-0 left-0 right-0 bg-[linear-gradient(to_top,#006400_0%,#32cd32_50%,#00ff00_100%)] transition-[height] duration-500 ease-[ease] shadow-[0_0_8px_rgba(0,255,0,1),0_0_15px_rgba(50,205,50,0.8),inset_0_1px_2px_rgba(255,255,255,0.3)]"
            style={{ height: `${oxygenPercent}%` }}
          />
          <div className="absolute inset-0 pointer-events-none transition-opacity duration-300">
            {oxygenSteps.map((o) => {
              if (o === 0 || o === 14) return null;
              const position = (o / 14) * 100;
              const bonus = oxygenBonusMap.get(o);
              const showBonus = bonus && oxygen < o;
              return (
                <div
                  key={o}
                  className="absolute w-full transition-opacity duration-300"
                  style={{
                    bottom: `${position}%`,
                    transform: "translateY(50%)",
                    opacity: oxygen >= o ? 0.3 : 1,
                  }}
                >
                  {showBonus && !isHovered ? (
                    <div className="flex items-center justify-center">
                      <BonusIcon rewardType={bonus.rewardType} isHovered={false} />
                    </div>
                  ) : (
                    <div
                      className={`text-[9px] font-orbitron font-bold text-[#00ff00] [text-shadow:0_0_3px_rgba(0,255,0,0.8)] text-center transition-opacity duration-300 ${isHovered ? "opacity-0" : "opacity-100"}`}
                    >
                      {o}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
          {/* Current value indicator */}
          <div
            className={`absolute inset-0 w-full flex items-center justify-center text-sm font-orbitron font-bold text-white [text-shadow:0_0_4px_rgba(0,0,0,1),0_0_8px_rgba(0,0,0,0.8)] pointer-events-none transition-opacity duration-300 ${isHovered ? "opacity-100" : "opacity-0"}`}
            style={{ zIndex: getZIndex("LOCAL", 20) }}
          >
            {oxygen}%
          </div>
        </div>

        {/* Gap */}
        <div style={{ width: GAUGE_GAP, backgroundColor: BORDER_COLOR }} />

        {/* Temperature Gauge */}
        <div
          className="relative h-full bg-black overflow-hidden transition-[width] duration-300 ease-out"
          style={{ width: gaugeWidth }}
        >
          <div
            className="absolute bottom-0 left-0 right-0 bg-[linear-gradient(to_top,#87ceeb_0%,#ffb347_50%,#ff8c00_100%)] transition-[height] duration-500 ease-[ease] shadow-[0_0_8px_rgba(255,140,0,1),0_0_15px_rgba(255,179,71,0.8),inset_0_1px_2px_rgba(255,255,255,0.3)]"
            style={{ height: `${temperaturePercent}%` }}
          />
          <div className="absolute inset-0 pointer-events-none transition-opacity duration-300">
            {tempSteps.map((t) => {
              if (t === -30 || t === 8) return null;
              const position = ((t + 30) / 38) * 100;
              const bonus = tempBonusMap.get(t);
              const showBonus = bonus && temperature < t;
              return (
                <div
                  key={t}
                  className="absolute w-full transition-opacity duration-300"
                  style={{
                    bottom: `${position}%`,
                    transform: "translateY(50%)",
                    opacity: temperature >= t ? 0.3 : 1,
                  }}
                >
                  {showBonus && !isHovered ? (
                    <div className="flex items-center justify-center">
                      <BonusIcon rewardType={bonus.rewardType} isHovered={false} />
                    </div>
                  ) : (
                    <div
                      className={`text-[9px] font-orbitron font-bold text-[#ff8c00] [text-shadow:0_0_3px_rgba(255,140,0,0.8)] text-center transition-opacity duration-300 ${isHovered ? "opacity-0" : "opacity-100"}`}
                    >
                      {t}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
          {/* Current value indicator */}
          <div
            className={`absolute inset-0 w-full flex items-center justify-center text-sm font-orbitron font-bold text-white [text-shadow:0_0_4px_rgba(0,0,0,1),0_0_8px_rgba(0,0,0,0.8)] pointer-events-none transition-opacity duration-300 ${isHovered ? "opacity-100" : "opacity-0"}`}
            style={{ zIndex: getZIndex("LOCAL", 20) }}
          >
            {temperature}°
          </div>
        </div>
      </div>
    </div>
  );
};

const VENUS_ANGLE = 10;
const VENUS_CAP_HEIGHT = 30;

interface VenusCapProps {
  width: number;
}

const VenusTopCap: React.FC<VenusCapProps> = ({ width }) => {
  const h = VENUS_CAP_HEIGHT;
  const a = VENUS_ANGLE;
  const fillPoints = `0,${a} ${width},0 ${width},${h - a} 0,${h}`;

  return (
    <div className="relative transition-[width] duration-300 ease-out" style={{ width, height: h }}>
      <svg
        className="absolute inset-0 w-full h-full"
        viewBox={`0 0 ${width} ${h}`}
        preserveAspectRatio="none"
      >
        <polygon points={fillPoints} fill={BACKGROUND_COLOR} />
        <line x1={0} y1={a} x2={width} y2={0} stroke={BORDER_COLOR} strokeWidth="2" />
        <line x1={width} y1={0} x2={width} y2={h - a} stroke={BORDER_COLOR} strokeWidth="2" />
        <line x1={width} y1={h - a} x2={0} y2={h} stroke={THICK_BORDER_COLOR} strokeWidth="4" />
        <line x1={0} y1={h} x2={0} y2={a} stroke={BORDER_COLOR} strokeWidth="2" />
      </svg>
    </div>
  );
};

const VenusBottomCap: React.FC<VenusCapProps> = ({ width }) => {
  const h = VENUS_CAP_HEIGHT;
  const a = VENUS_ANGLE;
  const fillPoints = `0,0 ${width},${a} ${width},${h} 0,${h - a}`;

  return (
    <div className="relative transition-[width] duration-300 ease-out" style={{ width, height: h }}>
      <svg
        className="absolute inset-0 w-full h-full"
        viewBox={`0 0 ${width} ${h}`}
        preserveAspectRatio="none"
      >
        <polygon points={fillPoints} fill={BACKGROUND_COLOR} />
        <line x1={0} y1={0} x2={width} y2={a} stroke={THICK_BORDER_COLOR} strokeWidth="4" />
        <line x1={width} y1={a} x2={width} y2={h} stroke={BORDER_COLOR} strokeWidth="2" />
        <line x1={width} y1={h} x2={0} y2={h - a} stroke={BORDER_COLOR} strokeWidth="2" />
        <line x1={0} y1={h - a} x2={0} y2={0} stroke={BORDER_COLOR} strokeWidth="2" />
      </svg>
      <div
        className="relative h-full flex items-center justify-center"
        style={{ zIndex: getZIndex("LOCAL", 10) }}
      >
        <div style={{ transform: "translateY(2px) scale(0.7)" }}>
          <GameIcon iconType="venus" size="small" />
        </div>
      </div>
    </div>
  );
};

interface VenusGaugeSectionProps {
  venus: number;
  width: number;
  isHovered: boolean;
  bonuses?: GlobalParameterBonusDto[];
}

const VenusGaugeSection: React.FC<VenusGaugeSectionProps> = ({
  venus,
  width,
  isHovered,
  bonuses,
}) => {
  const venusPercent = Math.max(0, (venus / 30) * 100);
  const venusSteps = [0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 22, 24, 26, 28, 30];

  const venusBonusMap = useMemo(() => {
    const map = new Map<number, GlobalParameterBonusDto>();
    if (bonuses) {
      for (const b of bonuses) {
        if (b.parameter === "venus") {
          map.set(b.threshold, b);
        }
      }
    }
    return map;
  }, [bonuses]);

  return (
    <div
      className="relative pointer-events-auto flex-1 transition-[width] duration-300 ease-out"
      style={{ width }}
    >
      <svg className="absolute inset-0 w-full h-full" preserveAspectRatio="none">
        <line
          x1={0}
          y1={VENUS_ANGLE}
          x2={width}
          y2={0}
          stroke={THICK_BORDER_COLOR}
          strokeWidth="4"
        />
        <line x1={width} y1={0} x2={width} y2="100%" stroke={BORDER_COLOR} strokeWidth="2" />
        <line
          x1={width}
          y1="100%"
          x2={0}
          y2={`calc(100% - ${VENUS_ANGLE}px)`}
          stroke={BORDER_COLOR}
          strokeWidth="2"
        />
        <line
          x1={0}
          y1={`calc(100% - ${VENUS_ANGLE}px)`}
          x2={0}
          y2={VENUS_ANGLE}
          stroke={BORDER_COLOR}
          strokeWidth="2"
        />
      </svg>

      <div
        className="relative h-full bg-black overflow-hidden"
        style={{
          borderLeft: `${GAUGE_GAP}px solid ${BORDER_COLOR}`,
          zIndex: getZIndex("LOCAL", 10),
        }}
      >
        <div
          className="absolute bottom-0 left-0 right-0 bg-[linear-gradient(to_top,#8B6914_0%,#DAA520_50%,#FFD700_100%)] transition-[height] duration-500 ease-[ease] shadow-[0_0_8px_rgba(255,215,0,1),0_0_15px_rgba(218,165,32,0.8),inset_0_1px_2px_rgba(255,255,255,0.3)]"
          style={{ height: `${venusPercent}%` }}
        />
        <div className="absolute inset-0 pointer-events-none transition-opacity duration-300">
          {venusSteps.map((v) => {
            if (v === 0 || v === 30) return null;
            const position = (v / 30) * 100;
            const bonus = venusBonusMap.get(v);
            const showBonus = bonus && venus < v;
            return (
              <div
                key={v}
                className="absolute w-full transition-opacity duration-300"
                style={{
                  bottom: `${position}%`,
                  transform: "translateY(50%)",
                  opacity: venus >= v ? 0.3 : 1,
                }}
              >
                {showBonus && !isHovered ? (
                  <div className="flex items-center justify-center">
                    <BonusIcon rewardType={bonus.rewardType} isHovered={false} />
                  </div>
                ) : (
                  <div
                    className={`text-[9px] font-orbitron font-bold text-[#FFD700] [text-shadow:0_0_3px_rgba(255,215,0,0.8)] text-center transition-opacity duration-300 ${isHovered ? "opacity-0" : "opacity-100"}`}
                  >
                    {v}
                  </div>
                )}
              </div>
            );
          })}
        </div>
        <div
          className={`absolute inset-0 w-full flex items-center justify-center text-sm font-orbitron font-bold text-white [text-shadow:0_0_4px_rgba(0,0,0,1),0_0_8px_rgba(0,0,0,0.8)] pointer-events-none transition-opacity duration-300 ${isHovered ? "opacity-100" : "opacity-0"}`}
          style={{ zIndex: getZIndex("LOCAL", 20) }}
        >
          {venus}%
        </div>
      </div>
    </div>
  );
};

interface GaugeLegendPanelProps {
  width: number;
  height: number;
}

const GaugeLegendPanel: React.FC<GaugeLegendPanelProps> = ({ width, height }) => {
  const fillPoints = `0,0 ${width},${ANGLE_INDENT} ${width},${height} 0,${height - ANGLE_INDENT}`;
  const gaugeWidth = (width - GAUGE_GAP) / 2;

  return (
    <div
      className="relative pointer-events-auto transition-[width] duration-300 ease-out"
      style={{ width, height }}
    >
      <svg
        className="absolute inset-0 w-full h-full"
        viewBox={`0 0 ${width} ${height}`}
        preserveAspectRatio="none"
      >
        <polygon points={fillPoints} fill={BACKGROUND_COLOR} />
        <line
          x1={0}
          y1={0}
          x2={width}
          y2={ANGLE_INDENT}
          stroke={THICK_BORDER_COLOR}
          strokeWidth="4"
        />
        <line
          x1={width}
          y1={ANGLE_INDENT}
          x2={width}
          y2={height}
          stroke={BORDER_COLOR}
          strokeWidth="2"
        />
        <line
          x1={width}
          y1={height}
          x2={0}
          y2={height - ANGLE_INDENT}
          stroke={BORDER_COLOR}
          strokeWidth="2"
        />
        <line
          x1={0}
          y1={height - ANGLE_INDENT}
          x2={0}
          y2={0}
          stroke={BORDER_COLOR}
          strokeWidth="2"
        />
      </svg>
      <div
        className="relative h-full flex items-center justify-center"
        style={{ zIndex: getZIndex("LOCAL", 10) }}
      >
        <div
          className="flex items-center transition-[width] duration-300 ease-out"
          style={{
            width: gaugeWidth,
            justifyContent: "center",
            transform: "translateY(-5px) scale(0.8)",
          }}
        >
          <GameIcon iconType="oxygen" size="small" />
        </div>
        <div style={{ width: GAUGE_GAP }} />
        <div
          className="flex items-center transition-[width] duration-300 ease-out"
          style={{
            width: gaugeWidth,
            justifyContent: "center",
            transform: "translateY(5px) scale(0.8)",
          }}
        >
          <GameIcon iconType="temperature" size="small" />
        </div>
      </div>
    </div>
  );
};

interface OceansPanelProps {
  oceans: number;
  maxOceans: number;
  width: number;
  height: number;
}

const OceansPanel: React.FC<OceansPanelProps> = ({ oceans, maxOceans, width, height }) => {
  const fillPoints = `0,0 ${width},${ANGLE_INDENT} ${width},${height} 0,${height - ANGLE_INDENT}`;

  return (
    <div
      className="relative pointer-events-auto transition-[width] duration-300 ease-out"
      style={{ width, height }}
    >
      <svg
        className="absolute inset-0 w-full h-full"
        viewBox={`0 0 ${width} ${height}`}
        preserveAspectRatio="none"
      >
        <polygon points={fillPoints} fill={BACKGROUND_COLOR} />
        <line x1={0} y1={0} x2={width} y2={ANGLE_INDENT} stroke={BORDER_COLOR} strokeWidth="2" />
        <line
          x1={width}
          y1={ANGLE_INDENT}
          x2={width}
          y2={height}
          stroke={BORDER_COLOR}
          strokeWidth="2"
        />
        <line
          x1={width}
          y1={height}
          x2={0}
          y2={height - ANGLE_INDENT}
          stroke={BORDER_COLOR}
          strokeWidth="2"
        />
        <line
          x1={0}
          y1={height - ANGLE_INDENT}
          x2={0}
          y2={0}
          stroke={BORDER_COLOR}
          strokeWidth="2"
        />
      </svg>
      <div
        className="relative h-full flex flex-col items-center justify-center gap-1"
        style={{ zIndex: getZIndex("LOCAL", 10) }}
      >
        <div className="flex items-center justify-center w-6 h-6 brightness-[1.2]">
          <GameIcon iconType="ocean" size="small" />
        </div>
        <div className="flex items-center font-orbitron text-sm font-bold">
          <span className="text-[#00bfff] [text-shadow:0_0_3px_rgba(0,191,255,0.6)]">{oceans}</span>
          <span className="text-[#666]"> / </span>
          <span className="text-[#999]">{maxOceans}</span>
        </div>
      </div>
    </div>
  );
};

const RightSidebar: React.FC<RightSidebarProps> = ({
  globalParameters,
  generation,
  currentPlayer: _currentPlayer,
  showVenus = false,
}) => {
  const { open: isExpanded, triggerProps } = useTapReveal<HTMLDivElement>();

  const GEN_PANEL_HEIGHT = 80;
  const LEGEND_PANEL_HEIGHT = 60;
  const OCEANS_PANEL_HEIGHT = 80;

  const currentWidth = isExpanded ? Math.round(SIDEBAR_WIDTH * 1.5) : SIDEBAR_WIDTH;
  const venusWidth = (currentWidth - GAUGE_GAP) / 2;

  return (
    <div
      {...triggerProps}
      className="fixed right-0 flex flex-row items-center pointer-events-auto top-1/2 -translate-y-1/2 transition-all duration-300 ease-out"
      style={{ height: "70dvh", zIndex: Z_INDEX.RIGHT_SIDEBAR }}
    >
      {showVenus && (
        <div
          className="flex flex-col self-center"
          style={{
            height: "55%",
            marginRight: 0,
            marginTop: (-(GEN_PANEL_HEIGHT - ANGLE_INDENT) * 2) / 3,
          }}
        >
          <div
            style={{
              marginBottom: -VENUS_ANGLE,
              zIndex: getZIndex("LOCAL", 2),
              position: "relative",
            }}
          >
            <VenusTopCap width={venusWidth} />
          </div>
          <div
            style={{
              zIndex: getZIndex("LOCAL", 1),
              position: "relative",
              flex: 1,
              display: "flex",
              flexDirection: "column",
            }}
          >
            <VenusGaugeSection
              venus={globalParameters?.venus ?? 0}
              width={venusWidth}
              isHovered={isExpanded}
              bonuses={globalParameters?.bonuses}
            />
          </div>
          <div
            style={{ marginTop: -VENUS_ANGLE, zIndex: getZIndex("LOCAL", 2), position: "relative" }}
          >
            <VenusBottomCap width={venusWidth} />
          </div>
        </div>
      )}

      <div className="flex flex-col h-full">
        <div
          style={{
            marginBottom: -ANGLE_INDENT,
            zIndex: getZIndex("LOCAL", 2),
            position: "relative",
          }}
        >
          <GenerationPanel
            generation={generation || 1}
            width={currentWidth}
            height={GEN_PANEL_HEIGHT}
          />
        </div>
        <div
          style={{
            zIndex: getZIndex("LOCAL", 1),
            position: "relative",
            flex: 1,
            display: "flex",
            flexDirection: "column",
          }}
        >
          <GaugesSection
            oxygen={globalParameters?.oxygen ?? 0}
            temperature={globalParameters?.temperature ?? -30}
            width={currentWidth}
            isHovered={isExpanded}
            bonuses={globalParameters?.bonuses}
          />
        </div>
        <div
          style={{ marginTop: -ANGLE_INDENT, zIndex: getZIndex("LOCAL", 2), position: "relative" }}
        >
          <GaugeLegendPanel width={currentWidth} height={LEGEND_PANEL_HEIGHT} />
        </div>
        <div
          style={{ marginTop: -ANGLE_INDENT, zIndex: getZIndex("LOCAL", 2), position: "relative" }}
        >
          <OceansPanel
            oceans={globalParameters?.oceans ?? 0}
            maxOceans={globalParameters?.maxOceans ?? 9}
            width={currentWidth}
            height={OCEANS_PANEL_HEIGHT}
          />
        </div>
      </div>
    </div>
  );
};

export default RightSidebar;
