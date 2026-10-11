package card_effects_test

import (
	"context"
	"testing"

	baseaction "openmars/internal/action"
	cardAction "openmars/internal/action/card"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

// Synthetic test cards for requirement testing
func createTempReqTestCard() gamecards.Card {
	return gamecards.Card{
		ID:        "card-temp-req-test",
		Name:      "Temp Req Test",
		Type:      gamecards.CardTypeAutomated,
		GameModes: []string{"classic"},
		Cost:      5,
		Requirements: &gamecards.CardRequirements{Items: []gamecards.Requirement{
			{Type: gamecards.RequirementTemperature, Min: testutil.IntPtr(-24)},
		}},
	}
}

func createOxygenMaxReqTestCard() gamecards.Card {
	return gamecards.Card{
		ID:        "card-oxygen-max-req-test",
		Name:      "Oxygen Max Req Test",
		Type:      gamecards.CardTypeAutomated,
		GameModes: []string{"classic"},
		Cost:      5,
		Requirements: &gamecards.CardRequirements{Items: []gamecards.Requirement{
			{Type: gamecards.RequirementOxygen, Max: testutil.IntPtr(5)},
		}},
	}
}

// TestIndenturedWorkers_DiscountAppliedToNextCard verifies that playing Indentured Workers
// creates a temporary 8 M€ discount that applies to the next card played.
func TestIndenturedWorkers_DiscountAppliedToNextCard(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()
	ctx := context.Background()

	indenturedWorkersID := testutil.CardID("Indentured Workers")
	powerPlantID := testutil.CardID("Power Plant")

	players := testGame.GetAllPlayers()
	player := players[0]
	player.SetCorporationID(testutil.CardID("Tharsis Republic"))

	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, player.ID(), 2), "SetCurrentTurn failed")

	player.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	player.Hand().AddCard(indenturedWorkersID)
	player.Hand().AddCard(powerPlantID) // Cost 4

	// Play Indentured Workers (cost 0)
	playAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playAction.Execute(ctx, testGame.ID(), player.ID(), indenturedWorkersID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Failed to play Indentured Workers")

	// Verify: temporary discount effect is registered
	effects := player.Effects().List()
	testutil.AssertEqual(t, 1, len(effects), "Should have 1 effect after playing Indentured Workers")
	testutil.AssertEqual(t, indenturedWorkersID, effects[0].CardID, "Effect should be from Indentured Workers")

	// Verify: discount is calculated for next card
	calculator := gamecards.NewRequirementModifierCalculator(cardRegistry)
	powerPlantCard, err := cardRegistry.GetByID(powerPlantID)
	testutil.AssertNoError(t, err, "Power Plant card should exist")

	discount := calculator.CalculateCardDiscounts(player, powerPlantCard)
	testutil.AssertEqual(t, 8, discount, "Next card should have 8 credit discount from Indentured Workers")

	// Give player another action
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, player.ID(), 2), "SetCurrentTurn failed")

	// Play Power Plant (cost 4, but with 8 discount -> effective cost 0)
	payment = shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err = playAction.Execute(ctx, testGame.ID(), player.ID(), powerPlantID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Failed to play Power Plant with Indentured Workers discount")

	// Verify: temporary effect is removed after playing the next card
	effectsAfter := player.Effects().List()
	testutil.AssertEqual(t, 0, len(effectsAfter), "Temporary effect should be removed after next card is played")
}

// TestIndenturedWorkers_DiscountRemovedAfterOneCard verifies that the discount only
// applies to one card and is then removed.
func TestIndenturedWorkers_DiscountRemovedAfterOneCard(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()
	ctx := context.Background()

	indenturedWorkersID := testutil.CardID("Indentured Workers")
	powerPlantID := testutil.CardID("Power Plant")
	dustSealsID := testutil.CardID("Dust Seals")

	players := testGame.GetAllPlayers()
	player := players[0]
	player.SetCorporationID(testutil.CardID("Tharsis Republic"))

	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, player.ID(), 2), "SetCurrentTurn failed")

	player.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 200,
	})
	player.Hand().AddCard(indenturedWorkersID)
	player.Hand().AddCard(powerPlantID) // Cost 4
	player.Hand().AddCard(dustSealsID)  // Cost 2

	// Play Indentured Workers
	playAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	err := playAction.Execute(ctx, testGame.ID(), player.ID(), indenturedWorkersID, shared.Payment{Allocations: []shared.PaymentAllocation{}}, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Failed to play Indentured Workers")

	// Play Power Plant -> discount should apply
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, player.ID(), 2), "SetCurrentTurn failed")
	err = playAction.Execute(ctx, testGame.ID(), player.ID(), powerPlantID, shared.Payment{Allocations: []shared.PaymentAllocation{}}, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Failed to play Power Plant with discount")

	// Verify: discount is gone for the third card
	calculator := gamecards.NewRequirementModifierCalculator(cardRegistry)
	dustSealsCard, err := cardRegistry.GetByID(dustSealsID)
	testutil.AssertNoError(t, err, "Dust Seals card should exist")

	discount := calculator.CalculateCardDiscounts(player, dustSealsCard)
	testutil.AssertEqual(t, 0, discount, "No discount should remain for second card after Indentured Workers")
}

