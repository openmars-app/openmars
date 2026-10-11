package cards_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"openmars/internal/action/query"
	"openmars/internal/game"
	"openmars/internal/game/cards"
	"openmars/internal/game/mode"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func modeCatalogFixture() *cards.InMemoryCardRegistry {
	cost := 7
	requirement := 2
	definitions := []mode.Definition{
		{ID: "classic", Name: "Classic", MinPlayers: 1, MaxPlayers: 8, Features: []mode.Feature{{ID: "prelude", Default: true}}},
		{ID: "scenario", Name: "Scenario", MinPlayers: 1, MaxPlayers: 5, Features: []mode.Feature{{ID: "prelude", Default: true}}, Parameters: []mode.Parameter{{ID: "deadline", Default: 6, Values: []int{4, 6, 8}}}, Runtime: shared.ModeRuntimeDefinition{State: map[string]int{"contact": 1}}},
	}
	list := []cards.Card{
		{ID: "shared", Name: "Shared", Type: cards.CardTypeAutomated, Cost: 12, GameModes: []string{"classic", "scenario"}, Requirements: &cards.CardRequirements{Items: []cards.Requirement{{Type: cards.RequirementCities, Min: &requirement}}}, Behaviors: []shared.CardBehavior{{ID: "income", Description: "base"}}, Modifiers: []cards.ModeModifier{{When: cards.ModifierWhen{GameModes: []string{"scenario"}}, Changes: []cards.CardChange{{Type: "set-cost", Amount: &cost}, {Type: "replace-behavior", BehaviorID: "income", Behavior: &shared.CardBehavior{Description: "scenario"}}}}}},
		{ID: "classic-only", Type: cards.CardTypeAutomated, GameModes: []string{"classic"}},
		{ID: "scenario-only", Type: cards.CardTypeEvent, GameModes: []string{"scenario"}},
		{ID: "prelude", Type: cards.CardTypePrelude, GameModes: []string{"classic", "scenario"}},
		{ID: "prelude-corp", Type: cards.CardTypeCorporation, GameModes: []string{"classic", "scenario"}, RequiresFeatures: []string{"prelude"}},
		{ID: "library-only", Type: cards.CardTypeAutomated, GameModes: []string{}},
	}
	return cards.NewInMemoryCardRegistry(list, definitions...)
}

