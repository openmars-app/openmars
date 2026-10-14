package game_test

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"

	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	baseaction "openmars/internal/action"
	"openmars/internal/action/confirmation"
	connectionaction "openmars/internal/action/connection"
	gameaction "openmars/internal/action/game"
	"openmars/internal/action/query"
	tileaction "openmars/internal/action/tile"
	turn "openmars/internal/action/turn_management"
	"openmars/internal/delivery/dto"
	httpdelivery "openmars/internal/delivery/http"
	wsdelivery "openmars/internal/delivery/websocket"
	"openmars/internal/delivery/websocket/core"
	gamehandler "openmars/internal/delivery/websocket/handler/game"
	"openmars/internal/events"
	"openmars/internal/game"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/colony"
	"openmars/internal/game/datastore"
	"openmars/internal/game/player"
	"openmars/internal/game/projectfunding"
	"openmars/internal/game/save"
	"openmars/internal/game/shared"
	"openmars/internal/game/standardproject"
	"openmars/test/testutil"
	"time"
)

func saveFixture(t *testing.T) (*game.Game, game.GameRepository, *game.InMemoryGameStateRepository, *gameaction.SaveGameAction) {
	t.Helper()
	ctx := context.Background()
	repo := testutil.NewTestGameRepository(t)
	cards := testutil.CreateTestModeCatalog()
	maps := testutil.CreateTestMapRegistry()
	log := testutil.TestLogger()
	catalog := save.Catalog{StandardProjects: standardproject.NewInMemoryStandardProjectRegistry(nil), Cards: cards, Maps: maps, Colonies: colony.NewInMemoryColonyRegistry(nil), Projects: projectfunding.NewInMemoryProjectFundingRegistry(nil), Awards: testutil.CreateTestAwardRegistry(), Milestones: testutil.CreateTestMilestoneRegistry()}
	g, err := gameaction.NewCreateGameAction(repo, cards, maps, log).Execute(ctx, shared.GameSettings{MaxPlayers: 2, GameModeID: "classic", Features: []string{}, ClaudeOAuthToken: "secret-do-not-export"})
	testutil.AssertNoError(t, err, "create")
	g.SetSeed(math.MaxUint64)
	for _, id := range []string{"alice", "bob"} {
		_, err := gameaction.NewJoinGameAction(repo, cards, log).Execute(ctx, g.ID(), id, id)
		testutil.AssertNoError(t, err, "add player")
	}
	testutil.AssertNoError(t, g.SetHostPlayerID(ctx, "alice"), "host")
	testutil.AssertNoError(t, turn.NewStartGameAction(repo, catalog.Colonies, catalog.Projects, catalog.Milestones, catalog.Awards, nil, log, cards).Execute(ctx, g.ID(), "alice"), "start")
	logs := game.NewInMemoryGameStateRepository()
	_, err = logs.Write(ctx, g.ID(), g, "start", shared.SourceTypeCardPlay, "alice", "Started game")
	testutil.AssertNoError(t, err, "log")
	action, err := gameaction.NewSaveGameAction(repo, logs, catalog, "test", nil, log)
	testutil.AssertNoError(t, err, "save action")
	return g, repo, logs, action
}

func encodeSave(t *testing.T, g *game.Game, a *gameaction.SaveGameAction) []byte {
	t.Helper()
	doc, err := a.Capture(context.Background(), g.ID(), g.HostPlayerID())
	testutil.AssertNoError(t, err, "capture")
	data, err := save.Encode(doc)
	testutil.AssertNoError(t, err, "encode")
	return data
}

func TestSaveRoundTripAndResume(t *testing.T) {
	g, repo, logs, a := saveFixture(t)
	ctx := context.Background()
	bob, err := g.GetPlayer("bob")
	testutil.AssertNoError(t, err, "receipt player")
	drawn, err := g.Deck().DrawProjectCards(ctx, 1)
	testutil.AssertNoError(t, err, "receipt draw")
	bob.Hand().ReceiveCards(drawn, "draw", "")
	testutil.AssertNoError(t, repo.DataStore().UpdatePlayer(g.ID(), "bob", func(p *datastore.PlayerState) { p.Resources.Heat = 7 }), "unlogged pending change")
	data := encodeSave(t, g, a)
	if bytes.Contains(data, []byte("secret-do-not-export")) {
		t.Fatal("credential leaked into save/history")
	}
	doc, err := a.Validate(data)
	testutil.AssertNoError(t, err, "validate export")
	testutil.AssertEqual(t, uint64(math.MaxUint64), doc.State.Seed, "lossless seed")
	count := len(doc.History)
	restored, err := importSave(t, a, doc, "alice", "Alice Again")
	testutil.AssertNoError(t, err, "import")
	if restored.ID() == g.ID() {
		t.Fatal("import reused source ID")
	}
	testutil.AssertEqual(t, g.ID(), doc.State.ID, "import leaves document untouched")
	loadedBob, err := restored.GetPlayer("bob")
	testutil.AssertNoError(t, err, "restored receipt player")
	if !reflect.DeepEqual(bob.Selection().CardReceipts(), loadedBob.Selection().CardReceipts()) {
		t.Fatal("existing receipt IDs or contents changed during restore")
	}
	if !slices.Equal(g.Deck().ProjectCards(), restored.Deck().ProjectCards()) {
		t.Fatal("deck changed")
	}
	if !slices.Equal(g.TurnOrder(), restored.TurnOrder()) {
		t.Fatal("turn changed")
	}
	testutil.AssertEqual(t, g.CurrentPhase(), restored.CurrentPhase(), "phase preserved")
	if restored.ResumeLobby() == nil {
		t.Fatal("missing resume lobby")
	}
	history, err := repo.DataStore().GetGameHistory(restored.ID())
	testutil.AssertNoError(t, err, "history")
	testutil.AssertEqual(t, count, len(history), "history restored")
	originalHistory, err := repo.DataStore().GetGameHistory(g.ID())
	testutil.AssertNoError(t, err, "original history")
	for _, h := range originalHistory {
		testutil.AssertEqual(t, g.ID(), h.State.ID, "source history untouched")
	}
	if err := a.Resume(ctx, restored.ID(), "alice"); err == nil {
		t.Fatal("resumed without players")
	}
	host, _ := restored.GetPlayer("alice")
	host.SetConnected(true)
	testutil.AssertNoError(t, a.ClaimSeat(ctx, restored.ID(), "", "bob", "Replacement"), "claim")
	if err := a.ClaimSeat(ctx, restored.ID(), "", "bob", "Other"); err == nil {
		t.Fatal("double claim allowed")
	}
	testutil.AssertNoError(t, a.Resume(ctx, restored.ID(), "alice"), "resume")
	if restored.ResumeLobby() != nil {
		t.Fatal("resume gate remains")
	}
	one, err := g.Deck().DrawProjectCards(ctx, 3)
	testutil.AssertNoError(t, err, "draw original")
	two, err := restored.Deck().DrawProjectCards(ctx, 3)
	testutil.AssertNoError(t, err, "draw restored")
	if !slices.Equal(one, two) {
		t.Fatal("next draws differ")
	}
	oldLog, err := logs.GetDiff(ctx, g.ID())
	testutil.AssertNoError(t, err, "original log")
	entry, err := logs.Write(ctx, restored.ID(), restored, "next", shared.SourceTypeCardPlay, "bob", "Next action")
	testutil.AssertNoError(t, err, "continued log")
	testutil.AssertEqual(t, oldLog[len(oldLog)-1].SequenceNumber+1, entry.SequenceNumber, "log sequence continues")
	originalNext, err := logs.Write(ctx, g.ID(), g, "next", shared.SourceTypeCardPlay, "bob", "Next action")
	testutil.AssertNoError(t, err, "original continuation log")
	if !reflect.DeepEqual(originalNext.Changes, entry.Changes) {
		t.Fatal("lost the last logged baseline for a pending action")
	}
	testutil.AssertEqual(t, "bob", history[len(history)-1].State.Players["bob"].Name, "historical name retained")
}

