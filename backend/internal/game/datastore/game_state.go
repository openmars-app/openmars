package datastore

import (
	"time"

	"openmars/internal/game/board"
	"openmars/internal/game/colony"
	"openmars/internal/game/projectfunding"
	"openmars/internal/game/shared"
)

// GameState holds all game data.
type GameState struct {
	ModeRuntime  shared.ModeRuntimeState `json:"modeRuntime" save:"state"`
	ID           string                  `json:"id" save:"state"`
	CreatedAt    time.Time               `json:"createdAt" save:"state"`
	UpdatedAt    time.Time               `json:"updatedAt" save:"state"`
	Status       shared.GameStatus       `json:"status" save:"state"`
	Settings     shared.GameSettings     `json:"settings" save:"state"`
	HostPlayerID string                  `json:"hostPlayerId" save:"state"`

	// Seed is the master RNG seed for this game. All randomness (deck shuffles,
	// turn order, milestone/award/colony selection) is derived deterministically
	// from it, so a game can be reproduced from (Seed + the sequence of actions).
	Seed uint64 `json:"seed,string" save:"state"`

	CurrentPhase shared.GamePhase `json:"currentPhase" save:"state"`
	Generation   int              `json:"generation" save:"state"`

	Temperature int `json:"temperature" save:"state"`
	Oxygen      int `json:"oxygen" save:"state"`
	Oceans      int `json:"oceans" save:"state"`
	MaxOceans   int `json:"maxOceans" save:"state"`
	Venus       int `json:"venus" save:"state"`

	CurrentTurnPlayerID     string `json:"currentTurnPlayerId" save:"state"` // empty = no current turn
	CurrentTurnActions      int    `json:"currentTurnActions" save:"state"`  // -1 = unlimited, 0 = none, >0 = specific count
	CurrentTurnTotalActions int    `json:"currentTurnTotalActions" save:"state"`
	GlobalActionCounter     int    `json:"globalActionCounter" save:"state"`

	Tiles []board.Tile `json:"tiles" save:"state"`

	ProjectCards   []string `json:"projectCards" save:"state"`
	Corporations   []string `json:"corporations" save:"state"`
	DiscardPile    []string `json:"discardPile" save:"state"`
	RemovedCards   []string `json:"removedCards" save:"state"`
	PreludeCards   []string `json:"preludeCards" save:"state"`
	DrawnCardCount int      `json:"drawnCardCount" save:"state"`
	ShuffleCount   int      `json:"shuffleCount" save:"state"`

	PlayerOrder []string                `json:"playerOrder" save:"state"`
	TurnOrder   []string                `json:"turnOrder" save:"state"`
	Players     map[string]*PlayerState `json:"players" save:"state"`

	ClaimedMilestones  []shared.ClaimedMilestone `json:"claimedMilestones" save:"state"`
	FundedAwards       []shared.FundedAward      `json:"fundedAwards" save:"state"`
	SelectedMilestones []string                  `json:"selectedMilestones" save:"state"`
	SelectedAwards     []string                  `json:"selectedAwards" save:"state"`

	FinalScores []shared.FinalScore `json:"finalScores" save:"state"`
	WinnerID    string              `json:"winnerId" save:"state"`
	IsTie       bool                `json:"isTie" save:"state"`

	Spectators   map[string]*shared.SpectatorState `json:"-" save:"session"`
	ChatMessages []shared.ChatMessage              `json:"chatMessages" save:"state"`

	// BotSpendUSD is the total LLM spend of all bots in this game.
	BotSpendUSD float64 `json:"botSpendUsd" save:"state"`

	PendingTileSelections      map[string]*shared.PendingTileSelection      `json:"pendingTileSelections" save:"state"`
	PendingTileSelectionQueues map[string]*shared.PendingTileSelectionQueue `json:"pendingTileSelectionQueues" save:"state"`
	ForcedFirstActions         map[string]*shared.ForcedFirstAction         `json:"forcedFirstActions" save:"state"`
	ProductionPhases           map[string]*shared.ProductionPhase           `json:"productionPhases" save:"state"`
	SelectCorporationPhases    map[string]*shared.SelectCorporationPhase    `json:"selectCorporationPhases" save:"state"`
	SelectStartingCardsPhases  map[string]*shared.SelectStartingCardsPhase  `json:"selectStartingCardsPhases" save:"state"`
	SelectPreludeCardsPhases   map[string]*shared.SelectPreludeCardsPhase   `json:"selectPreludeCardsPhases" save:"state"`
	DeferredStartingChoices    map[string]*shared.DeferredStartingChoices   `json:"deferredStartingChoices" save:"state"`

	InitPhasePlayerIndex       int  `json:"initPhasePlayerIndex" save:"state"`
	InitPhaseWaitingForConfirm bool `json:"initPhaseWaitingForConfirm" save:"state"`
	InitPhaseConfirmVersion    int  `json:"initPhaseConfirmVersion" save:"state"`
	InitPhaseRoster            bool `json:"initPhaseRoster" save:"state"`

	NextGenTurnOrderFrozen bool `json:"nextGenTurnOrderFrozen" save:"state"`

	ColonyStates         []*colony.ColonyState          `json:"colonyStates" save:"state"`
	TradeFleets          map[string]colony.TradeFleet   `json:"tradeFleets" save:"state"`
	ProjectFundingStates []*projectfunding.ProjectState `json:"projectFundingStates" save:"state"`

	TriggeredEffects []shared.TriggeredEffect `json:"-" save:"session"`
}

