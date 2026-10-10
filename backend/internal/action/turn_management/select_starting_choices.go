package turn_management

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	baseaction "openmars/internal/action"

	"openmars/internal/events"
	"openmars/internal/game"
	"openmars/internal/game/award"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
)

// SelectStartingChoicesAction handles the combined selection of corporation, preludes, and starting cards.
// Selections are validated and stored but effects are NOT applied until the init_apply phases.
type SelectStartingChoicesAction struct {
	gameRepo      game.GameRepository
	cardRegistry  gamecards.CardRegistry
	awardRegistry award.AwardRegistry
	corpProc      *gamecards.CorporationProcessor
	logger        *slog.Logger
}

// NewSelectStartingChoicesAction creates a new select starting choices action
func NewSelectStartingChoicesAction(
	gameRepo game.GameRepository,
	cardRegistry gamecards.CardRegistry,
	awardRegistry award.AwardRegistry,
	logger *slog.Logger,
) *SelectStartingChoicesAction {
	return &SelectStartingChoicesAction{
		gameRepo:      gameRepo,
		cardRegistry:  cardRegistry,
		awardRegistry: awardRegistry,
		corpProc:      gamecards.NewCorporationProcessor(cardRegistry, awardRegistry, slog.Default()),
		logger:        logger,
	}
}

// Execute validates and stores starting selections without applying effects.
// Effects are deferred to init_apply_corp and init_apply_prelude phases.
func (a *SelectStartingChoicesAction) Execute(ctx context.Context, gameID string, playerID string, corporationID string, preludeIDs []string, cardIDs []string, payment shared.Payment) error {
	log := a.logger.With(
		slog.String("game_id", gameID),
		slog.String("player_id", playerID),
		slog.String("action", "select_starting_choices"),
		slog.String("corporation_id", corporationID),
		slog.Any("prelude_ids", preludeIDs),
		slog.Any("card_ids", cardIDs),
	)
	log.Debug("Player selecting starting choices")

	g, err := a.gameRepo.Get(ctx, gameID)
	if err != nil {
		log.Error("Failed to get game", slog.Any("error", err))
		return fmt.Errorf("game not found: %s", gameID)
	}

	if g.CurrentPhase() != shared.GamePhaseStartingSelection {
		log.Error("Game not in starting selection phase", slog.String("phase", string(g.CurrentPhase())))
		return fmt.Errorf("game not in starting selection phase")
	}

	p, err := g.GetPlayer(playerID)
	if err != nil {
		log.Error("Player not found in game", slog.Any("error", err))
		return fmt.Errorf("player not found: %s", playerID)
	}

	if err := a.validateCorporation(g, p, corporationID, log); err != nil {
		return err
	}

	if err := a.validatePreludes(g, p, preludeIDs, log); err != nil {
		return err
	}

	if err := a.validateStartingCards(g, p, corporationID, cardIDs, log); err != nil {
		return err
	}

	quote, err := gamecards.QuoteStartingPayment(gamecards.ForGame(a.cardRegistry, g), corporationID, len(cardIDs), g.Settings().DemoGame)
	if err != nil {
		return err
	}
	paymentPlan, err := gamecards.ValidatePayment(quote, payment)
	if err != nil {
		return err
	}

	p.SetCorporationID(corporationID)

	if err := g.SetDeferredStartingChoices(ctx, playerID, &shared.DeferredStartingChoices{
		CorporationID: corporationID,
		PreludeIDs:    preludeIDs,
		CardIDs:       cardIDs,
		Payment:       paymentPlan.Payment,
	}); err != nil {
		return fmt.Errorf("failed to store deferred starting choices: %w", err)
	}

	// Remove unchosen preludes permanently (preludes must never enter the discard/draw cycle)
	preludePhase := g.GetSelectPreludeCardsPhase(p.ID())
	if preludePhase != nil {
		selectedSet := make(map[string]bool, len(preludeIDs))
		for _, id := range preludeIDs {
			selectedSet[id] = true
		}
		var unselected []string
		for _, id := range preludePhase.AvailablePreludes {
			if !selectedSet[id] {
				unselected = append(unselected, id)
			}
		}
		if len(unselected) > 0 {
			if err := g.Deck().Remove(ctx, unselected); err != nil {
				log.Error("Failed to remove unselected preludes", slog.Any("error", err))
				return fmt.Errorf("failed to remove unselected preludes: %w", err)
			}
		}
	}

	// Discard unchosen project cards
	selectionPhase := g.GetSelectStartingCardsPhase(p.ID())
	if selectionPhase != nil {
		selectedSet := make(map[string]bool, len(cardIDs))
		for _, id := range cardIDs {
			selectedSet[id] = true
		}
		var unselected []string
		for _, cardID := range selectionPhase.AvailableCards {
			if !selectedSet[cardID] {
				unselected = append(unselected, cardID)
			}
		}
		if len(unselected) > 0 {
			if err := g.Deck().Discard(ctx, unselected); err != nil {
				log.Error("Failed to discard unselected project cards", slog.Any("error", err))
				return fmt.Errorf("failed to discard unselected project cards: %w", err)
			}
		}
	}

	// Clear all selection phases to signal completion
	if err := g.SetSelectCorporationPhase(ctx, p.ID(), nil); err != nil {
		return fmt.Errorf("failed to clear corporation phase: %w", err)
	}
	if err := g.SetSelectPreludeCardsPhase(ctx, p.ID(), nil); err != nil {
		return fmt.Errorf("failed to clear prelude phase: %w", err)
	}
	if err := g.SetSelectStartingCardsPhase(ctx, p.ID(), nil); err != nil {
		return fmt.Errorf("failed to clear starting cards phase: %w", err)
	}

	a.checkAndAdvanceToInitApplyCorp(ctx, g, log)

	log.Info("Starting choices stored")
	return nil
}

