package game_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	tileaction "openmars/internal/action/tile"
	"openmars/internal/game/award"
	"openmars/internal/game/milestone"
	"reflect"
	"slices"
	"strings"
	"testing"

	cardaction "openmars/internal/action/card"
	"openmars/internal/action/confirmation"
	gameaction "openmars/internal/action/game"
	pfaction "openmars/internal/action/projectfunding"
	resconv "openmars/internal/action/resource_conversion"
	turn "openmars/internal/action/turn_management"
	"openmars/internal/events"
	"openmars/internal/game"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/colony"
	"openmars/internal/game/datastore"
	"openmars/internal/game/mode"
	"openmars/internal/game/projectfunding"
	"openmars/internal/game/save"
	"openmars/internal/game/shared"
	"openmars/internal/game/standardproject"
	"openmars/test/testutil"
)

type saveWorld struct {
	g       *game.Game
	repo    game.GameRepository
	logs    *game.InMemoryGameStateRepository
	action  *gameaction.SaveGameAction
	catalog save.Catalog
}

func fullSaveCatalog(t testing.TB) save.Catalog {
	t.Helper()
	colonies, err := colony.LoadColoniesFromJSON("../../../assets/colonies.json")
	if err != nil {
		t.Fatal(err)
	}
	projects, err := projectfunding.LoadProjectsFromJSON("../../../assets/project_funding.json")
	if err != nil {
		t.Fatal(err)
	}
	standard, err := standardproject.LoadStandardProjectsFromJSON("../../../assets/standard_projects.json")
	if err != nil {
		t.Fatal(err)
	}
	return save.Catalog{Cards: testutil.CreateTestModeCatalog(), Maps: testutil.CreateTestMapRegistry(), Colonies: colony.NewInMemoryColonyRegistry(colonies), Projects: projectfunding.NewInMemoryProjectFundingRegistry(projects), StandardProjects: standardproject.NewInMemoryStandardProjectRegistry(standard), Awards: testutil.CreateTestAwardRegistry(), Milestones: testutil.CreateTestMilestoneRegistry()}
}

func newSaveWorld(t testing.TB, count int, settings shared.GameSettings) *saveWorld {
	t.Helper()
	c := fullSaveCatalog(t)
	settings.MaxPlayers = count
	if settings.GameModeID == "" {
		settings.GameModeID = "classic"
	}
	if len(settings.Systems) > 0 {
		// A test-only mode preserves the upstream combined-system save coverage.
		// It is never added to the published catalog or selectable in the application.
		settings.GameModeID = "save-conformance"
		definitions := gamecards.ModeDefinitions(c.Cards)
		definitions = append(definitions, mode.Definition{ID: settings.GameModeID, MinPlayers: 1, MaxPlayers: 8, Systems: settings.Systems, Features: []mode.Feature{{ID: "prelude"}, {ID: "project-funding"}}})
		all := c.Cards.GetAll()
		for i := range all {
			if !strings.HasPrefix(all[i].ID, "ES") && !strings.HasPrefix(all[i].ID, "EP") && !strings.HasPrefix(all[i].ID, "EC") {
				all[i].GameModes = append(all[i].GameModes, settings.GameModeID)
			}
		}
		c.Cards = gamecards.NewInMemoryCardRegistry(all, definitions...)
		awards := c.Awards.GetAll()
		for i := range awards {
			awards[i].GameModes = append(awards[i].GameModes, settings.GameModeID)
		}
		c.Awards = award.NewInMemoryAwardRegistry(awards)
		milestones := c.Milestones.GetAll()
		for i := range milestones {
			milestones[i].GameModes = append(milestones[i].GameModes, settings.GameModeID)
		}
		c.Milestones = milestone.NewInMemoryMilestoneRegistry(milestones)
		standard := c.StandardProjects.GetAll()
		for i := range standard {
			standard[i].GameModes = append(standard[i].GameModes, settings.GameModeID)
		}
		c.StandardProjects = standardproject.NewInMemoryStandardProjectRegistry(standard)
	}
	repo := testutil.NewTestGameRepository(t)
	logs := game.NewInMemoryGameStateRepository()
	a, err := gameaction.NewSaveGameAction(repo, logs, c, "test", nil, testutil.TestLogger())
	testutil.AssertNoError(t, err, "save service")
	g, err := gameaction.NewCreateGameAction(repo, c.Cards, c.Maps, testutil.TestLogger()).Execute(context.Background(), settings)
	testutil.AssertNoError(t, err, "create")
	g.SetSeed(42)
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("p%d", i)
		_, err := gameaction.NewJoinGameAction(repo, c.Cards, testutil.TestLogger()).Execute(context.Background(), g.ID(), id, id)
		testutil.AssertNoError(t, err, "join")
	}
	testutil.AssertNoError(t, turn.NewStartGameAction(repo, c.Colonies, c.Projects, c.Milestones, c.Awards, nil, testutil.TestLogger(), c.Cards).Execute(context.Background(), g.ID(), "p0"), "start")
	return &saveWorld{g, repo, logs, a, c}
}

