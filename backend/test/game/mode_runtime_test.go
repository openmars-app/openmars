package game_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"openmars/internal/action"
	cardaction "openmars/internal/action/card"
	"openmars/internal/action/confirmation"
	"openmars/internal/action/connection"
	"openmars/internal/action/turn_management"
	"openmars/internal/delivery/dto"
	"openmars/internal/events"
	"openmars/internal/game"
	"openmars/internal/game/board"
	"openmars/internal/game/cards"
	"openmars/internal/game/datastore"
	"openmars/internal/game/mode"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func modeFixture(t *testing.T) (*game.Game, *player.Player) {
	g, p, _ := modeActionFixture(t)
	return g, p
}
func modeActionFixture(t *testing.T) (*game.Game, *player.Player, game.GameRepository) {
	t.Helper()
	ds, err := datastore.NewDataStore()
	if err != nil {
		t.Fatal(err)
	}
	g := game.NewGame(ds, t.Name(), "p", shared.GameSettings{MaxPlayers: 2, GameModeID: "classic"}, board.GenerateMarsBoard(false))
	repo := game.NewMemDBGameRepository(ds, datastore.NewRuntimeManager())
	if err := repo.Create(context.Background(), g); err != nil {
		t.Fatal(err)
	}
	p, err := g.AddNewPlayer(context.Background(), "p", "Player")
	if err != nil {
		t.Fatal(err)
	}
	zero, one := 0, 1
	def := shared.ModeRuntimeDefinition{State: map[string]int{"contact": 1, "withdrawn": 0}, Deadlines: []shared.StateDeadline{{State: "deadline", Parameter: "last", ActiveState: "contact", OccurredState: "withdrawn", ActiveValue: 1, InactiveValue: 0, MaxParameterAdjustment: 2, MinGenerationOffset: 1, PerPlayerGenerationLimit: 1}}, PresentationRules: []shared.StatePresentationRule{{State: "contact", To: &zero, Title: "Support ended"}, {State: "contact", To: &one, Title: "Support restored"}}}
	if err := g.ConfigureModeRuntime(def, map[string]int{"last": 6}); err != nil {
		t.Fatal(err)
	}
	return g, p, repo
}
func mustGeneration(t *testing.T, g *game.Game, n int) {
	t.Helper()
	if err := g.SetGeneration(context.Background(), n); err != nil {
		t.Fatal(err)
	}
}
func pulse() *shared.StateCondition {
	return &shared.StateCondition{ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceScheduleStateWindow, Target: "game", Amount: 1}, State: "contact", Value: 1, Start: &shared.WindowBoundary{Boundary: "production-start"}, End: &shared.WindowBoundary{Boundary: "production-end"}}
}
func fullWindow() *shared.StateCondition {
	o := pulse()
	o.Start = &shared.WindowBoundary{GenerationOffset: 1, Boundary: "research-start"}
	o.End = &shared.WindowBoundary{GenerationOffset: 1, Boundary: "production-end"}
	return o
}
func addSupply(p *player.Player, id string, d shared.DependencyGroup, rt shared.ResourceType, amount int) {
	p.Resources().AddDependentProduction(shared.ProductionContribution{CardID: id, BehaviorID: "supply", Dependency: d, Resource: rt, Amount: amount})
}

func TestModeDeadlineBoundsAndActorCaps(t *testing.T) {
	g, p := modeFixture(t)
	ctx := context.Background()
	if err := g.ChangeModeState(ctx, p.ID(), "deadline", -1); err != nil {
		t.Fatal(err)
	}
	if err := g.ChangeModeState(ctx, p.ID(), "deadline", 1); err == nil {
		t.Fatal("same player changed deadline twice")
	}
	if err := g.ChangeModeState(ctx, "other", "deadline", -1); err != nil {
		t.Fatal(err)
	}
	if err := g.ChangeModeState(ctx, "third", "deadline", -1); err == nil {
		t.Fatal("deadline exceeded initial bounds")
	}
	mustGeneration(t, g, 4)
	if err := g.ChangeModeState(ctx, p.ID(), "deadline", -1); err == nil {
		t.Fatal("deadline skipped notice period")
	}
	g.ResolveModeBoundary(ctx, "production-end", false)
	if err := g.ChangeModeState(ctx, p.ID(), "deadline", 1); err == nil {
		t.Fatal("historical deadline moved")
	}
}

func TestModeWithdrawalRestoresSignedSourcesWithoutRepeatingHistory(t *testing.T) {
	g, p := modeFixture(t)
	ctx := context.Background()
	d := shared.DependencyGroup{ID: "supply", State: "contact", Equals: 1, Importable: true}
	addSupply(p, "ordinary", d, shared.ResourceCreditProduction, 3)
	addSupply(p, "ordinary", d, shared.ResourceEnergyProduction, 1)
	addSupply(p, "ordinary", d, shared.ResourceCreditProduction, -1)
	lease := d
	lease.RetireOnState = &shared.StateRequirement{State: "withdrawn", Equals: 1}
	addSupply(p, "lease", lease, shared.ResourceSteelProduction, 2)
	withdrawals := 0
	events.Subscribe(g.EventBus(), func(e events.ModeStateChangedEvent) {
		if e.State == "withdrawn" && e.New == 1 {
			withdrawals++
		}
	})
	if got := p.Resources().Production(); got.Credits != 2 || got.Energy != 1 || got.Steel != 2 {
		t.Fatalf("initial source totals: %+v", got)
	}
	mustGeneration(t, g, 6)
	g.ResolveModeBoundary(ctx, "production-end", false)
	if got := p.Resources().Production(); got.Credits != 0 || got.Energy != 0 || got.Steel != 0 {
		t.Fatalf("supply survived withdrawal: %+v", got)
	}
	mustGeneration(t, g, 7)
	if err := g.ScheduleStateWindow(pulse(), "pulse"); err != nil {
		t.Fatal(err)
	}
	if err := g.ScheduleStateWindow(pulse(), "duplicate"); err == nil {
		t.Fatal("duplicate pulse allowed")
	}
	if err := g.ScheduleStateWindow(fullWindow(), "full"); err != nil {
		t.Fatal(err)
	}
	g.ResolveModeBoundary(ctx, "production-start", false)
	if got := p.Resources().Production(); got.Credits != 2 || got.Energy != 1 || got.Steel != 0 {
		t.Fatalf("signed supply restoration/lease: %+v", got)
	}
	g.ResolveModeBoundary(ctx, "production-end", false)
	mustGeneration(t, g, 8)
	g.ResolveModeBoundary(ctx, "research-start", false)
	if got := p.Resources().Production().Credits; got != 2 {
		t.Fatalf("full window did not start before research: %d", got)
	}
	g.ResolveModeBoundary(ctx, "production-end", false)
	if withdrawals != 1 {
		t.Fatalf("historical withdrawal repeated %d times", withdrawals)
	}
	if got := len(g.ModeRuntime().Events); got != 5 {
		t.Fatalf("want five presentation transitions, got %d", got)
	}
}

func TestModeImportRetainsWholeBundleAndCannotFundReductionBeforeImport(t *testing.T) {
	g, p := modeFixture(t)
	ctx := context.Background()
	d := shared.DependencyGroup{ID: "supply", State: "contact", Equals: 1, Importable: true}
	addSupply(p, "technology", d, shared.ResourceEnergyProduction, 2)
	addSupply(p, "technology", d, shared.ResourceCreditProduction, -1)
	applier := cards.NewBehaviorApplier(p, g, "spend", slog.Default())
	if err := applier.ValidateInputs([]shared.BehaviorCondition{shared.NewProductionCondition(shared.ResourceEnergyProduction, 1, "self-player")}); err == nil {
		t.Fatal("temporary production paid permanent reduction")
	}
	if err := p.Resources().SetDependencyStatus("technology", "supply", true); err != nil {
		t.Fatal(err)
	}
	if got := p.Resources().Production(); got.Energy != 2 || got.Credits != -1 {
		t.Fatalf("import doubled production or dropped upkeep: %+v", got)
	}
	if err := applier.ValidateInputs([]shared.BehaviorCondition{shared.NewProductionCondition(shared.ResourceEnergyProduction, 1, "self-player")}); err != nil {
		t.Fatal(err)
	}
	mustGeneration(t, g, 6)
	g.ResolveModeBoundary(ctx, "production-end", false)
	if got := p.Resources().Production(); got.Energy != 2 || got.Credits != -1 {
		t.Fatalf("import ceased: %+v", got)
	}
	if err := p.Resources().SetDependencyStatus("technology", "supply", true); err == nil {
		t.Fatal("duplicate import allowed")
	}
}

func TestModeFinalProductionUsesPulseAndCancelsFutureReservation(t *testing.T) {
	g, p := modeFixture(t)
	ctx := context.Background()
	d := shared.DependencyGroup{ID: "supply", State: "contact", Equals: 1}
	addSupply(p, "source", d, shared.ResourceSteelProduction, 2)
	mustGeneration(t, g, 6)
	g.ResolveModeBoundary(ctx, "production-end", false)
	mustGeneration(t, g, 7)
	if err := g.ScheduleStateWindow(pulse(), "pulse"); err != nil {
		t.Fatal(err)
	}
	if err := g.ScheduleStateWindow(fullWindow(), "full"); err != nil {
		t.Fatal(err)
	}
	if err := turn_management.ExecuteFinalProductionPhase(ctx, g, []*player.Player{p}, slog.Default()); err != nil {
		t.Fatal(err)
	}
	if p.Resources().Get().Steel != 2 {
		t.Fatal("final production ignored pulse")
	}
	r := g.ModeRuntime()
	if r.Windows[0].Status != "completed" || r.Windows[1].Status != "cancelled" {
		t.Fatalf("bad final windows: %+v", r.Windows)
	}
	if p.Resources().Production().Steel != 0 {
		t.Fatal("pulse stayed active after final production")
	}
}

