package bot

import (
	"openmars/internal/delivery/dto"
)

// IsMyTurn checks if it's currently the given player's turn to act.
func IsMyTurn(game *dto.GameDto, myPlayerID string) bool {
	if game == nil || game.ResumeLobby != nil || myPlayerID == "" {
		return false
	}

	p := &game.CurrentPlayer

	// Action phase (or final phase) and it's our turn
	if (game.CurrentPhase == dto.GamePhaseAction || game.CurrentPhase == dto.GamePhaseFinalPhase) && game.CurrentTurn != nil && *game.CurrentTurn == myPlayerID {
		return true
	}

	// Player status checks
	if p.Status == dto.PlayerStatusActive {
		return true
	}
	if p.Status == dto.PlayerStatusSelectingStartingCards {
		return true
	}
	if p.Status == dto.PlayerStatusSelectingProductionCards {
		return true
	}

	// Pending selections
	if p.PendingTileSelection != nil {
		return true
	}
	if p.PendingCardSelection != nil {
		return true
	}
	if p.PendingColonySelection != nil || p.PendingColonyResourceSelection != nil || p.PendingAwardFundSelection != nil || p.PendingFreeTradeSelection != nil {
		return true
	}
	if p.PendingCardDrawSelection != nil || p.PendingCardPlaySelection != nil {
		return true
	}
	if len(p.PendingBehaviorResolutions) > 0 {
		return true
	}
	if p.PendingCardReveal != nil {
		return true
	}
	if p.PendingResourceRemovalSelection != nil || p.PendingEffectSelection != nil {
		return true
	}

	// Forced first action
	if p.ForcedFirstAction != nil && game.CurrentPhase == dto.GamePhaseAction {
		return true
	}

	// Starting selection phase — only if player still has choices to make
	if game.CurrentPhase == dto.GamePhaseStartingSelection {
		if p.SelectCorporationPhase != nil || p.SelectStartingCardsPhase != nil || p.SelectPreludeCardsPhase != nil {
			return true
		}
	}

	// Production phase with incomplete selection
	if game.CurrentPhase == dto.GamePhaseProductionAndCardDraw &&
		p.ProductionPhase != nil &&
		!p.ProductionPhase.SelectionComplete {
		return true
	}

	return false
}

// GetPendingActionType returns the type of pending action requiring resolution.
func GetPendingActionType(game *dto.GameDto) string {
	if game == nil {
		return ""
	}
	p := &game.CurrentPlayer

	if p.PendingTileSelection != nil {
		return "tile-selection"
	}
	if p.PendingCardSelection != nil {
		return "card-selection"
	}
	if p.PendingColonySelection != nil {
		return "colony-selection"
	}
	if p.PendingColonyResourceSelection != nil {
		return "colony-resource-selection"
	}
	if p.PendingAwardFundSelection != nil {
		return "award-fund-selection"
	}
	if p.PendingFreeTradeSelection != nil {
		return "free-trade-selection"
	}
	if p.PendingCardPlaySelection != nil {
		return "card-play-selection"
	}
	if p.PendingCardDrawSelection != nil {
		return "card-draw-selection"
	}
	if len(p.PendingBehaviorResolutions) > 0 {
		return "behavior-resolution"
	}
	if p.PendingCardReveal != nil {
		return "card-reveal"
	}
	if p.PendingResourceRemovalSelection != nil || p.PendingEffectSelection != nil {
		return "effect-selection"
	}

	if p.ForcedFirstAction != nil && game.CurrentPhase == dto.GamePhaseAction {
		return "forced-first-action"
	}
	return ""
}