// GameStateHistoryEntry stores a full GameState snapshot at a point in time.
type GameStateHistoryEntry struct {
	GameID       string                        `json:"gameId" save:"state"`
	Sequence     int64                         `json:"sequence" save:"state"`
	Timestamp    time.Time                     `json:"timestamp" save:"state"`
	State        *GameState                    `json:"state" save:"state"`
	VPBreakdowns map[string]shared.VPBreakdown `json:"vpBreakdowns" save:"state"`
}

// PlayerState holds a single player's data.
type PlayerState struct {
	ProductionContributions []shared.ProductionContribution         `json:"productionContributions" save:"state"`
	DependencyStatuses      map[string]shared.DependencyStatus      `json:"dependencyStatuses" save:"state"`
	DependencyGroups        map[string]shared.DependencyGroup       `json:"dependencyGroups" save:"state"`
	BehaviorUses            map[string]int                          `json:"behaviorUses" save:"state"`
	PendingEffectSelection  *shared.PendingEffectSelection          `json:"pendingEffectSelection" save:"state"`
	PendingCardReveal       *shared.PendingCardReveal               `json:"pendingCardReveal" save:"state"`
	ResolvedProductionBoxes map[string][]shared.ProductionCondition `json:"resolvedProductionBoxes" save:"state"`
	ID                      string                                  `json:"id" save:"state"`
	Name                    string                                  `json:"name" save:"state"`
	Connected               bool                                    `json:"-" save:"session"`
	PlayerType              string                                  `json:"playerType" save:"state"`
	BotStatus               string                                  `json:"-" save:"session"`
	BotPersona              string                                  `json:"botPersona" save:"state"`
	BotError                string                                  `json:"-" save:"session"`

	CorporationID      string                     `json:"corporationId" save:"state"`
	Color              string                     `json:"color" save:"state"`
	HasPassed          bool                       `json:"hasPassed" save:"state"`
	HasExited          bool                       `json:"hasExited" save:"state"`
	PendingDemoChoices *shared.PendingDemoChoices `json:"pendingDemoChoices" save:"state"`

	HandCardIDs   []string `json:"handCardIds" save:"state"`
	PlayedCardIDs []string `json:"playedCardIds" save:"state"`

	Resources       shared.Resources  `json:"resources" save:"state"`
	Production      shared.Production `json:"production" save:"state"`
	TerraformRating int               `json:"terraformRating" save:"state"`

	ResourceStorage map[string]int `json:"resourceStorage" save:"state"`

	PaymentSubstitutes []shared.PaymentSubstitute  `json:"paymentSubstitutes" save:"state"`
	ValueModifiers     map[shared.ResourceType]int `json:"valueModifiers" save:"state"`

	SelectCorporationPhase          *shared.SelectCorporationPhase          `json:"selectCorporationPhase" save:"state"`
	SelectStartingCardsPhase        *shared.SelectStartingCardsPhase        `json:"selectStartingCardsPhase" save:"state"`
	SelectPreludeCardsPhase         *shared.SelectPreludeCardsPhase         `json:"selectPreludeCardsPhase" save:"state"`
	PendingCardSelection            *shared.PendingCardSelection            `json:"pendingCardSelection" save:"state"`
	CardReceipts                    []shared.CardReceipt                    `json:"cardReceipts" save:"state"`
	PendingCardPlaySelection        *shared.PendingCardPlaySelection        `json:"pendingCardPlaySelection" save:"state"`
	PendingCardDrawSelection        *shared.PendingCardDrawSelection        `json:"pendingCardDrawSelection" save:"state"`
	PendingBehaviorResolutions      []*shared.PendingBehaviorResolution     `json:"pendingBehaviorResolutions" save:"state"`
	PendingResourceRemovalSelection *shared.PendingResourceRemovalSelection `json:"pendingResourceRemovalSelection" save:"state"`
	PendingColonyResourceSelection  *shared.PendingColonyResourceSelection  `json:"pendingColonyResourceSelection" save:"state"`
	PendingColonyResourceQueue      []shared.PendingColonyResourceSelection `json:"pendingColonyResourceQueue" save:"state"`
	PendingAwardFundSelection       *shared.PendingAwardFundSelection       `json:"pendingAwardFundSelection" save:"state"`
	PendingColonySelection          *shared.PendingColonySelection          `json:"pendingColonySelection" save:"state"`
	PendingFreeTradeSelection       *shared.PendingFreeTradeSelection       `json:"pendingFreeTradeSelection" save:"state"`

	Actions []shared.CardAction `json:"actions" save:"state"`

	Effects []shared.CardEffect `json:"effects" save:"state"`

	BonusTags map[shared.CardTag]int `json:"bonusTags" save:"state"`

	GenerationalEvents map[shared.GenerationalEvent]int `json:"generationalEvents" save:"state"`

	VPGranters []shared.VPGranter `json:"vpGranters" save:"state"`
}
