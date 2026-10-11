package bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"openmars/internal/action"
	colonyAction "openmars/internal/action/colony"
	pfAction "openmars/internal/action/projectfunding"
	"openmars/internal/game/shared"
)

type noInput struct{}

type preferInput struct {
	Prefer map[string]int `json:"prefer,omitempty" jsonschema:"substitute resources to spend first, e.g. {\"steel\": 4} or {\"titanium\": 2} or {\"heat\": 3}; omit to pay with your own resources"`
}

type playCardInput struct {
	CardID             string   `json:"cardId" jsonschema:"ID of the card in your hand"`
	ChoiceIndex        *int     `json:"choiceIndex,omitempty" jsonschema:"choice index for cards with choices"`
	SelectedAmount     *int     `json:"selectedAmount,omitempty" jsonschema:"amount for cards with a variable amount"`
	TargetPlayerID     *string  `json:"targetPlayerId,omitempty" jsonschema:"player ID targeted by the card"`
	CardStorageTargets []string `json:"cardStorageTargets,omitempty" jsonschema:"card IDs that receive resources"`
	CardStorageSources []string `json:"cardStorageSources,omitempty" jsonschema:"card IDs whose stored resources are spent"`
	preferInput
}

type useCardActionInput struct {
	CardID             string   `json:"cardId" jsonschema:"ID of the played card with the action"`
	BehaviorIndex      int      `json:"behaviorIndex" jsonschema:"behavior index listed under CARD ACTIONS"`
	ChoiceIndex        *int     `json:"choiceIndex,omitempty"`
	SelectedAmount     *int     `json:"selectedAmount,omitempty"`
	TargetPlayerID     *string  `json:"targetPlayerId,omitempty"`
	SourceCardForInput *string  `json:"sourceCardForInput,omitempty" jsonschema:"card to take input resources from"`
	ReuseSourceCardID  *string  `json:"reuseSourceCardId,omitempty" jsonschema:"card granting an action reuse, when reusing"`
	CardStorageTargets []string `json:"cardStorageTargets,omitempty"`
	CardStorageSources []string `json:"cardStorageSources,omitempty"`
	preferInput
}

type standardProjectInput struct {
	Project string `json:"project" jsonschema:"one of: sell-patents, power-plant, asteroid, aquifer, greenery, city, air-scrapping"`
	preferInput
}

type milestoneInput struct {
	MilestoneType string `json:"milestoneType" jsonschema:"milestone ID in brackets under MILESTONES, e.g. terraformer"`
	preferInput
}

type awardInput struct {
	AwardType string `json:"awardType" jsonschema:"award ID in brackets under AWARDS, e.g. banker"`
	preferInput
}

type colonyTradeInput struct {
	ColonyID    string `json:"colonyId"`
	PaymentType string `json:"paymentType" jsonschema:"credits, energy or titanium"`
	TrackSteps  int    `json:"trackSteps" jsonschema:"track option from COLONIES (0 when there is no choice)"`
}

type colonyBuildInput struct {
	ColonyID string `json:"colonyId"`
	preferInput
}

type fundSeatInput struct {
	ProjectID string `json:"projectId"`
	Steel     int    `json:"steel,omitempty" jsonschema:"steel to spend if the project accepts it"`
	Titanium  int    `json:"titanium,omitempty" jsonschema:"titanium to spend if the project accepts it"`
}

type selectTileInput struct {
	Hex string `json:"hex" jsonschema:"q,r,s from the available hexes list"`
}

type startingChoicesInput struct {
	CorporationID string   `json:"corporationId"`
	PreludeIDs    []string `json:"preludeIds"`
	CardIDs       []string `json:"cardIds" jsonschema:"starting cards to buy (may be empty)"`
}

type cardIDsInput struct {
	CardIDs []string `json:"cardIds"`
}

type cardDrawInput struct {
	CardsToTake []string `json:"cardsToTake"`
	CardsToBuy  []string `json:"cardsToBuy"`
}

type cardDiscardInput struct {
	ResolutionID   string   `json:"resolutionId"`
	CardsToDiscard []string `json:"cardsToDiscard"`
}

type optionalBehaviorInput struct {
	ResolutionID string `json:"resolutionId"`
	Accept       bool   `json:"accept"`
}

type behaviorChoiceInput struct {
	ResolutionID       string   `json:"resolutionId"`
	ChoiceIndex        int      `json:"choiceIndex"`
	CardStorageTargets []string `json:"cardStorageTargets,omitempty"`
}

type effectSelectionInput struct {
	OptionIndex int `json:"optionIndex"`
}

