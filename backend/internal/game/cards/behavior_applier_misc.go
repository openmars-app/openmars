package cards

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"openmars/internal/game/shared"
)

func (a *BehaviorApplier) applyColonyOutput(ctx context.Context, o *shared.ColonyCondition, amount int, log *slog.Logger) error {
	switch o.ResourceType {
	case shared.ResourceTradeFleet:
		if a.game == nil || a.player == nil || !a.game.HasColonies() {
			return fmt.Errorf("trade fleet requires colonies and player context")
		}
		a.game.Colonies().AddTradeFleets(a.player.ID(), amount)
	case shared.ResourceColonyTileAdd:
		if a.game == nil || a.player == nil || !a.game.HasColonies() {
			return fmt.Errorf("adding a colony tile requires colonies and player context")
		}
		ids := []string{}
		for _, def := range a.game.Colonies().UnusedDefinitions() {
			ids = append(ids, def.ID)
		}
		if len(ids) > 0 {
			a.player.Selection().SetPendingColonySelection(&shared.PendingColonySelection{Remaining: amount, AddTile: true, AvailableColonyIDs: ids, Source: a.source, SourceCardID: a.sourceCardID})
		}
	case shared.ResourceColony:
		if a.game == nil || a.player == nil {
			return fmt.Errorf("cannot apply colony tile: missing game or player context")
		}
		if !a.game.HasColonies() {
			log.Warn("Colony tile output ignored: colonies expansion not enabled")
			return nil
		}
		colonyIDs := a.game.Colonies().GetPlaceableIDs(a.player.ID(), o.AllowDuplicatePlayerColony)
		if len(colonyIDs) == 0 {
			log.Warn("No colony tiles available for placement")
			return nil
		}
		a.player.Selection().SetPendingColonySelection(&shared.PendingColonySelection{
			AvailableColonyIDs:         colonyIDs,
			AllowDuplicatePlayerColony: o.AllowDuplicatePlayerColony,
			Source:                     a.source,
			SourceCardID:               a.sourceCardID,
		})
		log.Debug("Set pending colony selection",
			slog.Int("available_colonies", len(colonyIDs)), slog.Bool("allow_duplicate", o.AllowDuplicatePlayerColony))

	case shared.ResourceColonyBonus:
		if a.game == nil || a.player == nil {
			return fmt.Errorf("cannot apply colony bonus: missing game or player context")
		}
		if !a.game.HasColonies() {
			log.Warn("Colony bonus output ignored: colonies expansion not enabled")
			return nil
		}
		if a.colonyBonusLookup == nil {
			return fmt.Errorf("cannot apply colony bonus: no colony bonus lookup configured")
		}
		a.applyColonyBonuses(ctx, log)

	case shared.ResourceColonyCount:
		return fmt.Errorf("colony-count is a count specification, not an effect")
	case shared.ResourceColonyTrackStep:
		return fmt.Errorf("colony track movement requires a complete effect selection")

	default:
		log.Warn("Unhandled colony type", slog.String("type", string(o.ResourceType)))
	}
	return nil
}

func (a *BehaviorApplier) applyMiscOutput(ctx context.Context, o *shared.MiscCondition, amount int, log *slog.Logger) error {
	switch o.ResourceType {
	case shared.ResourceExtraActions:
		if a.game == nil {
			return fmt.Errorf("cannot apply extra actions: no game context")
		}
		currentTurn := a.game.CurrentTurn()
		if currentTurn != nil {
			currentTurn.AddExtraActions(amount)
		}
		log.Debug("Granted extra actions", slog.Int("amount", amount))

	case shared.ResourceBonusTags:
		if a.player == nil {
			return fmt.Errorf("cannot apply bonus tags: no player context")
		}
		// amount is already scaled by Per (e.g. one tag per earth tag the player has).
		tagToGrant := shared.CardTag(o.ResourceType)
		if len(o.Selectors) > 0 && len(o.Selectors[0].Tags) > 0 {
			tagToGrant = o.Selectors[0].Tags[0]
		}
		if amount > 0 {
			a.player.AddBonusTags(tagToGrant, amount)
		}
		log.Debug("Added bonus tags", slog.String("tag_type", string(tagToGrant)), slog.Int("count", amount))

	case shared.ResourceFreeTrade:
		if a.game == nil || a.player == nil {
			return fmt.Errorf("cannot apply free trade: missing game or player context")
		}
		if !a.game.HasColonies() {
			log.Warn("Free trade output ignored: colonies expansion not enabled")
			return nil
		}
		if a.game.Colonies().TradeFleet(a.player.ID()).Available() == 0 {
			log.Warn("Free trade output ignored: no trade fleet available")
			return nil
		}
		tradeableColonyIDs := a.game.Colonies().GetTradeableIDs()
		if len(tradeableColonyIDs) == 0 {
			log.Warn("Free trade output ignored: no colonies available for trading")
			return nil
		}
		a.player.Selection().SetPendingFreeTradeSelection(&shared.PendingFreeTradeSelection{
			AvailableColonyIDs: tradeableColonyIDs,
			Source:             a.source,
			SourceCardID:       a.sourceCardID,
		})
		log.Debug("Set pending free trade selection", slog.Int("available_colonies", len(tradeableColonyIDs)))

	case shared.ResourceWorldTreeTile:
		log.Debug("World tree tile output", slog.Int("amount", amount))

	case shared.ResourceAwardFund:
		if a.awardRegistry == nil || a.game == nil || a.player == nil {
			return fmt.Errorf("award funding requires registry and player context")
		}
		selected := a.game.SelectedAwards()
		var ids []string
		for _, def := range a.awardRegistry.GetAll() {
			if !a.game.Settings().AllowsContent(def.GameModes, def.RequiresSystems, def.RequiresFeatures) {
				continue
			}
			if len(selected) > 0 && !slices.Contains(selected, def.ID) {
				continue
			}
			if !a.game.Awards().IsFunded(shared.AwardType(def.ID)) {
				ids = append(ids, def.ID)
			}
		}
		if len(ids) > 0 {
			a.player.Selection().SetPendingAwardFundSelection(&shared.PendingAwardFundSelection{AvailableAwards: ids, Source: a.source})
		}

	default:
		log.Warn("Unhandled misc output type", slog.String("type", string(o.ResourceType)))
	}
	return nil
}
