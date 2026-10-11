package card_packs_test

import (
	"openmars/internal/delivery/dto"
	"openmars/internal/game/datastore"

	"context"
	baseaction "openmars/internal/action"
	awardaction "openmars/internal/action/award"
	colonyaction "openmars/internal/action/colony"
	"openmars/internal/action/confirmation"
	milestoneaction "openmars/internal/action/milestone"
	"openmars/internal/action/standard_project"
	"openmars/internal/game/colony"
	"openmars/internal/game/standardproject"
	"testing"

	"openmars/internal/action/admin"
	cardAction "openmars/internal/action/card"
	tileAction "openmars/internal/action/tile"
	"openmars/internal/events"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func TestCrediCor_StartingResources(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("CrediCor"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed for CrediCor")

	p, _ := testGame.GetPlayer(playerID)
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 57, resources.Credits, "CrediCor should start with 57 credits")

	production := p.Resources().Production()
	testutil.AssertEqual(t, 0, production.Credits, "Credit production should be 0")
	testutil.AssertEqual(t, 0, production.Steel, "Steel production should be 0")
	testutil.AssertEqual(t, 0, production.Titanium, "Titanium production should be 0")
	testutil.AssertEqual(t, 0, production.Plants, "Plant production should be 0")
	testutil.AssertEqual(t, 0, production.Energy, "Energy production should be 0")
	testutil.AssertEqual(t, 0, production.Heat, "Heat production should be 0")
}

func TestCrediCor_Gain4MCWhenPlayingExpensiveCard(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("CrediCor"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed for CrediCor")

	p, _ := testGame.GetPlayer(playerID)
	res := p.Resources().Get()
	res.Credits = 100
	p.Resources().Set(res)

	cardID := testutil.CardID("Comet")
	p.Hand().AddCard(cardID)

	creditsBefore := p.Resources().Get().Credits

	playCard := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 21)
	err = playCard.Execute(ctx, testGame.ID(), playerID, cardID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "PlayCard should succeed for Comet")

	expected := creditsBefore - 21 + 4
	actual := p.Resources().Get().Credits
	testutil.AssertEqual(t, expected, actual, "CrediCor should gain 4 MC back after playing a card costing 20+ MC")
}

func TestEcoline_StartingResources(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Ecoline"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed for Ecoline")

	p, _ := testGame.GetPlayer(playerID)
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 36, resources.Credits, "Ecoline should start with 36 credits")
	testutil.AssertEqual(t, 3, resources.Plants, "Ecoline should start with 3 plants")

	production := p.Resources().Production()
	testutil.AssertEqual(t, 2, production.Plants, "Ecoline should start with 2 plant production")
}

func TestEcoline_DiscountEffectRegistered(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Ecoline"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed for Ecoline")

	p, _ := testGame.GetPlayer(playerID)
	effects := p.Effects().List()

	found := false
	for _, effect := range effects {
		if effect.CardName == "Ecoline" && effect.BehaviorIndex == 1 {
			found = true
			testutil.AssertEqual(t, shared.ResourceDiscount, effect.Behavior.Outputs[0].GetResourceType(),
				"Ecoline effect output should be a discount")
			testutil.AssertEqual(t, 1, effect.Behavior.Outputs[0].GetAmount(),
				"Ecoline discount amount should be 1")
			break
		}
	}
	testutil.AssertTrue(t, found, "Ecoline should have a registered discount effect at behavior index 1")

	calculator := gamecards.NewRequirementModifierCalculator(cardRegistry)
	discounts := calculator.CalculateStandardProjectDiscounts(p, shared.StandardProjectConvertPlantsToGreenery)
	testutil.AssertEqual(t, 1, discounts[shared.ResourcePlant],
		"Ecoline should provide 1 plant discount for convert-plants-to-greenery")
}

