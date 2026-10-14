package player

import (
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"

	"openmars/internal/events"
	"openmars/internal/game/datastore"
	"openmars/internal/game/shared"
)

// Hand manages player card hand as a pure DataStore adapter.
type Hand struct {
	ds       *datastore.DataStore
	eventBus *events.EventBusImpl
	gameID   string
	playerID string
}

func newHand(ds *datastore.DataStore, eventBus *events.EventBusImpl, gameID, playerID string) *Hand {
	return &Hand{
		ds:       ds,
		eventBus: eventBus,
		gameID:   gameID,
		playerID: playerID,
	}
}

func (h *Hand) update(fn func(s *datastore.PlayerState)) {
	if err := h.ds.UpdatePlayer(h.gameID, h.playerID, fn); err != nil {
		slog.Default().Warn("Failed to update player state", slog.String("game_id", h.gameID), slog.String("player_id", h.playerID), slog.Any("error", err))
	}
}

func (h *Hand) read(fn func(s *datastore.PlayerState)) {
	if err := h.ds.ReadPlayer(h.gameID, h.playerID, fn); err != nil {
		slog.Default().Warn("Failed to read player state", slog.String("game_id", h.gameID), slog.String("player_id", h.playerID), slog.Any("error", err))
	}
}

func (h *Hand) Cards() []string {
	var cardsCopy []string
	h.read(func(s *datastore.PlayerState) {
		cardsCopy = make([]string, len(s.HandCardIDs))
		copy(cardsCopy, s.HandCardIDs)
	})
	return cardsCopy
}

func (h *Hand) CardCount() int {
	var count int
	h.read(func(s *datastore.PlayerState) {
		count = len(s.HandCardIDs)
	})
	return count
}

func (h *Hand) HasCard(cardID string) bool {
	var found bool
	h.read(func(s *datastore.PlayerState) {
		for _, id := range s.HandCardIDs {
			if id == cardID {
				found = true
				return
			}
		}
	})
	return found
}

func (h *Hand) SetCards(cards []string) {
	var cardsCopy []string
	h.update(func(s *datastore.PlayerState) {
		if cards == nil {
			s.HandCardIDs = []string{}
		} else {
			s.HandCardIDs = make([]string, len(cards))
			copy(s.HandCardIDs, cards)
		}
		cardsCopy = make([]string, len(s.HandCardIDs))
		copy(cardsCopy, s.HandCardIDs)
	})

	if h.eventBus != nil {
		events.Publish(h.eventBus, events.CardHandUpdatedEvent{
			GameID:    h.gameID,
			PlayerID:  h.playerID,
			CardIDs:   cardsCopy,
			Timestamp: time.Now(),
		})
	}
}

// AddCard adds an already-reviewed card, such as a selected starting or research card.
// Newly received rewards use ReceiveCards to record their private acknowledgement together
// with the hand mutation. SetCards remains the state replacement operation.
func (h *Hand) AddCard(cardID string) {
	h.addCards([]string{cardID}, nil)
}

// ReceiveCards commits a previously unseen grant and its receipt in one state update.
// Empty grants and cards already in hand do not create receipts or duplicate cards.
func (h *Hand) ReceiveCards(cardIDs []string, source, sourceCardID string) {
	h.addCards(cardIDs, &shared.CardReceipt{Source: source, SourceCardID: sourceCardID})
}

func (h *Hand) addCards(cardIDs []string, receipt *shared.CardReceipt) {
	if len(cardIDs) == 0 {
		return
	}
	var added, cardsCopy []string
	if err := h.ds.UpdatePlayer(h.gameID, h.playerID, func(s *datastore.PlayerState) {
		for _, cardID := range cardIDs {
			if cardID == "" || slices.Contains(s.HandCardIDs, cardID) {
				continue
			}
			s.HandCardIDs = append(s.HandCardIDs, cardID)
			added = append(added, cardID)
		}
		if len(added) == 0 {
			return
		}
		if receipt != nil {
			receipt.ID = uuid.NewString()
			receipt.Cards = slices.Clone(added)
			s.CardReceipts = append(s.CardReceipts, *receipt)
		}
		cardsCopy = slices.Clone(s.HandCardIDs)
	}); err != nil {
		slog.Default().Warn("Failed to acquire cards", slog.String("game_id", h.gameID), slog.String("player_id", h.playerID), slog.Any("error", err))
		return
	}
	if len(added) == 0 || h.eventBus == nil {
		return
	}
	for _, cardID := range added {
		events.Publish(h.eventBus, events.CardAddedToHandEvent{
			GameID: h.gameID, PlayerID: h.playerID, CardID: cardID, Timestamp: time.Now(),
		})
	}
	events.Publish(h.eventBus, events.CardHandUpdatedEvent{
		GameID: h.gameID, PlayerID: h.playerID, CardIDs: cardsCopy, Timestamp: time.Now(),
	})
	if receipt != nil {
		events.Publish(h.eventBus, events.PlayerSelectionChangedEvent{GameID: h.gameID, PlayerID: h.playerID})
	}
}

func (h *Hand) RemoveCard(cardID string) bool {
	var removed bool
	var cardsCopy []string
	h.update(func(s *datastore.PlayerState) {
		for i, id := range s.HandCardIDs {
			if id == cardID {
				s.HandCardIDs = append(s.HandCardIDs[:i], s.HandCardIDs[i+1:]...)
				removed = true
				break
			}
		}
		cardsCopy = make([]string, len(s.HandCardIDs))
		copy(cardsCopy, s.HandCardIDs)
	})

	if removed && h.eventBus != nil {
		events.Publish(h.eventBus, events.CardHandUpdatedEvent{
			GameID:    h.gameID,
			PlayerID:  h.playerID,
			CardIDs:   cardsCopy,
			Timestamp: time.Now(),
		})
	}

	return removed
}