// TestSpecialDesign_LenienceAppliedToNextCard verifies that playing Special Design
// allows the next card to be played with ±2 global parameter lenience.
func TestSpecialDesign_LenienceAppliedToNextCard(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	additionalCards := []gamecards.Card{createTempReqTestCard()}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	logger := testutil.TestLogger()
	ctx := context.Background()

	specialDesignID := testutil.CardID("Special Design")
	tempReqTestID := "card-temp-req-test"

	players := testGame.GetAllPlayers()
	player := players[0]
	player.SetCorporationID(testutil.CardID("Tharsis Republic"))

	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, player.ID(), 2), "SetCurrentTurn failed")

	player.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	player.Hand().AddCard(specialDesignID)
	player.Hand().AddCard(tempReqTestID) // Requires temp >= -24C

	// Temperature is at -30C (default). Card requires -24C.
	// Let's increase temperature to -26
	_, err := testGame.GlobalParameters().IncreaseTemperature(ctx, 2, "") // -30 + 2*2 = -26
	testutil.AssertNoError(t, err, "IncreaseTemperature failed")

	// Without Special Design, temp is -26 and card requires -24 -> can't play
	tempReqCard, err := cardRegistry.GetByID(tempReqTestID)
	testutil.AssertNoError(t, err, "Temp req card should exist")

	state := baseaction.CalculatePlayerCardState(tempReqCard, player, testGame, cardRegistry)
	hasErrors := len(state.Errors) > 0
	testutil.AssertTrue(t, hasErrors, "Card should NOT be playable without lenience (temp -26, need -24)")

	// Play Special Design
	playAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	err = playAction.Execute(ctx, testGame.ID(), player.ID(), specialDesignID, shared.NativePayment(shared.
		ResourceCredit, 4), nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Failed to play Special Design")

	// Verify: lenience effect is registered
	effects := player.Effects().List()
	testutil.AssertEqual(t, 1, len(effects), "Should have 1 effect after playing Special Design")

	// Verify: lenience is calculated
	calculator := gamecards.NewRequirementModifierCalculator(cardRegistry)
	lenience := calculator.CalculateGlobalParameterRequirementOffset(player, "temperature")
	testutil.AssertEqual(t, 4, lenience, "Special Design should allow 4 degrees")

	// Two temperature steps lower the effective minimum from -24 to -28 degrees.
	state = baseaction.CalculatePlayerCardState(tempReqCard, player, testGame, cardRegistry)
	playable := len(state.Errors) == 0
	testutil.AssertTrue(t, playable, "Card should be playable with Special Design lenience (temp -26, need -24, allowance 4 -> effective -28)")

	// Play the temperature requirement card
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, player.ID(), 2), "SetCurrentTurn failed")
	err = playAction.Execute(ctx, testGame.ID(), player.ID(), tempReqTestID, shared.NativePayment(shared.
		ResourceCredit, 5), nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Should be able to play with Special Design lenience")

	// Verify: temporary effect is removed after playing the next card
	effectsAfter := player.Effects().List()
	testutil.AssertEqual(t, 0, len(effectsAfter), "Temporary effect should be removed after next card is played")
}

// TestSpecialDesign_LenienceWithMaxRequirement verifies lenience also works for max requirements.
func TestSpecialDesign_LenienceWithMaxRequirement(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	additionalCards := []gamecards.Card{createOxygenMaxReqTestCard()}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	logger := testutil.TestLogger()
	ctx := context.Background()

	specialDesignID := testutil.CardID("Special Design")
	oxygenTestID := "card-oxygen-max-req-test"

	players := testGame.GetAllPlayers()
	player := players[0]
	player.SetCorporationID(testutil.CardID("Tharsis Republic"))

	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, player.ID(), 2), "SetCurrentTurn failed")

	player.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	player.Hand().AddCard(specialDesignID)
	player.Hand().AddCard(oxygenTestID) // Requires oxygen <= 5%

	// Increase oxygen to 7% (3.5 steps from default 0)
	_, err := testGame.GlobalParameters().IncreaseOxygen(ctx, 7, "")
	testutil.AssertNoError(t, err, "IncreaseOxygen failed")

	// Without lenience: oxygen 7, max 5 -> can't play
	oxygenCard, err := cardRegistry.GetByID(oxygenTestID)
	testutil.AssertNoError(t, err, "Oxygen card should exist")

	state := baseaction.CalculatePlayerCardState(oxygenCard, player, testGame, cardRegistry)
	hasErrors := len(state.Errors) > 0
	testutil.AssertTrue(t, hasErrors, "Card should NOT be playable without lenience (oxygen 7, max 5)")

	// Play Special Design
	playAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	err = playAction.Execute(ctx, testGame.ID(), player.ID(), specialDesignID, shared.NativePayment(shared.
		ResourceCredit, 4), nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Failed to play Special Design")

	// With lenience 2: effective max = 5 + 2 = 7. Oxygen 7 <= 7 -> playable
	state = baseaction.CalculatePlayerCardState(oxygenCard, player, testGame, cardRegistry)
	playable := len(state.Errors) == 0
	testutil.AssertTrue(t, playable, "Card should be playable with lenience (oxygen 7, max 5+2=7)")

	// Play the card
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, player.ID(), 2), "SetCurrentTurn failed")
	err = playAction.Execute(ctx, testGame.ID(), player.ID(), oxygenTestID, shared.NativePayment(shared.
		ResourceCredit, 5), nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Should be able to play with Special Design lenience")
}

