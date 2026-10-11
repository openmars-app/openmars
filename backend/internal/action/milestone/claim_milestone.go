package milestone

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	baseaction "openmars/internal/action"
	"openmars/internal/game"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/milestone"
	"openmars/internal/game/shared"
)

// ClaimMilestoneAction handles the business logic for claiming a milestone
type ClaimMilestoneAction struct {
	baseaction.BaseAction
	milestoneRegistry milestone.MilestoneRegistry
}

// NewClaimMilestoneAction creates a new claim milestone action
func NewClaimMilestoneAction(
	gameRepo game.GameRepository,
	cardRegistry gamecards.CardRegistry,
	stateRepo game.GameStateRepository,
	milestoneRegistry milestone.MilestoneRegistry,
	logger *slog.Logger,
) *ClaimMilestoneAction {
	return &ClaimMilestoneAction{
		BaseAction:        baseaction.NewBaseActionWithStateRepo(gameRepo, cardRegistry, stateRepo),
		milestoneRegistry: milestoneRegistry,
	}
}

// Execute claims a milestone for the player
func (a *ClaimMilestoneAction) Execute(ctx context.Context, gameID string, playerID string, milestoneType string, payment shared.Payment) error {
	log := a.InitLogger(gameID, playerID).With(slog.String("action", "claim_milestone"), slog.String("milestone", milestoneType))
	log.Debug("Claiming milestone")

	def, err := a.milestoneRegistry.GetByID(milestoneType)
	if err != nil {
		log.Warn("Invalid milestone type", slog.String("milestone_type", milestoneType))
		return fmt.Errorf("invalid milestone type: %s", milestoneType)
	}

	g, err := baseaction.ValidateActiveGame(ctx, a.GameRepository(), gameID, log)
	if err != nil {
		return err
	}

	if !g.Settings().AllowsContent(def.GameModes, def.RequiresSystems, def.RequiresFeatures) {
		return fmt.Errorf("milestone is not available in this game mode")
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

	// Validate milestone is in the selected set for this game
	if selected := g.SelectedMilestones(); len(selected) > 0 && !slices.Contains(selected, milestoneType) {
		log.Warn("Milestone not available in this game", slog.String("milestone", milestoneType))
		return fmt.Errorf("milestone %s is not available in this game", milestoneType)
	}

	ms := g.Milestones()
	mt := shared.MilestoneType(milestoneType)
	if ms.IsClaimed(mt) {
		log.Warn("Milestone already claimed", slog.String("milestone", milestoneType))
		return fmt.Errorf("milestone %s is already claimed", milestoneType)
	}

	if !ms.CanClaimMore() {
		log.Warn("Maximum milestones already claimed", slog.Int("max", game.MaxClaimedMilestones))
		return fmt.Errorf("maximum milestones (%d) already claimed", game.MaxClaimedMilestones)
	}

	quote, err := gamecards.QuotePayment(player, g, a.CardRegistryForGame(g), gamecards.PaymentContext{Costs: map[shared.ResourceType]int{shared.ResourceCredit: def.ClaimCost}, Action: "claim-milestone"})
	if err != nil {
		return err
	}
	paymentPlan, err := gamecards.ValidatePayment(quote, payment)
	if err != nil {
		return err
	}

	if !gamecards.CanClaimMilestone(def, player, g.Board(), a.CardRegistryForGame(g)) {
		progress := gamecards.CalculateMilestoneProgress(def, player, g.Board(), a.CardRegistryForGame(g))
		required := def.GetRequired()
		log.Warn("Player does not meet milestone requirements",
			slog.String("requirement", def.Description),
			slog.Int("required", required),
			slog.Int("current", progress))
		return fmt.Errorf("requirements not met: %s (have %d, need %d)", def.Description, progress, required)
	}

	gamecards.ApplyPayment(player, paymentPlan)
	log.Debug("Deducted milestone cost",
		slog.Int("cost", def.ClaimCost),
		slog.Int("remaining_credits", player.Resources().Get().Credits))

	if err := ms.ClaimMilestone(ctx, mt, playerID, g.Generation()); err != nil {
		log.Error("Failed to claim milestone", slog.Any("error", err))
		return fmt.Errorf("failed to claim milestone: %w", err)
	}

	a.ConsumePlayerAction(g, log)

	a.WriteStateLog(ctx, g, def.Name, shared.SourceTypeMilestone, playerID, fmt.Sprintf("Claimed %s milestone", def.Name))

	log.Info("Milestone claimed",
		slog.String("milestone", milestoneType),
		slog.Int("total_claimed", ms.ClaimedCount()))

	return nil
}