func (w *saveWorld) restore(t *testing.T) *saveWorld {
	t.Helper()
	doc, err := w.action.Capture(context.Background(), w.g.ID(), w.g.HostPlayerID())
	testutil.AssertNoError(t, err, "capture")
	data, err := w.action.Encode(doc)
	testutil.AssertNoError(t, err, "encode")
	repo := testutil.NewTestGameRepository(t)
	logs := game.NewInMemoryGameStateRepository()
	action, err := gameaction.NewSaveGameAction(repo, logs, w.catalog, "test", nil, testutil.TestLogger())
	testutil.AssertNoError(t, err, "fresh service")
	decoded, err := action.Validate(data)
	testutil.AssertNoError(t, err, "validate in fresh runtime")
	host := w.g.HostPlayerID()
	g, err := importSave(t, action, decoded, host, w.g.State().Players[host].Name)
	testutil.AssertNoError(t, err, "fresh import")
	for _, id := range g.PlayerOrder() {
		p := g.State().Players[id]
		if !p.HasExited && p.PlayerType == "human" {
			testutil.AssertNoError(t, action.ClaimSeat(context.Background(), g.ID(), id, id, p.Name), "rejoin")
		}
	}
	testutil.AssertNoError(t, action.Resume(context.Background(), g.ID(), host), "resume")
	return &saveWorld{g, repo, logs, action, w.catalog}
}

// compareSavedPositions preserves null pointers, arrays, numeric precision, and all gameplay fields.
func compareSavedPositions(t *testing.T, a, b *saveWorld) {
	t.Helper()
	position := func(w *saveWorld) map[string]any {
		data, err := json.Marshal(w.g.State())
		testutil.AssertNoError(t, err, "marshal")
		var value map[string]any
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		testutil.AssertNoError(t, decoder.Decode(&value), "decode position")
		delete(value, "id")
		delete(value, "updatedAt")
		for _, field := range []string{"fundedAwards", "claimedMilestones"} {
			if entries, ok := value[field].([]any); ok {
				for _, entry := range entries {
					if m, ok := entry.(map[string]any); ok {
						delete(m, "FundedAt")
						delete(m, "ClaimedAt")
					}
				}
			}
		}
		// Hydration initializes only directly owned map fields. Normalize those exact fields.
		normalize := func(node map[string]any, typ reflect.Type) {
			for i := 0; i < typ.NumField(); i++ {
				f := typ.Field(i)
				if f.Type.Kind() == reflect.Map && f.Tag.Get("json") != "-" {
					name := f.Tag.Get("json")
					if node[name] == nil {
						node[name] = map[string]any{}
					}
				}
			}
		}
		normalize(value, reflect.TypeFor[datastore.GameState]())
		for _, p := range value["players"].(map[string]any) {
			normalize(p.(map[string]any), reflect.TypeFor[datastore.PlayerState]())
		}
		// Tile appearance is seeded by the newly assigned game ID.
		if tiles, ok := value["tiles"].([]any); ok {
			for _, raw := range tiles {
				tile := raw.(map[string]any)
				if occupant, ok := tile["occupiedBy"].(map[string]any); ok {
					if visual, ok := occupant["visual"].(map[string]any); ok {
						delete(visual, "seed")
					}
				}
			}
		}
		return value
	}
	left, right := position(a), position(b)
	if !reflect.DeepEqual(left, right) {
		x, _ := json.Marshal(left)
		y, _ := json.Marshal(right)
		t.Fatalf("continuations differ:\n%s\n%s", x, y)
	}
}

func (w *saveWorld) active(t *testing.T) {
	t.Helper()
	testutil.AssertNoError(t, w.repo.DataStore().UpdateGame(w.g.ID(), func(s *datastore.GameState) {
		s.CurrentPhase = shared.GamePhaseAction
		s.CurrentTurnPlayerID = "p0"
		s.CurrentTurnActions = 2
		s.CurrentTurnTotalActions = 2
		s.SelectCorporationPhases = nil
		s.SelectStartingCardsPhases = nil
		s.SelectPreludeCardsPhases = nil
		s.DeferredStartingChoices = nil
		corps := []string{testutil.CardID("Helion"), testutil.CardID("Ecoline"), testutil.CardID("Tharsis Republic")}
		for i, id := range s.PlayerOrder {
			s.Players[id].CorporationID = corps[i]
			s.Players[id].Resources = shared.Resources{Credits: 100, Energy: 10, Plants: 12}
			s.Corporations = slices.DeleteFunc(s.Corporations, func(c string) bool { return c == corps[i] })
		}
	}), "active fixture")
	w.g.Colonies().SetDefinitions(w.catalog.Colonies.GetAll())
}