func TestSavePendingConditionsRoundTrip(t *testing.T) {
	g, repo, _, a := saveFixture(t)
	output := &shared.BasicResourceCondition{ConditionBase: shared.ConditionBase{ResourceType: shared.ResourcePlant, Amount: 2, Target: "all-opponents"}}
	err := repo.DataStore().UpdatePlayer(g.ID(), "alice", func(p *datastore.PlayerState) {
		p.PendingBehaviorResolutions = []*shared.PendingBehaviorResolution{{ID: "choice", Kind: "choice", PendingOutputs: []shared.BehaviorCondition{output}, Choices: []shared.Choice{{Outputs: []shared.BehaviorCondition{output}}}}}
		p.PendingEffectSelection = &shared.PendingEffectSelection{Outputs: []shared.BehaviorCondition{output}, Options: []shared.EffectSelectionOption{{TargetPlayerID: "bob", Outputs: []shared.BehaviorCondition{output}}}}
	})
	testutil.AssertNoError(t, err, "set pending")
	testutil.AssertNoError(t, repo.DataStore().UpdateGame(g.ID(), func(s *datastore.GameState) {
		s.PendingTileSelections["alice"] = &shared.PendingTileSelection{TileType: "city", OnComplete: &shared.TileCompletionCallback{Type: "adjacent-removal", Output: output}}
	}), "tile continuation")
	data := encodeSave(t, g, a)
	doc, err := a.Validate(data)
	testutil.AssertNoError(t, err, "validate pending")
	restored, err := importSave(t, a, doc, "alice", "alice")
	testutil.AssertNoError(t, err, "restore pending")
	p := restored.State().Players["alice"]
	if _, ok := p.PendingBehaviorResolutions[0].PendingOutputs[0].(*shared.BasicResourceCondition); !ok {
		t.Fatal("lost concrete output")
	}
	if _, ok := p.PendingBehaviorResolutions[0].Choices[0].Outputs[0].(*shared.BasicResourceCondition); !ok {
		t.Fatal("lost choice output")
	}
	if _, ok := p.PendingEffectSelection.Options[0].Outputs[0].(*shared.BasicResourceCondition); !ok {
		t.Fatal("lost effect option")
	}
	if restored.State().PendingTileSelections["alice"].OnComplete.Output.Amount != 2 {
		t.Fatal("lost typed tile continuation")
	}
	if len(doc.History) < 3 {
		t.Fatal("pending conditions stopped history recording")
	}
}

func TestSaveRejectsInvalidDocuments(t *testing.T) {
	g, repo, _, a := saveFixture(t)
	original := encodeSave(t, g, a)
	tests := map[string]func(*save.Document){
		"format":    func(d *save.Document) { d.FormatVersion++ },
		"content":   func(d *save.Document) { d.ContentFingerprint = "wrong" },
		"completed": func(d *save.Document) { d.State.Status = shared.GameStatusCompleted },
		"turn":      func(d *save.Document) { d.State.CurrentTurnPlayerID = "missing" },
		"deck":      func(d *save.Document) { d.State.ProjectCards = append(d.State.ProjectCards, d.State.ProjectCards[0]) },
		"card":      func(d *save.Document) { d.State.Players["alice"].HandCardIDs = []string{"unknown-card"} },
		"history":   func(d *save.Document) { d.History[1].Sequence = d.History[0].Sequence },
		"resource":  func(d *save.Document) { d.State.Players["alice"].Resources.Plants = -1 },
		"callback": func(d *save.Document) {
			d.State.PendingTileSelections["alice"] = &shared.PendingTileSelection{OnComplete: &shared.TileCompletionCallback{Type: "unknown"}}
		},
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			doc, err := save.Decode(original)
			testutil.AssertNoError(t, err, "decode")
			change(doc)
			data, err := save.Encode(doc)
			testutil.AssertNoError(t, err, "encode invalid")
			if _, err := a.Validate(data); err == nil {
				t.Fatal("invalid save accepted")
			}
		})
	}
	for _, data := range [][]byte{append(append([]byte{}, original...), []byte("{}")...), bytes.Replace(original, []byte(`"formatVersion":1`), []byte(`"formatVersion":1,"formatVersion":1`), 1), bytes.Replace(original, []byte(`"generation":1`), []byte(`"generation":1,"surprise":true`), 1)} {
		if _, err := a.Validate(data); err == nil {
			t.Fatal("ambiguous JSON accepted")
		}
	}
	games, err := repo.List(context.Background(), nil)
	testutil.AssertNoError(t, err, "list")
	testutil.AssertEqual(t, 1, len(games), "invalid imports leave no games")
}

