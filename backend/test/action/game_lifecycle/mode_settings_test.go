package game_lifecycle_test

import (
	"context"
	"slices"
	"testing"

	"openmars/internal/action"
	gameAction "openmars/internal/action/game"
	"openmars/internal/game/cards"
	"openmars/internal/game/mode"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func TestModeSwitchRebuildsDeckAndUsesFreshDefinitions(t *testing.T) {
	cost := 3
	registry := cards.NewInMemoryCardRegistry([]cards.Card{
		{ID: "common", Type: cards.CardTypeAutomated, Cost: 20, GameModes: []string{"classic", "scenario"}, Modifiers: []cards.ModeModifier{{When: cards.ModifierWhen{GameModes: []string{"scenario"}}, Changes: []cards.CardChange{{Type: "set-cost", Amount: &cost}}}}},
		{ID: "classic", Type: cards.CardTypeAutomated, GameModes: []string{"classic"}},
		{ID: "scenario", Type: cards.CardTypeAutomated, GameModes: []string{"scenario"}},
	}, mode.Definition{ID: "classic", MinPlayers: 1, MaxPlayers: 8}, mode.Definition{ID: "scenario", MinPlayers: 1, MaxPlayers: 5, Parameters: []mode.Parameter{{ID: "deadline", Default: 6, Values: []int{4, 6, 8}}}, Runtime: shared.ModeRuntimeDefinition{State: map[string]int{"contact": 1, "withdrawn": 0}, Deadlines: []shared.StateDeadline{{State: "cutoff", Parameter: "deadline", ActiveState: "contact", OccurredState: "withdrawn", ActiveValue: 1, InactiveValue: 0}}}})
	ctx := context.Background()
	repo := testutil.NewTestGameRepository(t)
	maps := testutil.CreateTestMapRegistry()
	log := testutil.TestLogger()
	create := gameAction.NewCreateGameAction(repo, registry, maps, log)
	g, err := create.Execute(ctx, shared.GameSettings{DevelopmentMode: true, DemoGame: true})
	if err != nil {
		t.Fatal(err)
	}
	host, err := g.AddNewPlayer(ctx, "host", "Host")
	if err != nil {
		t.Fatal(err)
	}
	if err := g.SetHostPlayerID(ctx, host.ID()); err != nil {
		t.Fatal(err)
	}
	action.SetupPlayerCardStore(host, g, registry)
	host.SetPendingDemoChoices(&shared.PendingDemoChoices{CorporationID: "old"})
	change := gameAction.NewUpdateGameSettingsAction(repo, registry, maps, log)
	if err := change.Execute(ctx, g.ID(), host.ID(), &gameAction.SettingsPatch{GameModeID: strPtr("scenario")}); err != nil {
		t.Fatal(err)
	}
	if host.HasPendingDemoChoices() {
		t.Fatal("mode switch retained demo choices")
	}
	if ids := g.Deck().ProjectCards(); len(ids) != 2 || slices.Contains(ids, "classic") || !slices.Contains(ids, "scenario") {
		t.Fatalf("deck not rebuilt: %v", ids)
	}
	if deadline, ok := g.ModeStateValue("cutoff"); !ok || deadline != 6 {
		t.Fatal("runtime parameters not initialized")
	}
	host.Hand().AddCard("common")
	state, ok := host.CardStateStore().GetState("common")
	if !ok || state.Cost["credit"] != 3 {
		t.Fatalf("card store retained old mode cost: %+v", state)
	}
	quote, err := action.QuoteActionPayment(g, host, registry, nil, nil, nil, action.PaymentIntent{Action: "play-card", CardID: "common"})
	if err != nil {
		t.Fatal(err)
	}
	if quote.Costs[shared.ResourceCredit] != 3 {
		t.Fatalf("payment quote ignored mode: %+v", quote)
	}
	before := g.Settings()
	before.GameModeParameters["deadline"] = 99
	if g.Settings().GameModeParameters["deadline"] != 6 {
		t.Fatal("settings snapshot leaked mutation")
	}
	host.SetPendingDemoChoices(&shared.PendingDemoChoices{CorporationID: "still-valid"})
	if err := change.Execute(ctx, g.ID(), host.ID(), &gameAction.SettingsPatch{DemoGame: boolPtr(false), GameModeID: strPtr("missing")}); err == nil {
		t.Fatal("unknown mode accepted")
	}
	if !host.HasPendingDemoChoices() || !g.Settings().DemoGame || g.Settings().GameModeID != "scenario" {
		t.Fatal("invalid patch partially mutated lobby")
	}
	if err := change.Execute(ctx, g.ID(), host.ID(), &gameAction.SettingsPatch{GameModeID: strPtr("classic")}); err != nil {
		t.Fatal(err)
	}
	if len(g.Settings().GameModeParameters) != 0 || len(g.ModeRuntime().Values) != 0 {
		t.Fatal("Classic retained scenario state")
	}
}
