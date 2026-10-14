package behavior_test

import (
	"context"
	"fmt"
	"log/slog"
	baseaction "openmars/internal/action"
	"testing"

	cardAction "openmars/internal/action/card"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

// ============================================================================
// Card resource tests
// ============================================================================

// --- CEO's Favorite Project (149) ---
// "Add 1 resource to a card with at least 1 resource on it"

func TestCardResource_CEOsFavoriteProject_AddsToAnimalCard(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	ceosFavorite := gamecards.Card{
		ID:   "card-ceos-favorite",
		Name: "CEO's Favorite Project",
		Type: gamecards.CardTypeEvent,
		Cost: 1,
		Behaviors: []shared.CardBehavior{
			{
				Triggers: []shared.Trigger{{Type: shared.TriggerTypeAuto}},
				Outputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceCardResource, 1, "any-card"),
				},
			},
		},
	}

	animalCard := gamecards.Card{
		ID: "card-animal-host", Name: "Animal Host", Type: gamecards.CardTypeActive, Cost: 0,
		Tags:            []shared.CardTag{shared.TagAnimal},
		ResourceStorage: &gamecards.ResourceStorage{Type: shared.ResourceAnimal, Starting: 0},
	}

	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{ceosFavorite, animalCard})

	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))

	err := testGame.UpdateStatus(ctx, shared.GameStatusActive)
	testutil.AssertNoError(t, err, "UpdateStatus failed")
	err = testGame.UpdatePhase(ctx, shared.GamePhaseAction)
	testutil.AssertNoError(t, err, "UpdatePhase failed")
	err = testGame.SetCurrentTurn(ctx, p.ID(), 2)
	testutil.AssertNoError(t, err, "SetCurrentTurn failed")

	// Set up played card with animal storage (with 1 existing resource)
	p.PlayedCards().AddCard("card-animal-host", "Animal Host", "active", []string{"animal"})
	p.Resources().AddToStorage("card-animal-host", 1) // Has 1 resource already

	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard("card-ceos-favorite")

	// Play CEO's Favorite Project targeting the animal card
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 1)
	targetCardID := "card-animal-host"
	err = playCardAction.Execute(ctx, testGame.ID(), p.ID(), "card-ceos-favorite", payment, nil, []string{targetCardID}, nil, nil, nil)
	testutil.AssertNoError(t, err, "CEO's Favorite Project should play successfully")

	// Verify 1 resource was added (1 existing + 1 new = 2)
	storage := p.Resources().GetCardStorage("card-animal-host")
	testutil.AssertEqual(t, 2, storage, "Should have 2 animals on card (1 existing + 1 added)")
}

func TestCardResource_CEOsFavoriteProject_AddsToMicrobeCard(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	ceosFavorite := gamecards.Card{
		ID:   "card-ceos-favorite",
		Name: "CEO's Favorite Project",
		Type: gamecards.CardTypeEvent,
		Cost: 1,
		Behaviors: []shared.CardBehavior{
			{
				Triggers: []shared.Trigger{{Type: shared.TriggerTypeAuto}},
				Outputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceCardResource, 1, "any-card"),
				},
			},
		},
	}

	microbeCard := gamecards.Card{
		ID: "card-microbe-host", Name: "Microbe Host", Type: gamecards.CardTypeActive, Cost: 0,
		Tags:            []shared.CardTag{shared.TagMicrobe},
		ResourceStorage: &gamecards.ResourceStorage{Type: shared.ResourceMicrobe, Starting: 0},
	}

	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{ceosFavorite, microbeCard})

	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))

	err := testGame.UpdateStatus(ctx, shared.GameStatusActive)
	testutil.AssertNoError(t, err, "UpdateStatus failed")
	err = testGame.UpdatePhase(ctx, shared.GamePhaseAction)
	testutil.AssertNoError(t, err, "UpdatePhase failed")
	err = testGame.SetCurrentTurn(ctx, p.ID(), 2)
	testutil.AssertNoError(t, err, "SetCurrentTurn failed")

	// Set up played card with microbe storage
	p.PlayedCards().AddCard("card-microbe-host", "Microbe Host", "active", []string{"microbe"})
	p.Resources().AddToStorage("card-microbe-host", 3) // Has 3 microbes

	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard("card-ceos-favorite")

	// Play CEO's Favorite Project targeting the microbe card
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 1)
	targetCardID := "card-microbe-host"
	err = playCardAction.Execute(ctx, testGame.ID(), p.ID(), "card-ceos-favorite", payment, nil, []string{targetCardID}, nil, nil, nil)
	testutil.AssertNoError(t, err, "CEO's Favorite Project should play targeting microbe card")

	// Verify 1 resource was added (3 existing + 1 new = 4)
	storage := p.Resources().GetCardStorage("card-microbe-host")
	testutil.AssertEqual(t, 4, storage, "Should have 4 microbes on card (3 existing + 1 added)")
}

// --- Corroder Suits (219) ---
// "Increase your M$ production 2 steps. Add 1 resource to **any venus card**."