func (a *SelectStartingChoicesAction) validateCorporation(g *game.Game, p *player.Player, corporationID string, log *slog.Logger) error {
	corpPhase := g.GetSelectCorporationPhase(p.ID())
	if corpPhase == nil {
		return fmt.Errorf("not in corporation selection phase")
	}

	if p.HasCorporation() {
		return fmt.Errorf("corporation already selected")
	}

	corpAvailable := false
	for _, corpID := range corpPhase.AvailableCorporations {
		if corpID == corporationID {
			corpAvailable = true
			break
		}
	}
	if !corpAvailable {
		return fmt.Errorf("corporation %s not available", corporationID)
	}

	corpCard, err := gamecards.ForGame(a.cardRegistry, g).GetByID(corporationID)
	if err != nil {
		return fmt.Errorf("corporation card not found: %s", corporationID)
	}

	if corpCard.Type != gamecards.CardTypeCorporation {
		return fmt.Errorf("card %s is not a corporation card", corporationID)
	}

	return nil
}

func (a *SelectStartingChoicesAction) validatePreludes(g *game.Game, p *player.Player, preludeIDs []string, log *slog.Logger) error {
	preludePhase := g.GetSelectPreludeCardsPhase(p.ID())
	if preludePhase == nil {
		if len(preludeIDs) > 0 {
			return fmt.Errorf("prelude cards submitted but player has no prelude phase")
		}
		return nil
	}

	if len(preludeIDs) != preludePhase.MaxSelectable {
		return fmt.Errorf("must select exactly %d preludes, got %d", preludePhase.MaxSelectable, len(preludeIDs))
	}

	return baseaction.ValidateSelection(preludePhase.AvailablePreludes, preludeIDs, "prelude")
}

func (a *SelectStartingChoicesAction) validateStartingCards(g *game.Game, p *player.Player, corporationID string, cardIDs []string, log *slog.Logger) error {
	selectionPhase := g.GetSelectStartingCardsPhase(p.ID())
	if selectionPhase == nil {
		return fmt.Errorf("not in starting card selection phase")
	}

	return baseaction.ValidateSelection(selectionPhase.AvailableCards, cardIDs, "card")
}

// getCardBuyCost returns the per-card buy cost accounting for corporation effects (e.g., Polyphemos pays 5 instead of 3)
func getCardBuyCost(cardRegistry gamecards.CardRegistry, corporationID string) int {
	baseCost := 3
	corpCard, err := cardRegistry.GetByID(corporationID)
	if err != nil {
		return baseCost
	}
	discount := gamecards.CalculateActionDiscountsFromCard(corpCard, shared.ActionCardBuying)
	effectiveCost := baseCost - discount
	if effectiveCost < 0 {
		effectiveCost = 0
	}
	return effectiveCost
}

