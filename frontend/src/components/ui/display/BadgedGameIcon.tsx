import GameIcon from "./GameIcon";
import type { ReactNode } from "react";

/** A qualifier stays attached to its subject at every behavior scale. */
export default function BadgedGameIcon({
  icon,
  badges,
  size = "behavior",
  children,
}: {
  icon: string;
  badges: string[];
  size?: "behavior" | "small";
  children?: ReactNode;
}) {
  return (
    <span className="relative inline-flex shrink-0" data-qualified-icon={icon}>
      {children ?? <GameIcon iconType={icon} size={size} />}
      {badges.length > 0 && (
        <span className="absolute -right-1 -bottom-0.5 flex rounded-full bg-[#111b24] [--behavior-icon-size:14px]">
          {badges.map((badge, index) => (
            <GameIcon key={`${badge}-${index}`} iconType={badge} size="behavior" />
          ))}
        </span>
      )}
    </span>
  );
}