func TestCardResource_CorroderSuits_AddsToVenusCard(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	corroderSuits := gamecards.Card{
		ID:   "card-corroder-suits",
		Name: "Corroder Suits",
		Type: gamecards.CardTypeAutomated,
		Cost: 8,
		Tags: []shared.CardTag{shared.TagVenus},
		Behaviors: []shared.CardBehavior{
			{
				Triggers: []shared.Trigger{{Type: shared.TriggerTypeAuto}},
				Outputs: []shared.BehaviorCondition{
					shared.NewProductionCondition(shared.ResourceCreditProduction, 2, "self-player"),
					&shared.CardStorageCondition{
						ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceCardResource, Amount: 1, Target: "any-card"},
						Selectors:     []shared.Selector{{Tags: []shared.CardTag{shared.TagVenus}}},
					},
				},
			},
		},
	}

	venusFloaterCard := gamecards.Card{
		ID: "card-venus-floater", Name: "Venus Floater Card", Type: gamecards.CardTypeActive, Cost: 0,
		Tags:            []shared.CardTag{shared.TagVenus},
		ResourceStorage: &gamecards.ResourceStorage{Type: shared.ResourceFloater, Starting: 0},
	}

	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{corroderSuits, venusFloaterCard})

	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))

	err := testGame.UpdateStatus(ctx, shared.GameStatusActive)
	testutil.AssertNoError(t, err, "UpdateStatus failed")
	err = testGame.UpdatePhase(ctx, shared.GamePhaseAction)
	testutil.AssertNoError(t, err, "UpdatePhase failed")
	err = testGame.SetCurrentTurn(ctx, p.ID(), 2)
	testutil.AssertNoError(t, err, "SetCurrentTurn failed")

	// Set up played venus card with floater storage
	p.PlayedCards().AddCard("card-venus-floater", "Venus Floater Card", "active", []string{"venus"})
	p.Resources().AddToStorage("card-venus-floater", 0)

	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard("card-corroder-suits")

	productionBefore := p.Resources().Production()

	// Play Corroder Suits targeting the venus floater card
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 8)
	targetCardID := "card-venus-floater"
	err = playCardAction.Execute(ctx, testGame.ID(), p.ID(), "card-corroder-suits", payment, nil, []string{targetCardID}, nil, nil, nil)
	testutil.AssertNoError(t, err, "Corroder Suits should play successfully")

	// Verify production increased
	productionAfter := p.Resources().Production()
	testutil.AssertEqual(t, productionBefore.Credits+2, productionAfter.Credits, "Should gain 2 credit production")

	// Verify 1 resource was added to the venus card
	storage := p.Resources().GetCardStorage("card-venus-floater")
	testutil.AssertEqual(t, 1, storage, "Should have 1 floater on venus card")
}

// --- Maxwell Base (238) - Card Action ---
// "Action: Add 1 resource to **another venus card**."

func TestCardResource_MaxwellBase_ActionAddsToVenusCard(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	maxwellBase := gamecards.Card{
		ID:   "card-maxwell-base",
		Name: "Maxwell Base",
		Type: gamecards.CardTypeActive,
		Cost: 18,
		Tags: []shared.CardTag{shared.TagCity, shared.TagVenus},
		Behaviors: []shared.CardBehavior{
			{
				Triggers: []shared.Trigger{{Type: shared.TriggerTypeAuto}},
				Outputs: []shared.BehaviorCondition{
					shared.NewProductionCondition(shared.ResourceEnergyProduction, -1, "self-player"),
				},
			},
			{
				Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
				Outputs: []shared.BehaviorCondition{
					&shared.CardStorageCondition{
						ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceCardResource, Amount: 1, Target: "any-card"},
						Selectors:     []shared.Selector{{Tags: []shared.CardTag{shared.TagVenus}}},
					},
				},
			},
		},
	}

	venusMicrobeCard := gamecards.Card{
		ID: "card-venus-microbe", Name: "Venus Microbe Card", Type: gamecards.CardTypeActive, Cost: 0,
		Tags:            []shared.CardTag{shared.TagVenus},
		ResourceStorage: &gamecards.ResourceStorage{Type: shared.ResourceMicrobe, Starting: 0},
	}

	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{maxwellBase, venusMicrobeCard})

	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))

	err := testGame.UpdateStatus(ctx, shared.GameStatusActive)
	testutil.AssertNoError(t, err, "UpdateStatus failed")
	err = testGame.UpdatePhase(ctx, shared.GamePhaseAction)
	testutil.AssertNoError(t, err, "UpdatePhase failed")
	err = testGame.SetCurrentTurn(ctx, p.ID(), 2)
	testutil.AssertNoError(t, err, "SetCurrentTurn failed")

	// Set up the venus microbe card
	p.PlayedCards().AddCard("card-venus-microbe", "Venus Microbe Card", "active", []string{"venus"})
	p.Resources().AddToStorage("card-venus-microbe", 2) // 2 existing microbes

	// Register Maxwell Base as a played card with its manual action
	p.PlayedCards().AddCard("card-maxwell-base", "Maxwell Base", "active", []string{"city", "venus"})
	p.Actions().AddAction(shared.CardAction{
		CardID:        maxwellBase.ID,
		CardName:      maxwellBase.Name,
		BehaviorIndex: 1,
		Behavior:      maxwellBase.Behaviors[1],
	})

	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})

	// Use Maxwell Base action targeting the venus microbe card
	useCardAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	targetCardID := "card-venus-microbe"
	err = useCardAction.Execute(ctx, testGame.ID(), p.ID(), "card-maxwell-base", 1, nil, []string{targetCardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Maxwell Base action should execute successfully")

	// Verify 1 resource was added (2 existing + 1 new = 3)
	storage := p.Resources().GetCardStorage("card-venus-microbe")
	testutil.AssertEqual(t, 3, storage, "Should have 3 microbes on venus card (2 existing + 1 added)")
}

// --- card-resource fails without target ---

func TestCardResource_FailsWithoutTargetCard(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	ceosFavorite := gamecards.Card{
		ID:   "card-ceos-favorite",
		Name: "CEO's Favorite Project",
		Type: gamecards.CardTypeEvent,
		Cost: 1,
		Behaviors: []shared.CardBehavior{
			{
				Triggers: []shared.Trigger{{Type: shared.TriggerTypeAuto}},
				Outputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceCardResource, 1, "any-card"),
				},
			},
		},
	}

	animalCard := gamecards.Card{
		ID: "card-animal-host", Name: "Animal Host", Type: gamecards.CardTypeActive, Cost: 0,
		Tags:            []shared.CardTag{shared.TagAnimal},
		ResourceStorage: &gamecards.ResourceStorage{Type: shared.ResourceAnimal, Starting: 0},
	}

	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{ceosFavorite, animalCard})

	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))

	err := testGame.UpdateStatus(ctx, shared.GameStatusActive)
	testutil.AssertNoError(t, err, "UpdateStatus failed")
	err = testGame.UpdatePhase(ctx, shared.GamePhaseAction)
	testutil.AssertNoError(t, err, "UpdatePhase failed")
	err = testGame.SetCurrentTurn(ctx, p.ID(), 2)
	testutil.AssertNoError(t, err, "SetCurrentTurn failed")

	p.PlayedCards().AddCard("card-animal-host", "Animal Host", "active", []string{"animal"})
	p.Resources().AddToStorage("card-animal-host", 1)

	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard("card-ceos-favorite")

	// Play without specifying a target card — should fail
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 1)
	err = playCardAction.Execute(ctx, testGame.ID(), p.ID(), "card-ceos-favorite", payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "select a card for resource storage", "Should fail without target card for card-resource output")
	testutil.AssertTrue(t, p.Hand().HasCard("card-ceos-favorite"), "card stays in hand")
}