func TestHelion_StartingResources(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Helion"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)

	resources := p.Resources().Get()
	testutil.AssertEqual(t, 42, resources.Credits, "Should have 42 credits")

	production := p.Resources().Production()
	testutil.AssertEqual(t, 3, production.Heat, "Should have 3 heat production")
	testutil.AssertEqual(t, 0, production.Credits, "Credit production should be 0")
	testutil.AssertEqual(t, 0, production.Steel, "Steel production should be 0")
	testutil.AssertEqual(t, 0, production.Titanium, "Titanium production should be 0")
	testutil.AssertEqual(t, 0, production.Plants, "Plant production should be 0")
	testutil.AssertEqual(t, 0, production.Energy, "Energy production should be 0")
}

func TestHelion_CanPayWithHeat(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Helion"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)

	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceHeat: 10})

	p.Hand().AddCard(testutil.CardID("Virus"))

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{{Source: shared.PaymentSource{
		Target:   "self-player",
		Resource: shared.ResourceHeat}, TargetResource: shared.ResourceCredit, Amount: 1}},
	}
	err = playCardAction.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Virus"), payment, testutil.IntPtr(1), nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Playing Virus with heat payment should succeed")

	resources := p.Resources().Get()
	testutil.AssertEqual(t, 9, resources.Heat, "Heat should decrease by 1")
}

func TestInterplanetaryCinematics_StartingResources(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Interplanetary Cinematics"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)

	resources := p.Resources().Get()
	testutil.AssertEqual(t, 30, resources.Credits, "Should have 30 credits")
	testutil.AssertEqual(t, 20, resources.Steel, "Should have 20 steel")
}

func TestInterplanetaryCinematics_Gain2MCWhenPlayingEvent(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Interplanetary Cinematics"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)

	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})

	p.Hand().AddCard(testutil.CardID("Virus"))

	creditsBefore := p.Resources().Get().Credits

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 1)
	err = playCardAction.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Virus"), payment, testutil.IntPtr(1), nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Playing Virus should succeed")

	creditsAfter := p.Resources().Get().Credits
	testutil.AssertEqual(t, creditsBefore-1+2, creditsAfter, "Should gain 2 MC from IC effect after paying 1 for Virus")
}

func TestInventrix_StartingResources(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Inventrix"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 45, resources.Credits, "Inventrix should start with 45 credits")
}

func TestInventrix_GlobalParameterLenienceRegistered(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Inventrix"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	calculator := gamecards.NewRequirementModifierCalculator(cardRegistry)
	lenience := calculator.CalculateGlobalParameterRequirementOffset(p, "temperature")
	testutil.AssertEqual(t, 4, lenience, "Inventrix should allow 4 degrees")
}

func TestInventrix_LenienceStacksWithSpecialDesign(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Inventrix"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)

	calculator := gamecards.NewRequirementModifierCalculator(cardRegistry)
	lenienceBefore := calculator.CalculateGlobalParameterRequirementOffset(p, "temperature")
	testutil.AssertEqual(t, 4, lenienceBefore, "Inventrix alone should allow 4 degrees")

	specialDesignID := testutil.CardID("Special Design")
	p.Hand().AddCard(specialDesignID)
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})

	playCard := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 4)
	err = playCard.Execute(ctx, testGame.ID(), playerID, specialDesignID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Special Design should play successfully")

	lenienceAfter := calculator.CalculateGlobalParameterRequirementOffset(p, "temperature")
	testutil.AssertEqual(t, 8, lenienceAfter, "Inventrix and Special Design should allow 8 degrees")
}

func TestMiningGuild_StartingResources(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Mining Guild"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 30, resources.Credits, "Mining Guild should start with 30 credits")
	testutil.AssertEqual(t, 5, resources.Steel, "Mining Guild should start with 5 steel")

	production := p.Resources().Production()
	testutil.AssertEqual(t, 1, production.Steel, "Mining Guild should start with 1 steel production")
}