type resourceRemovalInput struct {
	SelectionID    string `json:"selectionId"`
	TargetPlayerID string `json:"targetPlayerId" jsonschema:"empty to skip"`
	Amount         int    `json:"amount"`
}

type colonyIDInput struct {
	ColonyID string `json:"colonyId"`
}

type cardIDInput struct {
	CardID string `json:"cardId" jsonschema:"empty to skip"`
}

type awardTypeInput struct {
	AwardType string `json:"awardType"`
}

type freeTradeInput struct {
	ColonyID   string `json:"colonyId"`
	TrackSteps int    `json:"trackSteps"`
}

type textInput struct {
	Text string `json:"text"`
}

type emoteInput struct {
	Emote string `json:"emote" jsonschema:"one of: angry, celebrate, thinking, applause, shock, laugh, sad, cool"`
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

func (ts *ToolServer) registerTools(g *grant) {
	ts.registerReadTools(g)
	if g.role == RoleExecutor {
		ts.registerExpressionTools(g)
		ts.registerActionTools(g)
	}
}

func (ts *ToolServer) registerReadTools(g *grant) {
	mcp.AddTool(g.server, &mcp.Tool{Name: "get_state", Description: "Full game state from your point of view: pending actions, your status, hand, actions, board, colonies, opponents, recent log and chat."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
			var text string
			var err error
			if doErr := ts.exec.Do(ctx, func() {
				var snap *Snapshot
				if snap, err = g.hooks.Snapshot(ctx); err == nil {
					text = snap.Describe()
				}
			}); doErr != nil {
				return nil, nil, doErr
			}
			if err != nil {
				return nil, nil, err
			}
			return textResult(text), nil, nil
		})
	mcp.AddTool(g.server, &mcp.Tool{Name: "get_plan", Description: "Your current plan and notes from earlier turns."},
		func(_ context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
			return textResult(g.hooks.Plan().Describe()), nil, nil
		})
	mcp.AddTool(g.server, &mcp.Tool{Name: "set_plan", Description: "Replace your plan. Keep it current: what you want, what you race for, who your rivals are, ranked next moves, and notes to remember."},
		func(_ context.Context, _ *mcp.CallToolRequest, plan Plan) (*mcp.CallToolResult, any, error) {
			g.hooks.SetPlan(plan)
			return textResult("Plan saved."), nil, nil
		})
	mcp.AddTool(g.server, &mcp.Tool{Name: "thought", Description: "Show a short in-character thought bubble (max 80 characters) above your player card. Keep it vague: never reveal cards in your hand."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in textInput) (*mcp.CallToolResult, any, error) {
			if err := ts.exec.Do(ctx, func() { g.hooks.Think(in.Text) }); err != nil {
				return nil, nil, err
			}
			return textResult("Shown."), nil, nil
		})
}

func (ts *ToolServer) registerExpressionTools(g *grant) {
	mcp.AddTool(g.server, &mcp.Tool{Name: "chat", Description: "Say one in-character sentence in the game chat. At most two per turn. Do not narrate your moves."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in textInput) (*mcp.CallToolResult, any, error) {
			var err error
			if doErr := ts.exec.Do(ctx, func() { err = g.hooks.Say(ctx, in.Text) }); doErr != nil {
				return nil, nil, doErr
			}
			if err != nil {
				return nil, nil, err
			}
			return textResult("Sent."), nil, nil
		})
	mcp.AddTool(g.server, &mcp.Tool{Name: "emote", Description: "Show an emote over your player card."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in emoteInput) (*mcp.CallToolResult, any, error) {
			var err error
			if doErr := ts.exec.Do(ctx, func() { err = g.hooks.Emote(in.Emote) }); doErr != nil {
				return nil, nil, doErr
			}
			if err != nil {
				return nil, nil, err
			}
			return textResult("Shown."), nil, nil
		})
}

// gameAction runs fn as a paced, serialised game action and reports the outcome with the new state.
func (ts *ToolServer) gameAction(ctx context.Context, g *grant, fn func(snap *Snapshot) error) (*mcp.CallToolResult, any, error) {
	g.actionMu.Lock()
	defer g.actionMu.Unlock()
	if err := g.hooks.BeforeAction(ctx); err != nil {
		return nil, nil, err
	}
	var brief string
	var err error
	if doErr := ts.exec.Do(ctx, func() {
		var snap *Snapshot
		if snap, err = g.hooks.Snapshot(ctx); err != nil {
			return
		}
		if snap.Game != nil && snap.Game.ResumeLobby() != nil {
			err = fmt.Errorf("game is paused while players join")
			return
		}
		if err = fn(snap); err != nil {
			err = fmt.Errorf("rejected: %w", err)
			return
		}
		g.hooks.AfterAction()
		if after, snapErr := g.hooks.Snapshot(ctx); snapErr == nil {
			brief = " " + after.Brief()
		}
	}); doErr != nil {
		return nil, nil, doErr
	}
	if err != nil {
		return nil, nil, err
	}
	return textResult("Done." + brief), nil, nil
}

func addAction[In any](ts *ToolServer, g *grant, name, description string, fn func(ctx context.Context, snap *Snapshot, in In) error) {
	mcp.AddTool(g.server, &mcp.Tool{Name: name, Description: description},
		func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
			return ts.gameAction(ctx, g, func(snap *Snapshot) error { return fn(ctx, snap, in) })
		})
}

