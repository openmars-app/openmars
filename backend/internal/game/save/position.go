package save

import (
	"fmt"
	"reflect"
	"slices"

	"openmars/internal/game/board"
	"openmars/internal/game/datastore"
	"openmars/internal/game/projectfunding"
	"openmars/internal/game/shared"
)

// validatePosition checks resumable command boundaries. Historical snapshots can
// represent intermediate mutations, so these cross-field checks apply only now.
func (c Catalog) validatePosition(s *datastore.GameState) error {
	for _, name := range []string{"PendingTileSelections", "PendingTileSelectionQueues", "ForcedFirstActions", "ProductionPhases", "SelectCorporationPhases", "SelectStartingCardsPhases", "SelectPreludeCardsPhases", "DeferredStartingChoices"} {
		entries := reflect.ValueOf(s).Elem().FieldByName(name).MapRange()
		for entries.Next() {
			if s.Players[entries.Key().String()] == nil || entries.Value().IsNil() {
				return fmt.Errorf("%s: invalid player or null continuation", name)
			}
		}
	}
	for id, choices := range s.DeferredStartingChoices {
		if choices.PreludesAppliedCount < 0 || choices.PreludesAppliedCount > len(choices.PreludeIDs) || choices.CorporationID != s.Players[id].CorporationID {
			return fmt.Errorf("deferredStartingChoices: inconsistent progress")
		}
	}
	switch s.CurrentPhase {
	case shared.GamePhaseStartingSelection:
		for id, p := range s.Players {
			if p.HasExited || s.DeferredStartingChoices[id] != nil {
				continue
			}
			if s.SelectCorporationPhases[id] == nil || len(s.SelectCorporationPhases[id].AvailableCorporations) == 0 || s.SelectStartingCardsPhases[id] == nil {
				return fmt.Errorf("starting selection: missing choices for %s", id)
			}
			if s.Settings.HasPrelude() && s.SelectPreludeCardsPhases[id] == nil {
				return fmt.Errorf("starting selection: missing preludes")
			}
		}
	case shared.GamePhaseInitApplyCorp, shared.GamePhaseInitApplyPrelude:
		if s.InitPhasePlayerIndex < 0 || s.InitPhasePlayerIndex >= len(s.TurnOrder) {
			return fmt.Errorf("initPhasePlayerIndex: out of range")
		}
		for id, p := range s.Players {
			if !p.HasExited && s.DeferredStartingChoices[id] == nil {
				return fmt.Errorf("init phase: missing saved choices for %s", id)
			}
		}
	case shared.GamePhaseAction, shared.GamePhaseProductionAndCardDraw, shared.GamePhaseFinalPhase:
		for _, p := range s.Players {
			if !p.HasExited && p.CorporationID == "" {
				return fmt.Errorf("active player has no corporation")
			}
		}
	}
	hexes := map[string]bool{}
	for _, tile := range s.Tiles {
		hexes[tile.Coordinates.String()] = true
	}
	for _, selection := range s.PendingTileSelections {
		if !board.ValidPlaceableTileType(selection.TileType) {
			return fmt.Errorf("pendingTileSelections: unknown tile type")
		}
		for _, hex := range selection.AvailableHexes {
			if !hexes[hex] {
				return fmt.Errorf("pendingTileSelections: unknown hex")
			}
		}
	}
	for _, queue := range s.PendingTileSelectionQueues {
		for _, tile := range queue.Items {
			if !board.ValidPlaceableTileType(tile) {
				return fmt.Errorf("pendingTileSelectionQueues: unknown tile type")
			}
		}
	}
	for _, p := range s.Players {
		for id, amount := range p.ResourceStorage {
			if err := c.cardID(id, s); err != nil {
				return err
			}
			if amount < 0 {
				return fmt.Errorf("resourceStorage: negative storage")
			}
		}
		for _, pending := range p.PendingBehaviorResolutions {
			if pending == nil || pending.ID == "" || !slices.Contains([]string{"choice", "card-discard", "optional"}, pending.Kind) || pending.MinCards < 0 || pending.MaxCards < pending.MinCards {
				return fmt.Errorf("pendingBehaviorResolutions: invalid selection")
			}
			if (pending.Kind == "optional") != (pending.OptionalBehavior != nil) {
				return fmt.Errorf("pendingBehaviorResolutions: missing or unexpected optional behavior")
			}
		}
		if pending := p.PendingCardSelection; pending != nil && (pending.MinCards < 0 || pending.MaxCards < pending.MinCards || pending.MaxCards > len(pending.AvailableCards)) {
			return fmt.Errorf("pendingCardSelection: invalid count")
		}
		if pending := p.PendingCardDrawSelection; pending != nil && (pending.MinFreeTakeCount < 0 || pending.FreeTakeCount < pending.MinFreeTakeCount || pending.MaxBuyCount < 0 || pending.CardBuyCost < 0) {
			return fmt.Errorf("pendingCardDrawSelection: invalid count or price")
		}
		if pending := p.PendingResourceRemovalSelection; pending != nil && (pending.Output == nil || pending.Amount < 0) {
			return fmt.Errorf("pendingResourceRemovalSelection: invalid removal")
		}
	}
	seenAwards := map[shared.AwardType]bool{}
	for _, funded := range s.FundedAwards {
		if seenAwards[funded.Type] {
			return fmt.Errorf("fundedAwards: duplicate award")
		}
		seenAwards[funded.Type] = true
		if !slices.Contains(s.SelectedAwards, string(funded.Type)) {
			return fmt.Errorf("fundedAwards: award not selected")
		}
	}
	seenMilestones := map[shared.MilestoneType]bool{}
	for _, claimed := range s.ClaimedMilestones {
		if seenMilestones[claimed.Type] {
			return fmt.Errorf("claimedMilestones: duplicate milestone")
		}
		seenMilestones[claimed.Type] = true
		if !slices.Contains(s.SelectedMilestones, string(claimed.Type)) {
			return fmt.Errorf("claimedMilestones: milestone not selected")
		}
	}
	for _, project := range s.ProjectFundingStates {
		if project == nil {
			return fmt.Errorf("projectFundingStates: null project")
		}
		definition, err := c.Projects.GetByID(project.DefinitionID)
		if err != nil {
			return err
		}
		seats := projectfunding.ScaledSeatCount(len(definition.Seats), len(s.Players))
		if len(project.SeatOwners) > seats || (project.IsCompleted && len(project.SeatOwners) != seats) {
			return fmt.Errorf("projectFundingStates: invalid progress")
		}
	}
	return nil
}