func TestSaveStateFieldsAreClassified(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeFor[datastore.GameState](), reflect.TypeFor[datastore.PlayerState]()} {
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if f.Tag.Get("save") == "" || f.Tag.Get("json") == "" {
				t.Errorf("%s.%s needs explicit save classification and JSON field", typ.Name(), f.Name)
			}
		}
	}
}

func TestSaveCanonicalDecodeEncode(t *testing.T) {
	g, _, _, a := saveFixture(t)
	data := encodeSave(t, g, a)
	doc, err := save.Decode(data)
	testutil.AssertNoError(t, err, "decode")
	again, err := json.Marshal(doc)
	testutil.AssertNoError(t, err, "encode")
	if !bytes.Equal(data, again) {
		t.Fatal("canonical save changes after round trip")
	}
}

func importSave(t *testing.T, a *gameaction.SaveGameAction, doc *save.Document, seatID, name string) (*game.Game, error) {
	t.Helper()
	prepared, err := a.PrepareImport(doc, seatID, name, "")
	if err != nil {
		return nil, err
	}
	return a.Import(context.Background(), prepared)
}

func TestSaveContinuesStartingSelectionAndInit(t *testing.T) {
	g, repo, logs, a := saveFixture(t)
	ctx := context.Background()
	doc, err := a.Validate(encodeSave(t, g, a))
	testutil.AssertNoError(t, err, "validate")
	restored, err := importSave(t, a, doc, "alice", "alice")
	testutil.AssertNoError(t, err, "restore")
	for _, id := range []string{"alice", "bob"} {
		testutil.AssertNoError(t, a.ClaimSeat(ctx, restored.ID(), id, id, id), "connect")
	}
	testutil.AssertNoError(t, a.Resume(ctx, restored.ID(), "alice"), "resume")
	selectChoices := turn.NewSelectStartingChoicesAction(repo, testutil.GetCardDB(), testutil.CreateTestAwardRegistry(), testutil.TestLogger())
	for _, id := range g.TurnOrder() {
		corp := g.GetSelectCorporationPhase(id).AvailableCorporations[0]
		for _, match := range []*game.Game{g, restored} {
			testutil.AssertNoError(t, selectChoices.Execute(ctx, match.ID(), id, corp, nil, nil, shared.Payment{}), "choose corporation")
		}
		assertSamePosition(t, g, restored)
		_, err = a.Validate(encodeSave(t, restored, a))
		testutil.AssertNoError(t, err, "re-save during selection")
	}
	confirm := turn.NewConfirmInitAdvanceAction(repo, testutil.GetCardDB(), testutil.CreateTestAwardRegistry(), logs, testutil.TestLogger())
	for i := 0; i < 12 && g.CurrentPhase() != shared.GamePhaseAction; i++ {
		for _, match := range []*game.Game{g, restored} {
			testutil.AssertNoError(t, confirm.Execute(ctx, match.ID(), "alice"), "advance init")
		}
		assertSamePosition(t, g, restored)
		_, err = a.Validate(encodeSave(t, restored, a))
		testutil.AssertNoError(t, err, "re-save during init")
	}
	testutil.AssertEqual(t, shared.GamePhaseAction, restored.CurrentPhase(), "entered action phase")
}

func assertSamePosition(t *testing.T, a, b *game.Game) {
	t.Helper()
	one := *a.State()
	two := *b.State()
	one.ID = two.ID
	one.UpdatedAt = two.UpdatedAt
	left, err := json.Marshal(one)
	testutil.AssertNoError(t, err, "compare original")
	right, err := json.Marshal(two)
	testutil.AssertNoError(t, err, "compare restored")
	// Runtime initialization normalizes empty maps; compare semantic JSON values.
	var x, y any
	testutil.AssertNoError(t, json.Unmarshal(left, &x), "compare")
	testutil.AssertNoError(t, json.Unmarshal(right, &y), "compare")
	// Receipts produced independently after continuing have fresh UUIDs. Their
	// source and granted cards must still match; existing receipt IDs are checked
	// by the exact save round-trip test.
	for _, value := range []any{x, y} {
		for _, raw := range value.(map[string]any)["players"].(map[string]any) {
			if receipts, ok := raw.(map[string]any)["cardReceipts"].([]any); ok {
				for _, receipt := range receipts {
					delete(receipt.(map[string]any), "ID")
				}
			}
		}
	}
	normalizeEmptyMaps(x)
	normalizeEmptyMaps(y)
	if !reflect.DeepEqual(x, y) {
		t.Fatalf("continued positions differ:\n%s\n%s", left, right)
	}
}

