package card_effects_test

import (
	"context"
	"testing"

	"openmars/internal/action"
	"openmars/internal/game"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/colony"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func enableColonies(t *testing.T, testGame *game.Game) {
	t.Helper()
	settings := testGame.Settings()
	settings.Systems = append(settings.Systems, "colonies")
	testGame.UpdateSettings(context.Background(), settings)

	testGame.Colonies().SetStates([]*colony.ColonyState{
		{DefinitionID: "luna", MarkerPosition: 1, PlayerColonies: nil},
		{DefinitionID: "europa", MarkerPosition: 1, PlayerColonies: nil},
	})
}

func createColonyRequirementCard(id string, cost int, req gamecards.Requirement) gamecards.Card {
	return gamecards.Card{
		ID:   id,
		Name: "Colony Req Test Card " + id,
		Type: gamecards.CardTypeAutomated,
		Cost: cost,
		Requirements: &gamecards.CardRequirements{
			Items: []gamecards.Requirement{req},
		},
	}
}

func hasErrorCode(state player.EntityState, code player.StateErrorCode) bool {
	for _, err := range state.Errors {
		if err.Code == code {
			return true
		}
	}
	return false
}

func TestColonyRequirement_MaxColonies_Rejected(t *testing.T) {
	testGame, _, _, playerID, otherPlayerID := testutil.SetupTwoPlayerGame(t)
	enableColonies(t, testGame)

	p, _ := testGame.GetPlayer(playerID)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})

	// Player has 2 colonies — exceeds max 1
	testGame.Colonies().SetStates([]*colony.ColonyState{
		{DefinitionID: "luna", MarkerPosition: 1, PlayerColonies: []string{playerID}},
		{DefinitionID: "europa", MarkerPosition: 1, PlayerColonies: []string{playerID, otherPlayerID}},
	})

	card := createColonyRequirementCard("test-max-colony", 10, gamecards.Requirement{
		Type: gamecards.RequirementColony,
		Max:  testutil.IntPtr(1),
	})
	registry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{card})
	cardPtr, _ := registry.GetByID("test-max-colony")

	state := action.CalculatePlayerCardState(cardPtr, p, testGame, registry)

	if state.Available() {
		t.Error("Expected card to be unavailable — player has 2 colonies, max is 1")
	}
	if !hasErrorCode(state, player.ErrorCodeColoniesTooMany) {
		t.Errorf("Expected colonies-too-many error, got: %+v", state.Errors)
	}
}

func TestColonyRequirement_MaxColonies_AcceptedAtLimit(t *testing.T) {
	testGame, _, _, playerID, _ := testutil.SetupTwoPlayerGame(t)
	enableColonies(t, testGame)

	p, _ := testGame.GetPlayer(playerID)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})

	// Player has exactly 1 colony — meets max 1
	testGame.Colonies().SetStates([]*colony.ColonyState{
		{DefinitionID: "luna", MarkerPosition: 1, PlayerColonies: []string{playerID}},
		{DefinitionID: "europa", MarkerPosition: 1, PlayerColonies: nil},
	})

	card := createColonyRequirementCard("test-max-colony", 10, gamecards.Requirement{
		Type: gamecards.RequirementColony,
		Max:  testutil.IntPtr(1),
	})
	registry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{card})
	cardPtr, _ := registry.GetByID("test-max-colony")

	state := action.CalculatePlayerCardState(cardPtr, p, testGame, registry)

	if hasErrorCode(state, player.ErrorCodeColoniesTooMany) {
		t.Errorf("Did not expect colonies-too-many error — player has exactly 1 colony")
	}
}

func TestColonyRequirement_MaxColonies_AcceptedWithZero(t *testing.T) {
	testGame, _, _, playerID, _ := testutil.SetupTwoPlayerGame(t)
	enableColonies(t, testGame)

	p, _ := testGame.GetPlayer(playerID)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})

	// Player has 0 colonies
	card := createColonyRequirementCard("test-max-colony", 10, gamecards.Requirement{
		Type: gamecards.RequirementColony,
		Max:  testutil.IntPtr(1),
	})
	registry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{card})
	cardPtr, _ := registry.GetByID("test-max-colony")

	state := action.CalculatePlayerCardState(cardPtr, p, testGame, registry)

	if hasErrorCode(state, player.ErrorCodeColoniesTooMany) {
		t.Errorf("Did not expect colonies-too-many error — player has 0 colonies")
	}
}

func TestColonyRequirement_MinColonies_Rejected(t *testing.T) {
	testGame, _, _, playerID, _ := testutil.SetupTwoPlayerGame(t)
	enableColonies(t, testGame)

	p, _ := testGame.GetPlayer(playerID)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})

	// Player has 0 colonies — doesn't meet min 1
	card := createColonyRequirementCard("test-min-colony", 10, gamecards.Requirement{
		Type: gamecards.RequirementColony,
		Min:  testutil.IntPtr(1),
	})
	registry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{card})
	cardPtr, _ := registry.GetByID("test-min-colony")

	state := action.CalculatePlayerCardState(cardPtr, p, testGame, registry)

	if state.Available() {
		t.Error("Expected card to be unavailable — player has 0 colonies, min is 1")
	}
	if !hasErrorCode(state, player.ErrorCodeColoniesTooFew) {
		t.Errorf("Expected colonies-too-few error, got: %+v", state.Errors)
	}
}

func TestColonyRequirement_MinColonies_Accepted(t *testing.T) {
	testGame, _, _, playerID, _ := testutil.SetupTwoPlayerGame(t)
	enableColonies(t, testGame)

	p, _ := testGame.GetPlayer(playerID)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})

	// Player has 1 colony — meets min 1
	testGame.Colonies().SetStates([]*colony.ColonyState{
		{DefinitionID: "luna", MarkerPosition: 1, PlayerColonies: []string{playerID}},
		{DefinitionID: "europa", MarkerPosition: 1, PlayerColonies: nil},
	})

	card := createColonyRequirementCard("test-min-colony", 10, gamecards.Requirement{
		Type: gamecards.RequirementColony,
		Min:  testutil.IntPtr(1),
	})
	registry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{card})
	cardPtr, _ := registry.GetByID("test-min-colony")

	state := action.CalculatePlayerCardState(cardPtr, p, testGame, registry)

	if hasErrorCode(state, player.ErrorCodeColoniesTooFew) {
		t.Errorf("Did not expect colonies-too-few error — player has 1 colony")
	}
}