func TestMiningGuild_SteelProductionOnPlacementBonus(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Mining Guild"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	productionBefore := p.Resources().Production().Steel
	testutil.AssertEqual(t, 1, productionBefore, "Mining Guild should start with 1 steel production")

	events.Publish(testGame.EventBus(), events.PlacementBonusGainedEvent{
		GameID:   testGame.ID(),
		PlayerID: playerID,
		Resources: map[string]int{
			"steel": 1,
		},
	})

	productionAfter := p.Resources().Production().Steel
	testutil.AssertEqual(t, 2, productionAfter, "Steel production should be 2 after placement bonus trigger")
}

func TestPhoboLog_StartingResources(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("PhoboLog"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 23, resources.Credits, "PhoboLog should start with 23 credits")
	testutil.AssertEqual(t, 10, resources.Titanium, "PhoboLog should start with 10 titanium")
}

func TestPhoboLog_TitaniumWorthExtra(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("PhoboLog"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})

	transNeptuneProbeID := testutil.CardID("Trans-Neptune Probe")
	p.Hand().AddCard(transNeptuneProbeID)

	titaniumBefore := p.Resources().Get().Titanium

	playCard := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{{Source: shared.PaymentSource{
		Target: "self-player",

		Resource:               "titanium"}, TargetResource: shared.
		ResourceCredit, Amount: 2}},
	}
	err = playCard.Execute(ctx, testGame.ID(), playerID, transNeptuneProbeID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Playing Trans-Neptune Probe with 2 titanium should succeed (4 M€ each = 8 M€ covers 6-cost card)")

	titaniumAfter := p.Resources().Get().Titanium
	testutil.AssertEqual(t, titaniumBefore-2, titaniumAfter, "Titanium should decrease by 2")
}

func TestTharsisRepublic_StartingResources(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 40, resources.Credits, "Tharsis Republic should start with 40 credits")
}

func TestTharsisRepublic_ForcedFirstActionSetup(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	forcedAction := testGame.GetForcedFirstAction(playerID)
	testutil.AssertTrue(t, forcedAction != nil, "Tharsis Republic should create a forced first action")
	testutil.AssertEqual(t, "resolving", forcedAction.State, "First action should be resolving")
	testutil.AssertEqual(t, testutil.CardID("Tharsis Republic"), forcedAction.CorporationID, "Forced first action should reference Tharsis Republic")

	err = testGame.ProcessNextTile(ctx, playerID)
	testutil.AssertNoError(t, err, "Starting city should become selectable")
	selection := testGame.GetPendingTileSelection(playerID)
	if selection == nil || len(selection.AvailableHexes) == 0 {
		t.Fatal("Starting city has no placement selection")
	}
	testutil.AssertEqual(t, testutil.CardID("Tharsis Republic"), selection.SourceCardID, "Starting city should retain its source card")
	place := tileAction.NewSelectTileAction(repo, cardRegistry, nil, logger)
	_, err = place.Execute(ctx, testGame.ID(), playerID, selection.AvailableHexes[0])
	testutil.AssertNoError(t, err, "Starting city should be placed")
	for _, tile := range testGame.Board().Tiles() {
		if tile.Coordinates.String() == selection.AvailableHexes[0] {
			if tile.OccupiedBy == nil {
				t.Fatal("Starting city is missing")
			}
			testutil.AssertEqual(t, "Tharsis Republic", tile.OccupiedBy.DisplayName, "Starting city should display its corporation name")
			return
		}
	}
	t.Fatal("Starting city space is missing")
}

func TestTharsisRepublic_GainCreditsAndProductionOnCityPlacement(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	creditsBefore := p.Resources().Get().Credits
	creditProductionBefore := p.Resources().Production().Credits

	events.Publish(testGame.EventBus(), events.TilePlacedEvent{
		GameID:   testGame.ID(),
		PlayerID: playerID,
		TileType: string(shared.ResourceCityTile),
	})

	testutil.AssertEqual(t, creditsBefore+3, p.Resources().Get().Credits, "Self city placement should gain 3 M€")
	testutil.AssertEqual(t, creditProductionBefore+1, p.Resources().Production().Credits, "City on mars should increase M€ production by 1")
}

