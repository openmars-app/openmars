package dto

// DiffValueStringDto represents old/new values for string fields
type DiffValueStringDto struct {
	Old string `json:"old"`
	New string `json:"new"`
}

// DiffValueIntDto represents old/new values for integer fields
type DiffValueIntDto struct {
	Old int `json:"old"`
	New int `json:"new"`
}

// DiffValueBoolDto represents old/new values for boolean fields
type DiffValueBoolDto struct {
	Old bool `json:"old"`
	New bool `json:"new"`
}

// TilePlacementDto records a single tile placement on the board
type TilePlacementDto struct {
	HexID    string `json:"hexId"`
	TileType string `json:"tileType"`
	OwnerID  string `json:"ownerId,omitempty"`
}

// BoardChangesDto contains all changes to the game board
type BoardChangesDto struct {
	TilesPlaced []TilePlacementDto `json:"tilesPlaced,omitempty"`
}

// PlayerChangesDto contains all changes to a single player's state
type PlayerChangesDto struct {
	Credits            *DiffValueIntDto    `json:"credits,omitempty"`
	Steel              *DiffValueIntDto    `json:"steel,omitempty"`
	Titanium           *DiffValueIntDto    `json:"titanium,omitempty"`
	Plants             *DiffValueIntDto    `json:"plants,omitempty"`
	Energy             *DiffValueIntDto    `json:"energy,omitempty"`
	Heat               *DiffValueIntDto    `json:"heat,omitempty"`
	TerraformRating    *DiffValueIntDto    `json:"terraformRating,omitempty"`
	CreditsProduction  *DiffValueIntDto    `json:"creditsProduction,omitempty"`
	SteelProduction    *DiffValueIntDto    `json:"steelProduction,omitempty"`
	TitaniumProduction *DiffValueIntDto    `json:"titaniumProduction,omitempty"`
	PlantsProduction   *DiffValueIntDto    `json:"plantsProduction,omitempty"`
	EnergyProduction   *DiffValueIntDto    `json:"energyProduction,omitempty"`
	HeatProduction     *DiffValueIntDto    `json:"heatProduction,omitempty"`
	CardsAdded         []string            `json:"cardsAdded,omitempty"`
	CardsRemoved       []string            `json:"cardsRemoved,omitempty"`
	CardsPlayed        []string            `json:"cardsPlayed,omitempty"`
	Corporation        *DiffValueStringDto `json:"corporation,omitempty"`
	Passed             *DiffValueBoolDto   `json:"passed,omitempty"`
}

// GameChangesDto contains all changes in a single state transition
type GameChangesDto struct {
	Status              *DiffValueStringDto          `json:"status,omitempty"`
	Phase               *DiffValueStringDto          `json:"phase,omitempty"`
	Generation          *DiffValueIntDto             `json:"generation,omitempty"`
	CurrentTurnPlayerID *DiffValueStringDto          `json:"currentTurnPlayerId,omitempty"`
	Temperature         *DiffValueIntDto             `json:"temperature,omitempty"`
	Oxygen              *DiffValueIntDto             `json:"oxygen,omitempty"`
	Oceans              *DiffValueIntDto             `json:"oceans,omitempty"`
	PlayerChanges       map[string]*PlayerChangesDto `json:"playerChanges,omitempty"`
	BoardChanges        *BoardChangesDto             `json:"boardChanges,omitempty"`
}

// CalculatedOutputDto represents an actual output value that was applied
type CalculatedOutputDto struct {
	Operation    *StateConditionDto `json:"operation,omitempty"`
	ResourceType string             `json:"resourceType"`
	Amount       int                `json:"amount"`
	IsScaled     bool               `json:"isScaled"`
}

// ComputedBehaviorValueDto holds pre-computed per-condition output values for a behavior.
// Target uses the format "behaviors::N" where N is the behavior index.
type ComputedBehaviorValueDto struct {
	Target  string                `json:"target"`
	Outputs []CalculatedOutputDto `json:"outputs"`
}

// LogDisplayDataDto contains pre-computed display information for log entries
type LogDisplayDataDto struct {
	RevealedCards []RevealedCardDto `json:"revealedCards,omitempty"`
	Behaviors     []CardBehaviorDto `json:"behaviors,omitempty"`
	Tags          []CardTag         `json:"tags,omitempty"`
	VPConditions  []VPConditionDto  `json:"vpConditions,omitempty"`
}

// StateDiffDto represents the difference between two consecutive game states
type StateDiffDto struct {
	SequenceNumber    int64                 `json:"sequenceNumber"`
	Timestamp         string                `json:"timestamp"`
	GameID            string                `json:"gameId"`
	Changes           *GameChangesDto       `json:"changes"`
	Source            string                `json:"source"`
	SourceType        string                `json:"sourceType"`
	PlayerID          string                `json:"playerId"`
	Description       string                `json:"description"`
	ChoiceIndex       *int                  `json:"choiceIndex,omitempty"`
	CalculatedOutputs []CalculatedOutputDto `json:"calculatedOutputs,omitempty"`
	DisplayData       *LogDisplayDataDto    `json:"displayData,omitempty"`
}

// DiffLogDto contains the complete history of state changes for a game
type DiffLogDto struct {
	GameID          string         `json:"gameId"`
	Diffs           []StateDiffDto `json:"diffs"`
	CurrentSequence int64          `json:"currentSequence"`
}