func (ts *ToolServer) registerActionTools(g *grant) {
	a := ts.actions
	gid, pid := g.gameID, g.playerID

	addAction(ts, g, "play_card", "Play a card from your hand (one action).",
		func(ctx context.Context, snap *Snapshot, in playCardInput) error {
			payment, err := ts.quotePayment(snap.Game, snap.Player, action.PaymentIntent{
				Action: "play-card", CardID: in.CardID, ChoiceIndex: in.ChoiceIndex,
				SelectedAmount: in.SelectedAmount, CardStorageSources: in.CardStorageSources,
			}, in.Prefer)
			if err != nil {
				return err
			}
			return a.PlayCard.Execute(ctx, gid, pid, in.CardID, payment, in.ChoiceIndex, in.CardStorageTargets, in.TargetPlayerID, in.SelectedAmount, in.CardStorageSources)
		})

	addAction(ts, g, "use_card_action", "Use an action of a played card (one action).",
		func(ctx context.Context, snap *Snapshot, in useCardActionInput) error {
			payment, err := ts.quotePayment(snap.Game, snap.Player, action.PaymentIntent{
				Action: "card-action", CardID: in.CardID, BehaviorIndex: in.BehaviorIndex, ChoiceIndex: in.ChoiceIndex,
				SelectedAmount: in.SelectedAmount, CardStorageSources: in.CardStorageSources,
			}, in.Prefer)
			if err != nil {
				return err
			}
			return a.UseCardAction.Execute(ctx, gid, pid, in.CardID, in.BehaviorIndex, in.ChoiceIndex, in.CardStorageTargets, in.TargetPlayerID, in.SourceCardForInput, in.SelectedAmount, &payment, in.ReuseSourceCardID, in.CardStorageSources)
		})

	addAction(ts, g, "standard_project", "Run a standard project (one action). sell-patents is followed by confirm_sell_patents.",
		func(ctx context.Context, snap *Snapshot, in standardProjectInput) error {
			payment, err := ts.quotePayment(snap.Game, snap.Player, action.PaymentIntent{Action: "standard-project", ProjectID: in.Project}, in.Prefer)
			if err != nil {
				return err
			}
			return a.ExecuteStandardProject.Execute(ctx, gid, pid, in.Project, payment)
		})

	addAction(ts, g, "convert_plants", "Convert plants into a greenery tile (one action).",
		func(ctx context.Context, snap *Snapshot, in preferInput) error {
			payment, err := ts.quotePayment(snap.Game, snap.Player, action.PaymentIntent{Action: "convert-plants"}, in.Prefer)
			if err != nil {
				return err
			}
			return a.ConvertPlants.Execute(ctx, gid, pid, payment)
		})

	addAction(ts, g, "convert_heat", "Convert heat to raise the temperature (one action).",
		func(ctx context.Context, snap *Snapshot, in preferInput) error {
			payment, err := ts.quotePayment(snap.Game, snap.Player, action.PaymentIntent{Action: "convert-heat"}, in.Prefer)
			if err != nil {
				return err
			}
			return a.ConvertHeat.Execute(ctx, gid, pid, payment)
		})

	addAction(ts, g, "claim_milestone", "Claim a milestone (one action).",
		func(ctx context.Context, snap *Snapshot, in milestoneInput) error {
			in.MilestoneType = ts.milestoneID(in.MilestoneType)
			payment, err := ts.quotePayment(snap.Game, snap.Player, action.PaymentIntent{Action: "claim-milestone", MilestoneType: in.MilestoneType}, in.Prefer)
			if err != nil {
				return err
			}
			return a.ClaimMilestone.Execute(ctx, gid, pid, in.MilestoneType, payment)
		})

	addAction(ts, g, "fund_award", "Fund an award (one action).",
		func(ctx context.Context, snap *Snapshot, in awardInput) error {
			in.AwardType = ts.awardID(in.AwardType)
			payment, err := ts.quotePayment(snap.Game, snap.Player, action.PaymentIntent{Action: "fund-award", AwardType: in.AwardType}, in.Prefer)
			if err != nil {
				return err
			}
			return a.FundAward.Execute(ctx, gid, pid, in.AwardType, payment)
		})

	addAction(ts, g, "colony_trade", "Trade with a colony using a trade fleet (one action).",
		func(ctx context.Context, snap *Snapshot, in colonyTradeInput) error {
			payment, err := ts.quotePayment(snap.Game, snap.Player, action.PaymentIntent{Action: "colony-trade", PaymentType: in.PaymentType}, nil)
			if err != nil {
				return err
			}
			return a.ColonyTrade.Execute(ctx, gid, pid, in.ColonyID, colonyAction.TradePaymentType(in.PaymentType), in.TrackSteps, payment)
		})

	addAction(ts, g, "colony_build", "Build a colony on a colony tile (one action).",
		func(ctx context.Context, snap *Snapshot, in colonyBuildInput) error {
			payment, err := ts.quotePayment(snap.Game, snap.Player, action.PaymentIntent{Action: "build-colony"}, in.Prefer)
			if err != nil {
				return err
			}
			return a.ColonyBuild.Execute(ctx, gid, pid, in.ColonyID, payment)
		})

	addAction(ts, g, "project_fund_seat", "Buy the next seat of a funding project (one action). Credits cover whatever steel and titanium do not.",
		func(ctx context.Context, snap *Snapshot, in fundSeatInput) error {
			payment, err := fundSeatPayment(snap, in)
			if err != nil {
				return err
			}
			return a.FundSeat.Execute(ctx, gid, pid, in.ProjectID, payment)
		})

	addAction(ts, g, "skip_action", "Pass: skip your remaining actions this generation.",
		func(ctx context.Context, _ *Snapshot, _ noInput) error {
			return a.SkipAction.Execute(ctx, gid, pid)
		})

	addAction(ts, g, "select_tile", "Place a pending tile on one of the available hexes.",
		func(ctx context.Context, _ *Snapshot, in selectTileInput) error {
			_, err := a.SelectTile.Execute(ctx, gid, pid, strings.TrimSpace(in.Hex))
			return err
		})

	addAction(ts, g, "select_starting_choices", "Choose your corporation, preludes and starting cards to buy.",
		func(ctx context.Context, snap *Snapshot, in startingChoicesInput) error {
			payment, err := ts.quotePayment(snap.Game, snap.Player, action.PaymentIntent{Action: "select-starting-choices", CorporationID: in.CorporationID, CardIDs: in.CardIDs}, nil)
			if err != nil {
				return err
			}
			return a.SelectStartingChoices.Execute(ctx, gid, pid, in.CorporationID, in.PreludeIDs, in.CardIDs, payment)
		})

	addAction(ts, g, "confirm_production_cards", "Choose which drawn research cards to buy in the production phase (may be empty).",
		func(ctx context.Context, snap *Snapshot, in cardIDsInput) error {
			payment, err := ts.quotePayment(snap.Game, snap.Player, action.PaymentIntent{Action: "confirm-production-cards", CardIDs: in.CardIDs}, nil)
			if err != nil {
				return err
			}
			return a.ConfirmProductionCards.Execute(ctx, gid, pid, in.CardIDs, false, payment)
		})

	addAction(ts, g, "confirm_card_draw", "Resolve a pending card draw: cards to take for free and cards to buy.",
		func(ctx context.Context, snap *Snapshot, in cardDrawInput) error {
			payment, err := ts.quotePayment(snap.Game, snap.Player, action.PaymentIntent{Action: "confirm-card-draw", CardsToBuy: in.CardsToBuy}, nil)
			if err != nil {
				return err
			}
			return a.ConfirmCardDraw.Execute(ctx, gid, pid, in.CardsToTake, in.CardsToBuy, payment)
		})

	addAction(ts, g, "confirm_card_discard", "Resolve a pending discard.",
		func(ctx context.Context, _ *Snapshot, in cardDiscardInput) error {
			return a.ConfirmCardDiscard.Execute(ctx, gid, pid, in.ResolutionID, in.CardsToDiscard)
		})

	addAction(ts, g, "confirm_optional_behavior", "Accept or skip a pending optional effect. Guaranteed card gains are already applied.",
		func(ctx context.Context, _ *Snapshot, in optionalBehaviorInput) error {
			return a.ConfirmBehaviorChoice.ExecuteOptional(ctx, gid, pid, in.ResolutionID, in.Accept)
		})

	addAction(ts, g, "confirm_behavior_choice", "Resolve a pending behavior choice.",
		func(ctx context.Context, _ *Snapshot, in behaviorChoiceInput) error {
			return a.ConfirmBehaviorChoice.Execute(ctx, gid, pid, in.ResolutionID, in.ChoiceIndex, in.CardStorageTargets)
		})

	addAction(ts, g, "confirm_sell_patents", "Choose the cards to sell after starting sell-patents.",
		func(ctx context.Context, _ *Snapshot, in cardIDsInput) error {
			return a.ConfirmSellPatents.Execute(ctx, gid, pid, in.CardIDs)
		})

	addAction(ts, g, "confirm_effect_selection", "Resolve a pending effect selection by option index.",
		func(ctx context.Context, _ *Snapshot, in effectSelectionInput) error {
			return a.ConfirmEffectSelection.Execute(ctx, gid, pid, in.OptionIndex)
		})

	addAction(ts, g, "confirm_card_reveal", "Acknowledge a public card reveal.",
		func(ctx context.Context, _ *Snapshot, _ noInput) error {
			return a.ConfirmCardReveal.Execute(ctx, gid, pid)
		})

	addAction(ts, g, "confirm_resource_removal", "Resolve a pending resource removal.",
		func(ctx context.Context, _ *Snapshot, in resourceRemovalInput) error {
			return a.ConfirmResourceRemoval.Execute(ctx, gid, pid, in.SelectionID, in.TargetPlayerID, in.Amount)
		})

	addAction(ts, g, "confirm_colony_placement", "Choose the colony tile for a pending colony placement.",
		func(ctx context.Context, _ *Snapshot, in colonyIDInput) error {
			return a.ConfirmColonyPlacement.Execute(ctx, gid, pid, in.ColonyID)
		})

	addAction(ts, g, "confirm_colony_resource", "Choose the card that receives a pending colony resource.",
		func(ctx context.Context, _ *Snapshot, in cardIDInput) error {
			return a.ConfirmColonyResource.Execute(ctx, gid, pid, in.CardID)
		})

	addAction(ts, g, "confirm_award_fund", "Choose the award for a pending free award funding.",
		func(ctx context.Context, _ *Snapshot, in awardTypeInput) error {
			return a.ConfirmAwardFund.Execute(ctx, gid, pid, ts.awardID(in.AwardType))
		})

	addAction(ts, g, "confirm_free_trade", "Resolve a pending free trade: colony and track option.",
		func(ctx context.Context, _ *Snapshot, in freeTradeInput) error {
			return a.ConfirmFreeTrade.Execute(ctx, gid, pid, in.ColonyID, in.TrackSteps)
		})

	addAction(ts, g, "confirm_init_advance", "Confirm advancing the setup showcase when it is waiting on you.",
		func(ctx context.Context, _ *Snapshot, _ noInput) error {
			return a.ConfirmInitAdvance.Execute(ctx, gid, pid)
		})
}

