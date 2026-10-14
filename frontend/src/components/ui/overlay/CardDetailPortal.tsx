import { useOverlayPortalRoot } from "@/contexts/OverlayEnvironment";
import type { ReactNode } from "react";
import { createPortal } from "react-dom";
import type { CardDto, PlayerCardDto } from "@/types/generated/api-types.ts";
import { Z_INDEX } from "@/constants/zIndex.ts";
import MobileCardDetail from "../../mobile/MobileCardDetail.tsx";

interface CardDetailPortalProps {
  card: CardDto | PlayerCardDto;
  actions?: ReactNode;
  notice?: ReactNode;
  onClose: () => void;
}

export default function CardDetailPortal({
  card,
  actions,
  notice,
  onClose,
}: CardDetailPortalProps) {
  const portalRoot = useOverlayPortalRoot();
  return createPortal(
    <div className="relative" style={{ zIndex: Z_INDEX.CARD_PREVIEW_OVERLAY }}>
      <MobileCardDetail card={card} notice={notice} onClose={onClose} actions={actions} />
    </div>,
    portalRoot,
  );
}