func TestThorGate_StartingResources(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("ThorGate"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 48, resources.Credits, "ThorGate should start with 48 credits")

	production := p.Resources().Production()
	testutil.AssertEqual(t, 1, production.Energy, "ThorGate should start with 1 energy production")
}

func TestThorGate_DiscountEffectRegistered(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("ThorGate"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)

	effects := p.Effects().List()
	found := false
	for _, effect := range effects {
		if effect.CardName == "ThorGate" && effect.BehaviorIndex == 1 {
			found = true
			break
		}
	}
	testutil.AssertTrue(t, found, "ThorGate should have registered its discount effect at behavior index 1")

	calculator := gamecards.NewRequirementModifierCalculator(cardRegistry)

	powerCard := &gamecards.Card{
		ID:   "test-power-card",
		Name: "Test Power Card",
		Type: gamecards.CardTypeAutomated,
		Cost: 10,
		Tags: []shared.CardTag{shared.TagPower},
	}
	discount := calculator.CalculateCardDiscounts(p, powerCard)
	testutil.AssertEqual(t, 3, discount, "ThorGate should provide 3 M€ discount for power cards")

	nonPowerCard := &gamecards.Card{
		ID:   "test-building-card",
		Name: "Test Building Card",
		Type: gamecards.CardTypeAutomated,
		Cost: 10,
		Tags: []shared.CardTag{shared.TagBuilding},
	}
	nonPowerDiscount := calculator.CalculateCardDiscounts(p, nonPowerCard)
	testutil.AssertEqual(t, 0, nonPowerDiscount, "ThorGate should provide no discount for non-power cards")
}

func TestUNMI_StartingResources(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("United Nations Mars Initiative"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 40, resources.Credits, "UNMI should start with 40 credits")
}