func TestModeBehaviorCapsPersistAcrossBranches(t *testing.T) {
	g, p := modeFixture(t)
	ctx := context.Background()
	b := shared.CardBehavior{ID: "first", UsageGroup: "shared", MaxUsesPerGeneration: 1, MaxUsesPerGame: 1, Outputs: []shared.BehaviorCondition{shared.NewBasicResourceCondition(shared.ResourceCredit, 2, "self-player")}}
	apply := func(b shared.CardBehavior) error {
		return cards.NewBehaviorApplier(p, g, "test", slog.Default()).WithSourceCardID("source").WithBehavior(b).ApplyOutputs(ctx, b.Outputs)
	}
	if err := apply(b); err != nil {
		t.Fatal(err)
	}
	other := b
	other.ID = "second"
	if err := apply(other); err == nil {
		t.Fatal("shared branch cap bypassed")
	}
	mustGeneration(t, g, 2)
	if err := apply(b); err == nil {
		t.Fatal("game cap reset")
	}
	if err := apply(other); err != nil {
		t.Fatal(err)
	}
}

func TestModeProductionBehaviorRunsForAllPlayersAndKeepsSnapshot(t *testing.T) {
	g, p := modeFixture(t)
	ctx := context.Background()
	r := g.ModeRuntime()
	r.Definition.Behaviors = []shared.CardBehavior{{ID: "stipend", Scope: "each-player", StateRequirements: []shared.StateRequirement{{State: "contact", Equals: 1}}, Triggers: []shared.Trigger{{Type: "auto", Condition: &shared.ResourceTriggerCondition{Type: "production"}}}, Outputs: []shared.BehaviorCondition{shared.NewBasicResourceCondition(shared.ResourceCredit, 2, "self-player")}}}
	if err := g.ConfigureModeRuntime(r.Definition, map[string]int{"last": 6}); err != nil {
		t.Fatal(err)
	}
	action.SetupModeBehaviors(ctx, g, cards.NewInMemoryCardRegistry(nil))
	g.InitDeck(nil, nil, nil)
	d := shared.DependencyGroup{ID: "supply", State: "contact", Equals: 1}
	addSupply(p, "supply", d, shared.ResourceSteelProduction, 2)
	mustGeneration(t, g, 6)
	if err := turn_management.ExecuteProductionPhase(ctx, g, []*player.Player{p}, slog.Default()); err != nil {
		t.Fatal(err)
	}
	if p.Resources().Get().Credits != 22 {
		t.Fatalf("stipend missing, got %d", p.Resources().Get().Credits)
	}
	if phase := g.GetProductionPhase(p.ID()); phase.BeforeResources.Credits != 0 || phase.AfterResources.Credits != 22 || phase.CreditsIncome != 22 {
		t.Fatalf("production snapshot omitted stipend: %+v", phase)
	}
	if phase := g.GetProductionPhase(p.ID()); phase.ProductionUsed.Steel != 2 || p.Resources().Production().Steel != 0 {
		t.Fatal("production snapshot lost at cutoff")
	}
}

func TestModeBehaviorJSONPreservesOperationsAndCaps(t *testing.T) {
	raw := []byte(`{"id":"retire","usageGroup":"operation","maxUsesPerGame":1,"maxUsesPerGeneration":1,"resourceRequirements":[{"type":"energy-production","min":1,"permanent":true}],"outputs":[{"type":"retire-dependency","amount":1,"target":"any-card","dependency":"supply","minimumProduction":{"type":"credit-production","amount":2}}]}`)
	var b shared.CardBehavior
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatal(err)
	}
	cp := b.DeepCopy()
	out, err := json.Marshal(cp)
	if err != nil {
		t.Fatal(err)
	}
	var round shared.CardBehavior
	if err := json.Unmarshal(out, &round); err != nil {
		t.Fatal(err)
	}
	if round.MaxUsesPerGame != 1 || round.UsageGroup != "operation" || round.Outputs[0].(*shared.StateCondition).MinimumProduction.Amount != 2 {
		t.Fatalf("roundtrip dropped runtime semantics: %s", out)
	}
}

func TestModeCopyKeepsAuthoredDependencyEvenAfterImport(t *testing.T) {
	g, p := modeFixture(t)
	ctx := context.Background()
	d := shared.DependencyGroup{ID: "supply", State: "contact", Equals: 1, Importable: true}
	behavior := shared.CardBehavior{ID: "income", Dependency: "supply", ProductionBox: "evaluate", Triggers: []shared.Trigger{{Type: "auto"}}, Outputs: []shared.BehaviorCondition{shared.NewProductionCondition(shared.ResourceCreditProduction, 3, "self-player")}}
	source := cards.Card{ID: "technology", Name: "Technology", Type: cards.CardTypeAutomated, DependencyGroups: []shared.DependencyGroup{d}, Behaviors: []shared.CardBehavior{behavior}}
	registry := cards.NewInMemoryCardRegistry([]cards.Card{source})
	p.PlayedCards().AddCard(source.ID, source.Name, string(source.Type), nil)
	if err := cards.NewBehaviorApplier(p, g, source.Name, slog.Default()).WithCardRegistry(registry).WithSourceCardID(source.ID).WithBehavior(behavior).ApplyOutputs(ctx, behavior.Outputs); err != nil {
		t.Fatal(err)
	}
	if err := p.Resources().SetDependencyStatus(source.ID, d.ID, true); err != nil {
		t.Fatal(err)
	}
	copyOutput := &shared.CopyCondition{ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceCopy, Amount: 1, Target: "self-player"}, Scope: "production-box", Zone: "played"}
	options, _, err := cards.EffectSelectionOptions([]shared.BehaviorCondition{copyOutput}, p, g, registry, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 1 {
		t.Fatalf("want one copy source, got %d", len(options))
	}
	if err := cards.NewBehaviorApplier(p, g, "Copy", slog.Default()).WithCardRegistry(registry).WithSourceCardID("copy").ApplyOutputs(ctx, options[0].Outputs); err != nil {
		t.Fatal(err)
	}
	if p.Resources().Production().Credits != 6 {
		t.Fatal("copy not active while connected")
	}
	mustGeneration(t, g, 6)
	g.ResolveModeBoundary(ctx, "production-end", false)
	if p.Resources().Production().Credits != 3 || p.Resources().PermanentProduction().Credits != 3 {
		t.Fatal("copy laundered temporary supply into permanent production")
	}
}

func TestModeInvalidImportDoesNotGrantEarlierOutputs(t *testing.T) {
	g, p := modeFixture(t)
	ctx := context.Background()
	d := shared.DependencyGroup{ID: "supply", State: "contact", Equals: 1, Importable: true}
	c := cards.Card{ID: "technology", Name: "Technology", Type: cards.CardTypeActive, DependencyGroups: []shared.DependencyGroup{d}}
	registry := cards.NewInMemoryCardRegistry([]cards.Card{c})
	outputs := []shared.BehaviorCondition{shared.NewBasicResourceCondition(shared.ResourceSteel, 1, "self-player"), &shared.StateCondition{ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceRemoveDependency, Amount: 1, Target: "any-card"}, Dependency: "supply"}}
	applier := cards.NewBehaviorApplier(p, g, "Import", slog.Default()).WithCardRegistry(registry).WithTargetCardIDs([]string{c.ID})
	if err := applier.ApplyOutputs(ctx, outputs); err == nil {
		t.Fatal("imported an unowned target")
	}
	if p.Resources().Get().Steel != 0 {
		t.Fatal("invalid import granted partial rewards")
	}
	p.PlayedCards().AddCard(c.ID, c.Name, string(c.Type), nil)
	if err := applier.ApplyOutputs(ctx, outputs); err != nil {
		t.Fatal(err)
	}
	if p.Resources().Get().Steel != 1 || !p.Resources().DependencyStatus(c.ID, d.ID).Removed {
		t.Fatal("valid import failed")
	}
}

func TestModeReserveCapAndWithdrawalSettlement(t *testing.T) {
	g, p := modeFixture(t)
	ctx := context.Background()
	maxAmount := 4
	b := shared.CardBehavior{ID: "reserve", ResourceRequirements: []shared.BehaviorResourceRequirement{{Type: shared.ResourceCardResource, Target: "self-card", Max: &maxAmount}}, Outputs: []shared.BehaviorCondition{shared.NewCardStorageCondition(shared.ResourceCardResource, 2, "self-card")}}
	apply := func() error {
		return cards.NewBehaviorApplier(p, g, "Reserve", slog.Default()).WithSourceCardID("reserve").WithBehavior(b).ApplyOutputs(ctx, b.Outputs)
	}
	for range 3 {
		if err := apply(); err != nil {
			t.Fatal(err)
		}
	}
	if err := apply(); err == nil {
		t.Fatal("reserve capacity exceeded")
	}
	target := "self-card"
	per := &shared.PerCondition{ResourceType: shared.ResourceCardResource, Amount: 1, Target: &target}
	steel := shared.NewBasicResourceCondition(shared.ResourceSteel, 1, "self-player")
	steel.Per = per
	consume := shared.NewCardStorageCondition(shared.ResourceCardResource, -1, "self-card")
	consume.Per = per
	if err := cards.NewBehaviorApplier(p, g, "Settlement", slog.Default()).WithSourceCardID("reserve").ApplyOutputs(ctx, []shared.BehaviorCondition{steel, consume}); err != nil {
		t.Fatal(err)
	}
	if p.Resources().Get().Steel != 6 || p.Resources().GetCardStorage("reserve") != 0 {
		t.Fatal("reserve did not convert and consume its full stock")
	}
}