func TestSaveDeckReshuffle(t *testing.T) {
	g, repo, _, a := saveFixture(t)
	ctx := context.Background()
	testutil.AssertNoError(t, repo.DataStore().UpdateGame(g.ID(), func(s *datastore.GameState) {
		s.DiscardPile = append(s.DiscardPile, s.ProjectCards[1:]...)
		s.ProjectCards = s.ProjectCards[:1]
	}), "exhaust deck")
	doc, err := a.Validate(encodeSave(t, g, a))
	testutil.AssertNoError(t, err, "validate")
	restored, err := importSave(t, a, doc, "alice", "alice")
	testutil.AssertNoError(t, err, "restore")
	one, err := g.Deck().DrawProjectCards(ctx, 4)
	testutil.AssertNoError(t, err, "original reshuffle")
	two, err := restored.Deck().DrawProjectCards(ctx, 4)
	testutil.AssertNoError(t, err, "restored reshuffle")
	if !slices.Equal(one, two) || g.Deck().ShuffleCount() != restored.Deck().ShuffleCount() {
		t.Fatal("reshuffle diverged")
	}
}

func normalizeEmptyMaps(value any) {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			// New game IDs change only the procedural appearance of newly placed tiles.
			if key == "visual" {
				if visual, ok := child.(map[string]any); ok {
					delete(visual, "seed")
				}
			}
			if child == nil {
				delete(v, key)
				continue
			}
			if m, ok := child.(map[string]any); ok && len(m) == 0 {
				delete(v, key)
				continue
			}
			normalizeEmptyMaps(child)
		}
	case []any:
		for _, child := range v {
			normalizeEmptyMaps(child)
		}
	}
}

func TestSaveRestoresPassiveSubscriptionsWithoutRewards(t *testing.T) {
	g, repo, _, a := saveFixture(t)
	ctx := context.Background()
	card, err := testutil.GetCardDB().GetByID(testutil.CardID("Arctic Algae"))
	testutil.AssertNoError(t, err, "card")
	testutil.AssertNoError(t, repo.DataStore().UpdateGame(g.ID(), func(s *datastore.GameState) {
		s.ProjectCards = slices.DeleteFunc(s.ProjectCards, func(id string) bool { return id == card.ID })
		for _, phase := range s.SelectStartingCardsPhases {
			phase.AvailableCards = slices.DeleteFunc(phase.AvailableCards, func(id string) bool { return id == card.ID })
		}
		s.Players["alice"].PlayedCardIDs = append(s.Players["alice"].PlayedCardIDs, card.ID)
	}), "played card")
	p, _ := g.GetPlayer("alice")
	effect := shared.CardEffect{CardID: card.ID, CardName: card.Name, BehaviorIndex: 1, Behavior: card.Behaviors[1]}
	p.Effects().AddEffect(effect)
	baseaction.SubscribePassiveEffectToEvents(ctx, g, p, effect, testutil.TestLogger(), testutil.GetCardDB())
	doc, err := a.Validate(encodeSave(t, g, a))
	testutil.AssertNoError(t, err, "validate effects")
	restored, err := importSave(t, a, doc, "alice", "alice")
	testutil.AssertNoError(t, err, "restore effects")
	restoredPlayer, _ := restored.GetPlayer("alice")
	testutil.AssertEqual(t, p.Resources().Get().Plants, restoredPlayer.Resources().Get().Plants, "hydrate grants no reward")
	for _, match := range []*game.Game{g, restored} {
		events.Publish(match.EventBus(), events.TilePlacedEvent{GameID: match.ID(), PlayerID: "bob", TileType: string(shared.ResourceOceanTile)})
	}
	testutil.AssertEqual(t, 2, restoredPlayer.Resources().Get().Plants, "restored subscription fires exactly once")
	assertSamePosition(t, g, restored)
}

func TestSaveRestoresForcedFirstActionExecutor(t *testing.T) {
	g, repo, _, a := saveFixture(t)
	ctx := context.Background()
	tharsis := testutil.CardID("Tharsis Republic")
	helion := testutil.CardID("Helion")
	testutil.AssertNoError(t, repo.DataStore().UpdateGame(g.ID(), func(s *datastore.GameState) {
		s.CurrentPhase = shared.GamePhaseStartingSelection
		s.CurrentTurnPlayerID = "alice"
		s.CurrentTurnActions = 2
		s.Players["alice"].CorporationID = tharsis
		s.Players["bob"].CorporationID = helion
		s.Corporations = slices.DeleteFunc(s.Corporations, func(id string) bool { return id == tharsis || id == helion })
		s.SelectCorporationPhases = nil
		s.SelectStartingCardsPhases = nil
	}), "queued first action position")
	corp, err := testutil.GetCardDB().GetByID(tharsis)
	testutil.AssertNoError(t, err, "corp")
	processor := gamecards.NewCorporationProcessor(testutil.GetCardDB(), testutil.CreateTestAwardRegistry(), testutil.TestLogger())
	testutil.AssertNoError(t, processor.SetupForcedFirstAction(ctx, corp, g, "alice"), "queue first action")
	testutil.AssertNoError(t, g.UpdatePhase(ctx, shared.GamePhaseAction), "action phase")
	doc, err := a.Validate(encodeSave(t, g, a))
	testutil.AssertNoError(t, err, "validate first action")
	restored, err := importSave(t, a, doc, "alice", "alice")
	testutil.AssertNoError(t, err, "restore first action")
	if restored.GetPendingTileSelection("alice") != nil {
		t.Fatal("hydration executed first action")
	}
	for _, match := range []*game.Game{g, restored} {
		testutil.AssertNoError(t, match.ExecuteFirstActionIfNeeded(ctx, "alice"), "execute restored first action")
	}
	if restored.GetPendingTileSelection("alice") == nil {
		t.Fatal("missing first-action city selection")
	}
	assertSamePosition(t, g, restored)
	_, err = a.Validate(encodeSave(t, restored, a))
	testutil.AssertNoError(t, err, "save resolving first action")
	selectTile := tileaction.NewSelectTileAction(repo, testutil.GetCardDB(), nil, testutil.TestLogger())
	hex := restored.GetPendingTileSelection("alice").AvailableHexes[0]
	for _, match := range []*game.Game{g, restored} {
		_, err := selectTile.Execute(ctx, match.ID(), "alice", hex)
		testutil.AssertNoError(t, err, "resolve city")
	}
	assertSamePosition(t, g, restored)
	_, err = a.Validate(encodeSave(t, restored, a))
	testutil.AssertNoError(t, err, "save occupied board")
}

