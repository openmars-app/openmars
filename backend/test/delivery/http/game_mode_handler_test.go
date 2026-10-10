package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"openmars/internal/action/query"
	"openmars/internal/delivery/dto"
	delivery "openmars/internal/delivery/http"
	"openmars/internal/game/cards"
	"openmars/internal/game/mode"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func TestGameModeCatalogHTTPAndStatePreviews(t *testing.T) {
	price := 4
	definition := mode.Definition{ID: "scenario", Name: "Scenario", MinPlayers: 1, MaxPlayers: 5, Runtime: shared.ModeRuntimeDefinition{State: map[string]int{"contact": 1, "withdrawn": 0}, Deadlines: []shared.StateDeadline{{State: "deadline", ActiveState: "contact", OccurredState: "withdrawn", ActiveValue: 1, InactiveValue: 0}}}}
	card := cards.Card{ID: "external", Name: "External", Type: cards.CardTypeAutomated, Cost: 9, GameModes: []string{"scenario"}, DependencyGroups: []shared.DependencyGroup{{ID: "supply", State: "contact", Equals: 1}}, Behaviors: []shared.CardBehavior{{ID: "income", Dependency: "supply"}}, Modifiers: []cards.ModeModifier{{When: cards.ModifierWhen{GameModes: []string{"scenario"}}, Changes: []cards.CardChange{{Type: "set-cost", Amount: &price}}}}}
	registry := cards.NewInMemoryCardRegistry([]cards.Card{card, {ID: "unassigned", GameModes: []string{}}}, definition)
	handler := delivery.NewGameHandler(nil, nil, nil, nil, nil, query.NewListCardsAction(registry, testutil.TestLogger()), registry, nil, nil)
	response := httptest.NewRecorder()
	handler.GameModes(response, httptest.NewRequest(http.MethodGet, "/api/v1/game-modes", nil))
	if response.Code != http.StatusOK {
		t.Fatal(response.Body.String())
	}
	var modes dto.GameModesResponse
	if err := json.Unmarshal(response.Body.Bytes(), &modes); err != nil {
		t.Fatal(err)
	}
	if len(modes.GameModes) != 1 || modes.GameModes[0].ID != "scenario" {
		t.Fatal("mode catalog response missing")
	}
	for _, preview := range []string{"connected", "disconnected", "restored"} {
		t.Run(preview, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ListCards(w, httptest.NewRequest(http.MethodGet, "/api/v1/cards?gameModeId=scenario&preview="+preview, nil))
			if w.Code != http.StatusOK {
				t.Fatal(w.Body.String())
			}
			var result dto.ListCardsResponse
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.TotalCount != 1 || len(result.Cards) != 1 || result.Cards[0].Cost != 4 {
				t.Fatal("browser failed to resolve catalog")
			}
			status := result.Cards[0].Behaviors[0].DependencyStatus
			if status == nil || status.Active != (preview != "disconnected") {
				t.Fatalf("wrong preview: %+v", status)
			}
		})
	}
	for _, url := range []string{"/api/v1/cards?gameModeId=unknown", "/api/v1/cards?preview=connected", "/api/v1/cards?gameModeId=scenario&preview=unknown", "/api/v1/cards?offset=-1"} {
		w := httptest.NewRecorder()
		handler.ListCards(w, httptest.NewRequest(http.MethodGet, url, nil))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected rejection for %s, got %d", url, w.Code)
		}
	}
	original, _ := registry.GetByID("external")
	if original.Cost != 9 {
		t.Fatal("preview mutated library")
	}
}
