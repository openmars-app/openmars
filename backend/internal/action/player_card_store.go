package action

import (
	"log/slog"
	"openmars/internal/events"
	"openmars/internal/game"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/player"
	"openmars/internal/logger"
)

// SetupPlayerCardStore wires event-driven state calculation for a player's hand cards.
// Subscribes to game events so that EntityState for each hand card is automatically
// computed and kept in sync. Must be called once per player after creation.
func SetupPlayerCardStore(p *player.Player, g *game.Game, cardRegistry gamecards.CardRegistry, colonyBonusLookup ...gamecards.ColonyBonusLookup) {
	eventBus := g.EventBus()
	store := p.CardStateStore()

	var lookup gamecards.ColonyBonusLookup
	if len(colonyBonusLookup) > 0 {
		lookup = colonyBonusLookup[0]
	}

	recalculate := func(cardID string) player.EntityState {
		card, err := gamecards.ForGame(cardRegistry, g).GetByID(cardID)
		if err != nil {
			logger.Get().Warn("Card not found in registry during recalculation",
				slog.String("card_id", cardID), slog.Any("error", err))
			return player.EntityState{}
		}
		return CalculatePlayerCardState(card, p, g, cardRegistry, lookup)
	}

	recalculateAll := func() {
		store.RecalculateAll(recalculate)
	}

	// When a card is added to hand, calculate its initial state
	events.Subscribe(eventBus, func(e events.CardAddedToHandEvent) {
		if e.PlayerID != p.ID() {
			return
		}
		card, err := gamecards.ForGame(cardRegistry, g).GetByID(e.CardID)
		if err != nil {
			logger.Get().Warn("Card not found in registry",
				slog.String("card_id", e.CardID), slog.Any("error", err))
			return
		}
		state := CalculatePlayerCardState(card, p, g, cardRegistry, lookup)
		store.SetState(e.CardID, state)
	})

	// When hand changes, remove stale entries
	events.Subscribe(eventBus, func(e events.CardHandUpdatedEvent) {
		if e.PlayerID != p.ID() {
			return
		}
		store.SyncWithHand(e.CardIDs)
	})

	// Recalculate all cards when game state changes
	events.Subscribe(eventBus, func(e events.ResourcesChangedEvent) {
		if e.PlayerID == p.ID() {
			recalculateAll()
		}
	})

	events.Subscribe(eventBus, func(_ events.TemperatureChangedEvent) {
		recalculateAll()
	})

	events.Subscribe(eventBus, func(_ events.OxygenChangedEvent) {
		recalculateAll()
	})

	events.Subscribe(eventBus, func(_ events.OceansChangedEvent) {
		recalculateAll()
	})

	events.Subscribe(eventBus, func(_ events.VenusChangedEvent) {
		recalculateAll()
	})

	events.Subscribe(eventBus, func(_ events.ResourceStorageChangedEvent) {
		recalculateAll()
	})

	events.Subscribe(eventBus, func(e events.PlayerEffectsChangedEvent) {
		if e.PlayerID == p.ID() {
			recalculateAll()
		}
	})

	events.Subscribe(eventBus, func(e events.GamePhaseChangedEvent) {
		if e.GameID == g.ID() {
			recalculateAll()
		}
	})

	events.Subscribe(eventBus, func(_ events.GameStateChangedEvent) {
		recalculateAll()
	})

	events.Subscribe(eventBus, func(e events.CardPlayedEvent) {
		if e.GameID == g.ID() && e.PlayerID == p.ID() {
			recalculateAll()
		}
	})

	events.Subscribe(eventBus, func(e events.ProductionChangedEvent) {
		if e.PlayerID == p.ID() {
			recalculateAll()
		}
	})

	events.Subscribe(eventBus, func(e events.TilePlacedEvent) {
		if e.GameID == g.ID() {
			recalculateAll()
		}
	})

	events.Subscribe(eventBus, func(e events.ColonyBuiltEvent) {
		if e.GameID == g.ID() {
			recalculateAll()
		}
	})

	events.Subscribe(eventBus, func(e events.PlayerSelectionChangedEvent) {
		if e.PlayerID == p.ID() {
			recalculateAll()
		}
	})
}