// --- card-resource fails when target card has no storage ---

func TestCardResource_FailsWhenTargetHasNoStorage(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	ceosFavorite := gamecards.Card{
		ID:   "card-ceos-favorite",
		Name: "CEO's Favorite Project",
		Type: gamecards.CardTypeEvent,
		Cost: 1,
		Behaviors: []shared.CardBehavior{
			{
				Triggers: []shared.Trigger{{Type: shared.TriggerTypeAuto}},
				Outputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceCardResource, 1, "any-card"),
				},
			},
		},
	}

	noStorageCard := gamecards.Card{
		ID: "card-no-storage", Name: "No Storage Card", Type: gamecards.CardTypeAutomated, Cost: 0,
	}

	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{ceosFavorite, noStorageCard})

	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))

	err := testGame.UpdateStatus(ctx, shared.GameStatusActive)
	testutil.AssertNoError(t, err, "UpdateStatus failed")
	err = testGame.UpdatePhase(ctx, shared.GamePhaseAction)
	testutil.AssertNoError(t, err, "UpdatePhase failed")
	err = testGame.SetCurrentTurn(ctx, p.ID(), 2)
	testutil.AssertNoError(t, err, "SetCurrentTurn failed")

	p.PlayedCards().AddCard("card-no-storage", "No Storage Card", "automated", []string{})

	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard("card-ceos-favorite")

	// Play targeting a card with no storage — should fail
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 1)
	targetCardID := "card-no-storage"
	err = playCardAction.Execute(ctx, testGame.ID(), p.ID(), "card-ceos-favorite", payment, nil, []string{targetCardID}, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "card cannot store that resource", "Should fail when target card has no resource storage")
	testutil.AssertTrue(t, p.Hand().HasCard("card-ceos-favorite"), "card stays in hand")
}

// Other outputs still apply when no eligible storage card exists.

func TestCardResource_AnyCardTarget_SkipsWhenNoTargetCard(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	jovianTag := shared.TagJovian
	selfPlayer := "self-player"

	storageExample := gamecards.Card{
		ID:   "card-storage-example",
		Name: "Storage example",
		Type: gamecards.CardTypeEvent,
		Cost: 11,
		Tags: []shared.CardTag{"space"},
		Behaviors: []shared.CardBehavior{
			{
				Triggers: []shared.Trigger{{Type: shared.TriggerTypeAuto}},
				Outputs: []shared.BehaviorCondition{
					shared.NewProductionCondition(shared.ResourceCreditProduction, 1, "self-player"),
					&shared.CardStorageCondition{
						ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceFloater, Amount: 1, Target: "any-card"},
						Selectors:     []shared.Selector{{Tags: []shared.CardTag{"venus"}}},
						Per: &shared.PerCondition{
							ResourceType: "tag",
							Amount:       1,
							Target:       &selfPlayer,
							Tag:          &jovianTag,
						},
					},
				},
			},
		},
	}

	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{storageExample})

	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))

	err := testGame.UpdateStatus(ctx, shared.GameStatusActive)
	testutil.AssertNoError(t, err, "UpdateStatus failed")
	err = testGame.UpdatePhase(ctx, shared.GamePhaseAction)
	testutil.AssertNoError(t, err, "UpdatePhase failed")
	err = testGame.SetCurrentTurn(ctx, p.ID(), 2)
	testutil.AssertNoError(t, err, "SetCurrentTurn failed")

	// Give player jovian tags (via played cards) but NO venus card with floater storage
	p.PlayedCards().AddCard("card-jovian-1", "Jovian Card", "automated", []string{"jovian"})
	p.PlayedCards().AddCard("card-jovian-2", "Jovian Card 2", "automated", []string{"jovian"})

	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard("card-storage-example")

	initialProduction := p.Resources().Production().Credits

	// Play without providing a target card — should succeed, skipping floater placement
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 11)
	err = playCardAction.Execute(ctx, testGame.ID(), p.ID(), "card-storage-example", payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Card should play successfully even without a valid floater target")

	// Verify the other output (production) was still applied
	newProduction := p.Resources().Production().Credits
	testutil.AssertEqual(t, initialProduction+1, newProduction, "Credit production should have increased by 1")

	// Verify card was removed from hand
	testutil.AssertEqual(t, false, p.Hand().HasCard("card-storage-example"), "Card should be removed from hand")
}

