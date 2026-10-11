package events

import "time"

// Domain events represent significant state changes in the game
// Published by repositories, subscribed to by effect handlers and broadcasters

// TemperatureChangedEvent is published when the global temperature parameter changes
type TemperatureChangedEvent struct {
	GameID    string
	OldValue  int
	NewValue  int
	ChangedBy string // PlayerID who triggered the change (empty if system-triggered)
	Timestamp time.Time
}

// OxygenChangedEvent is published when the global oxygen parameter changes
type OxygenChangedEvent struct {
	GameID    string
	OldValue  int
	NewValue  int
	ChangedBy string // PlayerID who triggered the change (empty if system-triggered)
	Timestamp time.Time
}

// OceansChangedEvent is published when the global oceans parameter changes
type OceansChangedEvent struct {
	GameID    string
	OldValue  int
	NewValue  int
	ChangedBy string // PlayerID who triggered the change (empty if system-triggered)
	Timestamp time.Time
}

// VenusChangedEvent is published when the global venus parameter changes
type VenusChangedEvent struct {
	GameID    string
	OldValue  int
	NewValue  int
	ChangedBy string // PlayerID who triggered the change (empty if system-triggered)
	Timestamp time.Time
}

// GamePhaseChangedEvent is published when the game phase transitions
type GamePhaseChangedEvent struct {
	GameID    string
	OldPhase  string
	NewPhase  string
	Timestamp time.Time
}

// GameStatusChangedEvent is published when the game status changes (lobby, active, completed)
type GameStatusChangedEvent struct {
	GameID    string
	OldStatus string
	NewStatus string
	Timestamp time.Time
}

// GameStateChangedEvent is a generic event for state changes that don't need specific event types
// Used to trigger automatic broadcasting without defining specialized events
type GameStateChangedEvent struct {
	GameID    string
	Timestamp time.Time
}

// TilePlacedEvent is published when a tile is placed on the board
type TilePlacedEvent struct {
	GameID    string
	PlayerID  string // Player who placed the tile
	TileType  string
	Q         int // Hex coordinate Q
	R         int // Hex coordinate R
	S         int // Hex coordinate S
	Timestamp time.Time
}

// GenerationAdvancedEvent is published when the game generation advances
type GenerationAdvancedEvent struct {
	GameID        string
	OldGeneration int
	NewGeneration int
	Timestamp     time.Time
}

// PlacementBonusGainedEvent is published when a player gains resources from tile placement bonuses
type PlacementBonusGainedEvent struct {
	GameID       string
	PlayerID     string
	Resources    map[string]int // Map of resource type to amount (e.g., {"steel": 2, "titanium": 1})
	SourceCardID string         // Card ID that triggered the tile placement (empty for standard projects)
	Q            int            // Hex coordinate Q
	R            int            // Hex coordinate R
	S            int            // Hex coordinate S
	Timestamp    time.Time
}

// PlayerJoinedEvent is published when a player joins a game
type PlayerJoinedEvent struct {
	GameID     string
	PlayerID   string
	PlayerName string
	Timestamp  time.Time
}

// ConnectionRegisteredEvent is published when a WebSocket connection is registered with a player
// This triggers broadcasting AFTER the connection is ready to receive messages
type ConnectionRegisteredEvent struct {
	GameID    string
	PlayerID  string
	Timestamp time.Time
}

// ResourcesChangedEvent is published when a player's resources change
type ResourcesChangedEvent struct {
	GameID       string
	PlayerID     string
	ResourceType string
	OldAmount    int
	NewAmount    int
	Changes      map[string]int
	Timestamp    time.Time
}

// ProductionChangedEvent is published when a player's production changes
type ProductionChangedEvent struct {
	GameID        string
	PlayerID      string
	ResourceType  string // "credits", "steel", "titanium", "plants", "energy", "heat"
	OldProduction int
	NewProduction int
	Timestamp     time.Time
}

// TerraformRatingChangedEvent is published when a player's terraform rating changes
type TerraformRatingChangedEvent struct {
	GameID    string
	PlayerID  string
	OldRating int
	NewRating int
	Timestamp time.Time
}

