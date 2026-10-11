package action

import (
	"fmt"
	"openmars/internal/game"
	"openmars/internal/game/award"
	"openmars/internal/game/cards"
	"openmars/internal/game/milestone"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
	"openmars/internal/game/standardproject"
)

// PaymentIntent identifies an operation and its choices, never a client-provided cost.
type PaymentIntent struct {
	Action             string   `json:"action"`
	CardID             string   `json:"cardId,omitempty"`
	BehaviorIndex      int      `json:"behaviorIndex,omitempty"`
	ChoiceIndex        *int     `json:"choiceIndex,omitempty"`
	SelectedAmount     *int     `json:"selectedAmount,omitempty"`
	CardStorageSources []string `json:"cardStorageSources,omitempty"`
	ProjectID          string   `json:"projectId,omitempty"`
	CorporationID      string   `json:"corporationId,omitempty"`
	CardIDs            []string `json:"cardIds,omitempty"`
	CardsToBuy         []string `json:"cardsToBuy,omitempty"`
	RandomBuy          bool     `json:"randomBuy,omitempty"`
	PaymentType        string   `json:"paymentType,omitempty"`
	MilestoneType      string   `json:"milestoneType,omitempty"`
	AwardType          string   `json:"awardType,omitempty"`
}

// QuoteActionPayment calculates costs and eligible sources without changing game state.
func QuoteActionPayment(g *game.Game, p *player.Player, registry cards.CardRegistry, projects standardproject.StandardProjectRegistry, milestones milestone.MilestoneRegistry, awards award.AwardRegistry, intent PaymentIntent) (shared.PaymentQuote, error) {
	registry = cards.ForGame(registry, g)
	context := cards.PaymentContext{Costs: map[shared.ResourceType]int{}}
	calc := cards.NewRequirementModifierCalculator(registry)
	credit := func(cost int) { context.Costs[shared.ResourceCredit] = cost }
	switch intent.Action {
	case "play-card":
		card, err := registry.GetByID(intent.CardID)
		if err != nil {
			return shared.PaymentQuote{}, err
		}
		credit(max(0, card.Cost-calc.CalculateCardDiscounts(p, card)))
		context.Card = card
		context.Action = shared.ActionCardPlaying
		var inputs []shared.BehaviorCondition
		for _, b := range card.Behaviors {
			if cards.HasAutoTrigger(b) && !b.Optional {
				if intent.ChoiceIndex != nil && (*intent.ChoiceIndex < 0 || *intent.ChoiceIndex >= len(b.Choices)) && len(b.Choices) > 0 {
					return shared.PaymentQuote{}, fmt.Errorf("invalid choice")
				}
				in, _ := b.ExtractInputsOutputs(intent.ChoiceIndex)
				inputs = append(inputs, in...)
			}
		}
		applier := cards.NewBehaviorApplier(p, g, card.Name, nil).WithSourceCardID(card.ID).WithCardRegistry(registry).WithInputCardIDs(intent.CardStorageSources)
		if intent.SelectedAmount != nil {
			applier = applier.WithSelectedAmount(*intent.SelectedAmount)
		}
		resources, storage, err := applier.InputReservations(inputs)
		if err != nil {
			return shared.PaymentQuote{}, err
		}
		context.ReservedResources = resources
		context.ReservedStorage = storage

	case "card-action":
		for _, a := range p.Actions().List() {
			if a.CardID != intent.CardID || a.BehaviorIndex != intent.BehaviorIndex {
				continue
			}
			if intent.ChoiceIndex != nil && (*intent.ChoiceIndex < 0 || *intent.ChoiceIndex >= len(a.Behavior.Choices)) {
				return shared.PaymentQuote{}, fmt.Errorf("invalid choice")
			}
			inputs, _ := a.Behavior.ExtractInputsOutputs(intent.ChoiceIndex)
			applier := cards.NewBehaviorApplier(p, g, a.CardName, nil).WithSourceCardID(a.CardID).WithCardRegistry(registry).WithInputCardIDs(intent.CardStorageSources)
			if intent.SelectedAmount != nil {
				applier = applier.WithSelectedAmount(*intent.SelectedAmount)
			}
			return applier.QuoteInputs(inputs)
		}
		return shared.PaymentQuote{}, fmt.Errorf("card action not found")
	case "standard-project", "convert-heat", "convert-plants":
		id := shared.StandardProject(intent.ProjectID)
		rt := shared.ResourceCredit
		cost := 0
		switch intent.Action {
		case "convert-heat":
			id = shared.StandardProjectConvertHeatToTemperature
			rt = shared.ResourceHeat
			cost = 8
		case "convert-plants":
			id = shared.StandardProjectConvertPlantsToGreenery
			rt = shared.ResourcePlant
			cost = 8
		default:
			definition, err := projects.GetByID(intent.ProjectID)
			if err != nil {
				return shared.PaymentQuote{}, err
			}
			if !g.Settings().AllowsContent(definition.GameModes, definition.RequiresSystems, definition.RequiresFeatures) {
				return shared.PaymentQuote{}, fmt.Errorf("content unavailable in this game mode")
			}
			cost = definition.CreditCost()
		}
		cost = max(0, cost-calc.CalculateStandardProjectDiscounts(p, id)[rt])
		if rt != shared.ResourceCredit {
			cost = max(1, cost)
		}
		context.Costs[rt] = cost
		context.Action = "standard-project"
		context.StandardProject = id
	case "select-starting-choices":
		return cards.QuoteStartingPayment(registry, intent.CorporationID, len(intent.CardIDs), g.Settings().DemoGame)
	case "confirm-production-cards":
		if g.GetProductionPhase(p.ID()) == nil {
			return shared.PaymentQuote{}, fmt.Errorf("no research selection")
		}
		count := len(intent.CardIDs)
		if intent.RandomBuy {
			count = 1
		}
		credit(count * max(0, 3-calc.CalculateActionDiscounts(p, shared.ActionCardBuying)[shared.ResourceCredit]))
		context.Action = shared.ActionCardBuying
	case "confirm-card-draw":
		selection := p.Selection().GetPendingCardDrawSelection()
		if selection == nil {
			return shared.PaymentQuote{}, fmt.Errorf("no pending card selection")
		}
		credit(len(intent.CardsToBuy) * selection.CardBuyCost)
		context.Action = shared.ActionCardBuying
	case "build-colony":
		credit(17)
		context.Action = "build-colony"
	case "colony-trade":
		rt := shared.ResourceType(intent.PaymentType)
		if intent.PaymentType == "credits" {
			rt = shared.ResourceCredit
		}
		if rt != shared.ResourceCredit && rt != shared.ResourceEnergy && rt != shared.ResourceTitanium {
			return shared.PaymentQuote{}, fmt.Errorf("invalid trade currency")
		}
		costs, _ := CalculateEffectiveTradeCosts(p, registry)
		context.Costs[rt] = costs[string(rt)]
		context.Action = shared.ActionColonyTrade
	case "claim-milestone":
		definition, err := milestones.GetByID(intent.MilestoneType)
		if err != nil {
			return shared.PaymentQuote{}, err
		}
		if !g.Settings().AllowsContent(definition.GameModes, definition.RequiresSystems, definition.RequiresFeatures) {
			return shared.PaymentQuote{}, fmt.Errorf("content unavailable in this game mode")
		}
		credit(definition.ClaimCost)
		context.Action = "claim-milestone"
	case "fund-award":
		definition, err := awards.GetByID(intent.AwardType)
		if err != nil {
			return shared.PaymentQuote{}, err
		}
		if !g.Settings().AllowsContent(definition.GameModes, definition.RequiresSystems, definition.RequiresFeatures) {
			return shared.PaymentQuote{}, fmt.Errorf("content unavailable in this game mode")
		}
		credit(definition.GetCostForFundedCount(g.Awards().FundedCount()))
		context.Action = "fund-award"
	default:
		return shared.PaymentQuote{}, fmt.Errorf("unknown payment action %q", intent.Action)
	}
	return cards.QuotePayment(p, g, registry, context)
}
