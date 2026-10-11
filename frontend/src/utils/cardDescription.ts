import type { CardDescriptionSectionDto } from "@/types/generated/api-types.ts";

type SectionType = CardDescriptionSectionDto["type"];

const SECTION_LABELS: Record<SectionType, string | null> = {
  generic: null,
  effect: "Effect",
  action: "Action",
  requirement: "Requirement",
};

export const getCardDescriptionSectionLabel = (type: SectionType): string | null =>
  SECTION_LABELS[type];

export const cardDescriptionToPlainText = (sections: CardDescriptionSectionDto[]): string =>
  sections
    .map((section) => {
      const label = SECTION_LABELS[section.type];
      const text =
        section.type === "requirement" ? formatRequirementDescription(section.text) : section.text;
      return label ? `${label}: ${text}` : text;
    })
    .join("\n");

export function formatRequirementDescription(text: string): string {
  const value = text
    .trim()
    .replace(/\.$/, "")
    .replace(/^requires\s+/i, "");
  if (/^(must be played )?before withdrawal$/i.test(value)) return "Before withdrawal.";
  const conditions = value
    .split(/;\s*/)
    .filter((part) => !(/^after withdrawal$/i.test(part) && /Earth disconnected/i.test(value)));
  const description = conditions.join(" and ");
  return description ? `${description[0].toUpperCase()}${description.slice(1)}.` : "";
}