type saveTestBots struct {
	prepared, started []string
	fail              bool
}

func (b *saveTestBots) PrepareBot(_, id string) { b.prepared = append(b.prepared, id) }
func (b *saveTestBots) StartBot(_, id string) error {
	if b.fail {
		return fmt.Errorf("test startup failure")
	}
	b.started = append(b.started, id)
	return nil
}
func (b *saveTestBots) StopBot(_, id string) {
	b.started = slices.DeleteFunc(b.started, func(s string) bool { return s == id })
}

func TestSaveBotsRequireFreshCredentialsAndReadiness(t *testing.T) {
	g, repo, logs, a := saveFixture(t)
	ctx := context.Background()
	testutil.AssertNoError(t, repo.DataStore().UpdatePlayer(g.ID(), "bob", func(p *datastore.PlayerState) { p.PlayerType = "bot"; p.BotPersona = "saved-persona" }), "bot")
	doc, err := a.Validate(encodeSave(t, g, a))
	testutil.AssertNoError(t, err, "validate bot")
	bots := &saveTestBots{}
	catalog := save.Catalog{StandardProjects: standardproject.NewInMemoryStandardProjectRegistry(nil), Cards: testutil.CreateTestModeCatalog(), Maps: testutil.CreateTestMapRegistry(), Colonies: colony.NewInMemoryColonyRegistry(nil), Projects: projectfunding.NewInMemoryProjectFundingRegistry(nil), Awards: testutil.CreateTestAwardRegistry(), Milestones: testutil.CreateTestMilestoneRegistry()}
	a, err = gameaction.NewSaveGameAction(repo, logs, catalog, "test", bots, testutil.TestLogger())
	testutil.AssertNoError(t, err, "bot action")
	restored, err := importSave(t, a, doc, "alice", "alice")
	testutil.AssertNoError(t, err, "restore bot")
	testutil.AssertNoError(t, a.ClaimSeat(ctx, restored.ID(), "alice", "alice", "alice"), "connect host")
	bot, _ := restored.GetPlayer("bob")
	testutil.AssertEqual(t, "saved-persona", bot.BotPersona(), "persona preserved")
	if len(bots.started) != 0 || len(bots.prepared) != 0 {
		t.Fatal("bot started without credentials")
	}
	if err := a.Resume(ctx, restored.ID(), "alice"); err == nil {
		t.Fatal("resumed with unavailable bot")
	}
	testutil.AssertNoError(t, a.SetBotToken(ctx, restored.ID(), "alice", "fresh-secret"), "new credential")
	testutil.AssertEqual(t, 1, len(bots.prepared), "bot prepared")
	bot.SetBotStatus(player.BotStatusReady)
	bots.fail = true
	if err := a.Resume(ctx, restored.ID(), "alice"); err == nil || restored.ResumeLobby() == nil {
		t.Fatal("failed startup released pause gate")
	}
	bots.fail = false
	testutil.AssertNoError(t, a.Resume(ctx, restored.ID(), "alice"), "resume bot")
	if bytes.Contains(encodeSave(t, restored, a), []byte("fresh-secret")) {
		t.Fatal("fresh credential leaked")
	}
}

