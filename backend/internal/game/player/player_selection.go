package player

import (
	"github.com/google/uuid"
	"log/slog"
	"openmars/internal/events"
	"openmars/internal/game/datastore"
	"openmars/internal/game/shared"
)

// Selection manages player-specific card selection state.
type Selection struct {
	ds       *datastore.DataStore
	eventBus *events.EventBusImpl
	gameID   string
	playerID string
}

func newSelection(ds *datastore.DataStore, eventBus *events.EventBusImpl, gameID, playerID string) *Selection {
	return &Selection{
		ds:       ds,
		eventBus: eventBus,
		gameID:   gameID,
		playerID: playerID,
	}
}

func (s *Selection) update(fn func(st *datastore.PlayerState)) {
	if err := s.ds.UpdatePlayer(s.gameID, s.playerID, fn); err != nil {
		slog.Default().Warn("Failed to update player state", slog.String("game_id", s.gameID), slog.String("player_id", s.playerID), slog.Any("error", err))
	}
	if s.eventBus != nil {
		events.Publish(s.eventBus, events.PlayerSelectionChangedEvent{
			GameID:   s.gameID,
			PlayerID: s.playerID,
		})
	}
}

func (s *Selection) read(fn func(st *datastore.PlayerState)) {
	if err := s.ds.ReadPlayer(s.gameID, s.playerID, fn); err != nil {
		slog.Default().Warn("Failed to read player state", slog.String("game_id", s.gameID), slog.String("player_id", s.playerID), slog.Any("error", err))
	}
}

func (s *Selection) GetSelectCorporationPhase() *shared.SelectCorporationPhase {
	var phase *shared.SelectCorporationPhase
	s.read(func(st *datastore.PlayerState) {
		phase = st.SelectCorporationPhase
	})
	return phase
}

func (s *Selection) SetSelectCorporationPhase(phase *shared.SelectCorporationPhase) {
	s.update(func(st *datastore.PlayerState) {
		st.SelectCorporationPhase = phase
	})
}

func (s *Selection) GetSelectStartingCardsPhase() *shared.SelectStartingCardsPhase {
	var phase *shared.SelectStartingCardsPhase
	s.read(func(st *datastore.PlayerState) {
		phase = st.SelectStartingCardsPhase
	})
	return phase
}

func (s *Selection) SetSelectStartingCardsPhase(phase *shared.SelectStartingCardsPhase) {
	s.update(func(st *datastore.PlayerState) {
		st.SelectStartingCardsPhase = phase
	})
}

func (s *Selection) GetSelectPreludeCardsPhase() *shared.SelectPreludeCardsPhase {
	var phase *shared.SelectPreludeCardsPhase
	s.read(func(st *datastore.PlayerState) {
		phase = st.SelectPreludeCardsPhase
	})
	return phase
}

func (s *Selection) SetSelectPreludeCardsPhase(phase *shared.SelectPreludeCardsPhase) {
	s.update(func(st *datastore.PlayerState) {
		st.SelectPreludeCardsPhase = phase
	})
}

func (s *Selection) GetPendingCardSelection() *shared.PendingCardSelection {
	var sel *shared.PendingCardSelection
	s.read(func(st *datastore.PlayerState) {
		sel = st.PendingCardSelection
	})
	return sel
}

func (s *Selection) SetPendingCardSelection(selection *shared.PendingCardSelection) {
	s.update(func(st *datastore.PlayerState) {
		st.PendingCardSelection = selection
	})
}

func (s *Selection) GetPendingCardPlaySelection() *shared.PendingCardPlaySelection {
	var result *shared.PendingCardPlaySelection
	s.read(func(st *datastore.PlayerState) {
		if st.PendingCardPlaySelection != nil {
			cp := *st.PendingCardPlaySelection
			cp.Selectors = shared.CloneSelectors(cp.Selectors)
			result = &cp
		}
	})
	return result
}
func (s *Selection) SetPendingCardPlaySelection(selection *shared.PendingCardPlaySelection) {
	s.update(func(st *datastore.PlayerState) { st.PendingCardPlaySelection = selection })
}

