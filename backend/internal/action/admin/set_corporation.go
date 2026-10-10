package admin

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
)

// SetCorporationAction handles the admin action to set a player's corporation
type SetCorporationAction struct {
	gameRepo      game.GameRepository
	cardRegistry  gamecards.CardRegistry
	awardRegistry award.AwardRegistry
	corpProc      *gamecards.CorporationProcessor
	logger        *slog.Logger
}

// NewSetCorporationAction creates a new set corporation admin action
func NewSetCorporationAction(
	gameRepo game.GameRepository,
	cardRegistry gamecards.CardRegistry,
	awardRegistry award.AwardRegistry,
	logger *slog.Logger,
) *SetCorporationAction {
	return &SetCorporationAction{
		gameRepo:      gameRepo,
		cardRegistry:  cardRegistry,
		awardRegistry: awardRegistry,
		corpProc:      gamecards.NewCorporationProcessor(cardRegistry, awardRegistry, slog.Default()),
		logger:        logger,
	}
}

// Execute performs the set corporation admin action
func (a *SetCorporationAction) Execute(ctx context.Context, gameID string, playerID string, corporationID string) error {
	log := a.logger.With(
		slog.String("game_id", gameID),
		slog.String("player_id", playerID),
		slog.String("action", "admin_set_corporation"),
		slog.String("corporation_id", corporationID),
	)
	log.Debug("Admin: Setting player corporation")

	g, err := a.gameRepo.Get(ctx, gameID)
	if err != nil {
		log.Error("Failed to get game", slog.Any("error", err))
		return fmt.Errorf("game not found: %s", gameID)
	}

	player, err := g.GetPlayer(playerID)
	if err != nil {
		log.Error("Player not found in game", slog.Any("error", err))
		return fmt.Errorf("player not found: %s", playerID)
	}

	corpCard, err := gamecards.ForGame(a.cardRegistry, g).GetByID(corporationID)
	if err != nil {
		log.Error("Failed to fetch corporation card", slog.Any("error", err))
		return fmt.Errorf("corporation card not found: %s", corporationID)
	}

	if corpCard.Type != gamecards.CardTypeCorporation {
		log.Error("Card is not a corporation", slog.String("card_type", string(corpCard.Type)))
		return fmt.Errorf("card %s is not a corporation card", corporationID)
	}

	oldCorpID := player.CorporationID()
	if oldCorpID != "" {
		log.Debug("Clearing old corporation effects", slog.String("old_corporation_id", oldCorpID))

		player.Effects().RemoveEffectsByCardID(oldCorpID)
		player.Actions().RemoveActionsByCardID(oldCorpID)
		player.Resources().RemoveCardStorage(oldCorpID)
		player.Resources().RemovePaymentSubstitutes(oldCorpID)
		player.Resources().ClearValueModifiers()
		player.VPGranters().RemoveByCardID(oldCorpID)

		log.Debug("Old corporation effects cleared")
	}

	player.SetCorporationID(corporationID)

	if corpCard.ResourceStorage != nil {
		g.Colonies().ActivateResource(string(corpCard.ResourceStorage.Type))
		player.Resources().AddToStorage(corporationID, corpCard.ResourceStorage.Starting)
	}

	log.Debug("Corporation ID set", slog.String("corporation_name", corpCard.Name))

	// Register trigger effects BEFORE applying starting effects so that
	// production-increased triggers (e.g. Manutech) fire on starting production
	triggerEffects := a.corpProc.GetTriggerEffects(corpCard)
	for _, effect := range triggerEffects {
		player.Effects().AddEffect(effect)
		log.Debug("Registered trigger effect",
			slog.String("card_name", effect.CardName),
			slog.Int("behavior_index", effect.BehaviorIndex))

		baseaction.SubscribePassiveEffectToEvents(ctx, g, player, effect, log, gamecards.ForGame(a.cardRegistry, g))
	}

	if err := a.corpProc.ApplyStartingEffects(ctx, corpCard, player, g); err != nil {
		log.Error("Failed to apply corporation starting effects", slog.Any("error", err))
		return fmt.Errorf("failed to apply corporation starting effects: %w", err)
	}

	if err := a.corpProc.ApplyAutoEffects(ctx, corpCard, player, g); err != nil {
		log.Error("Failed to apply corporation auto effects", slog.Any("error", err))
		return fmt.Errorf("failed to apply corporation auto effects: %w", err)
	}

	autoEffects := a.corpProc.GetAutoEffects(corpCard)
	for _, effect := range autoEffects {
		player.Effects().AddEffect(effect)
		log.Debug("Registered auto effect",
			slog.String("card_name", effect.CardName),
			slog.Int("behavior_index", effect.BehaviorIndex))
	}

	// Publish TagPlayedEvent for each corporation tag (triggers Saturn Systems, etc.)
	for _, tag := range corpCard.Tags {
		events.Publish(g.EventBus(), events.TagPlayedEvent{
			GameID:    gameID,
			PlayerID:  playerID,
			CardID:    corporationID,
			CardName:  corpCard.Name,
			Tag:       string(tag),
			Timestamp: time.Now(),
		})
	}

	g.RegisterCorporationVPGranter(playerID, corporationID)

	manualActions := a.corpProc.GetManualActions(corpCard)
	for _, action := range manualActions {
		player.Actions().AddAction(action)
		log.Debug("Registered manual action",
			slog.String("card_name", action.CardName),
			slog.Int("behavior_index", action.BehaviorIndex))
	}

	if err := a.corpProc.SetupForcedFirstAction(ctx, corpCard, g, playerID); err != nil {
		log.Error("Failed to setup forced first action", slog.Any("error", err))
		return fmt.Errorf("failed to setup forced first action: %w", err)
	}

	log.Info("Admin set corporation completed with all effects applied",
		slog.String("corporation_name", corpCard.Name))
	return nil
}