// checkAndAdvanceToInitApplyCorp checks if all players have stored their choices
// and transitions to the init_apply_corp phase
func (a *SelectStartingChoicesAction) checkAndAdvanceToInitApplyCorp(ctx context.Context, g *game.Game, log *slog.Logger) {
	allPlayers := g.GetAllPlayers()
	turnOrder := g.TurnOrder()

	for _, p := range allPlayers {
		if p.HasExited() {
			continue
		}
		if g.GetDeferredStartingChoices(p.ID()) == nil {
			log.Debug("Waiting for other players to complete starting selection")
			return
		}
	}

	log.Debug("All players stored starting choices, advancing to init_apply_corp phase")

	if err := g.UpdatePhase(ctx, shared.GamePhaseInitApplyCorp); err != nil {
		log.Error("Failed to transition to init_apply_corp phase", slog.Any("error", err))
		return
	}

	firstPlayerID := findFirstActivePlayer(g, turnOrder)
	if firstPlayerID == "" {
		return
	}

	firstIndex := findPlayerIndex(turnOrder, firstPlayerID)
	if err := g.SetInitPhasePlayerIndex(ctx, firstIndex); err != nil {
		log.Error("Failed to set init phase player index", slog.Any("error", err))
		return
	}

	if err := g.SetInitPhaseWaitingForConfirm(ctx, true); err != nil {
		log.Error("Failed to set waiting for confirm", slog.Any("error", err))
		return
	}
}

// ApplyCorpForPlayer applies corporation effects for a single player during init_apply_corp phase.
// This includes starting effects, auto effects, registering triggers/actions,
// and purchasing project cards.
func ApplyCorpForPlayer(ctx context.Context, g *game.Game, playerID string, cardRegistry gamecards.CardRegistry, corpProc *gamecards.CorporationProcessor, log *slog.Logger) error {
	choices := g.GetDeferredStartingChoices(playerID)
	if choices == nil {
		return fmt.Errorf("no deferred starting choices for player %s", playerID)
	}

	p, err := g.GetPlayer(playerID)
	if err != nil {
		return fmt.Errorf("player not found: %s", playerID)
	}

	corpCard, err := cardRegistry.GetByID(choices.CorporationID)
	if err != nil {
		return fmt.Errorf("corporation card not found: %s", choices.CorporationID)
	}

	if corpCard.ResourceStorage != nil {
		g.Colonies().ActivateResource(string(corpCard.ResourceStorage.Type))
		p.Resources().AddToStorage(choices.CorporationID, corpCard.ResourceStorage.Starting)
	}

	log.Debug("Applying corporation effects",
		slog.String("player_id", playerID),
		slog.String("corporation", corpCard.Name))

	// Register trigger effects BEFORE applying starting effects so that
	// production-increased triggers (e.g. Manutech) fire on starting production
	triggerEffects := corpProc.GetTriggerEffects(corpCard)
	for _, effect := range triggerEffects {
		p.Effects().AddEffect(effect)
		baseaction.SubscribePassiveEffectToEvents(ctx, g, p, effect, log, cardRegistry)
		g.AddTriggeredEffect(shared.TriggeredEffect{
			CardName:   corpCard.Name,
			PlayerID:   p.ID(),
			SourceType: shared.SourceTypeEffectAdded,
			Behaviors:  []shared.CardBehavior{effect.Behavior},
		})
	}

	if err := corpProc.ApplyStartingEffects(ctx, corpCard, p, g); err != nil {
		return fmt.Errorf("failed to apply corporation starting effects: %w", err)
	}

	if err := corpProc.ApplyAutoEffects(ctx, corpCard, p, g); err != nil {
		return fmt.Errorf("failed to apply corporation auto effects: %w", err)
	}

	autoEffects := corpProc.GetAutoEffects(corpCard)
	for _, effect := range autoEffects {
		p.Effects().AddEffect(effect)
		g.AddTriggeredEffect(shared.TriggeredEffect{
			CardName:   corpCard.Name,
			PlayerID:   p.ID(),
			SourceType: shared.SourceTypeEffectAdded,
			Behaviors:  []shared.CardBehavior{effect.Behavior},
		})
	}

	for _, tag := range corpCard.Tags {
		events.Publish(g.EventBus(), events.TagPlayedEvent{
			GameID:    g.ID(),
			PlayerID:  p.ID(),
			CardID:    choices.CorporationID,
			CardName:  corpCard.Name,
			Tag:       string(tag),
			Timestamp: time.Now(),
		})
	}

	g.RegisterCorporationVPGranter(p.ID(), choices.CorporationID)

	manualActions := corpProc.GetManualActions(corpCard)
	for _, action := range manualActions {
		p.Actions().AddAction(action)
		g.AddTriggeredEffect(shared.TriggeredEffect{
			CardName:   corpCard.Name,
			PlayerID:   p.ID(),
			SourceType: shared.SourceTypeActionAdded,
			Behaviors:  []shared.CardBehavior{action.Behavior},
		})
	}

	if err := corpProc.SetupForcedFirstAction(ctx, corpCard, g, p.ID()); err != nil {
		return fmt.Errorf("failed to setup forced first action: %w", err)
	}

	// Purchase project cards now that starting credits are applied (skip cost in demo mode)
	if !g.Settings().DemoGame {
		costPerCard := getCardBuyCost(cardRegistry, choices.CorporationID)
		cost := len(choices.CardIDs) * costPerCard
		quote, err := gamecards.QuotePayment(p, g, cardRegistry, gamecards.PaymentContext{Costs: map[shared.ResourceType]int{shared.ResourceCredit: cost}, Action: shared.ActionCardBuying})
		if err != nil {
			return err
		}
		plan, err := gamecards.ValidatePayment(quote, choices.Payment)
		if err != nil {
			return err
		}
		gamecards.ApplyPayment(p, plan)
	}
	if len(choices.CardIDs) > 0 {
		for _, cardID := range choices.CardIDs {
			p.Hand().AddCard(cardID)
		}
	}

	// In demo mode, override resources/production/TR with player's chosen values
	// This runs after corp effects so the player's overrides are authoritative
	if g.Settings().DemoGame {
		demoChoices := p.PendingDemoChoices()
		if demoChoices != nil {
			p.Resources().Set(demoChoices.Resources)
			p.Resources().SetProduction(demoChoices.Production)
			p.Resources().SetTerraformRating(demoChoices.TerraformRating)
		}
	}

	g.MarkCorpApplied(playerID)

	log.Debug("Corporation effects and card purchase complete",
		slog.String("player_id", playerID),
		slog.String("corporation", corpCard.Name))

	return nil
}