// TestTemporaryEffects_ClearedOnGenerationAdvance verifies that temporary effects
// are cleared when the generation advances.
func TestTemporaryEffects_ClearedOnGenerationAdvance(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()
	ctx := context.Background()

	indenturedWorkersID := testutil.CardID("Indentured Workers")

	players := testGame.GetAllPlayers()
	player := players[0]
	player.SetCorporationID(testutil.CardID("Tharsis Republic"))

	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, player.ID(), 2), "SetCurrentTurn failed")

	player.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	player.Hand().AddCard(indenturedWorkersID)

	// Play Indentured Workers
	playAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	err := playAction.Execute(ctx, testGame.ID(), player.ID(), indenturedWorkersID, shared.Payment{Allocations: []shared.PaymentAllocation{}}, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Failed to play Indentured Workers")

	// Verify effect is registered
	effects := player.Effects().List()
	testutil.AssertEqual(t, 1, len(effects), "Should have 1 effect")

	// Advance generation -> should clear temporary effects
	testutil.AssertNoError(t, testGame.AdvanceGeneration(ctx), "AdvanceGeneration failed")

	// Verify effects are cleared
	effectsAfter := player.Effects().List()
	testutil.AssertEqual(t, 0, len(effectsAfter), "Temporary effects should be cleared on generation advance")
}

// TestIndenturedWorkers_WithExistingPermanentDiscount verifies that playing
// Indentured Workers doesn't affect existing permanent discounts.
func TestIndenturedWorkers_WithExistingPermanentDiscount(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	cardRegistry := testutil.CreateTestCardRegistry()
	logger := testutil.TestLogger()
	ctx := context.Background()

	spaceStationID := testutil.CardID("Space Station")
	indenturedWorkersID := testutil.CardID("Indentured Workers")
	spaceMirrorsID := testutil.CardID("Space Mirrors")

	players := testGame.GetAllPlayers()
	player := players[0]
	player.SetCorporationID(testutil.CardID("Tharsis Republic"))

	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, player.ID(), 2), "SetCurrentTurn failed")

	player.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 200,
	})
	player.Hand().AddCard(spaceStationID) // Permanent -2 for space tags
	player.Hand().AddCard(indenturedWorkersID)
	player.Hand().AddCard(spaceMirrorsID) // Space tag, cost 3

	// Play Space Station (permanent space discount of 2)
	playAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	err := playAction.Execute(ctx, testGame.ID(), player.ID(), spaceStationID, shared.NativePayment(shared.
		ResourceCredit, 10), nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Failed to play Space Station")

	// Play Indentured Workers (temporary discount of 8)
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, player.ID(), 2), "SetCurrentTurn failed")
	err = playAction.Execute(ctx, testGame.ID(), player.ID(), indenturedWorkersID, shared.Payment{Allocations: []shared.PaymentAllocation{}}, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Failed to play Indentured Workers")

	// Verify: both effects exist (1 permanent + 1 temporary)
	effects := player.Effects().List()
	testutil.AssertEqual(t, 2, len(effects), "Should have 2 effects (permanent + temporary)")

	// Verify: combined discount for Space Mirrors = 2 (space) + 8 (indentured) = 10
	calculator := gamecards.NewRequirementModifierCalculator(cardRegistry)
	spaceMirrors, _ := cardRegistry.GetByID(spaceMirrorsID)
	discount := calculator.CalculateCardDiscounts(player, spaceMirrors)
	testutil.AssertEqual(t, 10, discount, "Space Mirrors should have combined discount of 10")

	// Play Space Mirrors -> temporary effect consumed
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, player.ID(), 2), "SetCurrentTurn failed")
	err = playAction.Execute(ctx, testGame.ID(), player.ID(), spaceMirrorsID, shared.Payment{Allocations: []shared.PaymentAllocation{}}, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Failed to play Space Mirrors")

	// Verify: only permanent effect remains
	effectsAfter := player.Effects().List()
	testutil.AssertEqual(t, 1, len(effectsAfter), "Only permanent effect should remain after temporary is consumed")
	testutil.AssertEqual(t, spaceStationID, effectsAfter[0].CardID, "Remaining effect should be Space Station")
}