// ============================================================================
// Card steal tests
// ============================================================================

func TestPredatorsStealAnimalFromOtherPlayer(t *testing.T) {
	testGame, repo, cardRegistry, playerID, otherPlayerID := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()

	p, _ := testGame.GetPlayer(playerID)
	other, _ := testGame.GetPlayer(otherPlayerID)

	predatorsID := testutil.CardID("Predators")
	p.PlayedCards().AddCard(predatorsID, "Predators", "active", []string{"animal"})
	p.Resources().AddToStorage(predatorsID, 0)

	targetCardID := testutil.CardID("Birds")
	other.PlayedCards().AddCard(targetCardID, "Birds", "active", []string{"animal"})
	other.Resources().AddToStorage(targetCardID, 3)

	predatorsBehavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Outputs: []shared.BehaviorCondition{
			shared.NewCardStorageCondition(shared.ResourceAnimal, 1, "steal-from-any-card"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        predatorsID,
			CardName:      "Predators",
			BehaviorIndex: 0,
			Behavior:      predatorsBehavior,
		},
	})

	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	ctx := context.Background()

	err := useAction.Execute(ctx, testGame.ID(), playerID, predatorsID, 0, nil, []string{predatorsID}, nil, &targetCardID, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Predators steal action should succeed")

	testutil.AssertEqual(t, 2, other.Resources().GetCardStorage(targetCardID), "Target card should have 2 animals after steal")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(predatorsID), "Predators card should have 1 animal after steal")
}

func TestPredatorsStealFromOwnCard(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()

	p, _ := testGame.GetPlayer(playerID)

	predatorsID := testutil.CardID("Predators")
	p.PlayedCards().AddCard(predatorsID, "Predators", "active", []string{"animal"})
	p.Resources().AddToStorage(predatorsID, 0)

	targetCardID := testutil.CardID("Fish")
	p.PlayedCards().AddCard(targetCardID, "Fish", "active", []string{"animal"})
	p.Resources().AddToStorage(targetCardID, 2)

	predatorsBehavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Outputs: []shared.BehaviorCondition{
			shared.NewCardStorageCondition(shared.ResourceAnimal, 1, "steal-from-any-card"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        predatorsID,
			CardName:      "Predators",
			BehaviorIndex: 0,
			Behavior:      predatorsBehavior,
		},
	})

	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	ctx := context.Background()

	err := useAction.Execute(ctx, testGame.ID(), playerID, predatorsID, 0, nil, []string{predatorsID}, nil, &targetCardID, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Predators steal from own card should succeed")

	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(targetCardID), "Target card should have 1 animal after steal")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(predatorsID), "Predators card should have 1 animal after steal")
}

func TestPredatorsRejectsWithNoSourceCard(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()

	p, _ := testGame.GetPlayer(playerID)

	predatorsID := testutil.CardID("Predators")
	p.PlayedCards().AddCard(predatorsID, "Predators", "active", []string{"animal"})
	p.Resources().AddToStorage(predatorsID, 0)

	predatorsBehavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Outputs: []shared.BehaviorCondition{
			shared.NewCardStorageCondition(shared.ResourceAnimal, 1, "steal-from-any-card"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        predatorsID,
			CardName:      "Predators",
			BehaviorIndex: 0,
			Behavior:      predatorsBehavior,
		},
	})

	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	ctx := context.Background()

	err := useAction.Execute(ctx, testGame.ID(), playerID, predatorsID, 0, nil, []string{predatorsID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "steal action requires a target card", "Predators without source card should be rejected")
	testutil.AssertEqual(t, 0, p.Resources().GetCardStorage(predatorsID), "Predators card should have 0 animals when no source card specified")
}

func TestPredatorsStealFromCardWithZeroAnimals(t *testing.T) {
	testGame, repo, cardRegistry, playerID, otherPlayerID := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()

	p, _ := testGame.GetPlayer(playerID)
	other, _ := testGame.GetPlayer(otherPlayerID)

	predatorsID := testutil.CardID("Predators")
	p.PlayedCards().AddCard(predatorsID, "Predators", "active", []string{"animal"})
	p.Resources().AddToStorage(predatorsID, 0)

	targetCardID := testutil.CardID("Birds")
	other.PlayedCards().AddCard(targetCardID, "Birds", "active", []string{"animal"})
	other.Resources().AddToStorage(targetCardID, 0)

	predatorsBehavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Outputs: []shared.BehaviorCondition{
			shared.NewCardStorageCondition(shared.ResourceAnimal, 1, "steal-from-any-card"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        predatorsID,
			CardName:      "Predators",
			BehaviorIndex: 0,
			Behavior:      predatorsBehavior,
		},
	})

	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	ctx := context.Background()

	err := useAction.Execute(ctx, testGame.ID(), playerID, predatorsID, 0, nil, []string{predatorsID}, nil, &targetCardID, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "insufficient resources on card", "Predators must reject an empty source")

	testutil.AssertEqual(t, 0, other.Resources().GetCardStorage(targetCardID), "Target card should still have 0 animals")
	testutil.AssertEqual(t, 0, p.Resources().GetCardStorage(predatorsID), "Predators card should have 0 animals when source had none")
}