func TestUNMI_Pay3MCToRaiseTR(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("United Nations Mars Initiative"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.GenerationalEvents().Increment(shared.GenerationalEventTRRaise)

	trBefore := p.Resources().TerraformRating()
	creditsBefore := p.Resources().Get().Credits

	cardID := testutil.CardID("United Nations Mars Initiative")
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err = useAction.Execute(ctx, testGame.ID(), playerID, cardID, 1, nil, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "UNMI action should succeed")

	testutil.AssertEqual(t, creditsBefore-3, p.Resources().Get().Credits, "UNMI action should cost 3 credits")
	testutil.AssertEqual(t, trBefore+1, p.Resources().TerraformRating(), "UNMI action should raise TR by 1")
}

func TestInventrix_RequirementStepBoundaries(t *testing.T) {
	checkRequirementStepBoundaries(t, []string{"Inventrix"}, [4]int{2, 2, 2, 2})
}

func TestInventrix_StacksWithSpecialDesignAtStepBoundaries(t *testing.T) {
	checkRequirementStepBoundaries(t, []string{"Inventrix", "Special Design"}, [4]int{4, 4, 4, 4})
}

func TestHelion_AllCreditPaymentContexts(t *testing.T) {
	for _, kind := range []string{"play-card", "card-action", "standard-project", "confirm-production-cards", "confirm-card-draw", "build-colony", "colony-trade", "claim-milestone", "fund-award"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			log := testutil.TestLogger()
			testutil.AssertNoError(t, admin.NewSetCorporationAction(repo, registry, nil, log).Execute(ctx, g.ID(), id, "B03"), "Helion")
			p.Resources().Set(shared.Resources{Heat: 50})
			projects, err := standardproject.LoadStandardProjectsFromJSON("../../../assets/standard_projects.json")
			testutil.AssertNoError(t, err, "projects")
			projectRegistry := standardproject.NewInMemoryStandardProjectRegistry(projects)
			milestones := testutil.CreateTestMilestoneRegistry()
			awards := testutil.CreateTestAwardRegistry()
			colonyDefs, err := colony.LoadColoniesFromJSON("../../../assets/colonies.json")
			testutil.AssertNoError(t, err, "colonies")
			colonyRegistry := colony.NewInMemoryColonyRegistry(colonyDefs)
			intent := baseaction.PaymentIntent{Action: kind}
			switch kind {
			case "play-card":
				intent.CardID = testutil.CardID("Power Plant")
				p.Hand().AddCard(intent.CardID)
			case "card-action":
				card := testutil.GetCardByName("Search For Life")
				intent.CardID = card.ID
				p.PlayedCards().AddCard(card.ID, card.Name, string(card.Type), []string{"science"})
				for i, b := range card.Behaviors {
					if gamecards.HasManualTrigger(b) {
						intent.BehaviorIndex = i
						p.Actions().AddAction(shared.CardAction{CardID: card.ID, CardName: card.Name, BehaviorIndex: i, Behavior: b})
					}
				}
			case "standard-project":
				intent.ProjectID = "power-plant"
			case "confirm-production-cards":
				intent.CardIDs = []string{testutil.CardID("Power Plant")}
				testutil.AssertNoError(t, g.UpdatePhase(ctx, shared.GamePhaseProductionAndCardDraw), "research")
				testutil.AssertNoError(t, g.SetProductionPhase(ctx, id, &shared.ProductionPhase{AvailableCards: intent.CardIDs}), "research cards")
			case "confirm-card-draw":
				intent.CardsToBuy = []string{testutil.CardID("Power Plant")}
				p.Selection().SetPendingCardDrawSelection(&shared.PendingCardDrawSelection{AvailableCards: intent.CardsToBuy, MaxBuyCount: 1, CardBuyCost: 3})
			case "build-colony", "colony-trade":
				settings := g.Settings()
				settings.Systems = append(settings.Systems, "colonies")
				g.UpdateSettings(ctx, settings)
				g.Colonies().SetStates([]*colony.ColonyState{{DefinitionID: "ganymede", MarkerPosition: 0}})
				g.Colonies().AddTradeFleets(id, 1)
				intent.PaymentType = "credits"
			case "claim-milestone":
				intent.MilestoneType = "terraformer"
				p.Resources().SetTerraformRating(35)
			case "fund-award":
				intent.AwardType = "landlord"
			}
			quote, err := baseaction.QuoteActionPayment(g, p, registry, projectRegistry, milestones, awards, intent)
			testutil.AssertNoError(t, err, "quote")
			cost := quote.Costs[shared.ResourceCredit]
			payment := shared.Payment{Allocations: []shared.PaymentAllocation{{Source: shared.PaymentSource{Target: "self-player", Resource: shared.ResourceHeat}, TargetResource: shared.ResourceCredit, Amount: cost}}}
			_, err = gamecards.ValidatePayment(quote, payment)
			testutil.AssertNoError(t, err, "heat eligible")
			testutil.AssertEqual(t, 50, p.Resources().Get().Heat, "quote must not spend")
			switch kind {
			case "play-card":
				err = cardAction.NewPlayCardAction(repo, registry, nil, log).Execute(ctx, g.ID(), id, intent.CardID, payment, nil, nil, nil, nil, nil)
			case "card-action":
				err = cardAction.NewUseCardActionAction(repo, registry, nil, log).Execute(ctx, g.ID(), id, intent.CardID, intent.BehaviorIndex, nil, nil, nil, nil, nil, &payment, nil, nil)
			case "standard-project":
				err = standard_project.NewExecuteStandardProjectAction(repo, registry, projectRegistry, nil, log).Execute(ctx, g.ID(), id, intent.ProjectID, payment)
			case "confirm-production-cards":
				err = confirmation.NewConfirmProductionCardsAction(repo, registry, nil, log).Execute(ctx, g.ID(), id, intent.CardIDs, false, payment)
			case "confirm-card-draw":
				err = confirmation.NewConfirmCardDrawAction(repo, registry, log).Execute(ctx, g.ID(), id, nil, intent.CardsToBuy, payment)
			case "build-colony":
				err = colonyaction.NewBuildColonyAction(repo, colonyRegistry, registry, nil, log).Execute(ctx, g.ID(), id, "ganymede", payment)
			case "colony-trade":
				err = colonyaction.NewTradeAction(repo, colonyRegistry, registry, nil, log).Execute(ctx, g.ID(), id, "ganymede", colonyaction.TradePaymentCredits, 0, payment)
			case "claim-milestone":
				err = milestoneaction.NewClaimMilestoneAction(repo, registry, nil, milestones, log).Execute(ctx, g.ID(), id, intent.MilestoneType, payment)
			case "fund-award":
				err = awardaction.NewFundAwardAction(repo, registry, nil, awards, log).Execute(ctx, g.ID(), id, intent.AwardType, payment)
			}
			testutil.AssertNoError(t, err, "execute with heat")
			testutil.AssertEqual(t, 50-cost, p.Resources().Get().Heat, "spend heat once")
			testutil.AssertEqual(t, 0, p.Resources().Get().Credits, "no credits created or consumed")
		})
	}
}

