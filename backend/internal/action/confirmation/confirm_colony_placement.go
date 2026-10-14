package confirmation

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	baseaction "openmars/internal/action"
	colonyaction "openmars/internal/action/colony"
	"openmars/internal/action/turn_management"
	"openmars/internal/game"
	"openmars/internal/game/cards"
	"openmars/internal/game/colony"
	"openmars/internal/game/shared"
)

// ConfirmColonyPlacementAction handles confirming a colony placement from a card effect
type ConfirmColonyPlacementAction struct {
	baseaction.BaseAction
	colonyRegistry colony.ColonyRegistry
}

// NewConfirmColonyPlacementAction creates a new confirm colony placement action
func NewConfirmColonyPlacementAction(
	gameRepo game.GameRepository,
	cardRegistry cards.CardRegistry,
	colonyRegistry colony.ColonyRegistry,
	logger *slog.Logger,
) *ConfirmColonyPlacementAction {
	return &ConfirmColonyPlacementAction{
		BaseAction:     baseaction.NewBaseAction(gameRepo, cardRegistry),
		colonyRegistry: colonyRegistry,
	}
}

// Execute places the selected colony for free and clears the pending selection
func (a *ConfirmColonyPlacementAction) Execute(ctx context.Context, gameID string, playerID string, colonyID string) error {
	log := a.InitLogger(gameID, playerID).With(
		slog.String("action", "confirm_colony_placement"),
		slog.String("colony_id", colonyID),
	)
	log.Debug("Confirming colony placement")

	g, err := baseaction.ValidateActiveGame(ctx, a.GameRepository(), gameID, log)
	if err != nil {
		return err
	}

	p, err := a.GetPlayerFromGame(g, playerID, log)
	if err != nil {
		return err
	}

	if err := baseaction.ValidateCurrentTurn(g, playerID, log); err != nil {
		return err
	}
	pending := p.Selection().GetPendingColonySelection()
	if pending == nil {
		return fmt.Errorf("no pending colony selection")
	}

	if !slices.Contains(pending.AvailableColonyIDs, colonyID) {
		return fmt.Errorf("colony %s is not available for selection", colonyID)
	}

	if pending.AddTile {
		if g.Colonies().GetState(colonyID) != nil {
			return fmt.Errorf("colony tile is already in play")
		}
		definition, err := a.colonyRegistry.GetByID(colonyID)
		if err != nil {
			return err
		}
		if err := g.Colonies().AddTile(cards.InitializeColonyTile(g, *definition, a.CardRegistryForGame(g))); err != nil {
			return err
		}
		p.Selection().SetPendingColonySelection(nil)
		if pending.Remaining > 1 {
			pending.Remaining--
			pending.AvailableColonyIDs = nil
			for _, def := range g.Colonies().UnusedDefinitions() {
				pending.AvailableColonyIDs = append(pending.AvailableColonyIDs, def.ID)
			}
			if len(pending.AvailableColonyIDs) > 0 {
				p.Selection().SetPendingColonySelection(pending)
			}
		}
		baseaction.AutoAdvanceTurnIfNeeded(g, playerID, log, a.CardRegistryForGame(g))
		return nil
	}
	tileState := g.Colonies().GetState(colonyID)
	if tileState != nil && tileState.AwaitingResource != "" {
		return fmt.Errorf("colony is not active")
	}
	if tileState == nil {
		return fmt.Errorf("colony tile not found: %s", colonyID)
	}

	definition, err := a.colonyRegistry.GetByID(colonyID)
	if err != nil {
		return fmt.Errorf("colony definition not found: %w", err)
	}

	maxColonies := len(definition.Colonies)
	if len(tileState.PlayerColonies) >= maxColonies {
		return fmt.Errorf("colony tile is full: %d/%d colonies", len(tileState.PlayerColonies), maxColonies)
	}

	if !pending.AllowDuplicatePlayerColony && slices.Contains(tileState.PlayerColonies, playerID) {
		return fmt.Errorf("player already has a colony on this tile")
	}

	source := pending.Source
	if source == "" {
		source = "Card Effect"
	}

	if err := colonyaction.PlaceColonyOnTile(ctx, g, p, definition, tileState, a.CardRegistryForGame(g), source, log); err != nil {
		return fmt.Errorf("failed to place colony: %w", err)
	}

	p.Selection().SetPendingColonySelection(nil)
	baseaction.AutoAdvanceTurnIfNeeded(g, playerID, log, a.CardRegistryForGame(g))

	log.Info("Colony placed from card effect",
		slog.String("colony_id", colonyID))

	if phase := g.CurrentPhase(); phase == shared.GamePhaseInitApplyCorp || phase == shared.GamePhaseInitApplyPrelude {
		advanced, err := turn_management.AdvanceInitPhaseAfterForcedAction(ctx, g, log)
		if err != nil {
			return fmt.Errorf("failed to advance init phase after forced colony placement: %w", err)
		}
		if advanced {
			log.Debug("Advanced init phase after forced colony placement",
				slog.String("colony_id", colonyID))
		}
	}

	return nil
}