func TestModeLifecycleRebateRequiresActorDirectionAndNeverRepeats(t *testing.T) {
	g, p := modeFixture(t)
	ctx := context.Background()
	self := "self-player"
	b := shared.CardBehavior{ID: "rebate", MaxUsesPerGame: 1, Triggers: []shared.Trigger{{Type: "auto", Condition: &shared.ResourceTriggerCondition{Type: "state-changed", State: "deadline", Direction: "decrease", Target: &self}}}, Outputs: []shared.BehaviorCondition{shared.NewBasicResourceCondition(shared.ResourceCredit, 2, "self-player")}}
	action.SubscribePassiveEffectToEvents(ctx, g, p, shared.CardEffect{CardID: "prelude", CardName: "Prelude", Behavior: b}, slog.Default())
	if err := g.ChangeModeState(ctx, "other", "deadline", -1); err != nil {
		t.Fatal(err)
	}
	if p.Resources().Get().Credits != 0 {
		t.Fatal("another actor triggered rebate")
	}
	if err := g.ChangeModeState(ctx, p.ID(), "deadline", 1); err != nil {
		t.Fatal(err)
	}
	if p.Resources().Get().Credits != 0 {
		t.Fatal("wrong direction triggered rebate")
	}
	mustGeneration(t, g, 2)
	if err := g.ChangeModeState(ctx, p.ID(), "deadline", -1); err != nil {
		t.Fatal(err)
	}
	if p.Resources().Get().Credits != 2 {
		t.Fatal("eligible change did not rebate")
	}
	mustGeneration(t, g, 3)
	if err := g.ChangeModeState(ctx, p.ID(), "deadline", -1); err != nil {
		t.Fatal(err)
	}
	if p.Resources().Get().Credits != 2 {
		t.Fatal("once-per-game rebate repeated")
	}
}

func TestModePermanentOutputCostCannotSpendSupply(t *testing.T) {
	g, p := modeFixture(t)
	d := shared.DependencyGroup{ID: "supply", State: "contact", Equals: 1}
	addSupply(p, "temporary", d, shared.ResourceEnergyProduction, 2)
	output := shared.NewProductionCondition(shared.ResourceEnergyProduction, -1, "self-player")
	if err := cards.NewBehaviorApplier(p, g, "conversion", slog.Default()).ApplyOutputs(context.Background(), []shared.BehaviorCondition{output}); err == nil {
		t.Fatal("production output spent supply")
	}
	if p.Resources().PermanentProduction().Energy != 0 || p.Resources().Production().Energy != 2 {
		t.Fatal("rejected cost mutated source")
	}
}

