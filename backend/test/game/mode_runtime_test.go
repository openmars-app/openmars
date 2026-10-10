package game_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"openmars/internal/action"
	cardaction "openmars/internal/action/card"
	"openmars/internal/action/connection"
	"openmars/internal/action/turn_management"
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

func TestModeProductionSnapshotIncludesStipendAndWithdrawalCardRewards(t *testing.T) {
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
			if phase.CreditsIncome != 22 || phase.AfterResources.Credits-before.Credits != 22 {
				t.Fatalf("snapshot omitted the connected stipend: %+v", phase)
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
	choice := 0
	if err := playAuthored(t, g, p, repo, registry, "ES016", &choice, nil); err != nil {
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
	choice := 0
	if err := playAuthored(t, g, p, repo, registry, "ES013", &choice, []string{"106"}); err == nil || !strings.Contains(err.Error(), "already changed") {
		t.Fatalf("expected successful-change cap, got %v", err)
	}
	if p.Resources().Get() != before || p.Resources().DependencyStatus("106", "earth-support").Retired {
		t.Fatal("invalid retirement paid or retired source")
	}
	mustGeneration(t, g, 2)
	p.Hand().RemoveCard("ES013")
	if err := playAuthored(t, g, p, repo, registry, "ES013", &choice, []string{"106"}); err != nil {
		t.Fatal(err)
	}
	if !p.Resources().DependencyStatus("106", "earth-support").Retired || p.Resources().Production().Credits != 0 {
		t.Fatal("valid retirement kept source")
	}
}