func TestSaveIndependentPendingContinuations(t *testing.T) {
	ctx := context.Background()
	log := testutil.TestLogger()
	type scenario struct {
		prepare func(*saveWorld)
		run     func(*saveWorld) error
	}
	cases := map[string]scenario{
		"card-reveal": {prepare: func(w *saveWorld) {
			w.g.State().Players["p0"].PendingCardReveal = &shared.PendingCardReveal{Source: "saved reveal", Cards: []shared.RevealedCard{{CardID: w.g.State().ProjectCards[0], Name: "revealed", Matched: true}}, Rewards: []shared.CalculatedOutput{{ResourceType: "plant", Amount: 3}}}
		}, run: func(w *saveWorld) error {
			return confirmation.NewConfirmCardRevealAction(w.repo).Execute(ctx, w.g.ID(), "p0")
		}},
		"tile-queue": {prepare: func(w *saveWorld) {
			p, _ := w.g.GetPlayer("p0")
			applier := gamecards.NewBehaviorApplier(p, w.g, "two cities", log).WithCardRegistry(w.catalog.Cards)
			testutil.AssertNoError(t, applier.ApplyOutputs(ctx, []shared.BehaviorCondition{&shared.TilePlacementCondition{ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceCityPlacement, Amount: 2, Target: "none"}}}), "tile queue")
		}, run: func(w *saveWorld) error {
			for i := 0; i < 2; i++ {
				pending := w.g.GetPendingTileSelection("p0")
				if pending == nil || len(pending.AvailableHexes) == 0 {
					return fmt.Errorf("missing queued tile %d", i)
				}
				_, err := tileaction.NewSelectTileAction(w.repo, w.catalog.Cards, w.logs, log).Execute(ctx, w.g.ID(), "p0", pending.AvailableHexes[0])
				if err != nil {
					return err
				}
			}
			return nil
		}},

		"choice": {prepare: func(w *saveWorld) {
			w.g.State().Players["p0"].PendingBehaviorResolutions = []*shared.PendingBehaviorResolution{{ID: "choice", Kind: "choice", Choices: []shared.Choice{{Outputs: []shared.BehaviorCondition{&shared.BasicResourceCondition{ConditionBase: shared.ConditionBase{ResourceType: shared.ResourcePlant, Amount: 3, Target: "self-player"}}}}}}}
		}, run: func(w *saveWorld) error {
			return confirmation.NewConfirmBehaviorChoiceAction(w.repo, w.catalog.Cards, w.logs, log).Execute(ctx, w.g.ID(), "p0", "choice", 0, nil)
		}},
		"discard": {prepare: func(w *saveWorld) {
			p := w.g.State().Players["p0"]
			p.HandCardIDs = []string{w.g.State().ProjectCards[0]}
			w.g.State().ProjectCards = w.g.State().ProjectCards[1:]
			p.PendingBehaviorResolutions = []*shared.PendingBehaviorResolution{{ID: "discard", Kind: "card-discard", MinCards: 1, MaxCards: 1, PendingOutputs: []shared.BehaviorCondition{&shared.BasicResourceCondition{ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceCredit, Amount: 2, Target: "self-player"}}}}}
		}, run: func(w *saveWorld) error {
			return confirmation.NewConfirmCardDiscardAction(w.repo, w.catalog.Cards, w.logs, log).Execute(ctx, w.g.ID(), "p0", "discard", w.g.State().Players["p0"].HandCardIDs)
		}},
		"draw": {prepare: func(w *saveWorld) {
			w.g.State().Players["p0"].PendingCardDrawSelection = &shared.PendingCardDrawSelection{AvailableCards: append([]string(nil), w.g.State().ProjectCards[:3]...), FreeTakeCount: 1, MinFreeTakeCount: 1}
			w.g.State().ProjectCards = w.g.State().ProjectCards[3:]
		}, run: func(w *saveWorld) error {
			return confirmation.NewConfirmCardDrawAction(w.repo, w.catalog.Cards, log).Execute(ctx, w.g.ID(), "p0", w.g.State().Players["p0"].PendingCardDrawSelection.AvailableCards[:1], nil, shared.Payment{})
		}},
		"sell-patents": {prepare: func(w *saveWorld) {
			p := w.g.State().Players["p0"]
			p.HandCardIDs = []string{w.g.State().ProjectCards[0]}
			w.g.State().ProjectCards = w.g.State().ProjectCards[1:]
			p.PendingCardSelection = &shared.PendingCardSelection{AvailableCards: p.HandCardIDs, MinCards: 0, MaxCards: 1, Source: "sell-patents"}
		}, run: func(w *saveWorld) error {
			return confirmation.NewConfirmSellPatentsAction(w.repo, w.logs, log).Execute(ctx, w.g.ID(), "p0", w.g.State().Players["p0"].HandCardIDs)
		}},
		"resource-removal": {prepare: func(w *saveWorld) {
			w.g.State().Players["p0"].PendingResourceRemovalSelection = &shared.PendingResourceRemovalSelection{ID: "remove", ResourceType: shared.ResourcePlant, Amount: 2, Output: &shared.BasicResourceCondition{ConditionBase: shared.ConditionBase{ResourceType: shared.ResourcePlant, Amount: 2, Target: "any-player"}}}
		}, run: func(w *saveWorld) error {
			return confirmation.NewConfirmResourceRemovalAction(w.repo, w.catalog.Cards, w.logs, log).Execute(ctx, w.g.ID(), "p0", "remove", "", 0)
		}},
		"award": {prepare: func(w *saveWorld) {
			w.g.State().Players["p0"].PendingAwardFundSelection = &shared.PendingAwardFundSelection{AvailableAwards: w.g.State().SelectedAwards}
		}, run: func(w *saveWorld) error {
			return confirmation.NewConfirmAwardFundAction(w.repo, w.catalog.Cards, w.catalog.Awards, log).Execute(ctx, w.g.ID(), "p0", w.g.State().SelectedAwards[0])
		}},
		"colony-placement": {prepare: func(w *saveWorld) {
			w.g.Colonies().SetStates([]*colony.ColonyState{{DefinitionID: "luna", MarkerPosition: 3}})
			w.g.State().Players["p0"].PendingColonySelection = &shared.PendingColonySelection{Remaining: 1, AvailableColonyIDs: []string{"luna"}}
		}, run: func(w *saveWorld) error {
			return confirmation.NewConfirmColonyPlacementAction(w.repo, w.catalog.Cards, w.catalog.Colonies, log).Execute(ctx, w.g.ID(), "p0", "luna")
		}},
		"free-trade": {prepare: func(w *saveWorld) {
			w.g.Colonies().SetStates([]*colony.ColonyState{{DefinitionID: "luna", MarkerPosition: 3}})
			w.g.Colonies().AddTradeFleets("p0", 1)
			w.g.State().Players["p0"].PendingFreeTradeSelection = &shared.PendingFreeTradeSelection{AvailableColonyIDs: []string{"luna"}}
		}, run: func(w *saveWorld) error {
			return confirmation.NewConfirmFreeTradeAction(w.repo, w.catalog.Cards, w.catalog.Colonies, w.logs).Execute(ctx, w.g.ID(), "p0", "luna", 0)
		}},
		"colony-resources-queue": {prepare: func(w *saveWorld) {
			w.g.State().Players["p0"].PendingColonyResourceQueue = []shared.PendingColonyResourceSelection{{ResourceType: "animal", Amount: 2, ColonyID: "miranda"}, {ResourceType: "microbe", Amount: 1, ColonyID: "enceladus"}}
		}, run: func(w *saveWorld) error {
			a := confirmation.NewConfirmColonyResourceAction(w.repo, w.catalog.Cards, w.logs, log)
			if err := a.Execute(ctx, w.g.ID(), "p0", ""); err != nil {
				return err
			}
			return a.Execute(ctx, w.g.ID(), "p0", "")
		}},
		"effect-selection": {prepare: func(w *saveWorld) {
			w.g.Colonies().SetStates([]*colony.ColonyState{{DefinitionID: "luna", MarkerPosition: 3}})
			outputs := []shared.BehaviorCondition{&shared.ColonyCondition{ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceColonyTrackStep, Amount: 1, Target: "none"}}}
			p, _ := w.g.GetPlayer("p0")
			options, _, err := gamecards.EffectSelectionOptions(outputs, p, w.g, w.catalog.Cards, w.catalog.Colonies)
			testutil.AssertNoError(t, err, "options")
			w.g.State().Players["p0"].PendingEffectSelection = &shared.PendingEffectSelection{Outputs: outputs, Options: options}
		}, run: func(w *saveWorld) error {
			return confirmation.NewConfirmEffectSelectionAction(w.repo, w.catalog.Cards, w.catalog.Colonies, w.logs).Execute(ctx, w.g.ID(), "p0", 0)
		}},
		"project-funding": {prepare: func(w *saveWorld) {
			w.g.SetProjectFundingStates([]*projectfunding.ProjectState{{DefinitionID: "pf_orbital_station"}})
		}, run: func(w *saveWorld) error {
			return pfaction.NewFundSeatAction(w.repo, w.catalog.Projects, w.logs).Execute(ctx, w.g.ID(), "p0", "pf_orbital_station", pfaction.FundSeatPayment{Credits: 6})
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			w := newSaveWorld(t, 2, shared.GameSettings{Features: []string{"prelude", "project-funding"}, Systems: []string{"venus", "colonies"}})
			w.active(t)
			tc.prepare(w)
			restored := w.restore(t)
			compareSavedPositions(t, w, restored)
			testutil.AssertNoError(t, tc.run(w), "original continuation")
			testutil.AssertNoError(t, tc.run(restored), "restored continuation")
			compareSavedPositions(t, w, restored)
			again := restored.restore(t)
			compareSavedPositions(t, restored, again)
		})
	}
}

