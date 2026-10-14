package confirmation_test

import (
	"context"
	"reflect"
	"testing"

	baseaction "openmars/internal/action"
	confirmation "openmars/internal/action/confirmation"
	"openmars/internal/delivery/dto"
	"openmars/internal/game/cards"
	"openmars/internal/game/colony"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func TestContinueImpossibleSelection(t *testing.T) {
	for _, kind := range []string{"colony", "tile", "trade", "award", "effect"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			g, repo, registry, playerID, otherID := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(playerID)
			settings := g.Settings()
			settings.Systems = append(settings.Systems, "colonies")
			g.UpdateSettings(ctx, settings)
			defs, err := colony.LoadColoniesFromJSON("../../../assets/colonies.json")
			testutil.AssertNoError(t, err, "Load colonies")
			colonies := colony.NewInMemoryColonyRegistry(defs)
			awards := testutil.CreateTestAwardRegistry()
			g.Colonies().SetDefinitions(defs)
			g.Colonies().SetStates([]*colony.ColonyState{{DefinitionID: "luna", MarkerPosition: 3}})
			g.Colonies().AddTradeFleets(playerID, 1)
			var run func(context.Context, string, string) error
			var invalidate func()
			var flag func(dto.PlayerDto) bool
			var remains func() bool
			switch kind {
			case "colony", "tile":
				id := "luna"
				if kind == "tile" {
					id = "pluto"
				}
				p.Selection().SetPendingColonySelection(&shared.PendingColonySelection{Source: "Test colony", AvailableColonyIDs: []string{id}, AddTile: kind == "tile", Remaining: 2})
				run = confirmation.NewConfirmColonyPlacementAction(repo, registry, colonies, testutil.TestLogger()).Continue
				invalidate = func() {
					g.Colonies().SetStates([]*colony.ColonyState{{DefinitionID: "luna", AwaitingResource: "animal"}, {DefinitionID: "pluto"}})
				}
				flag = func(view dto.PlayerDto) bool { return view.PendingColonySelection.CanContinueWithoutSelection }
				remains = func() bool { return p.Selection().GetPendingColonySelection() != nil }
			case "trade":
				p.Selection().SetPendingFreeTradeSelection(&shared.PendingFreeTradeSelection{Source: "Test trade", AvailableColonyIDs: []string{"luna"}})
				run = confirmation.NewConfirmFreeTradeAction(repo, registry, colonies, nil).Continue
				invalidate = func() {
					for g.Colonies().TradeFleet(playerID).Available() > 0 {
						testutil.AssertNoError(t, g.Colonies().UseTradeFleet(playerID), "Use fleet")
					}
				}
				flag = func(view dto.PlayerDto) bool { return view.PendingFreeTradeSelection.CanContinueWithoutSelection }
				remains = func() bool { return p.Selection().GetPendingFreeTradeSelection() != nil }
			case "award":
				p.Selection().SetPendingAwardFundSelection(&shared.PendingAwardFundSelection{Source: "Test award", AvailableAwards: []string{"banker"}})
				run = confirmation.NewConfirmAwardFundAction(repo, registry, awards, testutil.TestLogger()).Continue
				invalidate = func() {
					testutil.AssertNoError(t, g.Awards().FundAward(ctx, shared.AwardType("banker"), otherID, 0), "Fund award")
				}
				flag = func(view dto.PlayerDto) bool { return view.PendingAwardFundSelection.CanContinueWithoutSelection }
				remains = func() bool { return p.Selection().GetPendingAwardFundSelection() != nil }
			case "effect":
				mine := testutil.GetCardByName("Mine")
				p.PlayedCards().AddCard(mine.ID, mine.Name, string(mine.Type), []string{"building"})
				workforce := testutil.GetCardByName("Robotic Workforce")
				outputs := workforce.Behaviors[0].Outputs
				options, _, err := cards.EffectSelectionOptions(outputs, p, g, registry, colonies)
				testutil.AssertNoError(t, err, "Resolve production copy options")
				testutil.AssertTrue(t, len(options) > 0, "Mine is a legal choice")
				p.Selection().SetPendingEffectSelection(&shared.PendingEffectSelection{Source: workforce.Name, SourceCardID: workforce.ID, Outputs: outputs, Options: options})
				run = confirmation.NewConfirmEffectSelectionAction(repo, registry, colonies, nil).Continue
				invalidate = func() { p.PlayedCards().RemoveCard(mine.ID) }
				flag = func(view dto.PlayerDto) bool { return view.PendingEffectSelection.CanContinueWithoutSelection }
				remains = func() bool { return p.Selection().GetPendingEffectSelection() != nil }
			}
			view := func() dto.PlayerDto { return dto.ToPlayerDto(p, g, registry, nil, awards, nil, colonies) }
			testutil.AssertFalse(t, flag(view()), "Legal choice cannot be skipped")
			testutil.AssertErrorContains(t, run(ctx, g.ID(), playerID), "legal choices", "Cannot bypass a legal choice")
			testutil.AssertTrue(t, remains(), "Rejected continuation preserves pending choice")
			invalidate()
			testutil.AssertTrue(t, flag(view()), "Stale choice exposes continuation")
			before := p.Resources().Get()
			testutil.AssertTrue(t, run(ctx, g.ID(), otherID) != nil, "Other player cannot clear actor's selection")
			testutil.AssertTrue(t, remains(), "Unauthorized request preserves selection")
			testutil.AssertNoError(t, run(ctx, g.ID(), playerID), "Resolve impossible effect")
			testutil.AssertFalse(t, remains(), "Only pending effect clears")
			testutil.AssertEqual(t, before, p.Resources().Get(), "Continuation neither refunds nor applies resources")
			testutil.AssertTrue(t, run(ctx, g.ID(), playerID) != nil, "Repeated continuation is rejected")
		})
	}
}

func TestPendingPlayabilityMatchesHandWithoutTurnBlockers(t *testing.T) {
	g, _, registry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(playerID)
	card := testutil.GetCardByName("Mine")
	p.Resources().Set(shared.Resources{Credits: 100})
	p.Selection().SetPendingCardSelection(&shared.PendingCardSelection{AvailableCards: []string{card.ID}, MinCards: 0, MaxCards: 1})
	preview := baseaction.CalculatePendingCardPlayability(&card, p, g, registry)
	hand := baseaction.CalculatePlayerCardState(&card, p, g, registry)
	testutil.AssertTrue(t, len(hand.Errors) > len(preview.Errors), "Pending flow blocks playing but not the preview")
	testutil.AssertTrue(t, reflect.DeepEqual(hand.Cost, preview.Cost), "Preview uses hand cost")
	testutil.AssertTrue(t, reflect.DeepEqual(hand.ComputedValues, preview.ComputedValues), "Preview uses hand computed values")
	p.Resources().Set(shared.Resources{})
	testutil.AssertTrue(t, len(baseaction.CalculatePendingCardPlayability(&card, p, g, registry).Errors) > 0, "Preview still checks affordability")
}