func (s *Selection) GetPendingCardDrawSelection() *shared.PendingCardDrawSelection {
	var sel *shared.PendingCardDrawSelection
	s.read(func(st *datastore.PlayerState) {
		sel = st.PendingCardDrawSelection
	})
	return sel
}

func (s *Selection) SetPendingCardDrawSelection(selection *shared.PendingCardDrawSelection) {
	s.update(func(st *datastore.PlayerState) {
		st.PendingCardDrawSelection = selection
	})
}

func (s *Selection) GetPendingBehaviorResolutions() []*shared.PendingBehaviorResolution {
	var result []*shared.PendingBehaviorResolution
	s.read(func(st *datastore.PlayerState) {
		result = append([]*shared.PendingBehaviorResolution{}, st.PendingBehaviorResolutions...)
	})
	return result
}

func (s *Selection) GetPendingBehaviorResolution(id string) *shared.PendingBehaviorResolution {
	for _, resolution := range s.GetPendingBehaviorResolutions() {
		if resolution.ID == id {
			return resolution
		}
	}
	return nil
}

func (s *Selection) AddPendingBehaviorResolution(resolution *shared.PendingBehaviorResolution) string {
	resolution.ID = uuid.NewString()
	s.update(func(st *datastore.PlayerState) {
		st.PendingBehaviorResolutions = append(st.PendingBehaviorResolutions, resolution)
	})
	return resolution.ID
}

func (s *Selection) RemovePendingBehaviorResolution(id string) {
	s.update(func(st *datastore.PlayerState) {
		for i, resolution := range st.PendingBehaviorResolutions {
			if resolution.ID == id {
				st.PendingBehaviorResolutions = append(st.PendingBehaviorResolutions[:i], st.PendingBehaviorResolutions[i+1:]...)
				return
			}
		}
	})
}

func (s *Selection) ClearPendingBehaviorResolutions() {
	s.update(func(st *datastore.PlayerState) { st.PendingBehaviorResolutions = nil })
}

func (s *Selection) GetPendingResourceRemovalSelection() *shared.PendingResourceRemovalSelection {
	var sel *shared.PendingResourceRemovalSelection
	s.read(func(st *datastore.PlayerState) {
		sel = st.PendingResourceRemovalSelection
	})
	return sel
}

func (s *Selection) SetPendingResourceRemovalSelection(selection *shared.PendingResourceRemovalSelection) {
	s.update(func(st *datastore.PlayerState) {
		st.PendingResourceRemovalSelection = selection
	})
}

func (s *Selection) GetPendingColonyResourceSelection() *shared.PendingColonyResourceSelection {
	var sel *shared.PendingColonyResourceSelection
	s.read(func(st *datastore.PlayerState) {
		sel = st.PendingColonyResourceSelection
	})
	return sel
}

func (s *Selection) SetPendingColonyResourceSelection(selection *shared.PendingColonyResourceSelection) {
	s.update(func(st *datastore.PlayerState) {
		st.PendingColonyResourceSelection = selection
	})
}

func (s *Selection) GetPendingColonyResourceQueue() []shared.PendingColonyResourceSelection {
	var queue []shared.PendingColonyResourceSelection
	s.read(func(st *datastore.PlayerState) {
		queue = st.PendingColonyResourceQueue
	})
	return queue
}

func (s *Selection) AppendPendingColonyResource(selection shared.PendingColonyResourceSelection) {
	s.update(func(st *datastore.PlayerState) {
		st.PendingColonyResourceQueue = append(st.PendingColonyResourceQueue, selection)
	})
}

func (s *Selection) PopPendingColonyResource() *shared.PendingColonyResourceSelection {
	// Check first without triggering event
	var hasItems bool
	s.read(func(st *datastore.PlayerState) {
		hasItems = len(st.PendingColonyResourceQueue) > 0
	})
	if !hasItems {
		return nil
	}
	var result *shared.PendingColonyResourceSelection
	s.update(func(st *datastore.PlayerState) {
		if len(st.PendingColonyResourceQueue) == 0 {
			return
		}
		first := st.PendingColonyResourceQueue[0]
		result = &first
		st.PendingColonyResourceQueue = st.PendingColonyResourceQueue[1:]
	})
	return result
}