func TestSaveCatalogFingerprintStable(t *testing.T) {
	c := fullSaveCatalog(t)
	before, err := c.Fingerprint()
	testutil.AssertNoError(t, err, "fingerprint")
	for i := 0; i < 20; i++ {
		after, err := c.Fingerprint()
		testutil.AssertNoError(t, err, "fingerprint")
		if before != after {
			t.Fatal("fingerprint changed")
		}
	}
}

func TestSaveIndependentLifecycle(t *testing.T) {
	for _, count := range []int{1, 2} {
		t.Run(fmt.Sprintf("players-%d", count), func(t *testing.T) {
			ctx := context.Background()
			w := newSaveWorld(t, count, shared.GameSettings{Features: []string{"prelude"}})
			corps := []string{testutil.CardID("Helion"), testutil.CardID("Ecoline")}
			preludes := []string{"P01", "P03", "P04", "P07"}
			testutil.AssertNoError(t, w.repo.DataStore().UpdateGame(w.g.ID(), func(s *datastore.GameState) {
				// Fixed legal offerings keep this test independent of random card order.
				for i, id := range s.PlayerOrder {
					s.SelectCorporationPhases[id] = &shared.SelectCorporationPhase{AvailableCorporations: []string{corps[i]}}
					s.SelectPreludeCardsPhases[id] = &shared.SelectPreludeCardsPhase{AvailablePreludes: preludes[i*2 : i*2+2], MaxSelectable: 2}
				}
				s.Corporations = slices.DeleteFunc(s.Corporations, func(id string) bool { return slices.Contains(corps[:count], id) })
				s.PreludeCards = slices.DeleteFunc(s.PreludeCards, func(id string) bool { return slices.Contains(preludes[:count*2], id) })
			}), "offerings")
			restored := w.restore(t)
			advance := func(run func(*saveWorld) error) {
				t.Helper()
				testutil.AssertNoError(t, run(w), "original")
				testutil.AssertNoError(t, run(restored), "restored")
				compareSavedPositions(t, w, restored)
				if w.g.Status() == shared.GameStatusActive {
					restored = restored.restore(t)
					compareSavedPositions(t, w, restored)
				}
			}
			for i, id := range w.g.PlayerOrder() {
				advance(func(x *saveWorld) error {
					return turn.NewSelectStartingChoicesAction(x.repo, x.catalog.Cards, x.catalog.Awards, testutil.TestLogger()).Execute(ctx, x.g.ID(), id, corps[i], preludes[i*2:i*2+2], nil, shared.Payment{})
				})
			}
			phases := map[shared.GamePhase]bool{}
			for i := 0; i < 20 && w.g.CurrentPhase() != shared.GamePhaseAction; i++ {
				phases[w.g.CurrentPhase()] = true
				advance(func(x *saveWorld) error {
					return turn.NewConfirmInitAdvanceAction(x.repo, x.catalog.Cards, x.catalog.Awards, x.logs, testutil.TestLogger()).Execute(ctx, x.g.ID(), "p0")
				})
			}
			if !phases[shared.GamePhaseInitApplyCorp] || !phases[shared.GamePhaseInitApplyPrelude] || w.g.CurrentPhase() != shared.GamePhaseAction {
				t.Fatal("did not exercise corporation and prelude initialization")
			}
			for generation := 0; generation < 2; generation++ {
				for _, id := range w.g.TurnOrder() {
					advance(func(x *saveWorld) error {
						return turn.NewSkipActionAction(x.repo, gameaction.NewFinalScoringAction(x.repo, x.catalog.Cards, x.catalog.Awards, x.catalog.Milestones, testutil.TestLogger()), testutil.TestLogger()).Execute(ctx, x.g.ID(), id)
					})
				}
				testutil.AssertEqual(t, shared.GamePhaseProductionAndCardDraw, w.g.CurrentPhase(), "production reached")
				for _, id := range w.g.PlayerOrder() {
					advance(func(x *saveWorld) error {
						return confirmation.NewConfirmProductionCardsAction(x.repo, x.catalog.Cards, nil, testutil.TestLogger()).Execute(ctx, x.g.ID(), id, nil, false, shared.Payment{})
					})
				}
			}
			// Reach the endgame from a mature position, then continue the real production/scoring flow.
			for _, x := range []*saveWorld{w, restored} {
				maxAllGlobalParams(t, x.g)
				for _, p := range x.g.GetAllPlayers() {
					p.Resources().Set(shared.Resources{Plants: 16, Credits: 100})
				}
			}
			for _, id := range w.g.TurnOrder() {
				advance(func(x *saveWorld) error {
					return turn.NewSkipActionAction(x.repo, gameaction.NewFinalScoringAction(x.repo, x.catalog.Cards, x.catalog.Awards, x.catalog.Milestones, testutil.TestLogger()), testutil.TestLogger()).Execute(ctx, x.g.ID(), id)
				})
			}
			for _, id := range w.g.PlayerOrder() {
				advance(func(x *saveWorld) error {
					return confirmation.NewConfirmProductionCardsAction(x.repo, x.catalog.Cards, gameaction.NewFinalScoringAction(x.repo, x.catalog.Cards, x.catalog.Awards, x.catalog.Milestones, testutil.TestLogger()), testutil.TestLogger()).Execute(ctx, x.g.ID(), id, nil, false, shared.Payment{})
				})
			}
			testutil.AssertEqual(t, shared.GamePhaseFinalPhase, w.g.CurrentPhase(), "final greenery reached")
			for _, id := range w.g.TurnOrder() {
				advance(func(x *saveWorld) error {
					return turn.NewSkipActionAction(x.repo, gameaction.NewFinalScoringAction(x.repo, x.catalog.Cards, x.catalog.Awards, x.catalog.Milestones, testutil.TestLogger()), testutil.TestLogger()).Execute(ctx, x.g.ID(), id)
				})
			}
			testutil.AssertEqual(t, shared.GameStatusCompleted, w.g.Status(), "final scoring reached")
		})
	}
}