// ApplyNextPreludeForPlayer applies the player's next unapplied prelude during the
// init_apply_prelude phase. Preludes are applied one at a time so each card's tile
// placements and selections resolve before the next card is played.
func ApplyNextPreludeForPlayer(ctx context.Context, g *game.Game, playerID string, cardRegistry gamecards.CardRegistry, stateRepo game.GameStateRepository, log *slog.Logger) error {
	choices := g.GetDeferredStartingChoices(playerID)
	if choices == nil {
		return fmt.Errorf("no deferred starting choices for player %s", playerID)
	}
	if choices.PreludesDone() {
		return nil
	}

	p, err := g.GetPlayer(playerID)
	if err != nil {
		return fmt.Errorf("player not found: %s", playerID)
	}

	preludeID := choices.PreludeIDs[choices.PreludesAppliedCount]
	if err := ApplyPreludeCard(ctx, g, p, preludeID, cardRegistry, stateRepo, log); err != nil {
		return fmt.Errorf("failed to apply prelude %s: %w", preludeID, err)
	}
	g.MarkPreludeApplied(playerID)

	log.Debug("Prelude applied",
		slog.String("player_id", playerID),
		slog.String("prelude_id", preludeID))
	return nil
}

// ApplyPreludeCard applies a single prelude card's effects: adds to played cards,
// processes auto behaviors, registers trigger effects and manual actions.
func ApplyPreludeCard(ctx context.Context, g *game.Game, p *player.Player, preludeID string, cardRegistry gamecards.CardRegistry, stateRepo game.GameStateRepository, log *slog.Logger) error {
	card, err := cardRegistry.GetByID(preludeID)
	if err != nil {
		return fmt.Errorf("prelude card not found: %s", preludeID)
	}

	if card.Type != gamecards.CardTypePrelude {
		return fmt.Errorf("card %s is not a prelude card", preludeID)
	}

	tags := make([]string, len(card.Tags))
	for i, tag := range card.Tags {
		tags[i] = string(tag)
	}
	p.PlayedCards().AddCard(card.ID, card.Name, string(card.Type), tags)
	if card.ResourceStorage != nil {
		g.Colonies().ActivateResource(string(card.ResourceStorage.Type))
	}

	for behaviorIndex, behavior := range card.Behaviors {
		if !gamecards.HasAutoTrigger(behavior) {
			continue
		}

		_, outputs := behavior.ExtractInputsOutputs(nil)

		applier := gamecards.NewBehaviorApplier(p, g, card.Name, slog.Default()).
			WithSourceCardID(card.ID).
			WithCardRegistry(cardRegistry).
			WithSourceType(shared.SourceTypeCardPlay)

		_, err := applier.ApplyOutputsAndGetCalculated(ctx, outputs)
		if err != nil {
			return fmt.Errorf("failed to apply prelude behavior %d: %w", behaviorIndex, err)
		}

		if gamecards.HasPersistentEffects(behavior) {
			effect := shared.CardEffect{
				CardID:        card.ID,
				CardName:      card.Name,
				BehaviorIndex: behaviorIndex,
				Behavior:      behavior,
			}
			p.Effects().AddEffect(effect)

			events.Publish(g.EventBus(), events.PlayerEffectsChangedEvent{
				GameID:    g.ID(),
				PlayerID:  p.ID(),
				Timestamp: time.Now(),
			})
		}
	}

	for behaviorIndex, behavior := range card.Behaviors {
		if !gamecards.HasConditionalTrigger(behavior) {
			continue
		}

		effect := shared.CardEffect{
			CardID:        card.ID,
			CardName:      card.Name,
			BehaviorIndex: behaviorIndex,
			Behavior:      behavior,
		}
		p.Effects().AddEffect(effect)
		baseaction.SubscribePassiveEffectToEvents(ctx, g, p, effect, log, cardRegistry)
	}

	for behaviorIndex, behavior := range card.Behaviors {
		if !gamecards.HasManualTrigger(behavior) {
			continue
		}

		p.Actions().AddAction(shared.CardAction{
			CardID:        card.ID,
			CardName:      card.Name,
			BehaviorIndex: behaviorIndex,
			Behavior:      behavior,
		})
	}

	baseaction.ActivateSelfTagTriggers(g, p, card, cardRegistry, log)

	if stateRepo != nil {
		description := fmt.Sprintf("Played prelude %s", card.Name)
		displayData := baseaction.BuildCardDisplayData(card, shared.SourceTypeCardPlay)
		if _, err := stateRepo.WriteFull(ctx, g.ID(), g, card.Name, shared.SourceTypeCardPlay, p.ID(), description, nil, nil, displayData); err != nil {
			log.Warn("Failed to write prelude state log", slog.String("card", card.Name), slog.Any("error", err))
		}
	}

	return nil
}