func TestAntsStealMicrobeFromOtherPlayer(t *testing.T) {
	testGame, repo, cardRegistry, playerID, otherPlayerID := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()

	p, _ := testGame.GetPlayer(playerID)
	other, _ := testGame.GetPlayer(otherPlayerID)

	antsID := testutil.CardID("Ants")
	p.PlayedCards().AddCard(antsID, "Ants", "active", []string{"microbe"})
	p.Resources().AddToStorage(antsID, 0)

	targetCardID := testutil.CardID("Decomposers")
	other.PlayedCards().AddCard(targetCardID, "Decomposers", "active", []string{"microbe"})
	other.Resources().AddToStorage(targetCardID, 5)

	antsBehavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Outputs: []shared.BehaviorCondition{
			shared.NewCardStorageCondition(shared.ResourceMicrobe, 1, "steal-from-any-card"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        antsID,
			CardName:      "Ants",
			BehaviorIndex: 0,
			Behavior:      antsBehavior,
		},
	})

	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	ctx := context.Background()

	err := useAction.Execute(ctx, testGame.ID(), playerID, antsID, 0, nil, []string{antsID}, nil, &targetCardID, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Ants steal action should succeed")

	testutil.AssertEqual(t, 4, other.Resources().GetCardStorage(targetCardID), "Target card should have 4 microbes after steal")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(antsID), "Ants card should have 1 microbe after steal")
}

func TestAntsRejectsWithNoSourceCard(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()

	p, _ := testGame.GetPlayer(playerID)

	antsID := testutil.CardID("Ants")
	p.PlayedCards().AddCard(antsID, "Ants", "active", []string{"microbe"})
	p.Resources().AddToStorage(antsID, 0)

	antsBehavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Outputs: []shared.BehaviorCondition{
			shared.NewCardStorageCondition(shared.ResourceMicrobe, 1, "steal-from-any-card"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        antsID,
			CardName:      "Ants",
			BehaviorIndex: 0,
			Behavior:      antsBehavior,
		},
	})

	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	ctx := context.Background()

	err := useAction.Execute(ctx, testGame.ID(), playerID, antsID, 0, nil, []string{antsID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "steal action requires a target card", "Ants without source card should be rejected")
	testutil.AssertEqual(t, 0, p.Resources().GetCardStorage(antsID), "Ants card should have 0 microbes when no source card specified")
}

func TestCardStorage_VariableAmountInput(t *testing.T) {
	testGame, _, _, playerID, _ := testutil.SetupTwoPlayerGame(t)

	p, _ := testGame.GetPlayer(playerID)

	cardID := "test-microbe-card"
	p.PlayedCards().AddCard(cardID, "Test Microbe Card", "active", []string{"microbe"})
	p.Resources().AddToStorage(cardID, 5)

	input := &shared.CardStorageCondition{
		ConditionBase:  shared.ConditionBase{ResourceType: shared.ResourceMicrobe, Amount: 1, Target: "self-card"},
		VariableAmount: true,
	}

	applier := gamecards.NewBehaviorApplier(p, testGame, "test", slog.Default()).
		WithSelectedAmount(2).
		WithSourceCardID(cardID)
	err := applier.ApplyInputs(context.Background(), []shared.BehaviorCondition{input})
	testutil.AssertNoError(t, err, "ApplyInputs should succeed")

	testutil.AssertEqual(t, 3, p.Resources().GetCardStorage(cardID), "Should have 3 microbes remaining after spending 2")
}

func TestCardStorage_StealFromCardWithZeroResources(t *testing.T) {
	testGame, _, registry, playerID, otherPlayerID := testutil.SetupTwoPlayerGame(t)

	p, _ := testGame.GetPlayer(playerID)
	other, _ := testGame.GetPlayer(otherPlayerID)

	// Player 2 has a card with animal storage but 0 animals
	otherCardID := testutil.CardID("Birds")
	other.PlayedCards().AddCard(otherCardID, "Empty Animals", "active", []string{"animal"})
	other.Resources().AddToStorage(otherCardID, 0)

	// Player 1 has a card with animal storage and some animals
	selfCardID := testutil.CardID("Predators")
	p.PlayedCards().AddCard(selfCardID, "Predators", "active", []string{"animal"})
	p.Resources().AddToStorage(selfCardID, 3)

	output := shared.NewCardStorageCondition(shared.ResourceAnimal, 2, "steal-from-any-card")

	applier := gamecards.NewBehaviorApplier(p, testGame, "test", testutil.TestLogger()).WithCardRegistry(registry).WithSourceCardID(selfCardID).WithStealSourceCardID(otherCardID)
	testutil.AssertErrorContains(t, applier.ApplyOutputs(context.Background(), []shared.BehaviorCondition{output}), "insufficient resources on card", "empty source must be rejected")

	// Nothing to steal from empty card
	testutil.AssertEqual(t, 0, other.Resources().GetCardStorage(otherCardID),
		"Other player's card should still have 0 animals")
	testutil.AssertEqual(t, 3, p.Resources().GetCardStorage(selfCardID),
		"Player's card should still have original 3 animals (nothing stolen)")
}