func FuzzSaveSemanticValidation(f *testing.F) {
	w := newSaveWorld(f, 2, shared.GameSettings{Features: []string{}})
	doc, err := w.action.Capture(context.Background(), w.g.ID(), w.g.HostPlayerID())
	testutil.AssertNoError(f, err, "capture seed")
	seed, err := w.action.Encode(doc)
	testutil.AssertNoError(f, err, "encode seed")
	fingerprint, err := w.catalog.Fingerprint()
	testutil.AssertNoError(f, err, "fingerprint")
	f.Add(seed)
	f.Add([]byte(`{"formatVersion":1}`))
	f.Add([]byte(`[]`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		decoded, err := save.Decode(data)
		if err != nil {
			return
		}
		if err = save.Validate(decoded, w.catalog, fingerprint); err != nil {
			return
		}
		ds, err := datastore.NewDataStore()
		testutil.AssertNoError(t, err, "fresh store")
		_, err = w.action.RestoreRuntime(ds, decoded.State)
		testutil.AssertNoError(t, err, "validated position must hydrate")
		encoded, err := save.Encode(decoded)
		testutil.AssertNoError(t, err, "encode valid state")
		again, err := save.Decode(encoded)
		testutil.AssertNoError(t, err, "decode valid state")
		testutil.AssertNoError(t, save.Validate(again, w.catalog, fingerprint), "valid state remains valid")
	})
}