func TestModeFinalProductionEquivalentAfterPassAndKick(t *testing.T) {
	for _, path := range []string{"pass", "kick"} {
		t.Run(path, func(t *testing.T) {
			g, p, repo := modeActionFixture(t)
			ctx := context.Background()
			q, err := g.AddNewPlayer(ctx, "q", "Other")
			if err != nil {
				t.Fatal(err)
			}
			definition := g.ModeRuntime().Definition
			definition.Behaviors = []shared.CardBehavior{{ID: "stipend", Scope: "each-player", StateRequirements: []shared.StateRequirement{{State: "contact", Equals: 1}}, Triggers: []shared.Trigger{{Type: "auto", Condition: &shared.ResourceTriggerCondition{Type: "production"}}}, Outputs: []shared.BehaviorCondition{shared.NewBasicResourceCondition(shared.ResourceCredit, 2, "self-player")}}}
			if err := g.ConfigureModeRuntime(definition, map[string]int{"last": 6}); err != nil {
				t.Fatal(err)
			}
			action.SetupModeBehaviors(ctx, g, cards.NewInMemoryCardRegistry(nil))
			addSupply(p, "source", shared.DependencyGroup{ID: "supply", State: "contact", Equals: 1}, shared.ResourceSteelProduction, 2)
			mustGeneration(t, g, 6)
			g.ResolveModeBoundary(ctx, "production-end", false)
			mustGeneration(t, g, 7)
			if err := g.ScheduleStateWindow(pulse(), "pulse"); err != nil {
				t.Fatal(err)
			}
			if err := g.ScheduleStateWindow(fullWindow(), "full"); err != nil {
				t.Fatal(err)
			}
			if err := g.GlobalParameters().SetTemperature(ctx, 8); err != nil {
				t.Fatal(err)
			}
			if err := g.GlobalParameters().SetOxygen(ctx, 14); err != nil {
				t.Fatal(err)
			}
			if err := g.GlobalParameters().SetOceans(ctx, 9); err != nil {
				t.Fatal(err)
			}
			if err := g.UpdateStatus(ctx, shared.GameStatusActive); err != nil {
				t.Fatal(err)
			}
			if err := g.UpdatePhase(ctx, shared.GamePhaseAction); err != nil {
				t.Fatal(err)
			}
			if err := g.SetTurnOrder(ctx, []string{p.ID(), q.ID()}); err != nil {
				t.Fatal(err)
			}
			if err := g.SetCurrentTurn(ctx, q.ID(), 2); err != nil {
				t.Fatal(err)
			}
			p.SetPassed(true)
			if path == "pass" {
				err = turn_management.NewSkipActionAction(repo, nil, slog.Default()).Execute(ctx, g.ID(), q.ID())
			} else {
				err = connection.NewKickPlayerAction(repo, nil, nil, slog.Default()).Execute(ctx, g.ID(), p.ID(), q.ID())
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := p.Resources().Get(); got.Credits != 22 || got.Steel != 2 {
				t.Fatalf("wrong final rewards: %+v", got)
			}
			if g.CurrentPhase() != shared.GamePhaseProductionAndCardDraw || g.Generation() != 7 {
				t.Fatalf("final production/ending changed: %s gen%d", g.CurrentPhase(), g.Generation())
			}
			phase := g.GetProductionPhase(p.ID())
			if phase == nil || len(phase.AvailableCards) != 0 || phase.ProductionUsed.Steel != 2 {
				t.Fatal("final production snapshot missing or drew research cards")
			}
			if phase.BeforeResources.Credits != 0 || phase.AfterResources.Credits != 22 || phase.CreditsIncome != 22 {
				t.Fatalf("final pulse snapshot omitted stipend: %+v", phase)
			}
			runtime := g.ModeRuntime()
			if runtime.Windows[0].Status != "completed" || runtime.Windows[1].Status != "cancelled" || runtime.Values["withdrawn"] != 1 {
				t.Fatalf("bad final lifecycle: %+v", runtime)
			}
		})
	}
}

func TestModeNextWindowStartsBeforeResearchAndWithdrawalWaitsForEveryPlayer(t *testing.T) {
	g, p := modeFixture(t)
	ctx := context.Background()
	q, err := g.AddNewPlayer(ctx, "q", "Other")
	if err != nil {
		t.Fatal(err)
	}
	players := []*player.Player{p, q}
	d := shared.DependencyGroup{ID: "supply", State: "contact", Equals: 1}
	for _, owner := range players {
		addSupply(owner, "source", d, shared.ResourceSteelProduction, 2)
	}
	g.InitDeck([]string{"a", "b", "c", "d", "e", "f", "g", "h"}, nil, nil)
	observed := false
	events.Subscribe(g.EventBus(), func(e events.ModeStateChangedEvent) {
		if e.State == "withdrawn" {
			observed = true
			for _, owner := range players {
				if owner.Resources().Get().Steel != 2 {
					t.Errorf("withdrawal preceded player %s production", owner.ID())
				}
			}
		}
	})
	mustGeneration(t, g, 6)
	if err := turn_management.ExecuteProductionPhase(ctx, g, players, slog.Default()); err != nil {
		t.Fatal(err)
	}
	if !observed {
		t.Fatal("withdrawal never resolved")
	}
	if err := g.ScheduleStateWindow(fullWindow(), "full"); err != nil {
		t.Fatal(err)
	}
	if err := turn_management.ExecuteProductionPhase(ctx, g, players, slog.Default()); err != nil {
		t.Fatal(err)
	}
	if value, _ := g.ModeStateValue("contact"); value != 1 || g.Generation() != 8 || g.CurrentPhase() != shared.GamePhaseProductionAndCardDraw {
		t.Fatal("restoration did not precede next research selections")
	}
	for _, owner := range players {
		if owner.Resources().Get().Steel != 2 || owner.Resources().Production().Steel != 2 {
			t.Fatal("window leaked into previous production or missing at research")
		}
	}
}

func authoredModeFixture(t *testing.T) (*game.Game, *player.Player, game.GameRepository, cards.CardRegistry) {
	t.Helper()
	g, p, repo := modeActionFixture(t)
	registry := testutil.CreateTestModeCatalog()
	settings, definition, err := mode.Normalize(cards.ModeDefinitions(registry), shared.GameSettings{GameModeID: "earth-withdrawal", MaxPlayers: 2, Features: []string{"prelude"}})
	if err != nil {
		t.Fatal(err)
	}
	g.UpdateSettings(context.Background(), settings)
	if err := g.ConfigureModeRuntime(definition.Runtime, settings.GameModeParameters); err != nil {
		t.Fatal(err)
	}
	if err := g.UpdateStatus(context.Background(), shared.GameStatusActive); err != nil {
		t.Fatal(err)
	}
	if err := g.UpdatePhase(context.Background(), shared.GamePhaseAction); err != nil {
		t.Fatal(err)
	}
	if err := g.SetTurnOrder(context.Background(), []string{p.ID()}); err != nil {
		t.Fatal(err)
	}
	if err := g.SetCurrentTurn(context.Background(), p.ID(), -1); err != nil {
		t.Fatal(err)
	}
	p.Resources().Set(shared.Resources{Credits: 500})
	p.SetCorporationID("EC02")
	g.InitDeck([]string{"001", "002", "003", "004", "005", "006", "007", "008"}, nil, nil)
	return g, p, repo, registry
}
func playAuthored(t *testing.T, g *game.Game, p *player.Player, repo game.GameRepository, registry cards.CardRegistry, id string, choice *int, targets []string) error {
	t.Helper()
	card, err := cards.ForGame(registry, g).GetByID(id)
	if err != nil {
		t.Fatal(err)
	}
	p.Hand().AddCard(id)
	return cardaction.NewPlayCardAction(repo, registry, nil, slog.Default()).Execute(context.Background(), g.ID(), p.ID(), id, shared.NativePayment(shared.ResourceCredit, card.Cost), choice, targets, nil, nil, nil)
}
func TestModeAuthoredImportPreservesDeliveryAndBusinessNetworkUpkeep(t *testing.T) {
	g, p, repo, registry := authoredModeFixture(t)
	ctx := context.Background()
	if err := playAuthored(t, g, p, repo, registry, "ES001", nil, nil); err != nil {
		t.Fatal(err)
	}
	if p.Resources().Get().Steel != 2 || p.Resources().Production().Steel != 2 || p.Resources().PermanentProduction().Steel != 0 {
		t.Fatal("licensed delivery/conditional production incorrect")
	}
	if err := playAuthored(t, g, p, repo, registry, "ES021", nil, []string{"ES001"}); err != nil {
		t.Fatal(err)
	}
	if p.Resources().Get().Steel != 2 || p.Resources().Production().Steel != 2 || p.Resources().PermanentProduction().Steel != 2 {
		t.Fatal("technology replayed delivery or failed import")
	}
	if err := playAuthored(t, g, p, repo, registry, "110", nil, nil); err != nil {
		t.Fatal(err)
	}
	if p.Resources().Production().Credits != -1 || p.Resources().PermanentProduction().Credits != 0 {
		t.Fatal("business network upkeep is not conditional")
	}
	if err := playAuthored(t, g, p, repo, registry, "ES024", nil, []string{"110"}); err != nil {
		t.Fatal(err)
	}
	mustGeneration(t, g, 6)
	g.ResolveModeBoundary(ctx, "production-end", false)
	if p.Resources().Production().Steel != 2 || p.Resources().Production().Credits != -1 {
		t.Fatal("import lost production/upkeep at withdrawal")
	}
	network, _ := cards.ForGame(registry, g).GetByID("110")
	if err := cards.BehaviorAvailable(g, p, "110", network.Behaviors[1], cards.ForGame(registry, g)); err != nil {
		t.Fatalf("imported network action disabled: %v", err)
	}
}
func TestModeAuthoredWarehouseStoresAndReleasesOnlyOnce(t *testing.T) {
	g, p, repo, registry := authoredModeFixture(t)
	ctx := context.Background()
	if err := playAuthored(t, g, p, repo, registry, "ES018", nil, nil); err != nil {
		t.Fatal(err)
	}
	act := cardaction.NewUseCardActionAction(repo, registry, nil, slog.Default())
	if err := act.Execute(ctx, g.ID(), p.ID(), "ES018", 1, nil, nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if p.Resources().GetCardStorage("ES018") != 2 {
		t.Fatal("warehouse did not store")
	}
	mustGeneration(t, g, 6)
	g.ResolveModeBoundary(ctx, "production-end", false)
	if p.Resources().Get().Steel != 4 || p.Resources().GetCardStorage("ES018") != 0 {
		t.Fatal("warehouse did not release")
	}
	mustGeneration(t, g, 7)
	window := pulse()
	window.State = "earth-contact"
	if err := g.ScheduleStateWindow(window, "test"); err != nil {
		t.Fatal(err)
	}
	g.ResolveModeBoundary(ctx, "production-start", false)
	g.ResolveModeBoundary(ctx, "production-end", false)
	if p.Resources().Get().Steel != 4 {
		t.Fatal("warehouse release repeated")
	}
}
func TestModeAuthoredReconnectionRejectsOverlapBeforePayment(t *testing.T) {
	g, p, repo, registry := authoredModeFixture(t)
	ctx := context.Background()
	mustGeneration(t, g, 6)
	g.ResolveModeBoundary(ctx, "production-end", false)
	mustGeneration(t, g, 7)
	p.Resources().SetProduction(shared.Production{Energy: 2})
	if err := playAuthored(t, g, p, repo, registry, "ES042", nil, nil); err != nil {
		t.Fatal(err)
	}
	before := p.Resources().Get()
	if before.Energy != 2 || before.Steel != 2 {
		t.Fatal("reconnection grants missing")
	}
	p.AddBonusTags(shared.TagScience, 2)
	if err := playAuthored(t, g, p, repo, registry, "ES041", nil, nil); err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("expected overlap rejection, got %v", err)
	}
	if p.Resources().Get() != before {
		t.Fatal("rejected reconnect charged cost or granted outputs")
	}
}

func TestModeAuthoredContingencyPaysBeforeOrAfterWithdrawal(t *testing.T) {
	for _, late := range []bool{false, true} {
		name := "early"
		if late {
			name = "late"
		}
		t.Run(name, func(t *testing.T) {
			g, p, repo, registry := authoredModeFixture(t)
			ctx := context.Background()
			if late {
				mustGeneration(t, g, 6)
				g.ResolveModeBoundary(ctx, "production-end", false)
			}
			c, err := cards.ForGame(registry, g).GetByID("ES019")
			if err != nil {
				t.Fatal(err)
			}
			state := action.CalculatePlayerCardState(c, p, g, registry)
			if len(state.Errors) > 0 {
				t.Fatalf("conditional payout blocked card play: %+v", state.Errors)
			}
			if err := playAuthored(t, g, p, repo, registry, "ES019", nil, nil); err != nil {
				t.Fatal(err)
			}
			expected := 2
			if late {
				expected = 8
			}
			if got := p.Resources().Get().Plants; got != expected {
				t.Fatalf("initial plants %d want %d", got, expected)
			}
			if !late {
				mustGeneration(t, g, 6)
				g.ResolveModeBoundary(ctx, "production-end", false)
			}
			if p.Resources().Get().Plants != 8 {
				t.Fatal("first withdrawal failed contingency")
			}
			mustGeneration(t, g, 7)
			w := pulse()
			w.State = "earth-contact"
			if err := g.ScheduleStateWindow(w, "pulse"); err != nil {
				t.Fatal(err)
			}
			g.ResolveModeBoundary(ctx, "production-start", false)
			g.ResolveModeBoundary(ctx, "production-end", false)
			if p.Resources().Get().Plants != 8 {
				t.Fatal("temporary window repeated contingency")
			}
		})
	}
}

func TestModeProductionSnapshotExcludesGenerationIncomeAndIncludesWithdrawalRewards(t *testing.T) {
	for _, final := range []bool{false, true} {
		name := "normal"
		if final {
			name = "final"
		}
		t.Run(name, func(t *testing.T) {
			g, p, repo, registry := authoredModeFixture(t)
			ctx := context.Background()
			action.SetupModeBehaviors(ctx, g, registry)
			if err := playAuthored(t, g, p, repo, registry, "ES019", nil, nil); err != nil {
				t.Fatal(err)
			}
			before := p.Resources().Get()
			mustGeneration(t, g, 6)
			produce := turn_management.ExecuteProductionPhase
			if final {
				produce = turn_management.ExecuteFinalProductionPhase
			}
			if err := produce(ctx, g, []*player.Player{p}, slog.Default()); err != nil {
				t.Fatal(err)
			}
			phase := g.GetProductionPhase(p.ID())
			if phase.BeforeResources != before || phase.AfterResources != p.Resources().Get() {
				t.Fatalf("snapshot does not span all production rewards: before=%+v phase=%+v resources=%+v", before, phase, p.Resources().Get())
			}
			if phase.CreditsIncome != 20 || phase.AfterResources.Credits-before.Credits != 20 {
				t.Fatalf("production snapshot must exclude generation-start support: %+v", phase)
			}
			if phase.BeforeResources.Plants != 2 || phase.AfterResources.Plants != 8 {
				t.Fatalf("snapshot omitted the first-withdrawal contingency release: %+v", phase)
			}
		})
	}
}

func TestModeAuthoredLeaseExpiresAndNeverReturns(t *testing.T) {
	g, p, repo, registry := authoredModeFixture(t)
	ctx := context.Background()
	if err := playAuthored(t, g, p, repo, registry, "ES016", nil, nil); err != nil {
		t.Fatal(err)
	}
	resolved := p.Resources().ResolvedProductionBox("ES016")
	if len(resolved) != 1 || resolved[0].Dependency == nil || resolved[0].Dependency.RetireOnState == nil || resolved[0].Amount != -2 {
		t.Fatal("resolved lease production lost dependency or captured upfront resources")
	}
	if p.Resources().Production().Credits != -2 || p.Resources().PermanentProduction().Credits != 0 {
		t.Fatal("lease upkeep became permanent")
	}
	if value, _ := g.ModeStateValue("withdrawal-generation"); value != 7 {
		t.Fatal("lease did not delay deadline")
	}
	mustGeneration(t, g, 7)
	g.ResolveModeBoundary(ctx, "production-end", false)
	if p.Resources().Production().Credits != 0 {
		t.Fatal("lease survived withdrawal")
	}
	mustGeneration(t, g, 8)
	w := pulse()
	w.State = "earth-contact"
	if err := g.ScheduleStateWindow(w, "pulse"); err != nil {
		t.Fatal(err)
	}
	g.ResolveModeBoundary(ctx, "production-start", false)
	if p.Resources().Production().Credits != 0 {
		t.Fatal("expired lease returned during temporary contact")
	}
}

func TestModeAuthoredRetirementValidatesDeadlineBeforeRetiringSource(t *testing.T) {
	g, p, repo, registry := authoredModeFixture(t)
	ctx := context.Background()
	if err := playAuthored(t, g, p, repo, registry, "106", nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := g.ChangeModeState(ctx, p.ID(), "withdrawal-generation", 1); err != nil {
		t.Fatal(err)
	}
	before := p.Resources().Get()
	if err := playAuthored(t, g, p, repo, registry, "ES013", nil, []string{"106"}); err == nil || !strings.Contains(err.Error(), "already changed") {
		t.Fatalf("expected successful-change cap, got %v", err)
	}
	if p.Resources().Get() != before || p.Resources().DependencyStatus("106", "earth-support").Retired {
		t.Fatal("invalid retirement paid or retired source")
	}
	mustGeneration(t, g, 2)
	p.Hand().RemoveCard("ES013")
	if err := playAuthored(t, g, p, repo, registry, "ES013", nil, []string{"106"}); err != nil {
		t.Fatal(err)
	}
	if !p.Resources().DependencyStatus("106", "earth-support").Retired || p.Resources().Production().Credits != 0 {
		t.Fatal("valid retirement kept source")
	}
}

func TestModePresentationEffectsAreOrderedIndependentSnapshots(t *testing.T) {
	g, _ := modeFixture(t)
	definition := g.ModeRuntime().Definition
	definition.PresentationRules[0].Effects = []shared.ModePresentationEffect{
		{Kind: "link-transition", Link: &shared.LinkTransition{From: "earth", To: "mars", Connected: false}},
		{Kind: "future-capability"},
	}
	if err := g.ConfigureModeRuntime(definition, map[string]int{"last": 6}); err != nil {
		t.Fatal(err)
	}
	definition.PresentationRules[0].Effects[0].Link.From = "changed"
	mustGeneration(t, g, 6)
	g.ResolveModeBoundary(context.Background(), "production-end", false)
	snapshot := g.ModeRuntime()
	if len(snapshot.Events) != 1 || len(snapshot.Events[0].Effects) != 2 {
		t.Fatalf("unexpected presentation events: %+v", snapshot.Events)
	}
	first := snapshot.Events[0]
	if first.Effects[0].Link.From != "earth" || first.Effects[0].Link.Connected || first.Effects[1].Kind != "future-capability" {
		t.Fatalf("effects changed or reordered: %+v", first.Effects)
	}
	first.Effects[0].Link.From = "mutated snapshot"
	snapshot.Definition.PresentationRules[0].Effects[0].Link.To = "mutated definition"
	again := g.ModeRuntime()
	if again.Events[0].Effects[0].Link.From != "earth" || again.Definition.PresentationRules[0].Effects[0].Link.To != "mars" {
		t.Fatal("presentation data aliases live runtime state")
	}
	g.ResolveModeBoundary(context.Background(), "production-end", false)
	if len(g.ModeRuntime().Events) != 1 || g.ModeRuntime().Events[0].ID != first.ID {
		t.Fatal("unchanged state emitted a duplicate presentation")
	}
}

func TestModeAuthoredShutdownRemovesSupportWithoutDiscardingProject(t *testing.T) {
	for _, id := range []string{"ES001", "110", "C24", "013"} {
		t.Run(id, func(t *testing.T) {
			g, p, repo, registry := authoredModeFixture(t)
			ctx := context.Background()
			if id == "C24" {
				// The zoo's city requirement is unrelated to the shutdown operation.
				p.PlayedCards().AddCard(id, "Martian Zoo", string(cards.CardTypeActive), []string{"building", "animal"})
				c, _ := cards.ForGame(registry, g).GetByID(id)
				p.Resources().RegisterDependencyGroups(id, c.DependencyGroups)
				p.Resources().AddToStorage(id, 3)
			} else if err := playAuthored(t, g, p, repo, registry, id, nil, nil); err != nil {
				t.Fatal(err)
			}
			before := p.Resources().Get()
			storage := p.Resources().GetCardStorage(id)
			if err := playAuthored(t, g, p, repo, registry, "ES013", nil, []string{id}); err != nil {
				t.Fatal(err)
			}
			before.Credits -= 2
			if p.Resources().Get() != before || !p.PlayedCards().Contains(id) || p.Resources().GetCardStorage(id) != storage {
				t.Fatal("shutdown changed resources, stored resources, or discarded its target")
			}
			if !p.Resources().DependencyStatus(id, "earth-support").Retired {
				t.Fatal("shutdown did not retire the supported group")
			}
			if value, _ := g.ModeStateValue("withdrawal-generation"); value != 5 {
				t.Fatal("shutdown did not advance withdrawal")
			}
			c, _ := cards.ForGame(registry, g).GetByID(id)
			for _, b := range c.Behaviors {
				if b.Dependency != "earth-support" {
					continue
				}
				for _, trigger := range b.Triggers {
					if trigger.Type == "manual" || trigger.Condition != nil {
						if err := cards.BehaviorAvailable(g, p, id, b, cards.ForGame(registry, g)); err == nil {
							t.Fatal("shut-down action or passive effect remained available")
						}
					}
				}
			}
			if id == "110" && p.Resources().Production().Credits != 0 {
				t.Fatal("shutdown retained Business Network upkeep")
			}
			if id == "013" && p.Resources().Production().Titanium != 1 {
				t.Fatal("shutdown removed independent initial production")
			}
			mustGeneration(t, g, 5)
			g.ResolveModeBoundary(ctx, "production-end", false)
			mustGeneration(t, g, 6)
			w := pulse()
			w.State = "earth-contact"
			if err := g.ScheduleStateWindow(w, "test"); err != nil {
				t.Fatal(err)
			}
			g.ResolveModeBoundary(ctx, "production-start", false)
			if id == "ES001" && p.Resources().Production().Steel != 0 {
				t.Fatal("shut-down production returned during reconnection")
			}
		})
	}
}

func TestModeAuthoredShutdownRejectsInvalidTargetsAtomically(t *testing.T) {
	for _, target := range []string{"imported", "shut-down", "unowned", "prelude", "corporation", "lease"} {
		t.Run(target, func(t *testing.T) {
			g, p, repo, registry := authoredModeFixture(t)
			id := "ES001"
			switch target {
			case "imported", "shut-down":
				if err := playAuthored(t, g, p, repo, registry, id, nil, nil); err != nil {
					t.Fatal(err)
				}
				if err := p.Resources().SetDependencyStatus(id, "earth-support", target == "imported"); err != nil {
					t.Fatal(err)
				}
			case "prelude":
				id = "EP01"
				p.PlayedCards().AddCard(id, "Supply Charter", string(cards.CardTypePrelude), nil)
			case "corporation":
				id = "EC02"
				p.PlayedCards().AddCard(id, "Martian Independence Cooperative", string(cards.CardTypeCorporation), nil)
			case "lease":
				id = "ES016"
				if err := playAuthored(t, g, p, repo, registry, id, nil, nil); err != nil {
					t.Fatal(err)
				}
				mustGeneration(t, g, 2)
			}
			before, production := p.Resources().Get(), p.Resources().Production()
			deadline, _ := g.ModeStateValue("withdrawal-generation")
			if err := playAuthored(t, g, p, repo, registry, "ES013", nil, []string{id}); err == nil {
				t.Fatal("invalid target accepted")
			}
			if p.Resources().Get() != before || p.Resources().Production() != production || !p.Hand().HasCard("ES013") {
				t.Fatal("invalid shutdown mutated resources, production or hand")
			}
			if value, _ := g.ModeStateValue("withdrawal-generation"); value != deadline {
				t.Fatal("invalid shutdown moved withdrawal")
			}
			if err := g.ValidateModeStateChange(p.ID(), "withdrawal-generation", -1); err != nil {
				t.Fatalf("invalid shutdown consumed the deadline change: %v", err)
			}
		})
	}
}

func TestModeAuthoredLeaseIsNotEarthSupportAndCopiesExpire(t *testing.T) {
	g, p, repo, registry := authoredModeFixture(t)
	ctx := context.Background()
	effective := cards.ForGame(registry, g)
	corp, _ := effective.GetByID("EC01")
	action.SubscribePassiveEffectToEvents(ctx, g, p, shared.CardEffect{CardID: corp.ID, CardName: corp.Name, Behavior: corp.Behaviors[1]}, slog.Default())
	before := p.Resources().Get().Credits
	if err := playAuthored(t, g, p, repo, registry, "ES016", nil, nil); err != nil {
		t.Fatal(err)
	}
	if p.Resources().Get().Credits != before {
		t.Fatal("lease received an Earth-supported project rebate")
	}
	lease, _ := effective.GetByID("ES016")
	if cards.MatchesSelector(lease, shared.Selector{Dependency: "earth-support"}) {
		t.Fatal("lease is classified as Earth support")
	}
	if err := cards.ValidateDependencyTarget(p, effective, "ES016", "earth-support", true); err == nil {
		t.Fatal("lease can be imported")
	}
	copyOutput := &shared.CopyCondition{ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceCopy, Amount: 1, Target: "self-player"}, Scope: "production-box", Zone: "played"}
	options, _, err := cards.EffectSelectionOptions([]shared.BehaviorCondition{copyOutput}, p, g, effective, nil)
	if err != nil || len(options) != 1 {
		t.Fatalf("lease copy source missing: %v, %+v", err, options)
	}
	if err := cards.NewBehaviorApplier(p, g, "Copy", slog.Default()).WithCardRegistry(effective).WithSourceCardID("copy").ApplyOutputs(ctx, options[0].Outputs); err != nil {
		t.Fatal(err)
	}
	if p.Resources().Production().Credits != -4 {
		t.Fatal("copied lease lost its upkeep")
	}
	mustGeneration(t, g, 7)
	g.ResolveModeBoundary(ctx, "production-end", false)
	if p.Resources().Production().Credits != 0 {
		t.Fatal("copied lease outlived withdrawal")
	}
	mustGeneration(t, g, 8)
	w := pulse()
	w.State = "earth-contact"
	if err := g.ScheduleStateWindow(w, "pulse"); err != nil {
		t.Fatal(err)
	}
	g.ResolveModeBoundary(ctx, "production-start", false)
	if p.Resources().Production().Credits != 0 {
		t.Fatal("copied lease returned with Earth contact")
	}
}

func TestModeAuthoredMandatoryTimelineCardsRejectUnavailableDeadline(t *testing.T) {
	for _, id := range []string{"ES013", "ES016"} {
		for _, late := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/late=%t", id, late), func(t *testing.T) {
				g, p, repo, registry := authoredModeFixture(t)
				ctx := context.Background()
				if err := playAuthored(t, g, p, repo, registry, "ES001", nil, nil); err != nil {
					t.Fatal(err)
				}
				if late {
					mustGeneration(t, g, 6)
					g.ResolveModeBoundary(ctx, "production-end", false)
				} else if err := g.ChangeModeState(ctx, p.ID(), "withdrawal-generation", 1); err != nil {
					t.Fatal(err)
				}
				c, _ := cards.ForGame(registry, g).GetByID(id)
				if state := action.CalculatePlayerCardState(c, p, g, registry); len(state.Errors) == 0 {
					t.Fatal("unavailable mandatory deadline change shown as playable")
				}
				before, production := p.Resources().Get(), p.Resources().Production()
				if err := playAuthored(t, g, p, repo, registry, id, nil, []string{"ES001"}); err == nil {
					t.Fatal("unavailable mandatory deadline change accepted")
				}
				if p.Resources().Get() != before || p.Resources().Production() != production || p.Resources().DependencyStatus("ES001", "earth-support").Retired || !p.Hand().HasCard(id) {
					t.Fatal("failed play consumed payment, card, or target")
				}
			})
		}
	}
}

