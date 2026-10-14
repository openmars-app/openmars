package projectfunding

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	baseaction "openmars/internal/action"
	"openmars/internal/events"
	"openmars/internal/game"
	"openmars/internal/game/player"
	pf "openmars/internal/game/projectfunding"
	"openmars/internal/game/shared"
)

// FundSeatPayment describes how the player pays for a seat
type FundSeatPayment struct {
	Credits  int
	Steel    int
	Titanium int
}

// FundSeatAction handles the business logic for purchasing a project funding seat
type FundSeatAction struct {
	baseaction.BaseAction
	pfRegistry pf.ProjectFundingRegistry
}

// NewFundSeatAction creates a new fund seat action
func NewFundSeatAction(
	gameRepo game.GameRepository,
	pfReg pf.ProjectFundingRegistry,
	stateRepo game.GameStateRepository,
) *FundSeatAction {
	return &FundSeatAction{
		BaseAction: baseaction.NewBaseActionWithStateRepo(gameRepo, nil, stateRepo),
		pfRegistry: pfReg,
	}
}

// Execute performs the fund seat action
func (a *FundSeatAction) Execute(ctx context.Context, gameID string, playerID string, projectID string, payment FundSeatPayment) error {
	log := a.InitLogger(gameID, playerID).With(
		slog.String("action", "fund_project_seat"),
		slog.String("project_id", projectID),
	)
	log.Debug("Purchasing project seat")

	g, err := baseaction.ValidateActiveGame(ctx, a.GameRepository(), gameID, log)
	if err != nil {
		return err
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

	if !g.HasProjectFunding() {
		return fmt.Errorf("project funding expansion is not enabled")
	}

	player, err := a.GetPlayerFromGame(g, playerID, log)
	if err != nil {
		return err
	}

	projectState := g.GetProjectFundingState(projectID)
	if projectState == nil {
		return fmt.Errorf("project not found: %s", projectID)
	}

	if projectState.IsCompleted {
		return fmt.Errorf("project is already completed: %s", projectID)
	}

	definition, err := a.pfRegistry.GetByID(projectID)
	if err != nil {
		return fmt.Errorf("project definition not found: %w", err)
	}

	scaledSeats := pf.ScaledSeats(definition.Seats, len(g.GetAllPlayers()))

	seatIndex := len(projectState.SeatOwners)
	if seatIndex >= len(scaledSeats) {
		return fmt.Errorf("all seats are filled for project: %s", projectID)
	}

	seat := scaledSeats[seatIndex]
	cost := seat.Cost

	if payment.Credits < 0 || payment.Steel < 0 || payment.Titanium < 0 {
		return fmt.Errorf("payment amounts must be non-negative")
	}

	// Validate payment covers cost
	totalPayment := payment.Credits
	steelValue := 0
	titaniumValue := 0

	for _, sub := range seat.PaymentSubstitutes {
		switch sub.ResourceType {
		case "steel":
			steelValue = sub.ConversionRate
		case "titanium":
			titaniumValue = sub.ConversionRate
		}
	}

	if payment.Steel > 0 {
		if steelValue == 0 {
			return fmt.Errorf("steel cannot be used to pay for this seat")
		}
		totalPayment += payment.Steel * steelValue
	}

	if payment.Titanium > 0 {
		if titaniumValue == 0 {
			return fmt.Errorf("titanium cannot be used to pay for this seat")
		}
		totalPayment += payment.Titanium * titaniumValue
	}

	if totalPayment < cost {
		return fmt.Errorf("insufficient payment: need %d, provided %d", cost, totalPayment)
	}

	// Steel and titanium are spent as chosen; credits only cover what they leave, as in
	// every other payment, so an overpayment never costs extra credits.
	credits := min(payment.Credits, max(0, cost-payment.Steel*steelValue-payment.Titanium*titaniumValue))

	resources := player.Resources().Get()
	if resources.Credits < credits {
		return fmt.Errorf("insufficient credits: need %d, have %d", credits, resources.Credits)
	}
	if resources.Steel < payment.Steel {
		return fmt.Errorf("insufficient steel: need %d, have %d", payment.Steel, resources.Steel)
	}
	if resources.Titanium < payment.Titanium {
		return fmt.Errorf("insufficient titanium: need %d, have %d", payment.Titanium, resources.Titanium)
	}

	deductions := map[shared.ResourceType]int{}
	if credits > 0 {
		deductions[shared.ResourceCredit] = -credits
	}
	if payment.Steel > 0 {
		deductions[shared.ResourceSteel] = -payment.Steel
	}
	if payment.Titanium > 0 {
		deductions[shared.ResourceTitanium] = -payment.Titanium
	}
	player.Resources().Add(deductions)

	// Add seat owner
	projectState.SeatOwners = append(projectState.SeatOwners, playerID)

	calculatedOutputs := []shared.CalculatedOutput{
		{ResourceType: "project-seat", Amount: 1},
	}

	g.AddTriggeredEffect(shared.TriggeredEffect{
		CardName:          definition.Name,
		PlayerID:          playerID,
		SourceType:        shared.SourceTypeProjectFundingSeat,
		CalculatedOutputs: calculatedOutputs,
	})

	a.WriteStateLogFull(ctx, g, definition.Name, shared.SourceTypeProjectFundingSeat,
		playerID, fmt.Sprintf("Funded %s project", definition.Name), nil, calculatedOutputs, nil)

	events.Publish(g.EventBus(), events.ProjectSeatPurchasedEvent{
		GameID:    g.ID(),
		PlayerID:  playerID,
		ProjectID: projectID,
		SeatIndex: seatIndex,
		Timestamp: time.Now(),
	})

	a.ConsumePlayerAction(g, log)

	// Check for completion
	if len(projectState.SeatOwners) >= len(scaledSeats) {
		a.completeProject(ctx, g, projectState, definition, log)
	}

	log.Info("Project seat purchased",
		slog.String("project_id", projectID),
		slog.Int("seat_index", seatIndex))

	return nil
}

func (a *FundSeatAction) completeProject(ctx context.Context, g *game.Game, state *pf.ProjectState, def *pf.ProjectDefinition, log *slog.Logger) {
	state.IsCompleted = true

	allPlayers := g.GetAllPlayers()
	scaledTiers := pf.ScaledRewardTiers(def.RewardTiers, len(allPlayers))

	// Count seats per player
	seatCounts := map[string]int{}
	for _, ownerID := range state.SeatOwners {
		seatCounts[ownerID]++
	}

	// Apply tier rewards to each funder
	for playerID, count := range seatCounts {
		tier := pf.FindBestTier(scaledTiers, count)
		if tier == nil {
			continue
		}

		p, err := g.GetPlayer(playerID)
		if err != nil {
			log.Error("Failed to get player for tier rewards", slog.String("player_id", playerID), slog.Any("error", err))
			continue
		}

		tierOutputs := applyRewards(p, tier.Rewards)

		g.AddTriggeredEffect(shared.TriggeredEffect{
			CardName:          "Project Tier Reward: " + def.Name,
			PlayerID:          playerID,
			SourceType:        shared.SourceTypeProjectFundingCompletion,
			CalculatedOutputs: tierOutputs,
		})
	}

	// Apply static completion rewards to ALL players
	for _, p := range allPlayers {
		completionOutputs := applyRewards(p, def.CompletionEffect.Rewards)

		g.AddTriggeredEffect(shared.TriggeredEffect{
			CardName:          "Project Completed: " + def.Name,
			PlayerID:          p.ID(),
			SourceType:        shared.SourceTypeProjectFundingCompletion,
			CalculatedOutputs: completionOutputs,
		})
	}

	// Apply global completion effects
	if len(def.CompletionEffect.GlobalEffects) > 0 {
		completingPlayerID := state.SeatOwners[len(state.SeatOwners)-1]
		a.applyGlobalEffects(ctx, g, def, allPlayers, completingPlayerID, log)
	}

	a.WriteStateLogFull(ctx, g, "Project Completed: "+def.Name, shared.SourceTypeProjectFundingCompletion,
		"", fmt.Sprintf("Project %s completed", def.Name), nil, nil, nil)

	events.Publish(g.EventBus(), events.ProjectCompletedEvent{
		GameID:    g.ID(),
		ProjectID: def.ID,
		Timestamp: time.Now(),
	})

	log.Debug("Project completed",
		slog.String("project_id", def.ID),
		slog.Int("total_seats", len(state.SeatOwners)))
}

func (a *FundSeatAction) applyGlobalEffects(ctx context.Context, g *game.Game, def *pf.ProjectDefinition, allPlayers []*player.Player, completingPlayerID string, log *slog.Logger) {
	for _, effect := range def.CompletionEffect.GlobalEffects {
		switch effect.Type {
		case "temperature":
			if _, err := g.GlobalParameters().IncreaseTemperature(ctx, effect.Amount, completingPlayerID); err != nil {
				log.Error("Failed to increase temperature for project completion", slog.Any("error", err))
			}
		case "oxygen":
			if _, err := g.GlobalParameters().IncreaseOxygen(ctx, effect.Amount, completingPlayerID); err != nil {
				log.Error("Failed to increase oxygen for project completion", slog.Any("error", err))
			}
		case "freeze-turn-order":
			g.SetNextGenTurnOrderFrozen(true)
			log.Debug("Turn order frozen by project completion", slog.String("project", def.Name))
		case "production-choice":
			amount := effect.Amount
			if amount <= 0 {
				amount = 1
			}
			choices := buildProductionChoices(amount)
			for _, p := range allPlayers {
				p.Selection().AddPendingBehaviorResolution(&shared.PendingBehaviorResolution{Kind: "choice",
					Choices:      choices,
					Source:       "project-funding-completion",
					SourceCardID: def.ID,
				})
			}
			log.Debug("Production choice set for all players", slog.String("project", def.Name))
		case "card-draw":
			n := effect.Amount
			if n <= 0 {
				n = 1
			}
			for _, p := range allPlayers {
				cardIDs, err := g.Deck().DrawProjectCards(ctx, n)
				if err != nil {
					log.Error("Failed to draw cards for project completion", slog.Any("error", err))
					break
				}
				p.Hand().ReceiveCards(cardIDs, def.Name, "")
				log.Debug("Cards drawn for player",
					slog.String("player_id", p.ID()),
					slog.Int("count", len(cardIDs)))
			}
		}
	}
}

func buildProductionChoices(amount int) []shared.Choice {
	productionTypes := []shared.ResourceType{
		shared.ResourceCreditProduction,
		shared.ResourceSteelProduction,
		shared.ResourceTitaniumProduction,
		shared.ResourcePlantProduction,
		shared.ResourceEnergyProduction,
		shared.ResourceHeatProduction,
	}
	choices := make([]shared.Choice, len(productionTypes))
	for i, rt := range productionTypes {
		choices[i] = shared.Choice{
			Outputs: []shared.BehaviorCondition{
				shared.NewProductionCondition(rt, amount, "self-player"),
			},
		}
	}
	return choices
}

func applyRewards(p *player.Player, rewards []pf.Output) []shared.CalculatedOutput {
	var outputs []shared.CalculatedOutput
	resourceAdds := map[shared.ResourceType]int{}
	productionAdds := map[shared.ResourceType]int{}

	for _, reward := range rewards {
		outputs = append(outputs, shared.CalculatedOutput{
			ResourceType: reward.Type,
			Amount:       reward.Amount,
		})

		switch reward.Type {
		case "credit":
			resourceAdds[shared.ResourceCredit] += reward.Amount
		case "steel":
			resourceAdds[shared.ResourceSteel] += reward.Amount
		case "titanium":
			resourceAdds[shared.ResourceTitanium] += reward.Amount
		case "plant":
			resourceAdds[shared.ResourcePlant] += reward.Amount
		case "energy":
			resourceAdds[shared.ResourceEnergy] += reward.Amount
		case "heat":
			resourceAdds[shared.ResourceHeat] += reward.Amount
		case "tr":
			p.Resources().UpdateTerraformRating(reward.Amount)
		case "vp":
			p.VPGranters().Add(shared.VPGranter{
				CardID:      "pf_" + p.ID(),
				CardName:    "Project Funding",
				Description: fmt.Sprintf("%d VP from project funding", reward.Amount),
				VPConditions: []shared.VPCondition{
					{Amount: reward.Amount, Condition: shared.VPConditionFixed},
				},
				ComputedValue: reward.Amount,
			})
		case "credit-production":
			productionAdds[shared.ResourceCreditProduction] += reward.Amount
		case "steel-production":
			productionAdds[shared.ResourceSteelProduction] += reward.Amount
		case "titanium-production":
			productionAdds[shared.ResourceTitaniumProduction] += reward.Amount
		case "plant-production":
			productionAdds[shared.ResourcePlantProduction] += reward.Amount
		case "energy-production":
			productionAdds[shared.ResourceEnergyProduction] += reward.Amount
		case "heat-production":
			productionAdds[shared.ResourceHeatProduction] += reward.Amount
		}
	}

	if len(resourceAdds) > 0 {
		p.Resources().Add(resourceAdds)
	}
	if len(productionAdds) > 0 {
		p.Resources().AddProduction(productionAdds)
	}

	return outputs
}