func TestSaveMatureCombinedGame(t *testing.T) {
	ctx := context.Background()
	log := testutil.TestLogger()
	w := newSaveWorld(t, 2, shared.GameSettings{Features: []string{"prelude", "project-funding"}, Systems: []string{"venus", "colonies"}})
	w.active(t)
	testutil.AssertNoError(t, w.repo.DataStore().UpdateGame(w.g.ID(), func(s *datastore.GameState) {
		s.Generation = 5
		s.Oxygen = 14
		s.Venus = 30
		s.Players["p1"].HasPassed = true
		s.Players["p0"].Resources.Credits = 500
	}), "mature position")
	testutil.AssertNoError(t, w.g.SetCurrentTurn(ctx, "p0", -1), "unlimited turn")
	p, _ := w.g.GetPlayer("p0")
	for _, name := range []string{"Pets", "Power Plant", "Predators", "Dirigibles"} {
		id := testutil.CardID(name)
		card, err := w.catalog.Cards.GetByID(id)
		testutil.AssertNoError(t, err, "card")
		testutil.AssertNoError(t, w.repo.DataStore().UpdateGame(w.g.ID(), func(s *datastore.GameState) {
			s.ProjectCards = slices.DeleteFunc(s.ProjectCards, func(c string) bool { return c == id })
			s.Players["p0"].HandCardIDs = append(s.Players["p0"].HandCardIDs, id)
		}), "deal card")
		testutil.AssertNoError(t, cardaction.NewPlayCardAction(w.repo, w.catalog.Cards, w.logs, log, w.catalog.Colonies).Execute(ctx, w.g.ID(), "p0", id, shared.NativePayment(shared.ResourceCredit, card.Cost), nil, nil, nil, nil, nil), "play "+name)
	}
	p.Resources().AddToStorage(testutil.CardID("Predators"), 4)
	p.Resources().AddValueModifier(shared.ResourceTitanium, 1)
	actions := p.Actions().List()
	if len(actions) == 0 {
		t.Fatal("fixture has no manual actions")
	}
	actions[0].TimesUsedThisGeneration = 1
	p.Actions().SetActions(actions)
	testutil.AssertNoError(t, w.g.Awards().FundAward(ctx, shared.AwardType(w.g.SelectedAwards()[0]), "p0", 8), "fund award")
	testutil.AssertNoError(t, w.g.Milestones().ClaimMilestone(ctx, shared.MilestoneType(w.g.SelectedMilestones()[0]), "p0", 5), "claim milestone")
	w.g.Colonies().SetStates([]*colony.ColonyState{{DefinitionID: "luna", MarkerPosition: 3, PlayerColonies: []string{"p0"}}})
	w.g.SetProjectFundingStates([]*projectfunding.ProjectState{{DefinitionID: "pf_orbital_station", SeatOwners: []string{"p0"}}})
	// A city both changes the board and fires Pets' restored subscription.
	testutil.AssertNoError(t, gamecards.NewBehaviorApplier(p, w.g, "city", log).WithCardRegistry(w.catalog.Cards).ApplyOutputs(ctx, []shared.BehaviorCondition{&shared.TilePlacementCondition{ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceCityPlacement, Amount: 1, Target: "none"}}}), "city selection")
	restored := w.restore(t)
	hex := w.g.GetPendingTileSelection("p0").AvailableHexes[0]
	for _, x := range []*saveWorld{w, restored} {
		_, err := tileaction.NewSelectTileAction(x.repo, x.catalog.Cards, x.logs, log).Execute(ctx, x.g.ID(), "p0", hex)
		testutil.AssertNoError(t, err, "city")
	}
	compareSavedPositions(t, w, restored)
	restored = restored.restore(t)
	for _, x := range []*saveWorld{w, restored} {
		maxAllGlobalParams(t, x.g)
		testutil.AssertNoError(t, x.g.UpdatePhase(ctx, shared.GamePhaseFinalPhase), "final phase")
		testutil.AssertNoError(t, x.g.SetCurrentTurn(ctx, "p0", -1), "final turn")
		testutil.AssertNoError(t, resconv.NewConvertPlantsToGreeneryAction(x.repo, x.catalog.Cards, x.logs, log).Execute(ctx, x.g.ID(), "p0", shared.NativePayment(shared.ResourcePlant, 8)), "convert plants")
	}
	restored = restored.restore(t)
	compareSavedPositions(t, w, restored)
	hex = w.g.GetPendingTileSelection("p0").AvailableHexes[0]
	for _, x := range []*saveWorld{w, restored} {
		_, err := tileaction.NewSelectTileAction(x.repo, x.catalog.Cards, x.logs, log).Execute(ctx, x.g.ID(), "p0", hex)
		testutil.AssertNoError(t, err, "greenery callback")
	}
	compareSavedPositions(t, w, restored)
	for _, x := range []*saveWorld{w, restored} {
		testutil.AssertNoError(t, gameaction.NewFinalScoringAction(x.repo, x.catalog.Cards, x.catalog.Awards, x.catalog.Milestones, log).Execute(ctx, x.g.ID()), "score")
	}
	compareSavedPositions(t, w, restored)
}