// AdvanceToActionPhase transitions the game to the action phase and sets the first player's turn.
func AdvanceToActionPhase(ctx context.Context, g *game.Game, allPlayers []*player.Player, log *slog.Logger) {
	if err := g.UpdatePhase(ctx, shared.GamePhaseAction); err != nil {
		log.Error("Failed to transition game phase", slog.Any("error", err))
		return
	}

	activePlayerCount := 0
	for _, p := range allPlayers {
		if !p.HasExited() {
			activePlayerCount++
		}
	}

	turnOrder := g.TurnOrder()
	if len(turnOrder) > 0 {
		firstPlayerID := findFirstActivePlayer(g, turnOrder)
		if firstPlayerID == "" {
			return
		}

		availableActions := 2
		if activePlayerCount == 1 {
			availableActions = -1
			log.Debug("Solo mode detected - setting unlimited actions")
		}

		if err := g.SetCurrentTurn(ctx, firstPlayerID, availableActions); err != nil {
			log.Error("Failed to set current turn", slog.Any("error", err))
			return
		}

		log.Debug("Set first player turn with actions",
			slog.String("first_player_id", firstPlayerID),
			slog.Int("available_actions", availableActions))
	}
}

func findFirstActivePlayer(g *game.Game, turnOrder []string) string {
	for _, id := range turnOrder {
		p, err := g.GetPlayer(id)
		if err == nil && !p.HasExited() {
			return p.ID()
		}
	}
	return ""
}