func TestModeAuthoredContinuityDrawsTwoBeforeOrAfterWithdrawal(t *testing.T) {
	for _, late := range []bool{false, true} {
		t.Run(fmt.Sprintf("late=%t", late), func(t *testing.T) {
			g, p, repo, registry := authoredModeFixture(t)
			ctx := context.Background()
			if late {
				mustGeneration(t, g, 6)
				g.ResolveModeBoundary(ctx, "production-end", false)
			}
			if err := playAuthored(t, g, p, repo, registry, "ES032", nil, nil); err != nil {
				t.Fatal(err)
			}
			if !late {
				mustGeneration(t, g, 6)
				g.ResolveModeBoundary(ctx, "production-end", false)
			}
			if len(p.Hand().Cards()) != 2 {
				t.Fatalf("Continuity Drills granted %d cards, want 2", len(p.Hand().Cards()))
			}
			mustGeneration(t, g, 7)
			w := pulse()
			w.State = "earth-contact"
			if err := g.ScheduleStateWindow(w, "pulse"); err != nil {
				t.Fatal(err)
			}
			g.ResolveModeBoundary(ctx, "production-start", false)
			g.ResolveModeBoundary(ctx, "production-end", false)
			if len(p.Hand().Cards()) != 2 {
				t.Fatal("Continuity Drills repeated during reconnection")
			}
		})
	}
}