func TestStorageRemoval_InvalidSourcesPreserveActionsAndCosts(t *testing.T) {
	for _, reuse := range []bool{false, true} {
		for _, kind := range []string{"wrong-type", "unowned", "unknown", "empty", "protected"} {
			t.Run(fmt.Sprintf("%v/%s", reuse, kind), func(t *testing.T) {
				ctx := context.Background()
				g, repo, registry, id, otherID := testutil.SetupTwoPlayerGame(t)
				actor, _ := g.GetPlayer(id)
				owner, _ := g.GetPlayer(otherID)
				predator := testutil.GetCardByName("Predators")
				source := testutil.GetCardByName("Birds")
				if kind == "wrong-type" {
					source = testutil.GetCardByName("Decomposers")
				}
				actor.PlayedCards().AddCard(predator.ID, predator.Name, string(predator.Type), nil)
				if kind != "unowned" {
					owner.PlayedCards().AddCard(source.ID, source.Name, string(source.Type), nil)
				}
				if kind != "empty" {
					owner.Resources().AddToStorage(source.ID, 3)
				}
				sourceID := source.ID
				if kind == "unknown" {
					sourceID = "does-not-exist"
				}
				if kind == "protected" {
					defense := testutil.GetCardByName("Protected Habitats")
					owner.Effects().AddEffect(shared.CardEffect{CardID: defense.ID, Behavior: defense.Behaviors[0]})
				}
				behavior := shared.CardBehavior{
					Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
					Inputs:   []shared.BehaviorCondition{shared.NewBasicResourceCondition(shared.ResourceCredit, 2, "self-player")},
					Outputs:  []shared.BehaviorCondition{shared.NewBasicResourceCondition(shared.ResourceHeat, 2, "self-player"), shared.NewCardStorageCondition(shared.ResourceAnimal, 1, "steal-from-any-card")},
				}
				actions := []shared.CardAction{{CardID: predator.ID, CardName: predator.Name, Behavior: behavior}}
				var reuseID *string
				if reuse {
					actions[0].TimesUsedThisGeneration = 1
					id := "reuse-card"
					reuseID = &id
					actions = append(actions, shared.CardAction{CardID: id, Behavior: shared.CardBehavior{Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}}, Outputs: []shared.BehaviorCondition{shared.NewEffectCondition(shared.ResourceActionReuse, 1, "self-player")}}})
				}
				actor.Actions().SetActions(actions)
				actor.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 10})
				before := actor.Resources().Get()
				err := cardAction.NewUseCardActionAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, predator.ID, 0, nil, nil, nil, &sourceID, nil, nil, reuseID, nil)
				expected := map[string]string{"wrong-type": "does not store", "unowned": "not owned", "unknown": "invalid storage", "empty": "insufficient", "protected": "protected"}
				testutil.AssertErrorContains(t, err, expected[kind], "invalid source rejected")
				testutil.AssertEqual(t, before, actor.Resources().Get(), "inputs and other outputs unchanged")
				for i, action := range actor.Actions().List() {
					testutil.AssertEqual(t, actions[i].TimesUsedThisGeneration, action.TimesUsedThisGeneration, "generation use unchanged")
					testutil.AssertEqual(t, actions[i].TimesUsedThisTurn, action.TimesUsedThisTurn, "turn use unchanged")
				}
				testutil.AssertEqual(t, 2, g.CurrentTurn().ActionsRemaining(), "turn actions unchanged")
			})
		}
	}
}

func TestPredators_SelfSourceIsAvailableAndExecutes(t *testing.T) {
	ctx := context.Background()
	g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	card := testutil.GetCardByName("Predators")
	p.PlayedCards().AddCard(card.ID, card.Name, string(card.Type), nil)
	p.Resources().AddToStorage(card.ID, 1)
	for i, b := range card.Behaviors {
		if gamecards.HasManualTrigger(b) {
			p.Actions().SetActions([]shared.CardAction{{CardID: card.ID, CardName: card.Name, BehaviorIndex: i, Behavior: b}})
			state := baseaction.CalculatePlayerCardActionState(card.ID, b, 0, p, g, registry)
			testutil.AssertTrue(t, state.Available(), "self animal is a legal source")
			testutil.AssertNoError(t, cardAction.NewUseCardActionAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, card.ID, i, nil, nil, nil, &card.ID, nil, nil, nil, nil), "self consumption")
			testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(card.ID), "self transfer nets zero")
			testutil.AssertEqual(t, 1, p.Actions().List()[0].TimesUsedThisGeneration, "action consumed")
		}
	}
}