func TestSaveHTTPAndPausedMessageGate(t *testing.T) {
	g, repo, logs, a := saveFixture(t)
	hub := core.NewHub()
	broadcaster := wsdelivery.NewBroadcaster(repo, logs, hub, testutil.GetCardDB(), nil, nil, nil, testutil.CreateTestAwardRegistry(), testutil.CreateTestMilestoneRegistry(), nil)
	wsdelivery.RegisterSaveHandlers(hub, broadcaster, repo, a)
	hub.RegisterHandler(dto.MessageTypePlayerConnect, gamehandler.NewJoinGameHandler(gameaction.NewJoinGameAction(repo, testutil.GetCardDB(), testutil.TestLogger()), broadcaster))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)
	handler := httpdelivery.NewSaveHandler(a, hub)
	req := httptest.NewRequest(http.MethodGet, "/save?playerId=alice", nil)
	req = mux.SetURLVars(req, map[string]string{"gameId": g.ID()})
	out := httptest.NewRecorder()
	handler.Export(out, req)
	testutil.AssertEqual(t, http.StatusOK, out.Code, "export HTTP")
	doc, err := a.Validate(out.Body.Bytes())
	testutil.AssertNoError(t, err, "export validates")
	for _, entry := range doc.History {
		if len(entry.State.PlayerOrder) > 0 && entry.VPBreakdowns == nil {
			t.Fatal("save missed historical VP enrichment")
		}
	}
	denied := httptest.NewRecorder()
	handler.Export(denied, mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/save?playerId=bob", nil), map[string]string{"gameId": g.ID()}))
	if denied.Code == http.StatusOK {
		t.Fatal("non-host export allowed")
	}
	imported := httptest.NewRecorder()
	handler.Import(imported, httptest.NewRequest(http.MethodPost, "/import?seatId=alice&playerName=Alice", bytes.NewReader(out.Body.Bytes())))
	testutil.AssertEqual(t, http.StatusCreated, imported.Code, "import HTTP: "+imported.Body.String())
	var response dto.ImportGameSaveResponse
	testutil.AssertNoError(t, json.Unmarshal(imported.Body.Bytes(), &response), "import response")
	server := httptest.NewServer(http.HandlerFunc(core.NewHandler(hub).ServeWS))
	defer server.Close()
	dial := func() *websocket.Conn {
		t.Helper()
		c, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
		testutil.AssertNoError(t, err, "connect")
		testutil.AssertNoError(t, response.Body.Close(), "close handshake body")
		t.Cleanup(func() { _ = c.Close() })
		return c
	}
	read := func(c *websocket.Conn) dto.WebSocketMessage {
		t.Helper()
		testutil.AssertNoError(t, c.SetReadDeadline(time.Now().Add(20*time.Second)), "read deadline")
		var m dto.WebSocketMessage
		testutil.AssertNoError(t, c.ReadJSON(&m), "read response")
		data, err := json.Marshal(m.Payload)
		testutil.AssertNoError(t, err, "encode payload")
		switch m.Type {
		case dto.MessageTypeGameUpdated:
			var payload dto.GameUpdatedPayload
			testutil.AssertNoError(t, json.Unmarshal(data, &payload), "game payload")
			m.Payload = payload
		case dto.MessageTypeError:
			var payload dto.ErrorPayload
			testutil.AssertNoError(t, json.Unmarshal(data, &payload), "error payload")
			m.Payload = payload
		}
		return m
	}
	flush := func(c *websocket.Conn) {
		t.Helper()
		const barrier dto.MessageType = "save-test-barrier"
		testutil.AssertNoError(t, c.WriteJSON(dto.WebSocketMessage{Type: barrier}), "send barrier")
		for {
			m := read(c)
			if m.Type == dto.MessageTypeError && m.Payload.(dto.ErrorPayload).RequestType == barrier {
				return
			}
		}
	}
	send := func(c *websocket.Conn, kind dto.MessageType, payload map[string]any) dto.WebSocketMessage {
		t.Helper()
		flush(c)
		testutil.AssertNoError(t, c.WriteJSON(dto.WebSocketMessage{Type: kind, Payload: payload}), "send request")
		return read(c)
	}
	host, watcher := dial(), dial()
	send(host, dto.MessageTypePlayerConnect, map[string]any{"gameId": response.GameID, "playerId": "alice", "playerName": "Alice"})

	for _, kind := range []dto.MessageType{dto.MessageTypeActionStartGame, dto.MessageTypeKickPlayer, dto.MessageTypeUpdateGameSettings, dto.MessageTypePlayerTakeover, dto.MessageTypeEndGame, dto.MessageTypeAdminCommand} {
		m := send(host, kind, map[string]any{})
		if m.Type != dto.MessageTypeError || !strings.Contains(fmt.Sprint(m.Payload), "paused") {
			t.Fatalf("%s bypassed pause gate: %#v", kind, m)
		}
	}
	m := send(watcher, dto.MessageTypeUpdateGameSettings, map[string]any{"gameId": response.GameID})
	if m.Type != dto.MessageTypeError || !strings.Contains(fmt.Sprint(m.Payload), "paused") {
		t.Fatal("payload game ID bypassed pause gate")
	}
	m = send(watcher, dto.MessageTypeWatchResumeGame, map[string]any{"gameId": response.GameID})
	if m.Type != dto.MessageTypeGameUpdated {
		t.Fatalf("watch failed: %#v", m)
	}
	view := m.Payload.(dto.GameUpdatedPayload).Game
	if view.CurrentPlayer.ID != "" || len(view.Board.Tiles) > 0 {
		t.Fatal("resume watcher received private gameplay")
	}
	m = send(watcher, dto.MessageTypeClaimResumeSeat, map[string]any{"gameId": response.GameID, "seatId": "bob", "playerName": "Replacement"})
	if m.Type != dto.MessageTypeGameUpdated {
		t.Fatalf("claim failed: %#v", m)
	}
	competitor := dial()
	m = send(competitor, dto.MessageTypeClaimResumeSeat, map[string]any{"gameId": response.GameID, "seatId": "bob", "playerName": "Late joiner"})
	if m.Type != dto.MessageTypeError || m.Payload.(dto.ErrorPayload).Code != "seat_taken" {
		t.Fatalf("missing seat conflict response: %#v", m)
	}
	m = send(watcher, dto.MessageTypeClaimResumeSeat, map[string]any{"gameId": response.GameID, "seatId": "bob", "playerName": "Rename attempt"})
	if m.Type != dto.MessageTypeError || m.Payload.(dto.ErrorPayload).Code != "invalid_request" {
		t.Fatalf("rename command accepted: %#v", m)
	}
	secondTab := dial()
	send(secondTab, dto.MessageTypePlayerConnect, map[string]any{"gameId": response.GameID, "playerId": "bob", "playerName": "Replacement"})
	flush(watcher)
	flush(secondTab)
	m = send(host, dto.MessageTypeReleaseResumeSeat, map[string]any{"gameId": response.GameID, "seatId": "bob"})
	if m.Type != dto.MessageTypeGameUpdated {
		t.Fatalf("release failed: %#v", m)
	}
	released := read(watcher).Payload.(dto.GameUpdatedPayload).Game
	testutil.AssertEqual(t, "", released.ViewingPlayerID, "released socket loses its seat")
	released = read(secondTab).Payload.(dto.GameUpdatedPayload).Game
	testutil.AssertEqual(t, "", released.ViewingPlayerID, "all released sockets lose their seat")
	m = send(competitor, dto.MessageTypeClaimResumeSeat, map[string]any{"gameId": response.GameID, "seatId": "bob", "playerName": "Recovered"})
	if m.Type != dto.MessageTypeGameUpdated {
		t.Fatalf("recovery failed: %#v", m)
	}
	view = m.Payload.(dto.GameUpdatedPayload).Game
	testutil.AssertEqual(t, "bob", view.ViewingPlayerID, "successful claim confirms own identity")
	testutil.AssertEqual(t, "alice", view.HostPlayerID, "recovery leaves host unchanged")
}

func FuzzSaveDecode(f *testing.F) {
	f.Add([]byte(`{"formatVersion":1,"state":{"players":{"a":{"pendingBehaviorResolutions":[null]}}}}`))
	f.Add([]byte(`{"state":{"players":{"a":{"pendingEffectSelection":{"Outputs":[{"type":"plant","amount":2,"target":"self-player"}]}}}}}`))
	f.Add([]byte(`null`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		doc, err := save.Decode(data)
		if err == nil {
			_, err = save.Encode(doc)
			if err != nil {
				t.Fatal(err)
			}
		}
	})
}

