import type { ComponentType } from "react";
import type { CardDto } from "@/types/generated/api-types";

export interface PreviewProps {
  presetId: string;
  cards: CardDto[];
  onComplete: (message?: string) => void;
}

export interface PreviewDefinition {
  id: string;
  name: string;
  category: string;
  componentPaths: string[];
  keywords?: string[];
  presets: { id: string; name: string }[];
  Component: ComponentType<PreviewProps>;
}