func TestCardStorage_SelectedInputValidationAndAtomicCosts(t *testing.T) {
	for _, tc := range []struct {
		name      string
		sources   []string
		selectors []shared.Selector
		amount    int
		twice     bool
		reserved  int
		wantErr   string
	}{
		{name: "owned source", sources: []string{"C45"}, amount: 1},
		{name: "selector match", sources: []string{"C45"}, selectors: []shared.Selector{{Tags: []shared.CardTag{shared.TagJovian}}}, amount: 1},
		{name: "selector mismatch", sources: []string{"C45"}, selectors: []shared.Selector{{Tags: []shared.CardTag{shared.TagVenus}}}, amount: 1, wantErr: "select an eligible owned card"},
		{name: "missing source", amount: 1, wantErr: "select an eligible owned card"},
		{name: "unexpected source", sources: []string{"C45", "C45"}, amount: 1, wantErr: "unexpected storage input sources"},
		{name: "combined costs", sources: []string{"C45", "C45"}, amount: 2, twice: true, wantErr: "insufficient resources on card"},
		{name: "payment reservation", sources: []string{"C45"}, amount: 2, reserved: 2, wantErr: "insufficient resources on card"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, _, registry, id, _ := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			p.PlayedCards().AddCard("C45", "Titan Shuttles", "active", []string{"jovian", "space"})
			p.Resources().AddToStorage("C45", 3)
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 10})
			input := &shared.CardStorageCondition{ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceFloater, Amount: tc.amount, Target: "any-card"}, Selectors: tc.selectors}
			inputs := []shared.BehaviorCondition{shared.NewBasicResourceCondition(shared.ResourceCredit, 1, "self-player"), input}
			if tc.twice {
				inputs = append(inputs, input)
			}
			applier := gamecards.NewBehaviorApplier(p, g, "test", testutil.TestLogger()).WithCardRegistry(registry).WithInputCardIDs(tc.sources)
			applier.WithReservedInputs(nil, map[string]int{"C45": tc.reserved})
			before := p.Resources().Get().Credits
			err := applier.ApplyInputs(context.Background(), inputs)
			if tc.wantErr != "" {
				testutil.AssertErrorContains(t, err, tc.wantErr, "Invalid inputs rejected")
				testutil.AssertEqual(t, 3, p.Resources().GetCardStorage("C45"), "No partial storage payment")
				testutil.AssertEqual(t, before, p.Resources().Get().Credits, "No partial credit payment")
			} else {
				testutil.AssertNoError(t, err, "Valid inputs paid")
				testutil.AssertEqual(t, 3-tc.amount, p.Resources().GetCardStorage("C45"), "Selected storage spent")
				testutil.AssertEqual(t, before-1, p.Resources().Get().Credits, "Credit cost paid")
			}
		})
	}
}

func TestCardStorage_InputOptionsExcludeOpponentAndRespectSelectors(t *testing.T) {
	g, _, registry, id, otherID := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	other, _ := g.GetPlayer(otherID)
	p.PlayedCards().AddCard("C45", "Titan Shuttles", "active", []string{"jovian", "space"})
	p.PlayedCards().AddCard("222", "Dirigibles", "active", []string{"venus"})
	p.Resources().AddToStorage("C45", 4)
	p.Resources().AddToStorage("222", 10)
	other.SetCorporationID(testutil.CardID("Stormcraft Incorporated"))
	other.Resources().AddToStorage(other.CorporationID(), 20)
	input := &shared.CardStorageCondition{ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceFloater, Amount: 1, Target: "any-card"}, Selectors: []shared.Selector{{Tags: []shared.CardTag{shared.TagJovian}}}}
	options := gamecards.NewBehaviorApplier(p, g, "test", testutil.TestLogger()).WithCardRegistry(registry).InputOptions([]shared.BehaviorCondition{input})
	if len(options.StorageSources) != 1 || len(options.StorageSources[0]) != 1 || options.StorageSources[0][0] != "C45" {
		t.Fatalf("Unexpected source choices: %+v", options.StorageSources)
	}
	input.Target = "self-card"
	input.VariableAmount = true
	input.Amount = 2
	options = gamecards.NewBehaviorApplier(p, g, "test", testutil.TestLogger()).WithSourceCardID("C45").InputOptions([]shared.BehaviorCondition{input})
	testutil.AssertEqual(t, 2, options.VariableAmount.Max, "Amount bound accounts for input multiplier")
	p.Resources().AddToStorage("C45", -4)
	options = gamecards.NewBehaviorApplier(p, g, "test", testutil.TestLogger()).WithSourceCardID("C45").InputOptions([]shared.BehaviorCondition{input})
	testutil.AssertEqual(t, 0, options.VariableAmount.Max, "Empty storage still advertises zero amount")
}

func TestCardStorage_CardPaymentAndInputCannotSpendSameFloaters(t *testing.T) {
	g, repo, _, id, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	host := gamecards.Card{ID: "host", Name: "Host", Type: gamecards.CardTypeActive, ResourceStorage: &gamecards.ResourceStorage{Type: shared.ResourceFloater}}
	project := gamecards.Card{ID: "project", Name: "Project", Type: gamecards.CardTypeEvent, Cost: 3, Behaviors: []shared.CardBehavior{{Triggers: []shared.Trigger{{Type: shared.TriggerTypeAuto}}, Inputs: []shared.BehaviorCondition{shared.NewCardStorageCondition(shared.ResourceFloater, 1, "any-card")}, Outputs: []shared.BehaviorCondition{shared.NewBasicResourceCondition(shared.ResourceTitanium, 1, "self-player")}}}}
	registry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{host, project})
	p.PlayedCards().AddCard(host.ID, host.Name, "active", nil)
	p.Resources().AddToStorage(host.ID, 1)
	p.Hand().AddCard(project.ID)
	p.Resources().AddPaymentSubstitute(shared.PaymentSubstitute{Source: shared.PaymentSource{Target: "self-card", CardID: host.ID, Resource: shared.ResourceFloater}, ConversionRate: 3, TargetResource: shared.ResourceCredit})
	before := p.Resources().Get().Titanium
	play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
	err := play.Execute(context.Background(), g.ID(), id, project.ID, shared.Payment{Allocations: []shared.PaymentAllocation{{Source: shared.PaymentSource{Target: "self-card",
		Resource: "floater", CardID: host.ID}, TargetResource: shared.
		ResourceCredit,

		Amount: 1}},
	}, nil, nil, nil, nil, []string{host.ID})
	testutil.AssertErrorContains(t, err, "insufficient resources on card", "Cannot spend the same floater twice")
	testutil.AssertTrue(t, p.Hand().HasCard(project.ID), "Card remains in hand")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(host.ID), "Floater not deducted")
	testutil.AssertEqual(t, before, p.Resources().Get().Titanium, "No reward on failure")
}