func TestHelion_CorporationChangePreservesProjectPaymentRules(t *testing.T) {
	ctx := context.Background()
	g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	log := testutil.TestLogger()
	setCorp := admin.NewSetCorporationAction(repo, registry, nil, log)
	testutil.AssertNoError(t, setCorp.Execute(ctx, g.ID(), id, "B03"), "Helion")
	card := testutil.GetCardByName("Psychrophiles")
	p.Hand().AddCard(card.ID)
	testutil.AssertNoError(t, cardAction.NewPlayCardAction(repo, registry, nil, log).Execute(ctx, g.ID(), id, card.ID, shared.NativePayment(shared.ResourceCredit, card.Cost), nil, nil, nil, nil, nil), "project grant")
	testutil.AssertEqual(t, 2, len(p.Resources().PaymentSubstitutes()), "corporation and project grants")
	testutil.AssertNoError(t, setCorp.Execute(ctx, g.ID(), id, "B01"), "change corporation")
	rules := p.Resources().PaymentSubstitutes()
	testutil.AssertEqual(t, 1, len(rules), "only Helion grant removed")
	testutil.AssertEqual(t, card.ID, rules[0].GrantedByCardID, "keep project grant")
}

func TestInventrix_DeferredDrawReceipt(t *testing.T) {
	assertCorporationDrawReceipt(t, "Inventrix", []string{"001", "002", "003"}, 3, 0)
}