func TestModeAuthoredOptionalEffectsResolveAfterGuaranteedGains(t *testing.T) {
	for _, id := range []string{"ES011", "ES014", "ES017"} {
		for _, accept := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/accept=%v", id, accept), func(t *testing.T) {
				ctx := context.Background()
				g, p, repo, registry := authoredModeFixture(t)
				if id == "ES011" {
					p.Resources().Add(map[shared.ResourceType]int{shared.ResourceEnergy: 1})
				}
				if err := playAuthored(t, g, p, repo, registry, id, nil, nil); err != nil {
					t.Fatal(err)
				}
				pending := p.Selection().GetPendingBehaviorResolutions()
				if len(pending) != 1 || pending[0].Kind != "optional" {
					t.Fatalf("missing optional prompt: %+v", pending)
				}
				resolution := pending[0]
				if p.Hand().HasCard(id) {
					t.Fatal("optional prompt must follow committed card play")
				}
				if errs := action.CalculateOptionalEffectErrors(resolution, p, g, registry); len(errs) != 0 {
					t.Fatalf("guaranteed gains cannot fund optional effect: %+v", errs)
				}
				before := p.Resources().Get()
				production := p.Resources().PermanentProduction()
				handCount := len(p.Hand().Cards())
				if id == "ES014" && (handCount != 2 || len(p.Selection().CardReceipts()) != 1) {
					t.Fatal("guaranteed cards must be received before optional payment")
				}
				resolve := confirmation.NewConfirmBehaviorChoiceAction(repo, registry, nil, slog.Default())
				if err := resolve.ExecuteOptional(ctx, g.ID(), p.ID(), resolution.ID, accept); err != nil {
					t.Fatal(err)
				}
				wantDeadline := 6
				if accept {
					wantDeadline = 7
					if id == "ES011" {
						wantDeadline = 5
						before.Energy -= 3
					}
					if id == "ES014" {
						before.Credits -= 3
					}
					if id == "ES017" {
						production.Energy--
					}
				}
				deadline, _ := g.ModeStateValue("withdrawal-generation")
				if deadline != wantDeadline || p.Resources().Get() != before || p.Resources().PermanentProduction() != production || len(p.Hand().Cards()) != handCount {
					t.Fatalf("wrong optional result: deadline=%d resources=%+v production=%+v", deadline, p.Resources().Get(), p.Resources().PermanentProduction())
				}
				if len(p.Selection().GetPendingBehaviorResolutions()) != 0 {
					t.Fatal("optional effect remained pending")
				}
				if err := resolve.ExecuteOptional(ctx, g.ID(), p.ID(), resolution.ID, accept); err == nil {
					t.Fatal("duplicate resolution accepted")
				}
				if p.Resources().Get() != before {
					t.Fatal("duplicate resolution changed resources")
				}
				if accept {
					found := false
					for _, effect := range g.GetTriggeredEffects() {
						for _, output := range effect.CalculatedOutputs {
							if output.Operation != nil && output.Operation.State == "withdrawal-generation" {
								found = true
							}
						}
					}
					if !found {
						t.Fatal("notification lost typed state operation")
					}
				}
			})
		}
	}
}