func TestCardStorage_PreflightIncludesEnteringCard(t *testing.T) {
	for _, targetProvided := range []bool{false, true} {
		t.Run(fmt.Sprint(targetProvided), func(t *testing.T) {
			ctx := context.Background()
			g, repo, _, id, _ := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			p.SetCorporationID("")
			p.Resources().Set(shared.Resources{Credits: 10})
			tag := shared.TagJovian
			c := gamecards.Card{ID: "entering-storage", Name: "Entering storage", Type: gamecards.CardTypeActive, Cost: 1, Tags: []shared.CardTag{tag}, ResourceStorage: &gamecards.ResourceStorage{Type: shared.ResourceFloater}, Behaviors: []shared.CardBehavior{{Triggers: []shared.Trigger{{Type: shared.TriggerTypeAuto}}, Outputs: []shared.BehaviorCondition{&shared.CardStorageCondition{ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceFloater, Amount: 1, Target: "any-card"}, Per: &shared.PerCondition{ResourceType: "tag", Tag: &tag, Amount: 1}}}}}}
			registry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{c})
			p.Hand().AddCard(c.ID)
			var targets []string
			if targetProvided {
				targets = []string{c.ID}
			}
			err := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, c.ID, shared.NativePayment(shared.ResourceCredit, 1), nil, targets, nil, nil, nil)
			if targetProvided {
				testutil.AssertNoError(t, err, "entering card is eligible")
				testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(c.ID), "includes source Jovian tag")
			} else {
				testutil.AssertErrorContains(t, err, "select a card for resource storage", "must choose entering card before paying")
				testutil.AssertEqual(t, 10, p.Resources().Get().Credits, "no partial payment")
				testutil.AssertTrue(t, p.Hand().HasCard(c.ID), "card stays in hand")
			}
		})
	}
}

func TestCardStorage_ZeroOutputPreservesTargetPositions(t *testing.T) {
	ctx := context.Background()
	g, _, registry, id, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	host := testutil.GetCardByName("Dirigibles")
	p.PlayedCards().AddCard(host.ID, host.Name, string(host.Type), []string{"venus"})
	outputs := []shared.BehaviorCondition{shared.NewCardStorageCondition(shared.ResourceMicrobe, 0, "any-card"), shared.NewCardStorageCondition(shared.ResourceFloater, 2, "any-card")}
	applier := gamecards.NewBehaviorApplier(p, g, "positional test", testutil.TestLogger()).WithCardRegistry(registry).WithTargetCardIDs([]string{"", host.ID})
	testutil.AssertNoError(t, applier.ValidateResourceOutputs(outputs), "preflight does not consume positions")
	_, err := applier.ApplyOutputsAndGetCalculated(ctx, outputs)
	testutil.AssertNoError(t, err, "skip zero but retain next target")
	testutil.AssertEqual(t, 2, p.Resources().GetCardStorage(host.ID), "second output's target")
}

func TestCardStorage_MissingAnimalStoragePreservesMicrobeTarget(t *testing.T) {
	g, _, registry, id, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	ants := testutil.GetCardByName("Ants")
	p.PlayedCards().AddCard(ants.ID, ants.Name, string(ants.Type), []string{"microbe"})
	ecology := testutil.GetCardByName("Ecology Research")
	outputs := ecology.Behaviors[0].Outputs
	applier := gamecards.NewBehaviorApplier(p, g, ecology.Name, testutil.TestLogger()).
		WithSourceCardID(ecology.ID).WithCardRegistry(registry).WithTargetCardIDs([]string{"", ants.ID})
	testutil.AssertNoError(t, applier.ValidateResourceOutputs(outputs), "only the unavailable animal gain is skipped")
	testutil.AssertNoError(t, applier.ApplyOutputs(context.Background(), outputs), "apply the remaining Ecology Research gains")
	testutil.AssertEqual(t, 2, p.Resources().GetCardStorage(ants.ID), "Ants receives exactly two microbes, not the animal")
	testutil.AssertEqual(t, 0, p.Resources().GetCardStorage(ecology.ID), "Ecology Research does not store either resource")
}

func TestCardStorage_InvalidManualTargetDoesNotPay(t *testing.T) {
	ctx := context.Background()
	g, repo, registry, id, otherID := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	other, _ := g.GetPlayer(otherID)
	host := testutil.GetCardByName("Dirigibles")
	other.PlayedCards().AddCard(host.ID, host.Name, string(host.Type), []string{"venus"})
	p.Resources().Set(shared.Resources{Energy: 6})
	behavior := shared.CardBehavior{Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}}, Inputs: []shared.BehaviorCondition{shared.NewBasicResourceCondition(shared.ResourceEnergy, 6, "self-player")}, Outputs: []shared.BehaviorCondition{shared.NewCardStorageCondition(shared.ResourceFloater, 1, "any-card")}}
	p.Actions().SetActions([]shared.CardAction{{CardID: "manual-storage", CardName: "Manual storage", BehaviorIndex: 0, Behavior: behavior}})
	beforeActions := g.CurrentTurn().ActionsRemaining()
	err := cardAction.NewUseCardActionAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, "manual-storage", 0, nil, []string{host.ID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "target card is not yours or is not in play", "opponent destination rejected before input")
	testutil.AssertEqual(t, 6, p.Resources().Get().Energy, "input retained")
	testutil.AssertEqual(t, beforeActions, g.CurrentTurn().ActionsRemaining(), "turn action retained")
	testutil.AssertEqual(t, 0, p.Resources().GetCardStorage(host.ID), "no ghost own storage")
}
