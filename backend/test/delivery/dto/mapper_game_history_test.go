package dto_test

import (
	"testing"
	"time"

	"openmars/internal/delivery/dto"
	"openmars/internal/game/cards"
	"openmars/internal/game/datastore"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func TestToGameHistoryEntryDtos_TotalVP_FromBreakdown(t *testing.T) {
	playerID := "player-1"

	entry := &datastore.GameStateHistoryEntry{
		GameID:    "game-1",
		Sequence:  1,
		Timestamp: time.Now(),
		State: &datastore.GameState{
			Players: map[string]*datastore.PlayerState{
				playerID: {
					ID:              playerID,
					Name:            "Test Player",
					Color:           "red",
					TerraformRating: 25,
					Resources:       shared.Resources{},
					ResourceStorage: map[string]int{},
				},
			},
			ClaimedMilestones: []shared.ClaimedMilestone{},
			FundedAwards:      []shared.FundedAward{},
			Settings:          shared.GameSettings{},
		},
		VPBreakdowns: map[string]shared.VPBreakdown{
			playerID: {
				TerraformRating: 25,
				CardVP:          7,
				MilestoneVP:     5,
				AwardVP:         5,
				GreeneryVP:      2,
				CityVP:          2,
				TotalVP:         46,
			},
		},
	}

	dtos := dto.ToGameHistoryEntryDtos([]*datastore.GameStateHistoryEntry{entry}, nil)
	testutil.AssertEqual(t, 1, len(dtos), "Should have one entry")
	testutil.AssertEqual(t, 46, dtos[0].Players[playerID].TotalVP, "TotalVP should come from VPBreakdowns")
}

func TestToGameHistoryEntryDtos_TotalVP_NilBreakdownDegradesGracefully(t *testing.T) {
	playerID := "player-1"

	entry := &datastore.GameStateHistoryEntry{
		GameID:    "game-1",
		Sequence:  1,
		Timestamp: time.Now(),
		State: &datastore.GameState{
			Players: map[string]*datastore.PlayerState{
				playerID: {
					ID:              playerID,
					Name:            "Test Player",
					Color:           "blue",
					TerraformRating: 20,
					Resources:       shared.Resources{},
					ResourceStorage: map[string]int{},
				},
			},
			ClaimedMilestones: []shared.ClaimedMilestone{},
			FundedAwards:      []shared.FundedAward{},
			Settings:          shared.GameSettings{},
		},
		// VPBreakdowns deliberately nil — represents legacy entries or tests
		// where the snapshot enricher wasn't wired. Mapper should not panic
		// and should report 0.
	}

	dtos := dto.ToGameHistoryEntryDtos([]*datastore.GameStateHistoryEntry{entry}, nil)
	testutil.AssertEqual(t, 0, dtos[0].Players[playerID].TotalVP, "TotalVP should be 0 when VPBreakdowns is nil")
}

func TestHistoryUsesCapturedDependencyStateAndProduction(t *testing.T) {
	group := shared.DependencyGroup{ID: "operation", State: "contact", Equals: 1, Importable: true}
	registry := cards.NewInMemoryCardRegistry([]cards.Card{{ID: "owned", DependencyGroups: []shared.DependencyGroup{group}}, {ID: "hand", DependencyGroups: []shared.DependencyGroup{group}}})
	for _, test := range []struct {
		name       string
		contact    int
		imported   bool
		active     bool
		production int
	}{{"connected", 1, false, true, 5}, {"disconnected", 0, false, false, 2}, {"imported", 0, true, true, 5}, {"restored", 1, false, true, 5}} {
		t.Run(test.name, func(t *testing.T) {
			permanent := 2
			if test.imported {
				permanent = 5
			}
			snapshot := &datastore.GameStateHistoryEntry{State: &datastore.GameState{ModeRuntime: shared.ModeRuntimeState{Values: map[string]int{"contact": test.contact}}, Players: map[string]*datastore.PlayerState{"p": {ID: "p", Production: shared.Production{Steel: permanent}, HandCardIDs: []string{"hand"}, PlayedCardIDs: []string{"owned"}, DependencyStatuses: map[string]shared.DependencyStatus{"owned:operation": {Removed: test.imported}}, DependencyGroups: map[string]shared.DependencyGroup{"owned:operation": group}, ProductionContributions: []shared.ProductionContribution{{CardID: "owned", Dependency: group, Resource: shared.ResourceSteelProduction, Amount: 3}}}}}}
			result := dto.ToGameHistoryEntryDtos([]*datastore.GameStateHistoryEntry{snapshot}, registry)[0]
			if result.ModeRuntime.EffectiveValues["contact"] != test.contact {
				t.Fatal("history runtime differs")
			}
			player := result.Players["p"]
			if player.Production.Steel != test.production || player.PermanentProduction.Steel != permanent {
				t.Fatalf("incorrect captured production: %+v", player)
			}
			if player.DependencyStatuses["owned:operation"].Active != test.active || player.DependencyStatuses["owned:operation"].Imported != test.imported {
				t.Fatal("owned status differs")
			}
			if player.DependencyStatuses["hand:operation"].Active != (test.contact == 1) {
				t.Fatal("hand status missing or fabricated from owned import")
			}
		})
	}
}

func TestHistoryIncludesSelectedModePresentationMetadata(t *testing.T) {
	registry := testutil.CreateTestModeCatalog()
	entry := &datastore.GameStateHistoryEntry{State: &datastore.GameState{Settings: shared.GameSettings{GameModeID: "earth-withdrawal", MaxPlayers: 5, Features: []string{"prelude"}}, ModeRuntime: shared.ModeRuntimeState{Values: map[string]int{"earth-contact": 0}}}}
	mapped := dto.ToGameHistoryEntryDtos([]*datastore.GameStateHistoryEntry{entry}, registry)[0]
	for _, definition := range mapped.Settings.AvailableGameModes {
		if definition.ID == mapped.Settings.GameModeID {
			for _, status := range definition.Statuses {
				if status.State == "earth-contact" && status.Labels["0"] != "" {
					return
				}
			}
			t.Fatal("historical mode is missing connection presentation metadata")
		}
	}
	t.Fatal("historical settings omitted the selected mode catalog")
}

func TestHistoryDerivesIrreversibleRetirementDuringReconnection(t *testing.T) {
	registry := testutil.CreateTestModeCatalog()
	entry := &datastore.GameStateHistoryEntry{State: &datastore.GameState{
		Settings:    shared.GameSettings{GameModeID: "earth-withdrawal", MaxPlayers: 5},
		ModeRuntime: shared.ModeRuntimeState{Values: map[string]int{"earth-contact": 0, "withdrawal-occurred": 1}, Windows: []shared.StateWindow{{State: "earth-contact", Value: 1, Status: "active"}}},
		Players:     map[string]*datastore.PlayerState{"p": {ID: "p", PlayedCardIDs: []string{"ES016"}}},
	}}
	mapped := dto.ToGameHistoryEntryDtos([]*datastore.GameStateHistoryEntry{entry}, registry)[0]
	status := mapped.Players["p"].DependencyStatuses["ES016:earth-support"]
	if !status.Retired || status.Active || status.Imported {
		t.Fatalf("historical expired lease shown as reusable: %+v", status)
	}
	if mapped.ModeRuntime.EffectiveValues["earth-contact"] != 1 {
		t.Fatal("test must represent restored contact")
	}
}