func TestSaveEarthWithdrawalAndReconnectionContinuations(t *testing.T) {
	ctx := context.Background()
	w := newSaveWorld(t, 1, shared.GameSettings{GameModeID: "earth-withdrawal", Features: []string{}, GameModeParameters: map[string]int{"withdrawal-generation": 6}})
	w.active(t)
	testutil.AssertNoError(t, w.g.SetCurrentTurn(ctx, "p0", -1), "unlimited fixture turn")
	play := func(world *saveWorld, id string, targets []string) {
		t.Helper()
		registry := gamecards.ForGame(world.catalog.Cards, world.g)
		card, err := registry.GetByID(id)
		testutil.AssertNoError(t, err, "effective card")
		testutil.AssertNoError(t, world.repo.DataStore().UpdateGame(world.g.ID(), func(s *datastore.GameState) {
			s.ProjectCards = slices.DeleteFunc(s.ProjectCards, func(cid string) bool { return cid == id })
			s.Players["p0"].HandCardIDs = append(s.Players["p0"].HandCardIDs, id)
		}), "deal scenario card")
		err = cardaction.NewPlayCardAction(world.repo, world.catalog.Cards, world.logs, testutil.TestLogger(), world.catalog.Colonies).Execute(ctx, world.g.ID(), "p0", id, shared.NativePayment(shared.ResourceCredit, card.Cost), nil, targets, nil, nil, nil)
		testutil.AssertNoError(t, err, "play "+id)
	}
	for _, id := range []string{"ES001", "ES003", "ES005", "ES019", "ES012"} {
		play(w, id, nil)
	}
	play(w, "ES021", []string{"ES005"})
	p, _ := w.g.GetPlayer("p0")
	testutil.AssertNoError(t, p.Resources().SetDependencyStatus("ES003", "earth-support", false), "retire greenhouse operation")
	p.Resources().RecordBehaviorUse("ES012", "office", 1)
	testutil.AssertNoError(t, w.g.ChangeModeState(ctx, "p0", "withdrawal-generation", -1), "advance original deadline")
	// Keep a shared card in hand to verify restoration uses its scenario price.
	testutil.AssertNoError(t, w.repo.DataStore().UpdateGame(w.g.ID(), func(s *datastore.GameState) {
		s.ProjectCards = slices.DeleteFunc(s.ProjectCards, func(id string) bool { return id == "110" })
		s.Players["p0"].HandCardIDs = append(s.Players["p0"].HandCardIDs, "110")
	}), "deal Business Network")
	restored := w.restore(t)
	rp, _ := restored.g.GetPlayer("p0")
	deadline, _ := restored.g.ModeStateValue("withdrawal-generation")
	testutil.AssertEqual(t, 5, deadline, "changed deadline survives")
	testutil.AssertEqual(t, 1, rp.Resources().BehaviorUses("ES012", "office", 1), "shared usage survives")
	if !rp.Resources().DependencyStatus("ES005", "earth-support").Removed || !rp.Resources().DependencyStatus("ES003", "earth-support").Retired {
		t.Fatal("imported or retired status lost")
	}
	state, ok := rp.CardStateStore().GetState("110")
	if !ok || state.Cost["credit"] != 2 {
		t.Fatalf("restored card store ignored mode price: %+v", state)
	}
	testutil.AssertEqual(t, p.Resources().Production(), rp.Resources().Production(), "signed production survives")
	plants := p.Resources().Get().Plants
	for _, world := range []*saveWorld{w, restored} {
		testutil.AssertNoError(t, world.g.SetGeneration(ctx, 5), "reach changed deadline")
		world.g.ResolveModeBoundary(ctx, "production-end", false)
	}
	testutil.AssertEqual(t, plants+6, rp.Resources().Get().Plants, "restored first-withdrawal subscription pays once")
	testutil.AssertEqual(t, p.Resources().Get(), rp.Resources().Get(), "first-withdrawal continuation agrees")
	testutil.AssertEqual(t, 3, rp.Resources().Production().Energy, "imported energy survives disconnection")
	testutil.AssertEqual(t, 0, rp.Resources().Production().Credits, "imported upkeep offsets permanent office income after disconnection")
	testutil.AssertEqual(t, 0, rp.Resources().Production().Steel, "unimported production stops")
	play(restored, "ES042", nil)
	pending := restored.restore(t)
	if len(pending.g.ModeRuntime().Windows) != 1 || pending.g.ModeRuntime().Windows[0].Status != "pending" {
		t.Fatal("pending reconnection was not retained")
	}
	testutil.AssertNoError(t, pending.g.SetGeneration(ctx, 6), "next generation")
	pending.g.ResolveModeBoundary(ctx, "research-start", false)
	active := pending.restore(t)
	ap, _ := active.g.GetPlayer("p0")
	if contact, _ := active.g.ModeStateValue("earth-contact"); contact != 1 {
		t.Fatal("active reconnection lost")
	}
	testutil.AssertEqual(t, 2, ap.Resources().Production().Steel, "temporary source returns during restored window")
	testutil.AssertEqual(t, 0, ap.Resources().Production().Plants, "retired source remains retired")
	before := ap.Resources().Get()
	events.Publish(active.g.EventBus(), events.ProductionStartingEvent{GameID: active.g.ID()})
	testutil.AssertEqual(t, before.Credits+2, ap.Resources().Get().Credits, "restored stipend subscribed exactly once")
	active.g.ResolveModeBoundary(ctx, "production-end", false)
	testutil.AssertEqual(t, 0, ap.Resources().Production().Steel, "temporary source stops after restored window")
	testutil.AssertEqual(t, 3, ap.Resources().Production().Energy, "imported contribution is not duplicated")
	testutil.AssertEqual(t, before.Plants, ap.Resources().Get().Plants, "reconnection closure does not replay reserve")
	credits := ap.Resources().Get().Credits
	events.Publish(active.g.EventBus(), events.ProductionStartingEvent{GameID: active.g.ID()})
	testutil.AssertEqual(t, credits, ap.Resources().Get().Credits, "disconnected stipend stays disabled")
}