func TestModeOptionalUnavailableEffectCanBeSkipped(t *testing.T) {
	for _, cause := range []string{"no-energy", "no-credits", "withdrawn", "deadline-limit", "changed-after-prompt"} {
		t.Run(cause, func(t *testing.T) {
			ctx := context.Background()
			g, p, repo, registry := authoredModeFixture(t)
			id := "ES014"
			switch cause {
			case "no-energy":
				id = "ES011"
			case "no-credits":
				p.Resources().Set(shared.Resources{Credits: 7})
			case "withdrawn":
				mustGeneration(t, g, 6)
				g.ResolveModeBoundary(ctx, "production-end", false)
			case "deadline-limit":
				for _, generation := range []int{1, 2} {
					mustGeneration(t, g, generation)
					if err := g.ChangeModeState(ctx, p.ID(), "withdrawal-generation", 1); err != nil {
						t.Fatal(err)
					}
				}
				mustGeneration(t, g, 3)
			}
			if err := playAuthored(t, g, p, repo, registry, id, nil, nil); err != nil {
				t.Fatalf("optional effect blocked guaranteed card: %v", err)
			}
			resolution := p.Selection().GetPendingBehaviorResolutions()[0]
			if cause == "changed-after-prompt" {
				if errs := action.CalculateOptionalEffectErrors(resolution, p, g, registry); len(errs) > 0 {
					t.Fatal(errs)
				}
				p.Resources().Set(shared.Resources{})
			}
			before := p.Resources().Get()
			production := p.Resources().Production()
			deadline, _ := g.ModeStateValue("withdrawal-generation")
			if errs := action.CalculateOptionalEffectErrors(resolution, p, g, registry); len(errs) == 0 {
				t.Fatal("unavailable payment was offered")
			}
			resolve := confirmation.NewConfirmBehaviorChoiceAction(repo, registry, nil, slog.Default())
			if err := resolve.ExecuteOptional(ctx, g.ID(), p.ID(), resolution.ID, true); err == nil {
				t.Fatal("unavailable optional effect accepted")
			}
			if p.Selection().GetPendingBehaviorResolution(resolution.ID) == nil || p.Resources().Get() != before || p.Resources().Production() != production {
				t.Fatal("rejected optional payment partially applied")
			}
			other, err := g.AddNewPlayer(ctx, "other", "Other")
			if err != nil {
				t.Fatal(err)
			}
			if err := resolve.ExecuteOptional(ctx, g.ID(), other.ID(), resolution.ID, false); err == nil {
				t.Fatal("another player resolved pending effect")
			}
			if err := resolve.ExecuteOptional(ctx, g.ID(), p.ID(), resolution.ID, false); err != nil {
				t.Fatal(err)
			}
			afterDeadline, _ := g.ModeStateValue("withdrawal-generation")
			if deadline != afterDeadline || p.Resources().Get() != before || p.Resources().Production() != production {
				t.Fatal("skip changed resources or deadline")
			}
		})
	}
}

func TestModeAuthoredAlternativeActionsShareOneUse(t *testing.T) {
	for _, id := range []string{"ES012", "EC02"} {
		t.Run(id, func(t *testing.T) {
			g, p, repo, registry := authoredModeFixture(t)
			ctx := context.Background()
			first, second := 1, 2
			var firstChoice, secondChoice *int
			if id == "ES012" {
				second = 1
				petition, research := 0, 1
				firstChoice, secondChoice = &petition, &research
			}
			if id == "EC02" {
				first, second = 2, 1
				corp, _ := cards.ForGame(registry, g).GetByID(id)
				processor := cards.NewCorporationProcessor(cards.ForGame(registry, g), nil, slog.Default())
				p.Actions().SetActions(processor.GetManualActions(corp))
			} else if err := playAuthored(t, g, p, repo, registry, id, nil, nil); err != nil {
				t.Fatal(err)
			}
			use := cardaction.NewUseCardActionAction(repo, registry, nil, slog.Default())
			if err := use.Execute(ctx, g.ID(), p.ID(), id, first, firstChoice, nil, nil, nil, nil, nil, nil, nil); err != nil {
				t.Fatal(err)
			}
			before := p.Resources().Get()
			if err := use.Execute(ctx, g.ID(), p.ID(), id, second, secondChoice, nil, nil, nil, nil, nil, nil, nil); err == nil {
				t.Fatal("both alternatives were used in one generation")
			}
			if p.Resources().Get() != before {
				t.Fatal("unavailable alternative charged or granted resources")
			}
			mustGeneration(t, g, 2)
			p.Actions().ResetGenerationCounts()
			if id == "EC02" {
				if err := use.Execute(ctx, g.ID(), p.ID(), id, first, firstChoice, nil, nil, nil, nil, nil, nil, nil); err == nil {
					t.Fatal("corporation's once-per-game option repeated")
				}
			}
			if err := use.Execute(ctx, g.ID(), p.ID(), id, second, secondChoice, nil, nil, nil, nil, nil, nil, nil); err != nil {
				t.Fatalf("next generation's alternative unavailable: %v", err)
			}
		})
	}
}

func TestModeRequirementMessagesForCardsAndActions(t *testing.T) {
	g, p, _, registry := authoredModeFixture(t)
	for _, tc := range []struct {
		state   string
		equals  int
		message string
	}{
		{"earth-contact", 0, "Earth is still connected"},
		{"withdrawal-occurred", 1, "Earth has not withdrawn yet"},
	} {
		err := g.ValidateModeRequirements([]shared.StateRequirement{{State: tc.state, Equals: tc.equals}})
		if err == nil || err.Error() != tc.message {
			t.Fatalf("expected %q, got %v", tc.message, err)
		}
	}
	mustGeneration(t, g, 6)
	g.ResolveModeBoundary(context.Background(), "production-end", false)
	connected := []shared.StateRequirement{{State: "earth-contact", Equals: 1}}
	if err := g.ValidateModeRequirements(connected); err == nil || err.Error() != "Earth is disconnected" {
		t.Fatalf("connection error: %v", err)
	}
	card, err := cards.ForGame(registry, g).GetByID("ES002")
	if err != nil {
		t.Fatal(err)
	}
	state := action.CalculatePlayerCardState(card, p, g, registry)
	found := false
	for _, err := range state.Errors {
		if err.Message == "Earth is disconnected" {
			found = true
		}
	}
	if !found {
		t.Fatalf("card errors: %+v", state.Errors)
	}
	choice := shared.Choice{StateRequirements: []shared.StateRequirement{{State: "withdrawal-occurred", Equals: 0}}}
	errors := action.CalculateChoiceErrors(choice, p, g, registry)
	if len(errors) != 1 || errors[0].Message != "Withdrawal has already happened" {
		t.Fatalf("choice errors: %+v", errors)
	}
	if err := cards.BehaviorAvailable(g, p, "", shared.CardBehavior{StateRequirements: connected}, registry); err == nil || err.Error() != "Earth is disconnected" {
		t.Fatalf("action error: %v", err)
	}
	snapshot := g.ModeRuntime()
	snapshot.Definition.RequirementMessages["earth-contact"][1] = "changed"
	if err := g.ValidateModeRequirements(connected); err == nil || err.Error() != "Earth is disconnected" {
		t.Fatal("snapshot mutated requirement wording")
	}
}