func (s *Selection) GetPendingAwardFundSelection() *shared.PendingAwardFundSelection {
	var sel *shared.PendingAwardFundSelection
	s.read(func(st *datastore.PlayerState) {
		sel = st.PendingAwardFundSelection
	})
	return sel
}

func (s *Selection) SetPendingAwardFundSelection(selection *shared.PendingAwardFundSelection) {
	s.update(func(st *datastore.PlayerState) {
		st.PendingAwardFundSelection = selection
	})
}

func (s *Selection) GetPendingColonySelection() *shared.PendingColonySelection {
	var sel *shared.PendingColonySelection
	s.read(func(st *datastore.PlayerState) {
		sel = st.PendingColonySelection
	})
	return sel
}

func (s *Selection) SetPendingColonySelection(selection *shared.PendingColonySelection) {
	s.update(func(st *datastore.PlayerState) {
		st.PendingColonySelection = selection
	})
}

func (s *Selection) GetPendingFreeTradeSelection() *shared.PendingFreeTradeSelection {
	var sel *shared.PendingFreeTradeSelection
	s.read(func(st *datastore.PlayerState) {
		sel = st.PendingFreeTradeSelection
	})
	return sel
}

func (s *Selection) SetPendingFreeTradeSelection(selection *shared.PendingFreeTradeSelection) {
	s.update(func(st *datastore.PlayerState) {
		st.PendingFreeTradeSelection = selection
	})
}

// HasPendingSelection returns true if the player has any pending action selection.
// Does not include setup-phase selections (corporation, starting cards, prelude, production).
func (s *Selection) HasPendingSelection() bool {
	var has bool
	s.read(func(st *datastore.PlayerState) {
		has = st.PendingCardPlaySelection != nil || st.PendingCardSelection != nil ||
			st.PendingCardReveal != nil ||
			st.PendingCardDrawSelection != nil ||
			len(st.PendingBehaviorResolutions) > 0 ||
			st.PendingResourceRemovalSelection != nil ||
			len(st.PendingColonyResourceQueue) > 0 ||
			st.PendingAwardFundSelection != nil ||
			st.PendingColonySelection != nil ||
			st.PendingFreeTradeSelection != nil || st.PendingEffectSelection != nil
	})
	return has
}

func (s *Selection) GetPendingEffectSelection() *shared.PendingEffectSelection {
	var result *shared.PendingEffectSelection
	s.read(func(st *datastore.PlayerState) { result = st.PendingEffectSelection })
	return result
}
func (s *Selection) SetPendingEffectSelection(selection *shared.PendingEffectSelection) {
	s.update(func(st *datastore.PlayerState) { st.PendingEffectSelection = selection })
}

func (s *Selection) GetPendingCardReveal() *shared.PendingCardReveal {
	var result *shared.PendingCardReveal
	s.read(func(st *datastore.PlayerState) { result = st.PendingCardReveal })
	return result
}
func (s *Selection) SetPendingCardReveal(reveal *shared.PendingCardReveal) {
	s.update(func(st *datastore.PlayerState) { st.PendingCardReveal = reveal })
}

// CardReceipts returns unacknowledged card grants for this player.
func (s *Selection) CardReceipts() []shared.CardReceipt {
	var result []shared.CardReceipt
	s.read(func(st *datastore.PlayerState) { result = append([]shared.CardReceipt{}, st.CardReceipts...) })
	return result
}

// AcknowledgeCardReceipt only dismisses the named receipt, including on repeated requests.
func (s *Selection) AcknowledgeCardReceipt(id string) {
	s.update(func(st *datastore.PlayerState) {
		for i, receipt := range st.CardReceipts {
			if receipt.ID == id {
				st.CardReceipts = append(st.CardReceipts[:i], st.CardReceipts[i+1:]...)
				break
			}
		}
	})
}