func assertCorporationDrawReceipt(t *testing.T, name string, deck []string, count, discards int) {
	t.Helper()
	ctx := context.Background()
	g, repo, registry, first, owner := testutil.SetupTwoPlayerGame(t)
	g.InitDeck(deck, nil, nil)
	testutil.AssertNoError(t, repo.DataStore().UpdateGame(g.ID(), func(s *datastore.GameState) { s.ProjectCards = append([]string{}, deck...) }), "fix draw order")
	p, _ := g.GetPlayer(owner)
	before := len(p.Hand().Cards())
	testutil.AssertNoError(t, g.UpdatePhase(ctx, shared.GamePhaseInitApplyCorp), "enter setup")
	setter := admin.NewSetCorporationAction(repo, registry, testutil.CreateTestAwardRegistry(), testutil.TestLogger())
	testutil.AssertNoError(t, setter.Execute(ctx, g.ID(), owner, testutil.CardID(name)), "set corporation")
	testutil.AssertEqual(t, before, len(p.Hand().Cards()), "setup does not draw")
	testutil.AssertEqual(t, 0, len(p.Selection().CardReceipts()), "setup creates no receipt")
	testutil.AssertEqual(t, "queued", g.GetForcedFirstAction(owner).State, "first action queued")
	testutil.AssertNoError(t, g.UpdatePhase(ctx, shared.GamePhaseInitApplyPrelude), "prelude phase")
	testutil.AssertNoError(t, g.SetCurrentTurn(ctx, owner, 2), "prelude phase cannot run first action")
	testutil.AssertEqual(t, before, len(p.Hand().Cards()), "preludes precede first action")
	testutil.AssertNoError(t, g.UpdatePhase(ctx, shared.GamePhaseAction), "enter actions")
	testutil.AssertNoError(t, g.SetCurrentTurn(ctx, first, 2), "other player goes first")
	testutil.AssertEqual(t, before, len(p.Hand().Cards()), "wait for owner's turn")
	testutil.AssertNoError(t, g.SetCurrentTurn(ctx, owner, 2), "owner starts turn")
	testutil.AssertEqual(t, before+count, len(p.Hand().Cards()), "cards received immediately")
	testutil.AssertEqual(t, discards, len(g.Deck().DiscardPile()), "nonmatches discarded")
	testutil.AssertEqual(t, 1, g.CurrentTurn().ActionsRemaining(), "first action spends one action")
	testutil.AssertTrue(t, g.GetForcedFirstAction(owner) == nil, "draw action complete")
	receipts := p.Selection().CardReceipts()
	testutil.AssertEqual(t, 1, len(receipts), "one draw receipt")
	testutil.AssertEqual(t, count, len(receipts[0].Cards), "receipt contains only received cards")
	testutil.AssertEqual(t, 1, len(dto.ToGameDto(g, registry, owner).CurrentPlayer.CardReceipts), "owner receives receipt after reconnect")
	testutil.AssertEqual(t, 0, len(dto.ToGameDto(g, registry, first).CurrentPlayer.CardReceipts), "other player sees no receipt")
	testutil.AssertEqual(t, 0, len(dto.ToGameDto(g, registry, "").CurrentPlayer.CardReceipts), "spectator sees no receipt")
	ack := confirmation.NewConfirmCardDrawAction(repo, registry, testutil.TestLogger())
	testutil.AssertNoError(t, ack.AcknowledgeReceipt(ctx, g.ID(), first, receipts[0].ID), "other player cannot dismiss owner's receipt")
	testutil.AssertEqual(t, 1, len(p.Selection().CardReceipts()), "owner receipt preserved")
	for range 2 {
		testutil.AssertNoError(t, ack.AcknowledgeReceipt(ctx, g.ID(), owner, receipts[0].ID), "close is idempotent")
	}
	testutil.AssertEqual(t, 0, len(p.Selection().CardReceipts()), "receipt dismissed")
	testutil.AssertEqual(t, before+count, len(p.Hand().Cards()), "close grants nothing")
	testutil.AssertEqual(t, 1, g.CurrentTurn().ActionsRemaining(), "close consumes nothing")
	testutil.AssertNoError(t, g.ExecuteFirstActionIfNeeded(ctx, owner), "repeated execution request")
	testutil.AssertEqual(t, before+count, len(p.Hand().Cards()), "first action cannot repeat")
}

func TestInventrix_SoloFirstActionRetainsUnlimitedActions(t *testing.T) {
	g, repo, registry, id := testutil.SetupSoloGame(t)
	ctx := context.Background()
	testutil.AssertNoError(t, g.SetCurrentTurn(ctx, id, -1), "unlimited turn")
	counter := g.CurrentTurn().GlobalActionCounter()
	testutil.AssertNoError(t, admin.NewSetCorporationAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, testutil.CardID("Inventrix")), "first action")
	testutil.AssertEqual(t, -1, g.CurrentTurn().ActionsRemaining(), "solo remains unlimited")
	testutil.AssertEqual(t, counter+1, g.CurrentTurn().GlobalActionCounter(), "first action recorded once")
}