// CorporationSelectedEvent is published when a player selects their corporation
type CorporationSelectedEvent struct {
	GameID          string
	PlayerID        string
	CorporationID   string
	CorporationName string
	Timestamp       time.Time
}

// CardPlayedEvent is published when a player plays a card
type CardPlayedEvent struct {
	GameID    string
	PlayerID  string
	CardID    string
	CardName  string
	CardType  string // Type of card played (event, automated, active, corporation, prelude)
	Timestamp time.Time
}

// StandardProjectPlayedEvent is published when a standard project is executed
type StandardProjectPlayedEvent struct {
	GameID      string
	PlayerID    string
	ProjectType string
	ProjectCost int
	Timestamp   time.Time
}

// TagPlayedEvent is published when a tag is played (once per tag on a card)
type TagPlayedEvent struct {
	GameID    string
	PlayerID  string
	CardID    string
	CardName  string
	Tag       string
	Timestamp time.Time
}

// CardAddedToHandEvent is published when a card is added to a player's hand
type CardAddedToHandEvent struct {
	GameID    string
	PlayerID  string
	CardID    string
	Timestamp time.Time
}

// VictoryPointsChangedEvent is published when a player's victory points change
type VictoryPointsChangedEvent struct {
	GameID    string
	PlayerID  string
	OldPoints int
	NewPoints int
	Source    string // What caused the change (e.g., "card", "milestone", "award")
	Timestamp time.Time
}

// PlayerEffectAddedEvent is published when a passive effect is added to a player
type PlayerEffectAddedEvent struct {
	GameID     string
	PlayerID   string
	CardID     string
	CardName   string
	EffectType string
	Timestamp  time.Time
}

// ResourceStorageChangedEvent is published when resource storage on a card changes
type ResourceStorageChangedEvent struct {
	GameID       string
	PlayerID     string
	CardID       string
	ResourceType string
	OldAmount    int
	NewAmount    int
	Timestamp    time.Time
}

// CardHandUpdatedEvent is published when a player's card hand changes (cards added/removed)
type CardHandUpdatedEvent struct {
	GameID    string
	PlayerID  string
	CardIDs   []string // Current cards in hand
	Timestamp time.Time
}

// PlayerEffectsChangedEvent is published when a player's effects list changes
type PlayerEffectsChangedEvent struct {
	GameID    string
	PlayerID  string
	Timestamp time.Time
}

// MilestoneClaimedEvent is published when a player claims a milestone
type MilestoneClaimedEvent struct {
	GameID        string
	PlayerID      string
	MilestoneType string
	Timestamp     time.Time
}

// AwardFundedEvent is published when a player funds an award
type AwardFundedEvent struct {
	GameID      string
	PlayerID    string
	AwardType   string
	FundingCost int
	Timestamp   time.Time
}

// ColonyBuiltEvent is published when a player builds a colony on a colony tile
type ColonyBuiltEvent struct {
	GameID    string
	PlayerID  string
	ColonyID  string
	Timestamp time.Time
}

// ColonyTradedEvent is published when a player trades with a colony tile
type ColonyTradedEvent struct {
	GameID    string
	PlayerID  string
	ColonyID  string
	Timestamp time.Time
}

// ProjectSeatPurchasedEvent is published when a player purchases a seat in a project
type ProjectSeatPurchasedEvent struct {
	GameID    string
	PlayerID  string
	ProjectID string
	SeatIndex int
	Timestamp time.Time
}

// ProjectCompletedEvent is published when all seats in a project are filled
type ProjectCompletedEvent struct {
	GameID    string
	ProjectID string
	Timestamp time.Time
}

// PlayerSelectionChangedEvent is published when a player's pending selection state changes.
type PlayerSelectionChangedEvent struct {
	GameID   string
	PlayerID string
}

// GameEndedEvent is published when the game ends (all global parameters maxed)
type GameEndedEvent struct {
	GameID    string
	WinnerID  string
	IsTie     bool
	Timestamp time.Time
}

// ModeStateChangedEvent describes a baseline state change, excluding temporary overrides.
type ModeStateChangedEvent struct {
	PlayerID string
	GameID   string
	State    string
	Old      int
	New      int
}

// ProductionStartingEvent runs dependent recurring grants before production is settled.
type ProductionStartingEvent struct {
	GameID     string
	Generation int
}