func TestSaveContinuesProductionCardSelection(t *testing.T) {
	g, repo, _, a := saveFixture(t)
	ctx := context.Background()
	tharsis := testutil.CardID("Tharsis Republic")
	helion := testutil.CardID("Helion")
	testutil.AssertNoError(t, repo.DataStore().UpdateGame(g.ID(), func(s *datastore.GameState) {
		s.CurrentPhase = shared.GamePhaseProductionAndCardDraw
		s.Generation = 2
		s.Players["alice"].CorporationID = tharsis
		s.Players["bob"].CorporationID = helion
		s.Corporations = slices.DeleteFunc(s.Corporations, func(id string) bool { return id == tharsis || id == helion })
		s.SelectCorporationPhases = nil
		s.SelectStartingCardsPhases = nil
		for _, id := range s.PlayerOrder {
			s.ProductionPhases[id] = &shared.ProductionPhase{AvailableCards: append([]string(nil), s.ProjectCards[:4]...)}
			s.ProjectCards = s.ProjectCards[4:]
		}
	}), "production position")
	doc, err := a.Validate(encodeSave(t, g, a))
	testutil.AssertNoError(t, err, "validate production")
	restored, err := importSave(t, a, doc, "alice", "alice")
	testutil.AssertNoError(t, err, "restore production")
	confirm := confirmation.NewConfirmProductionCardsAction(repo, testutil.GetCardDB(), nil, testutil.TestLogger())
	for _, id := range g.PlayerOrder() {
		for _, match := range []*game.Game{g, restored} {
			testutil.AssertNoError(t, confirm.Execute(ctx, match.ID(), id, nil, false, shared.Payment{}), "confirm production")
		}
		assertSamePosition(t, g, restored)
		_, err = a.Validate(encodeSave(t, restored, a))
		testutil.AssertNoError(t, err, "save after research choice")
	}
	testutil.AssertEqual(t, shared.GamePhaseAction, restored.CurrentPhase(), "continued to action")
}

func TestSaveResumeIdentityAndRecovery(t *testing.T) {
	g, repo, _, a := saveFixture(t)
	ctx := context.Background()
	doc, err := a.Validate(encodeSave(t, g, a))
	testutil.AssertNoError(t, err, "validate")
	restored, err := importSave(t, a, doc, "bob", "New owner")
	testutil.AssertNoError(t, err, "import as another saved player")
	testutil.AssertEqual(t, "bob", restored.HostPlayerID(), "importer owns the lobby")
	join := gameaction.NewJoinGameAction(repo, testutil.GetCardDB(), testutil.TestLogger())
	_, err = join.Execute(ctx, restored.ID(), "New owner", "bob")
	testutil.AssertNoError(t, err, "host reconnects")
	testutil.AssertNoError(t, a.ClaimSeat(ctx, restored.ID(), "", "alice", "Replacement"), "initial name chosen")
	before, err := json.Marshal(restored.State())
	testutil.AssertNoError(t, err, "snapshot")
	for _, request := range []struct{ current, seat, name, code string }{
		{"bob", "alice", "New owner", "invalid_request"},
		{"alice", "bob", "Replacement", "invalid_request"},
		{"bob", "bob", "Renamed host", "invalid_request"},
		{"alice", "alice", "Renamed guest", "invalid_request"},
		{"", "alice", "Intruder", "seat_taken"},
	} {
		err := a.ClaimSeat(ctx, restored.ID(), request.current, request.seat, request.name)
		testutil.AssertEqual(t, request.code, save.PublicError(err, "").Code, "rejected identity mutation")
	}
	testutil.AssertNoError(t, a.ClaimSeat(ctx, restored.ID(), "alice", "alice", "Replacement"), "duplicate claim")
	after, err := json.Marshal(restored.State())
	testutil.AssertNoError(t, err, "snapshot after")
	if !bytes.Equal(before, after) {
		t.Fatal("rejected or duplicate requests changed game state")
	}
	for _, request := range [][2]string{{"alice", "bob"}, {"bob", "bob"}} {
		if a.ReleaseSeat(ctx, restored.ID(), request[0], request[1]) == nil {
			t.Fatal("unauthorized release")
		}
	}
	guest, _ := restored.GetPlayer("alice")
	disconnect := connectionaction.NewPlayerDisconnectedAction(repo, testutil.TestLogger())
	testutil.AssertNoError(t, disconnect.Execute(ctx, restored.ID(), "alice"), "guest disconnect")
	testutil.AssertNoError(t, disconnect.Execute(ctx, restored.ID(), "bob"), "host disconnect")
	testutil.AssertEqual(t, "bob", restored.HostPlayerID(), "disconnect retains owner")
	_, err = join.Execute(ctx, restored.ID(), "New owner", "bob")
	testutil.AssertNoError(t, err, "host reconnects after disconnect")
	if !restored.ResumeLobby().Claimed["alice"] {
		t.Fatal("disconnect lost claim")
	}
	_, err = join.Execute(ctx, restored.ID(), "Ignored reconnect name", "alice")
	testutil.AssertNoError(t, err, "guest reconnect")
	testutil.AssertEqual(t, "Replacement", guest.Name(), "reconnect cannot rename")
	testutil.AssertEqual(t, "bob", restored.HostPlayerID(), "host remains importer")
	testutil.AssertNoError(t, a.ReleaseSeat(ctx, restored.ID(), "bob", "alice"), "host recovery")
	if guest.IsConnected() || restored.ResumeLobby().Claimed["alice"] {
		t.Fatal("release retained connection or claim")
	}
	if _, err = join.Execute(ctx, restored.ID(), "Replacement", "alice"); err == nil {
		t.Fatal("released session reconnected without claiming")
	}
	if a.Resume(ctx, restored.ID(), "bob") == nil {
		t.Fatal("resumed before released seat rejoined")
	}
	testutil.AssertNoError(t, a.ClaimSeat(ctx, restored.ID(), "", "alice", "Recovered"), "reclaim released seat")
	testutil.AssertEqual(t, "Recovered", guest.Name(), "new name at rejoin")
	testutil.AssertEqual(t, 2, len(restored.GetAllPlayers()), "no extra player created")
	testutil.AssertEqual(t, "bob", restored.HostPlayerID(), "recovery preserves owner")
	testutil.AssertNoError(t, a.Resume(ctx, restored.ID(), "bob"), "resume after recovery")
}

