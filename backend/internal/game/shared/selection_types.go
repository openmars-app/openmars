package shared

// SelectCorporationPhase represents the corporation selection phase state
type SelectCorporationPhase struct {
	AvailableCorporations []string
}

// SelectStartingCardsPhase represents the starting cards selection phase state
type SelectStartingCardsPhase struct {
	AvailableCards    []string
	SelectionComplete bool
}

// SelectPreludeCardsPhase represents the prelude card selection phase state
type SelectPreludeCardsPhase struct {
	AvailablePreludes []string
	MaxSelectable     int
}

// ProductionPhase represents the production phase state for a player
type ProductionPhase struct {
	ProductionUsed    Production
	AvailableCards    []string
	SelectionComplete bool
	BeforeResources   Resources
	AfterResources    Resources
	EnergyConverted   int
	CreditsIncome     int
}

// PendingTileSelection represents a pending tile placement
type PendingTileSelection struct {
	TileRestrictions *TileRestrictions
	TileType         string
	AvailableHexes   []string
	Source           string
	SourceCardID     string
	OnComplete       *TileCompletionCallback
}

// TileCompletionCallback stores info about what to call when tile placement completes
type TileCompletionCallback struct {
	Type         string
	Output       *BasicResourceCondition
	Source       string
	SourceCardID string
}

// PendingTileSelectionQueue represents a queue of tile placements
type PendingTileSelectionQueue struct {
	Items            []string
	Source           string
	SourceCardID     string
	OnComplete       *TileCompletionCallback
	TileRestrictions *TileRestrictions
}

// PendingCardSelection represents a pending card selection
type PendingCardSelection struct {
	AvailableCards []string
	MinCards       int
	MaxCards       int
	CardCosts      map[string]int
	CardRewards    map[string]int
	Source         string
}

// CardActionRef identifies an owned card action independently of an effect source.
type CardActionRef struct {
	CardID        string
	BehaviorIndex int
}

// PendingCardPlaySelection requires playing a project from hand before continuing.
type PendingCardPlaySelection struct {
	Source        string
	SourceCardID  string
	CostReduction int
	Selectors     []Selector
}

// PendingCardDrawSelection represents a pending card draw/peek/take/buy action
type PendingCardDrawSelection struct {
	AvailableCards      []string
	FreeTakeCount       int
	MinFreeTakeCount    int
	CompleteAction      *CardActionRef
	MaxBuyCount         int
	CardBuyCost         int
	Source              string
	SourceCardID        string
	SourceBehaviorIndex int
	PlayAsPrelude       bool
}

// PendingBehaviorResolution is one independently resolvable discard or behavior choice.
type PendingBehaviorResolution struct {
	ID                  string
	Kind                string
	OptionalBehavior    *CardBehavior
	Source              string
	SourceCardID        string
	SourceBehaviorIndex int
	TriggeringCardID    string
	TriggeringPlayerID  string
	MinCards            int
	MaxCards            int
	PendingOutputs      []BehaviorCondition
	Choices             []Choice
}

// PendingResourceRemovalSelection represents a pending optional resource removal
type PendingResourceRemovalSelection struct {
	ID                string
	Output            *BasicResourceCondition
	Placement         *HexPosition
	MaxAmounts        map[string]int
	EligiblePlayerIDs []string
	ResourceType      ResourceType
	Amount            int
	Source            string
	SourceCardID      string
}

// PendingColonyResourceSelection represents a pending colony resource selection
type PendingColonyResourceSelection struct {
	ResourceType string
	Amount       int
	Source       string
	ColonyID     string
	Reason       string
}

// PendingAwardFundSelection represents a pending award fund selection (e.g., Vitor forced first action)
type PendingAwardFundSelection struct {
	AvailableAwards []string
	Source          string
}

// PendingColonySelection represents a pending colony tile selection from a card effect
type PendingColonySelection struct {
	Remaining                  int
	AddTile                    bool
	AvailableColonyIDs         []string
	AllowDuplicatePlayerColony bool
	Source                     string
	SourceCardID               string
}

// PendingFreeTradeSelection represents a pending free trade colony selection from a card effect
type PendingFreeTradeSelection struct {
	AvailableColonyIDs []string
	Source             string
	SourceCardID       string
}

// ForcedFirstAction represents an action that must be completed first
type ForcedFirstAction struct {
	CorporationID   string
	BehaviorIndices []int
	State           string
	Description     string
}

// PendingEffectSelection defers a complete effect until its source or targets are chosen.
type PendingEffectSelection struct {
	Source       string
	SourceCardID string
	Outputs      []BehaviorCondition
	Options      []EffectSelectionOption
}

// EffectSelectionOption identifies one complete, legal assignment of an effect's targets.
type EffectSelectionOption struct {
	CardID         string
	TargetPlayerID string
	ColonyIDs      []string
	Outputs        []BehaviorCondition
}

// CardReceipt records cards already granted, independently of action resolution.
type CardReceipt struct {
	ID           string
	Source       string
	SourceCardID string
	Cards        []string
}
