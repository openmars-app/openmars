import type { CardDescriptionSectionDto } from "@/types/generated/api-types.ts";
import {
  getCardDescriptionSectionLabel,
  formatRequirementDescription,
} from "@/utils/cardDescription.ts";
import { FormattedDescription } from "./FormattedDescription.tsx";

interface CardDescriptionSectionsProps {
  sections: CardDescriptionSectionDto[];
}

export const CardDescriptionSections = ({ sections }: CardDescriptionSectionsProps) => (
  <div className="flex flex-col gap-1">
    {sections.map((section, i) => {
      const label = getCardDescriptionSectionLabel(section.type);
      return (
        <p key={i} className="m-0">
          {label && <strong>{label}: </strong>}
          <FormattedDescription
            text={
              section.type === "requirement"
                ? formatRequirementDescription(section.text)
                : section.text
            }
          />
        </p>
      );
    })}
  </div>
);