func TestSovereigntyOfficeHasOneActionAndShowsDrawReceipt(t *testing.T) {
	g, p, repo, registry := authoredModeFixture(t)
	if err := playAuthored(t, g, p, repo, registry, "ES012", nil, nil); err != nil {
		t.Fatal(err)
	}
	actions := p.Actions().List()
	if len(actions) != 1 || len(actions[0].Behavior.Choices) != 2 {
		t.Fatalf("expected one action with two choices: %+v", actions)
	}
	mustGeneration(t, g, 6)
	g.ResolveModeBoundary(context.Background(), "production-end", false)
	state := action.CalculatePlayerCardActionState("ES012", actions[0].Behavior, 0, p, g, registry)
	if !state.Available() {
		t.Fatalf("research should remain available after withdrawal: %+v", state.Errors)
	}
	choiceErrors := action.CalculateChoiceErrors(actions[0].Behavior.Choices[0], p, g, registry)
	if len(choiceErrors) != 1 || choiceErrors[0].Message != "Withdrawal has already happened" {
		t.Fatalf("petition errors: %+v", choiceErrors)
	}
	creditsBefore := p.Resources().Get().Credits
	research := 1
	use := cardaction.NewUseCardActionAction(repo, registry, nil, slog.Default())
	if err := use.Execute(context.Background(), g.ID(), p.ID(), "ES012", 1, &research, nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	receipts := p.Selection().CardReceipts()
	if len(receipts) != 1 || len(receipts[0].Cards) != 1 || receipts[0].Source != "Sovereignty Office" {
		t.Fatalf("expected one card receipt from Sovereignty Office, got %+v", receipts)
	}
	if p.Resources().Get().Credits != creditsBefore-3 {
		t.Fatal("research must cost 3 M€")
	}
	if !p.Hand().HasCard(receipts[0].Cards[0]) {
		t.Fatal("receipt must show the card added to the hand")
	}
}

func TestModeGenerationIncomePaidTogetherAndOutsideProduction(t *testing.T) {
	g, p, repo, registry := authoredModeFixture(t)
	ctx := context.Background()
	q, err := g.AddNewPlayer(ctx, "q", "Other")
	if err != nil {
		t.Fatal(err)
	}
	p.Resources().Set(shared.Resources{})
	q.Resources().Set(shared.Resources{})
	turn_management.AdvanceToActionPhase(ctx, g, []*player.Player{p, q}, slog.Default())
	for _, receiver := range []*player.Player{p, q} {
		if receiver.Resources().Get().Credits != 2 {
			t.Fatal("all players must receive 2 M€ before generation 1 actions")
		}
	}
	action.BeginModeGeneration(ctx, g)
	if p.Resources().Get().Credits != 2 {
		t.Fatal("income repeated in the same generation")
	}
	if err := turn_management.ExecuteProductionPhase(ctx, g, []*player.Player{p, q}, slog.Default()); err != nil {
		t.Fatal(err)
	}
	for _, receiver := range []*player.Player{p, q} {
		production := g.GetProductionPhase(receiver.ID())
		if production.CreditsIncome != receiver.Resources().TerraformRating() {
			t.Fatal("support was included in production")
		}
		if receiver.Resources().Get().Credits != production.AfterResources.Credits {
			t.Fatal("generation 2 support must wait until all players finish card selection")
		}
	}
	beforeSelection := map[string]int{p.ID(): p.Resources().Get().Credits, q.ID(): q.Resources().Get().Credits}
	confirm := confirmation.NewConfirmProductionCardsAction(repo, registry, nil, slog.Default())
	if err := confirm.Execute(ctx, g.ID(), p.ID(), nil, false, shared.Payment{}); err != nil {
		t.Fatal(err)
	}
	for _, receiver := range []*player.Player{p, q} {
		if receiver.Resources().Get().Credits != beforeSelection[receiver.ID()] {
			t.Fatal("support paid while another player was still selecting cards")
		}
	}
	if g.CurrentPhase() != shared.GamePhaseProductionAndCardDraw || g.ModeRuntime().IncomeStreak != 1 {
		t.Fatal("generation started before everyone finished")
	}
	if err := confirm.Execute(ctx, g.ID(), q.ID(), nil, false, shared.Payment{}); err != nil {
		t.Fatal(err)
	}
	for _, receiver := range []*player.Player{p, q} {
		if receiver.Resources().Get().Credits != beforeSelection[receiver.ID()]+3 {
			t.Fatal("everyone must receive 3 M€ when the last player confirms")
		}
		if g.GetProductionPhase(receiver.ID()) != nil {
			t.Fatal("production must be finished before support arrives")
		}
	}
	if g.CurrentPhase() != shared.GamePhaseAction || g.ModeRuntime().IncomeStreak != 2 {
		t.Fatal("support must arrive as generation 2 actions begin")
	}
	action.BeginModeGeneration(ctx, g)
	if p.Resources().Get().Credits != beforeSelection[p.ID()]+3 {
		t.Fatal("income repeated after confirmation")
	}
	q.SetExited(true)
	before := q.Resources().Get().Credits
	mustGeneration(t, g, 3)
	action.BeginModeGeneration(ctx, g)
	if q.Resources().Get().Credits != before {
		t.Fatal("exited player received income")
	}
	if g.ModeRuntime().IncomeStreak != 3 {
		t.Fatal("connected streak did not increase")
	}
}

func TestModeGenerationIncomeWaitsForRemainingPlayersAfterKick(t *testing.T) {
	g, p, repo, registry := authoredModeFixture(t)
	ctx := context.Background()
	q, err := g.AddNewPlayer(ctx, "q", "Other")
	if err != nil {
		t.Fatal(err)
	}
	players := []*player.Player{p, q}
	turn_management.AdvanceToActionPhase(ctx, g, players, slog.Default())
	if err := turn_management.ExecuteProductionPhase(ctx, g, players, slog.Default()); err != nil {
		t.Fatal(err)
	}
	confirm := confirmation.NewConfirmProductionCardsAction(repo, registry, nil, slog.Default())
	if err := confirm.Execute(ctx, g.ID(), p.ID(), nil, false, shared.Payment{}); err != nil {
		t.Fatal(err)
	}
	before := p.Resources().Get().Credits
	exitedBefore := q.Resources().Get().Credits
	if err := connection.NewKickPlayerAction(repo, nil, nil, slog.Default()).Execute(ctx, g.ID(), p.ID(), q.ID()); err != nil {
		t.Fatal(err)
	}
	if g.CurrentPhase() != shared.GamePhaseAction || p.Resources().Get().Credits != before+3 {
		t.Fatal("remaining player must receive support when card selection finishes after a kick")
	}
	if q.Resources().Get().Credits != exitedBefore {
		t.Fatal("exited player must not receive support")
	}
}

func TestModeGenerationIncomeResetsAndWarnsOnLastConnectedGeneration(t *testing.T) {
	g, p, _, _ := authoredModeFixture(t)
	ctx := context.Background()
	for generation := 1; generation <= 6; generation++ {
		mustGeneration(t, g, generation)
		before := p.Resources().Get().Credits
		action.BeginModeGeneration(ctx, g)
		if p.Resources().Get().Credits-before != generation+1 {
			t.Fatalf("generation %d had incorrect income", generation)
		}
	}
	warnings := g.ModeRuntime().Events
	if len(warnings) != 1 || warnings[0].Title != "Last generation of Earth support" {
		t.Fatalf("missing final-generation warning: %+v", warnings)
	}
	action.BeginModeGeneration(ctx, g)
	if len(g.ModeRuntime().Events) != 1 {
		t.Fatal("duplicate warning")
	}
	g.ResolveModeBoundary(ctx, "production-end", false)
	mustGeneration(t, g, 7)
	before := p.Resources().Get().Credits
	action.BeginModeGeneration(ctx, g)
	if p.Resources().Get().Credits != before || g.ModeRuntime().IncomeStreak != 0 {
		t.Fatal("disconnected generation must reset without paying")
	}
	window := fullWindow()
	window.State = "earth-contact"
	if err := g.ScheduleStateWindow(window, "reconnect"); err != nil {
		t.Fatal(err)
	}
	mustGeneration(t, g, 8)
	g.ResolveModeBoundary(ctx, "research-start", false)
	action.BeginModeGeneration(ctx, g)
	if p.Resources().Get().Credits-before != 2 {
		t.Fatal("reconnection must restart at 2 M€")
	}
	g.ResolveModeBoundary(ctx, "production-end", false)
	mustGeneration(t, g, 9)
	action.BeginModeGeneration(ctx, g)
	pulse := pulse()
	pulse.State = "earth-contact"
	if err := g.ScheduleStateWindow(pulse, "production-pulse"); err != nil {
		t.Fatal(err)
	}
	g.ResolveModeBoundary(ctx, "production-start", false)
	before = p.Resources().Get().Credits
	action.BeginModeGeneration(ctx, g)
	if p.Resources().Get().Credits != before {
		t.Fatal("production-only reconnection paid generation income")
	}
}

func TestModeProductionPreviewIncludesImportsUpkeepAndLeaseExpiry(t *testing.T) {
	g, p, _, registry := authoredModeFixture(t)
	ctx := context.Background()
	p.Resources().SetProduction(shared.Production{Credits: 5, Steel: 1})
	dependency := shared.DependencyGroup{ID: "earth", State: "earth-contact", Equals: 1}
	addSupply(p, "support", dependency, shared.ResourceSteelProduction, 3)
	addSupply(p, "upkeep", dependency, shared.ResourceCreditProduction, -1)
	addSupply(p, "imported", dependency, shared.ResourceTitaniumProduction, 2)
	if err := p.Resources().SetDependencyStatus("imported", "earth", true); err != nil {
		t.Fatal(err)
	}
	lease := shared.DependencyGroup{ID: "lease", State: "withdrawal-occurred", Equals: 0, RetireOnState: &shared.StateRequirement{State: "withdrawal-occurred", Equals: 1}}
	addSupply(p, "lease", lease, shared.ResourceCreditProduction, -2)
	before := p.Resources().Production()
	preview := dto.ToPlayerDto(p, g, registry, nil, nil, nil, nil).ModeProduction
	if preview == nil || preview.Connected.Credits != 2 || preview.Disconnected.Credits != 5 || preview.Connected.Steel != 4 || preview.Disconnected.Steel != 1 || preview.Disconnected.Titanium != 2 {
		t.Fatalf("incorrect preview: %+v", preview)
	}
	if p.Resources().Production() != before {
		t.Fatal("preview changed production")
	}
	mustGeneration(t, g, 6)
	g.ResolveModeBoundary(ctx, "production-end", false)
	preview = dto.ToPlayerDto(p, g, registry, nil, nil, nil, nil).ModeProduction
	if preview.Connected.Credits != 4 {
		t.Fatal("reconnection preview restored expired lease payments")
	}
}

func TestModeAnnouncementSoundsFollowWarningAndContactClosures(t *testing.T) {
	g, _, _, _ := authoredModeFixture(t)
	ctx := context.Background()
	mustGeneration(t, g, 6)
	action.BeginModeGeneration(ctx, g)
	g.ResolveModeBoundary(ctx, "production-end", false)
	mustGeneration(t, g, 7)
	window := fullWindow()
	window.State = "earth-contact"
	if err := g.ScheduleStateWindow(window, "reconnect"); err != nil {
		t.Fatal(err)
	}
	mustGeneration(t, g, 8)
	g.ResolveModeBoundary(ctx, "research-start", false)
	g.ResolveModeBoundary(ctx, "production-end", false)

	presentation := dto.ToModeRuntimeDto(g)
	want := []string{"earth-withdrawal-warning", "earth-contact-lost", "", "earth-contact-lost"}
	if len(presentation.Events) != len(want) {
		t.Fatalf("unexpected announcements: %+v", presentation.Events)
	}
	for i, sound := range want {
		if presentation.Events[i].SoundID != sound {
			t.Errorf("event %d sound = %q, want %q", i, presentation.Events[i].SoundID, sound)
		}
	}
}
