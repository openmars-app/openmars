package award

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	baseaction "openmars/internal/action"
	"openmars/internal/game"
	"openmars/internal/game/award"
	"openmars/internal/game/cards"
	"openmars/internal/game/shared"
)

// FundAwardAction handles the business logic for funding an award
type FundAwardAction struct {
	baseaction.BaseAction
	awardRegistry award.AwardRegistry
}

// NewFundAwardAction creates a new fund award action
func NewFundAwardAction(
	gameRepo game.GameRepository,
	cardRegistry cards.CardRegistry,
	stateRepo game.GameStateRepository,
	awardRegistry award.AwardRegistry,
	logger *slog.Logger,
) *FundAwardAction {
	return &FundAwardAction{
		BaseAction:    baseaction.NewBaseActionWithStateRepo(gameRepo, cardRegistry, stateRepo),
		awardRegistry: awardRegistry,
	}
}

// Execute funds an award for the player
func (a *FundAwardAction) Execute(ctx context.Context, gameID string, playerID string, awardType string, payment shared.Payment) error {
	log := a.InitLogger(gameID, playerID).With(slog.String("action", "fund_award"), slog.String("award", awardType))
	log.Debug("Funding award")

	def, err := a.awardRegistry.GetByID(awardType)
	if err != nil {
		log.Warn("Invalid award type", slog.String("award_type", awardType))
		return fmt.Errorf("invalid award type: %s", awardType)
	}

	g, err := baseaction.ValidateActiveGame(ctx, a.GameRepository(), gameID, log)
	if err != nil {
		return err
	}

	if !g.Settings().AllowsContent(def.GameModes, def.RequiresSystems, def.RequiresFeatures) {
		return fmt.Errorf("award is not available in this game mode")
	}

	if err := baseaction.ValidateGamePhase(g, shared.GamePhaseAction, log); err != nil {
		return err
	}

	if err := baseaction.ValidateCurrentTurn(g, playerID, log); err != nil {
		return err
	}

	if err := baseaction.ValidateActionsRemaining(g, playerID, log); err != nil {
		return err
	}

	if err := baseaction.ValidateNoPendingSelections(g, playerID, log); err != nil {
		return err
	}

	player, err := a.GetPlayerFromGame(g, playerID, log)
	if err != nil {
		return err
	}

	// Validate award is in the selected set for this game
	if selected := g.SelectedAwards(); len(selected) > 0 && !slices.Contains(selected, awardType) {
		log.Warn("Award not available in this game", slog.String("award", awardType))
		return fmt.Errorf("award %s is not available in this game", awardType)
	}

	awardState := g.Awards()
	at := shared.AwardType(awardType)
	if awardState.IsFunded(at) {
		log.Warn("Award already funded", slog.String("award", awardType))
		return fmt.Errorf("award %s is already funded", awardType)
	}

	if !awardState.CanFundMore() {
		log.Warn("Maximum awards already funded", slog.Int("max", game.MaxFundedAwards))
		return fmt.Errorf("maximum awards (%d) already funded", game.MaxFundedAwards)
	}

	fundingCost := def.GetCostForFundedCount(awardState.FundedCount())
	quote, err := cards.QuotePayment(player, g, a.CardRegistryForGame(g), cards.PaymentContext{Costs: map[shared.ResourceType]int{shared.ResourceCredit: fundingCost}, Action: "fund-award"})
	if err != nil {
		return err
	}
	paymentPlan, err := cards.ValidatePayment(quote, payment)
	if err != nil {
		return err
	}
	cards.ApplyPayment(player, paymentPlan)
	log.Debug("Deducted award funding cost",
		slog.Int("cost", fundingCost),
		slog.Int("remaining_credits", player.Resources().Get().Credits))

	if err := awardState.FundAward(ctx, at, playerID, fundingCost); err != nil {
		log.Error("Failed to fund award", slog.Any("error", err))
		return fmt.Errorf("failed to fund award: %w", err)
	}

	a.ConsumePlayerAction(g, log)

	a.WriteStateLog(ctx, g, def.Name, shared.SourceTypeAward, playerID, fmt.Sprintf("Funded %s award", def.Name))

	log.Info("Award funded",
		slog.String("award", awardType),
		slog.Int("total_funded", awardState.FundedCount()))

	return nil
}