func TestSaveBrowseMetadata(t *testing.T) {
	g, repo, _, a := saveFixture(t)
	doc, err := a.Validate(encodeSave(t, g, a))
	testutil.AssertNoError(t, err, "validate")
	restored, err := importSave(t, a, doc, "bob", "Lobby owner")
	testutil.AssertNoError(t, err, "import")
	handler := httpdelivery.NewGameHandler(nil, nil, nil, nil, query.NewListGamesAction(repo, testutil.TestLogger()), nil, testutil.GetCardDB(), nil, nil)
	out := httptest.NewRecorder()
	handler.ListGames(out, httptest.NewRequest(http.MethodGet, "/games", nil))
	testutil.AssertEqual(t, http.StatusOK, out.Code, "browse response")
	var response dto.ListGamesResponse
	testutil.AssertNoError(t, json.Unmarshal(out.Body.Bytes(), &response), "decode listing")
	for _, view := range response.Games {
		if view.ID != restored.ID() {
			continue
		}
		if view.ResumeLobby == nil {
			t.Fatal("listing lost resume marker")
		}
		testutil.AssertEqual(t, "bob", view.HostPlayerID, "owner identity")
		ownerFound := false
		for _, seat := range view.ResumeLobby.Seats {
			if seat.ID == view.HostPlayerID {
				ownerFound = seat.Name == "Lobby owner" && seat.Claimed
			}
		}
		if !ownerFound {
			t.Fatal("owner name unavailable in listing")
		}
		testutil.AssertEqual(t, restored.Settings().MapID, view.Settings.MapID, "map")
		if restored.Settings().GameModeID != view.Settings.GameModeID || !reflect.DeepEqual(restored.Settings().Features, view.Settings.Features) || !reflect.DeepEqual(restored.Settings().GameModeParameters, view.Settings.GameModeParameters) {
			t.Fatal("missing game mode metadata")
		}
		if view.CurrentPlayer.ID != "" || len(view.OtherPlayers) > 0 || len(view.Board.Tiles) > 0 {
			t.Fatal("resume listing exposes gameplay")
		}
		return
	}
	t.Fatal("imported game absent from Browse")
}

func TestSaveResumeRejectsUnavailablePlayers(t *testing.T) {
	for _, kind := range []string{"bot", "exited"} {
		t.Run(kind, func(t *testing.T) {
			g, repo, _, a := saveFixture(t)
			ctx := context.Background()
			doc, err := a.Validate(encodeSave(t, g, a))
			testutil.AssertNoError(t, err, "validate")
			restored, err := importSave(t, a, doc, "alice", "Alice")
			testutil.AssertNoError(t, err, "import")
			testutil.AssertNoError(t, repo.DataStore().UpdatePlayer(restored.ID(), "bob", func(p *datastore.PlayerState) {
				if kind == "bot" {
					p.PlayerType = "bot"
				} else {
					p.HasExited = true
				}
			}), "unavailable player")
			if a.ClaimSeat(ctx, restored.ID(), "", "bob", "Replacement") == nil {
				t.Fatal("unavailable seat claimed")
			}
			if a.ReleaseSeat(ctx, restored.ID(), "alice", "bob") == nil {
				t.Fatal("unavailable player released")
			}
			testutil.AssertEqual(t, "alice", restored.HostPlayerID(), "owner unchanged")
		})
	}
}

func TestSavePendingCardPlayRoundTrip(t *testing.T) {
	g, repo, _, a := saveFixture(t)
	settings := g.Settings()
	settings.Features = append(settings.Features, "prelude")
	g.UpdateSettings(context.Background(), settings)
	// Move the fixture past starting selection, before introducing the Prelude continuation.
	testutil.AssertNoError(t, repo.DataStore().UpdateGame(g.ID(), func(s *datastore.GameState) {
		s.CurrentPhase = shared.GamePhaseAction
		s.CurrentTurnPlayerID = "alice"
		s.CurrentTurnActions = 2
		for id, phase := range s.SelectCorporationPhases {
			s.Players[id].CorporationID = phase.AvailableCorporations[0]
		}
		s.SelectCorporationPhases = nil
		s.SelectStartingCardsPhases = nil
	}), "complete starting choices")
	p, _ := g.GetPlayer("alice")
	id := testutil.CardID("Excentric Sponsor")

	p.PlayedCards().AddCard(id, "Excentric Sponsor", "prelude", nil)
	p.Selection().SetPendingCardPlaySelection(&shared.PendingCardPlaySelection{Source: "Excentric Sponsor", SourceCardID: id, CostReduction: 25})
	data := encodeSave(t, g, a)
	doc, err := a.Validate(data)
	testutil.AssertNoError(t, err, "validate pending card play")
	restored, err := importSave(t, a, doc, "alice", "alice")
	testutil.AssertNoError(t, err, "restore pending card play")
	loaded, _ := restored.GetPlayer("alice")
	testutil.AssertTrue(t, reflect.DeepEqual(p.Selection().GetPendingCardPlaySelection(), loaded.Selection().GetPendingCardPlaySelection()), "pending discount and source retained")
	doc.State.Players["alice"].PendingCardPlaySelection.CostReduction = 100
	invalid, err := save.Encode(doc)
	testutil.AssertNoError(t, err, "encode altered save")
	_, err = a.Validate(invalid)
	testutil.AssertTrue(t, err != nil, "forged discount rejected")
}