func fundSeatPayment(snap *Snapshot, in fundSeatInput) (pfAction.FundSeatPayment, error) {
	for _, project := range snap.View.ProjectFunding {
		if project.ID != in.ProjectID {
			continue
		}
		credits := project.NextSeatCost
		for _, sub := range project.PaymentSubstitutes {
			switch shared.ResourceType(sub.ResourceType) {
			case shared.ResourceSteel:
				credits -= in.Steel * sub.ConversionRate
			case shared.ResourceTitanium:
				credits -= in.Titanium * sub.ConversionRate
			}
		}
		return pfAction.FundSeatPayment{Credits: max(0, credits), Steel: in.Steel, Titanium: in.Titanium}, nil
	}
	return pfAction.FundSeatPayment{}, fmt.Errorf("unknown project %q", in.ProjectID)
}

// milestoneID resolves a milestone given by ID or display name, in any case, to its ID.
func (ts *ToolServer) milestoneID(given string) string {
	for _, def := range ts.registries.Milestones.GetAll() {
		if strings.EqualFold(def.ID, given) || strings.EqualFold(def.Name, given) {
			return def.ID
		}
	}
	return given
}

// awardID resolves an award given by ID or display name, in any case, to its ID.
func (ts *ToolServer) awardID(given string) string {
	for _, def := range ts.registries.Awards.GetAll() {
		if strings.EqualFold(def.ID, given) || strings.EqualFold(def.Name, given) {
			return def.ID
		}
	}
	return given
}