func TestModeCatalogIsolatesDefinitionsAndSelection(t *testing.T) {
	library := modeCatalogFixture()
	if err := cards.ValidateCatalog(library); err != nil {
		t.Fatal(err)
	}
	classic, err := cards.Resolve(library, shared.GameSettings{GameModeID: "classic", Features: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := cards.Resolve(library, shared.GameSettings{GameModeID: "scenario", Features: []string{"prelude"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(classic.GetAll()) != 2 || len(scenario.GetAll()) != 4 {
		t.Fatal("selection includes unrelated content")
	}
	if _, err := classic.GetByID("scenario-only"); err == nil {
		t.Fatal("cross-mode lookup succeeded")
	}
	if _, err := scenario.GetByID("library-only"); err == nil {
		t.Fatal("unassigned library card entered deck")
	}
	effective, _ := scenario.GetByID("shared")
	if effective.Cost != 7 || effective.Behaviors[0].Description != "scenario" || effective.Behaviors[0].ID != "income" {
		t.Fatal("modifier did not replace exact values")
	}
	effective.Cost = 99
	*effective.Requirements.Items[0].Min = 99
	effective.Behaviors[0].Description = "mutated"
	again, _ := scenario.GetByID("shared")
	base, _ := classic.GetByID("shared")
	original, _ := library.GetByID("shared")
	if again.Cost != 7 || *again.Requirements.Items[0].Min != 2 || again.Behaviors[0].Description != "scenario" || base.Cost != 12 || original.Cost != 12 {
		t.Fatal("catalog copy leaked mutation")
	}
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := cards.Resolve(library, shared.GameSettings{GameModeID: "scenario", Features: []string{"prelude"}})
			if e != nil {
				t.Error(e)
				return
			}
			c, _ := r.GetByID("shared")
			if c.Cost != 7 {
				t.Error("concurrent resolution leaked")
			}
		}()
	}
	wg.Wait()
}

func TestModeCatalogRejectsBrokenReferencesAndConflictingChanges(t *testing.T) {
	for _, kind := range []string{"unknown-mode", "unknown-state", "unknown-dependency", "unknown-behavior", "conflicting-cost", "duplicate-behavior", "unknown-feature"} {
		t.Run(kind, func(t *testing.T) {
			library := modeCatalogFixture()
			all := library.GetAll()
			c := &all[0]
			switch kind {
			case "unknown-mode":
				c.GameModes = []string{"missing"}
			case "unknown-state":
				c.DependencyGroups = []shared.DependencyGroup{{ID: "supply", State: "missing", Equals: 1}}
			case "unknown-dependency":
				c.Behaviors[0].Dependency = "missing"
			case "unknown-behavior":
				c.Modifiers[0].Changes[1].BehaviorID = "missing"
			case "conflicting-cost":
				c.Modifiers[0].Changes = append(c.Modifiers[0].Changes, c.Modifiers[0].Changes[0])
			case "duplicate-behavior":
				c.Behaviors = append(c.Behaviors, c.Behaviors[0])
			case "unknown-feature":
				c.RequiresFeatures = []string{"missing"}
			}
			if err := cards.ValidateCatalog(cards.NewInMemoryCardRegistry(all, cards.ModeDefinitions(library)...)); err == nil {
				t.Fatal("invalid catalog accepted")
			}
		})
	}
}

func TestModeCatalogRejectsDuplicateSourceIDs(t *testing.T) {
	directory := t.TempDir()
	library := modeCatalogFixture()
	list := library.GetAll()
	list = append(list, list[0])
	data, _ := json.Marshal(list)
	if err := os.WriteFile(filepath.Join(directory, "cards.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	data, _ = json.Marshal(cards.ModeDefinitions(library))
	if err := os.WriteFile(filepath.Join(directory, "gamemodes.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := cards.LoadCatalog(filepath.Join(directory, "cards.json")); err == nil {
		t.Fatal("duplicate source ID accepted")
	}
}

func TestModeSelectionDefaultsValidationAndBrowserQuery(t *testing.T) {
	library := modeCatalogFixture()
	defs := cards.ModeDefinitions(library)
	defaults, _, err := mode.Normalize(defs, shared.GameSettings{})
	if err != nil {
		t.Fatal(err)
	}
	if defaults.GameModeID != "classic" || defaults.MaxPlayers != 5 || !defaults.HasPrelude() {
		t.Fatal("wrong defaults")
	}
	scenario, _, err := mode.Normalize(defs, shared.GameSettings{GameModeID: "scenario", Features: []string{}, Systems: []string{"venus"}})
	if err != nil {
		t.Fatal(err)
	}
	if scenario.GameModeParameters["deadline"] != 6 || scenario.HasPrelude() || scenario.HasVenus() {
		t.Fatal("selection failed to derive systems or explicit feature choice")
	}
	for _, bad := range []shared.GameSettings{{GameModeID: "missing"}, {GameModeID: "scenario", MaxPlayers: 6}, {GameModeID: "scenario", GameModeParameters: map[string]int{"deadline": 5}}, {GameModeID: "classic", GameModeParameters: map[string]int{"deadline": 6}}, {Features: []string{"missing"}}, {Features: []string{"prelude", "prelude"}}} {
		if _, _, err := mode.Normalize(defs, bad); err == nil {
			t.Fatalf("invalid selection accepted: %+v", bad)
		}
	}
	action := query.NewListCardsAction(library, testutil.TestLogger())
	result, err := action.Execute(context.Background(), 0, 100, &scenario)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalCount != 2 {
		t.Fatalf("browser did not use selected catalog: %d", result.TotalCount)
	}
	if _, err := action.Execute(context.Background(), -1, 100, nil); err == nil {
		t.Fatal("negative pagination accepted")
	}
}

func TestPublishedModeDecksAndEffectiveCards(t *testing.T) {
	library := testutil.CreateTestModeCatalog()
	for _, test := range []struct {
		id                                           string
		projects, corporations, preludes, maxPlayers int
	}{{"classic", 216, 17, 35, 8}, {"earth-withdrawal", 156, 17, 35, 5}} {
		t.Run(test.id, func(t *testing.T) {
			settings, definition, err := mode.Normalize(cards.ModeDefinitions(library), shared.GameSettings{GameModeID: test.id})
			if err != nil {
				t.Fatal(err)
			}
			effective, err := cards.Resolve(library, settings)
			if err != nil {
				t.Fatal(err)
			}
			projects, corps, preludes := cards.GetCardIDs(effective)
			for _, id := range []string{"EXP004", "097"} {
				if !slices.Contains(projects, id) {
					t.Fatalf("nuclear card %s must be available in the project deck", id)
				}
			}
			if len(projects) != test.projects || len(corps) != test.corporations || len(preludes) != test.preludes {
				t.Fatalf("wrong deck: %d projects, %d corps, %d preludes", len(projects), len(corps), len(preludes))
			}
			if definition.MaxPlayers != test.maxPlayers || len(settings.Systems) != 0 {
				t.Fatal("unexpected mode limits or external systems")
			}
			for _, card := range effective.GetAll() {
				for _, err := range cards.ValidateCardJSON(&card) {
					t.Errorf("effective %s %s: %v", card.ID, card.Name, err)
				}
			}
			settings.Features = []string{}
			withoutPrelude, err := cards.Resolve(library, settings)
			if err != nil {
				t.Fatal(err)
			}
			_, _, disabled := cards.GetCardIDs(withoutPrelude)
			if len(disabled) != 0 {
				t.Fatal("Prelude disabled but prelude deck retained")
			}
			for _, id := range []string{"EXP004", "097"} {
				if _, err := withoutPrelude.GetByID(id); err != nil {
					t.Fatalf("nuclear card %s must not require Preludes", id)
				}
			}
			if _, err := withoutPrelude.GetByID("PC4"); err == nil {
				t.Fatal("Valley Trust requires Prelude")
			}
		})
	}
}

type wrappedModeCatalog struct{ *cards.InMemoryCardRegistry }

func TestForGamePreservesCatalogScopeAcrossLobbyChanges(t *testing.T) {
	library := modeCatalogFixture()
	repo := testutil.NewTestGameRepository(t)
	g := game.NewGame(repo.DataStore(), "mode-scope", "", shared.GameSettings{GameModeID: "classic", Features: []string{}}, nil)
	classic := cards.ForGame(library, g)
	settings := g.Settings()
	settings.GameModeID = "scenario"
	g.UpdateSettings(context.Background(), settings)
	for _, registry := range []cards.CardRegistry{library, wrappedModeCatalog{library}} {
		selected := cards.ForGame(registry, g)
		card, err := selected.GetByID("shared")
		if err != nil || card.Cost != 7 {
			t.Fatalf("current lobby mode not resolved: card=%+v err=%v", card, err)
		}
		if cards.ForGame(selected, g) != selected {
			t.Fatal("already effective catalog was rebuilt")
		}
	}
	if cards.ForGame(classic, g) != classic {
		t.Fatal("an immutable effective catalog changed scope")
	}
	standalone := cards.NewInMemoryCardRegistry(library.GetAll())
	if cards.ForGame(standalone, g) != standalone {
		t.Fatal("standalone fixture unexpectedly filtered")
	}
}

func TestModeStateVisualValidation(t *testing.T) {
	for _, test := range []struct {
		name   string
		state  string
		visual mode.StateVisual
		valid  bool
	}{
		{"declared state", "contact", mode.StateVisual{Icon: "connection", Label: "Contact", Values: map[string]mode.StateValueVisual{"0": {Icon: "connection-closed", Label: "Closed"}}}, true},
		{"undeclared state", "unknown", mode.StateVisual{Icon: "connection", Label: "Contact"}, false},
		{"missing icon", "contact", mode.StateVisual{Label: "Contact"}, false},
		{"missing value label", "contact", mode.StateVisual{Icon: "connection", Label: "Contact", Values: map[string]mode.StateValueVisual{"0": {Icon: "connection-closed"}}}, false},
		{"noninteger state value", "contact", mode.StateVisual{Icon: "connection", Label: "Contact", Values: map[string]mode.StateValueVisual{"closed": {Icon: "connection-closed", Label: "Closed"}}}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			definition := mode.Definition{ID: "test", MinPlayers: 1, MaxPlayers: 2, Runtime: shared.ModeRuntimeDefinition{State: map[string]int{"contact": 1}}, StateVisuals: map[string]mode.StateVisual{test.state: test.visual}}
			data, err := json.Marshal([]mode.Definition{definition})
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "modes.json")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			_, err = mode.LoadFromJSON(path)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v, error=%v", test.valid, err)
			}
		})
	}
}

func TestClassicExpansionOptionsSelectIndependentSystemsAndDecks(t *testing.T) {
	library := testutil.CreateTestModeCatalog()
	definitions := cards.ModeDefinitions(library)
	for selection := 0; selection < 8; selection++ {
		features := []string{}
		if selection&1 != 0 {
			features = append(features, "prelude")
		}
		if selection&2 != 0 {
			features = append(features, "venus")
		}
		if selection&4 != 0 {
			features = append(features, "colonies")
		}
		settings, _, err := mode.Normalize(definitions, shared.GameSettings{GameModeID: "classic", Features: features})
		if err != nil {
			t.Fatal(err)
		}
		if settings.HasVenus() != (selection&2 != 0) || settings.HasColonies() != (selection&4 != 0) || settings.HasPrelude() != (selection&1 != 0) {
			t.Fatalf("wrong systems for %v: %v", features, settings.Systems)
		}
		selected, err := cards.Resolve(library, settings)
		if err != nil {
			t.Fatal(err)
		}
		for _, card := range []struct {
			id      string
			enabled bool
		}{
			{"213", settings.HasVenus()}, {"252", settings.HasVenus()}, {"V01", settings.HasVenus()},
			{"C01", settings.HasColonies()}, {"C14", settings.HasColonies()}, {"CC3", settings.HasColonies()},
			{"P01", settings.HasPrelude()}, {"ES001", false}, {"T01", false},
		} {
			_, err := selected.GetByID(card.id)
			if (err == nil) != card.enabled {
				t.Fatalf("card %s availability for %v: %v", card.id, features, err)
			}
		}
		projects, corporations, _ := cards.GetCardIDs(selected)
		expansions := 0
		if settings.HasVenus() {
			expansions++
		}
		if settings.HasColonies() {
			expansions++
		}
		expectedCorps := 17 + expansions*5
		if !settings.HasPrelude() {
			expectedCorps--
		}
		if len(projects) != 216+expansions*49 || len(corporations) != expectedCorps {
			t.Fatalf("unexpected deck for %v: %d projects, %d corporations", features, len(projects), len(corporations))
		}
	}
	for _, feature := range []string{"venus", "colonies", "project-funding"} {
		if _, _, err := mode.Normalize(definitions, shared.GameSettings{GameModeID: "earth-withdrawal", Features: []string{feature}}); err == nil {
			t.Fatalf("Earth Withdrawal accepted %s", feature)
		}
	}
	earth, definition, err := mode.Normalize(definitions, shared.GameSettings{GameModeID: "earth-withdrawal"})
	if err != nil {
		t.Fatal(err)
	}
	if len(definition.Features) != 1 || definition.Features[0].ID != "prelude" {
		t.Fatal("Earth Withdrawal must only offer Preludes")
	}
	selected, err := cards.Resolve(library, earth)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"252", "C14", "CC3"} {
		card, err := selected.GetByID(id)
		if err != nil || len(card.RequiresSystems) != 0 {
			t.Fatalf("curated Earth card %s now requires an expansion: %v", id, err)
		}
	}
}
