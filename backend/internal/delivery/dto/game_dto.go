package dto

// GamePhase represents the current phase of the game
type GamePhase string

const (
	GamePhaseWaitingForGameStart   GamePhase = "waiting_for_game_start"
	GamePhaseStartingSelection     GamePhase = "starting_selection"
	GamePhaseStartGameSelection    GamePhase = "start_game_selection"
	GamePhaseInitApplyCorp         GamePhase = "init_apply_corp"
	GamePhaseInitApplyPrelude      GamePhase = "init_apply_prelude"
	GamePhaseAction                GamePhase = "action"
	GamePhaseProductionAndCardDraw GamePhase = "production_and_card_draw"
	GamePhaseFinalPhase            GamePhase = "final_phase"
	GamePhaseComplete              GamePhase = "complete"
)

// GameStatus represents the current status of the game
type GameStatus string

const (
	GameStatusLobby     GameStatus = "lobby"
	GameStatusActive    GameStatus = "active"
	GameStatusCompleted GameStatus = "completed"
)

// CardType represents different types of cards
type CardType string

const (
	CardTypeAutomated   CardType = "automated"
	CardTypeActive      CardType = "active"
	CardTypeEvent       CardType = "event"
	CardTypeCorporation CardType = "corporation"
	CardTypePrelude     CardType = "prelude"
)

// StandardProject represents the different types of standard projects
type StandardProject string

const (
	StandardProjectSellPatents StandardProject = "sell-patents"
	StandardProjectPowerPlant  StandardProject = "power-plant"
	StandardProjectAsteroid    StandardProject = "asteroid"
	StandardProjectAquifer     StandardProject = "aquifer"
	StandardProjectGreenery    StandardProject = "greenery"
	StandardProjectCity        StandardProject = "city"

	StandardProjectConvertPlantsToGreenery  StandardProject = "convert-plants-to-greenery"
	StandardProjectConvertHeatToTemperature StandardProject = "convert-heat-to-temperature"
)

// CardTag represents different card categories and attributes
type CardTag string

const (
	TagSpace    CardTag = "space"
	TagEarth    CardTag = "earth"
	TagScience  CardTag = "science"
	TagPower    CardTag = "power"
	TagBuilding CardTag = "building"
	TagMicrobe  CardTag = "microbe"
	TagAnimal   CardTag = "animal"
	TagPlant    CardTag = "plant"
	TagEvent    CardTag = "event"
	TagCity     CardTag = "city"
	TagVenus    CardTag = "venus"
	TagJovian   CardTag = "jovian"
	TagWildlife CardTag = "wildlife"
	TagWild     CardTag = "wild"
)

// ResourceType represents different types of resources for client consumption
// This is a 1:1 mapping from types.ResourceType
type ResourceType string

const (
	ResourceTypeCopy     ResourceType = "copy"
	ResourceTypeCredit   ResourceType = "credit"
	ResourceTypeSteel    ResourceType = "steel"
	ResourceTypeTitanium ResourceType = "titanium"
	ResourceTypePlant    ResourceType = "plant"
	ResourceTypeEnergy   ResourceType = "energy"
	ResourceTypeHeat     ResourceType = "heat"
	ResourceTypeMicrobe  ResourceType = "microbe"
	ResourceTypeAnimal   ResourceType = "animal"
	ResourceTypeFloater  ResourceType = "floater"
	ResourceTypeScience  ResourceType = "science"
	ResourceTypeAsteroid ResourceType = "asteroid"
	ResourceTypeFighter  ResourceType = "fighter"
	ResourceTypeDisease  ResourceType = "disease"
	ResourceTypeCamp     ResourceType = "camp"

	ResourceTypeCardDraw   ResourceType = "card-draw"
	ResourceTypeCardReveal ResourceType = "card-reveal"
	ResourceTypeCardTake   ResourceType = "card-take"
	ResourceTypeCardPeek   ResourceType = "card-peek"

	ResourceTypeCityPlacement     ResourceType = "city-placement"
	ResourceTypeOceanPlacement    ResourceType = "ocean-placement"
	ResourceTypeGreeneryPlacement ResourceType = "greenery-placement"

	ResourceTypeCommercialDistrictTile ResourceType = "commercial-district-tile"
	ResourceTypeIndustrialCenterTile   ResourceType = "industrial-center-tile"
	ResourceTypeCityTile               ResourceType = "city-tile"
	ResourceTypeOceanTile              ResourceType = "ocean-tile"
	ResourceTypeGreeneryTile           ResourceType = "greenery-tile"
	ResourceTypeColony                 ResourceType = "colony"
	ResourceTypeTradeFleet             ResourceType = "trade-fleet"

	ResourceTypeTemperature ResourceType = "temperature"
	ResourceTypeOxygen      ResourceType = "oxygen"
	ResourceTypeVenus       ResourceType = "venus"
	ResourceTypeTR          ResourceType = "tr"

	ResourceTypeCreditProduction   ResourceType = "credit-production"
	ResourceTypeSteelProduction    ResourceType = "steel-production"
	ResourceTypeTitaniumProduction ResourceType = "titanium-production"
	ResourceTypePlantProduction    ResourceType = "plant-production"
	ResourceTypeEnergyProduction   ResourceType = "energy-production"
	ResourceTypeHeatProduction     ResourceType = "heat-production"

	ResourceTypeEffect ResourceType = "effect"
	ResourceTypeTag    ResourceType = "tag"

	ResourceTypeGlobalParameterLenience ResourceType = "global-parameter-lenience"
	ResourceTypeDefense                 ResourceType = "defense"
	ResourceTypeDiscount                ResourceType = "discount"
	ResourceTypeValueModifier           ResourceType = "value-modifier"
)

// TargetType represents different targeting scopes for resource conditions for client consumption
type TargetType string

const (
	TargetGame           TargetType = "game"
	TargetSelfPlayer     TargetType = "self-player"
	TargetSelfCard       TargetType = "self-card"
	TargetTriggeringCard TargetType = "triggering-card"
	TargetTriggerColony  TargetType = "trigger-colony"
	TargetAnyCard        TargetType = "any-card"
	TargetAnyPlayer      TargetType = "any-player"
	TargetOpponent       TargetType = "opponent"
	TargetNone           TargetType = "none"
)

// CardApplyLocation represents different locations where card conditions can be evaluated for client consumption
type CardApplyLocation string

const (
	CardApplyLocationAnywhere CardApplyLocation = "anywhere"
	CardApplyLocationMars     CardApplyLocation = "mars"
)

// RequirementType represents different card requirement types for client consumption
type RequirementType string

const (
	RequirementTemperature RequirementType = "temperature"
	RequirementOxygen      RequirementType = "oxygen"
	RequirementOceans      RequirementType = "oceans"
	RequirementVenus       RequirementType = "venus"
	RequirementCities      RequirementType = "city"
	RequirementGreeneries  RequirementType = "greeneries"
	RequirementTags        RequirementType = "tags"
	RequirementProduction  RequirementType = "production"
	RequirementTR          RequirementType = "tr"
	RequirementResource    RequirementType = "resource"
)

// VPConditionType represents different types of VP conditions for client consumption
type VPConditionType string

const (
	VPConditionFixed       VPConditionType = "fixed"
	VPConditionPer         VPConditionType = "per"
	VPConditionResourcesOn VPConditionType = "resources-on"
)

// TriggerType represents different trigger conditions for client consumption
type TriggerType string

const (
	TriggerOceanPlaced           TriggerType = "ocean-placed"
	TriggerGlobalParameterRaised TriggerType = "global-parameter-raised"
	TriggerCityPlaced            TriggerType = "city-placed"
	TriggerCardPlayed            TriggerType = "card-played"
	TriggerTagPlayed             TriggerType = "tag-played"
	TriggerTilePlaced            TriggerType = "tile-placed"
)

// ResourceTriggerType represents different trigger types for resource exchanges for client consumption
type ResourceTriggerType string

const (
	ResourceTriggerManual                     ResourceTriggerType = "manual"
	ResourceTriggerAuto                       ResourceTriggerType = "auto"
	ResourceTriggerAutoCorporationFirstAction ResourceTriggerType = "auto-corporation-first-action"
	ResourceTriggerAutoCorporationStart       ResourceTriggerType = "auto-corporation-start"
)

// ResourceSet represents a collection of resources and their amounts
type ResourceSet struct {
	Credits  int `json:"credits"`
	Steel    int `json:"steel"`
	Titanium int `json:"titanium"`
	Plants   int `json:"plants"`
	Energy   int `json:"energy"`
	Heat     int `json:"heat"`
}

// TileRestrictionsDto represents tile placement restrictions for client consumption
type TileRestrictionsDto struct {
	BoardTags         []string `json:"boardTags,omitempty"`
	Adjacency         string   `json:"adjacency,omitempty"` // "none" = no adjacent occupied tiles
	Area              string   `json:"area,omitempty" tstype:"'land' | 'ocean'"`
	AdjacentToType    string   `json:"adjacentToType,omitempty"`    // "city", "greenery" = must be adjacent to this tile type
	MinAdjacentOfType *int     `json:"minAdjacentOfType,omitempty"` // min count of adjacent tiles of AdjacentToType
	AdjacentToOwned   *bool    `json:"adjacentToOwned,omitempty"`   // must be adjacent to a tile owned by the placing player
	OnBonusType       []string `json:"onBonusType,omitempty"`       // tile must have one of these bonus types
}

// TargetRestrictionDto represents restrictions on target player selection
type TargetRestrictionDto struct {
	Selectors []SelectorDto `json:"selectors,omitempty"`
	Adjacent  string        `json:"adjacent,omitempty"`
}

// SelectorDto represents matching criteria for cards, resources, or projects.
// Multiple fields within a Selector use AND logic (all must match).
// Multiple Selectors in a slice use OR logic (any match is sufficient).
type SelectorDto struct {
	Dependency           string            `json:"dependency,omitempty"`
	TagCount             *MinMaxValueDto   `json:"tagCount,omitempty"`
	Tags                 []CardTag         `json:"tags,omitempty"`
	CardTypes            []CardType        `json:"cardTypes,omitempty"`
	Resources            []string          `json:"resources,omitempty"`
	StandardProjects     []StandardProject `json:"standardProjects,omitempty"`
	RequiredOriginalCost *MinMaxValueDto   `json:"requiredOriginalCost,omitempty"`
	VP                   *MinMaxValueDto   `json:"vp,omitempty"`
	GlobalParameters     []string          `json:"globalParameters,omitempty"`
	Actions              []string          `json:"actions,omitempty"`
}

// BasicResourceConditionDto covers credit, steel, titanium, plant, energy, heat.
//
//tygo:emit export type ResourceCondition = BasicResourceConditionDto | ProductionConditionDto | TilePlacementConditionDto | GlobalParameterConditionDto | CardOperationConditionDto | CardStorageConditionDto | EffectConditionDto | ColonyConditionDto | TileModificationConditionDto | MiscConditionDto | CopyConditionDto | CardRevealConditionDto | PaymentSubstituteConditionDto | StateConditionDto;
type BasicResourceConditionDto struct {
	Type              string                `json:"type" tstype:"'credit' | 'steel' | 'titanium' | 'plant' | 'energy' | 'heat'"`
	Amount            int                   `json:"amount"`
	Target            TargetType            `json:"target"`
	Per               *PerConditionDto      `json:"per,omitempty"`
	VariableAmount    *bool                 `json:"variableAmount,omitempty"`
	TargetRestriction *TargetRestrictionDto `json:"targetRestriction,omitempty"`
	MaxTrigger        *int                  `json:"maxTrigger,omitempty"`
}

func (d BasicResourceConditionDto) GetConditionType() string { return d.Type }
func (d BasicResourceConditionDto) GetConditionAmount() int  { return d.Amount }

// ProductionConditionDto covers all production types.
type ProductionConditionDto struct {
	Type           string           `json:"type" tstype:"'credit-production' | 'steel-production' | 'titanium-production' | 'plant-production' | 'energy-production' | 'heat-production' | 'any-production'"`
	Amount         int              `json:"amount"`
	Target         TargetType       `json:"target"`
	Per            *PerConditionDto `json:"per,omitempty"`
	VariableAmount *bool            `json:"variableAmount,omitempty"`
}

func (d ProductionConditionDto) GetConditionType() string { return d.Type }
func (d ProductionConditionDto) GetConditionAmount() int  { return d.Amount }

// TilePlacementConditionDto covers tile placements and land claims.
type TilePlacementConditionDto struct {
	Type             string               `json:"type" tstype:"'city-placement' | 'ocean-placement' | 'greenery-placement' | 'volcano-placement' | 'tile-placement' | 'land-claim'"`
	Amount           int                  `json:"amount"`
	Target           TargetType           `json:"target"`
	TileRestrictions *TileRestrictionsDto `json:"tileRestrictions,omitempty"`
	TileType         string               `json:"tileType,omitempty"`
}

func (d TilePlacementConditionDto) GetConditionType() string { return d.Type }
func (d TilePlacementConditionDto) GetConditionAmount() int  { return d.Amount }

// GlobalParameterConditionDto covers temperature, oxygen, ocean, venus, tr, global-parameter.
type GlobalParameterConditionDto struct {
	Type   string           `json:"type" tstype:"'temperature' | 'oxygen' | 'ocean' | 'venus' | 'tr' | 'global-parameter'"`
	Amount int              `json:"amount"`
	Target TargetType       `json:"target"`
	Per    *PerConditionDto `json:"per,omitempty"`
}

func (d GlobalParameterConditionDto) GetConditionType() string { return d.Type }
func (d GlobalParameterConditionDto) GetConditionAmount() int  { return d.Amount }

// CardOperationConditionDto covers card-draw, card-take, card-peek, card-buy, card-discard.
type CardOperationConditionDto struct {
	Optional       bool             `json:"optional,omitempty"`
	Per            *PerConditionDto `json:"per,omitempty"`
	Type           string           `json:"type" tstype:"'card-draw' | 'card-take' | 'card-peek' | 'card-buy' | 'card-discard'"`
	Amount         int              `json:"amount"`
	Target         TargetType       `json:"target"`
	Selectors      []SelectorDto    `json:"selectors,omitempty"`
	VariableAmount *bool            `json:"variableAmount,omitempty"`
}

func (d CardOperationConditionDto) GetConditionType() string { return d.Type }
func (d CardOperationConditionDto) GetConditionAmount() int  { return d.Amount }

// CardStorageConditionDto covers microbe, animal, floater, science, asteroid, fighter, disease, camp, card-resource.
type CardStorageConditionDto struct {
	Type           string           `json:"type" tstype:"'microbe' | 'animal' | 'floater' | 'science' | 'asteroid' | 'fighter' | 'disease' | 'camp' | 'card-resource'"`
	Amount         int              `json:"amount"`
	Target         TargetType       `json:"target"`
	Selectors      []SelectorDto    `json:"selectors,omitempty"`
	Per            *PerConditionDto `json:"per,omitempty"`
	VariableAmount *bool            `json:"variableAmount,omitempty"`
}

func (d CardStorageConditionDto) GetConditionType() string { return d.Type }
func (d CardStorageConditionDto) GetConditionAmount() int  { return d.Amount }

type PaymentSubstituteConditionDto struct {
	Type           string           `json:"type" tstype:"'payment-substitute'"`
	Amount         int              `json:"amount"`
	Target         TargetType       `json:"target"`
	Source         PaymentSourceDto `json:"source"`
	TargetResource ResourceType     `json:"targetResource"`
	Selectors      []SelectorDto    `json:"selectors,omitempty"`
}

func (d PaymentSubstituteConditionDto) GetConditionType() string { return d.Type }
func (d PaymentSubstituteConditionDto) GetConditionAmount() int  { return d.Amount }

// EffectConditionDto covers discount, payment-substitute, and other effect types.
type EffectConditionDto struct {
	Against   string        `json:"against,omitempty" tstype:"'any-player' | 'opponents'"`
	Temporary string        `json:"temporary,omitempty"`
	Type      string        `json:"type" tstype:"'discount' | 'value-modifier' | 'global-parameter-lenience' | 'ignore-global-requirements' | 'ocean-adjacency-bonus' | 'defense' | 'action-reuse' | 'effect' | 'tag'"`
	Amount    int           `json:"amount"`
	Target    TargetType    `json:"target"`
	Selectors []SelectorDto `json:"selectors,omitempty"`
}

func (d EffectConditionDto) GetConditionType() string { return d.Type }
func (d EffectConditionDto) GetConditionAmount() int  { return d.Amount }

// ColonyConditionDto covers colony, colony-count, colony-bonus, colony-track-step.
type ColonyConditionDto struct {
	Optional       bool       `json:"optional,omitempty"`
	SelectionGroup string     `json:"selectionGroup,omitempty"`
	Type           string     `json:"type" tstype:"'colony-tile-add' | 'colony' | 'colony-count' | 'colony-bonus' | 'colony-track-step' | 'trade-fleet'"`
	Amount         int        `json:"amount"`
	Target         TargetType `json:"target"`
}

func (d ColonyConditionDto) GetConditionType() string { return d.Type }
func (d ColonyConditionDto) GetConditionAmount() int  { return d.Amount }

// TileModificationConditionDto covers tile-destruction and tile-replacement.
type TileModificationConditionDto struct {
	Type     string     `json:"type" tstype:"'tile-destruction' | 'tile-replacement'"`
	Amount   int        `json:"amount"`
	Target   TargetType `json:"target"`
	TileType string     `json:"tileType,omitempty"`
}

func (d TileModificationConditionDto) GetConditionType() string { return d.Type }
func (d TileModificationConditionDto) GetConditionAmount() int  { return d.Amount }

// MiscConditionDto covers extra-actions, bonus-tags, world-tree-tile, award-fund, trade.
type MiscConditionDto struct {
	Type      string           `json:"type" tstype:"'extra-actions' | 'bonus-tags' | 'world-tree-tile' | 'award-fund' | 'trade'"`
	Amount    int              `json:"amount"`
	Target    TargetType       `json:"target"`
	Per       *PerConditionDto `json:"per,omitempty"`
	Selectors []SelectorDto    `json:"selectors,omitempty"`
}

func (d MiscConditionDto) GetConditionType() string { return d.Type }
func (d MiscConditionDto) GetConditionAmount() int  { return d.Amount }

// PerConditionDto represents a per condition for client consumption
type PerConditionDto struct {
	Zone               string             `json:"zone,omitempty"`
	Selectors          []SelectorDto      `json:"selectors,omitempty"`
	IncludeSource      bool               `json:"includeSource,omitempty"`
	Type               ResourceType       `json:"type"`
	Amount             int                `json:"amount"`
	Location           *CardApplyLocation `json:"location,omitempty"`
	Target             *TargetType        `json:"target,omitempty"`
	Tag                *CardTag           `json:"tag,omitempty"`
	Tags               []CardTag          `json:"tags,omitempty"`
	AdjacentToSelfTile bool               `json:"adjacentToSelfTile"`
}

// ChoiceDto represents a choice for client consumption
type ChoiceDto struct {
	StateRequirements []StateRequirementDto    `json:"stateRequirements,omitempty"`
	InputOptions      *BehaviorInputOptionsDto `json:"inputOptions,omitempty"`
	StorageTargets    [][]string               `json:"storageTargets,omitempty"`
	OriginalIndex     int                      `json:"originalIndex"`
	Inputs            []any                    `json:"inputs,omitempty" tstype:"ResourceCondition[]"`
	Outputs           []any                    `json:"outputs,omitempty" tstype:"ResourceCondition[]"`
	Requirements      *CardRequirementsDto     `json:"requirements,omitempty"`
	Available         bool                     `json:"available"`
	Errors            []StateErrorDto          `json:"errors"`
}

// TriggerDto represents a trigger for client consumption
type TriggerDto struct {
	Type      ResourceTriggerType          `json:"type"`
	Condition *ResourceTriggerConditionDto `json:"condition,omitempty"`
}

// MinMaxValueDto represents a minimum and/or maximum value constraint for client consumption
type MinMaxValueDto struct {
	Min *int `json:"min,omitempty"`
	Max *int `json:"max,omitempty"`
}

// ResourceTriggerConditionDto represents a resource trigger condition for client consumption
type ResourceTriggerConditionDto struct {
	Direction              string                          `json:"direction,omitempty"`
	State                  string                          `json:"state,omitempty"`
	To                     *int                            `json:"to,omitempty"`
	Type                   TriggerType                     `json:"type"`
	ResourceTypes          []ResourceType                  `json:"resourceTypes,omitempty"`
	Location               *CardApplyLocation              `json:"location,omitempty"`
	Selectors              []SelectorDto                   `json:"selectors,omitempty"`
	Target                 *TargetType                     `json:"target,omitempty"`
	RequiredOriginalCost   *MinMaxValueDto                 `json:"requiredOriginalCost,omitempty"`
	RequiredResourceChange map[ResourceType]MinMaxValueDto `json:"requiredResourceChange,omitempty"`
	OnBonusType            []string                        `json:"onBonusType,omitempty"`
	Unique                 bool                            `json:"unique,omitempty"`
}

// ChoicePolicySelectDto describes an auto-selection rule for a choice policy
type ChoicePolicySelectDto struct {
	Option       int            `json:"option"`
	MinMax       MinMaxValueDto `json:"minMax"`
	ResourceType string         `json:"resourceType"`
	Tag          *string        `json:"tag,omitempty"`
}

// ChoicePolicyDto represents a choice policy for client consumption
type ChoicePolicyDto struct {
	Type    string                 `json:"type"`
	Default *int                   `json:"default,omitempty"`
	Select  *ChoicePolicySelectDto `json:"select,omitempty"`
}

// BehaviorInputOptionsDto contains server-calculated payment choices.
type BehaviorInputOptionsDto struct {
	StorageSources [][]string              `json:"storageSources,omitempty"`
	VariableAmount *VariableInputAmountDto `json:"variableAmount,omitempty"`
}

// VariableInputAmountDto describes an inclusive affordable repetition range.
type VariableInputAmountDto struct {
	ResourceType ResourceType `json:"resourceType"`
	Min          int          `json:"min"`
	Max          int          `json:"max"`
}

// CardBehaviorDto represents a card behavior for client consumption.
type CardBehaviorDto struct {
	ID                            string                            `json:"id,omitempty"`
	Dependency                    string                            `json:"dependency,omitempty"`
	DependencyStatus              *DependencyStatusDto              `json:"dependencyStatus,omitempty"`
	StateRequirements             []StateRequirementDto             `json:"stateRequirements,omitempty"`
	MaxUsesPerGame                int                               `json:"maxUsesPerGame,omitempty"`
	UsageGroup                    string                            `json:"usageGroup,omitempty"`
	ResourceRequirements          []BehaviorResourceRequirementDto  `json:"resourceRequirements,omitempty"`
	MaxUsesPerGeneration          int                               `json:"maxUsesPerGeneration,omitempty"`
	InputOptions                  *BehaviorInputOptionsDto          `json:"inputOptions,omitempty"`
	ProductionBox                 string                            `json:"productionBox,omitempty"`
	Description                   string                            `json:"description,omitempty"`
	Triggers                      []TriggerDto                      `json:"triggers,omitempty"`
	Inputs                        []any                             `json:"inputs,omitempty" tstype:"ResourceCondition[]"`
	Outputs                       []any                             `json:"outputs,omitempty" tstype:"ResourceCondition[]"`
	Choices                       []ChoiceDto                       `json:"choices,omitempty"`
	ChoicePolicy                  *ChoicePolicyDto                  `json:"choicePolicy,omitempty"`
	GenerationalEventRequirements []GenerationalEventRequirementDto `json:"generationalEventRequirements,omitempty"`
	Group                         string                            `json:"group,omitempty"`
}

// PaymentConstantsDto represents payment conversion rates
type PaymentConstantsDto struct {
	SteelValue    int `json:"steelValue"`
	TitaniumValue int `json:"titaniumValue"`
}

// CardRequirementsDto wraps requirement items with a description for client consumption
type CardRequirementsDto struct {
	Description string           `json:"description,omitempty"`
	Items       []RequirementDto `json:"items"`
}

// RequirementDto represents a card requirement for client consumption
type RequirementDto struct {
	Permanent bool               `json:"permanent,omitempty"`
	State     string             `json:"state,omitempty"`
	Equals    *int               `json:"equals,omitempty"`
	Type      RequirementType    `json:"type"`
	Min       *int               `json:"min,omitempty"`
	Max       *int               `json:"max,omitempty"`
	Location  *CardApplyLocation `json:"location,omitempty"`
	Tag       *CardTag           `json:"tag,omitempty"`
	Resource  *ResourceType      `json:"resource,omitempty"`
}

// ResourceStorageDto represents a card's resource storage for client consumption
type ResourceStorageDto struct {
	Type        ResourceType `json:"type"`
	Capacity    *int         `json:"capacity,omitempty"`
	Starting    int          `json:"starting"`
	Description string       `json:"description,omitempty"`
}

// VPConditionDto represents a victory point condition for client consumption
type VPConditionDto struct {
	StateRequirements []StateRequirementDto `json:"stateRequirements,omitempty"`
	Amount            int                   `json:"amount"`
	Condition         VPConditionType       `json:"condition"`
	MaxTrigger        *int                  `json:"maxTrigger,omitempty"`
	Per               *PerConditionDto      `json:"per,omitempty"`
	Description       string                `json:"description,omitempty"`
}

// CardDescriptionSectionDto is one labelled paragraph of a card's description.
// Text excludes the label and may contain inline **bold** emphasis.
type CardDescriptionSectionDto struct {
	Type string `json:"type" tstype:"'generic' | 'effect' | 'action' | 'requirement'"`
	Text string `json:"text"`
}

// CardDto represents a card for client consumption
type CardDto struct {
	Style       *CardStyleDto               `json:"style,omitempty"`
	ID          string                      `json:"id"`
	Name        string                      `json:"name"`
	Type        CardType                    `json:"type"`
	Cost        int                         `json:"cost"`
	Description []CardDescriptionSectionDto `json:"description"`

	GameModes        []string             `json:"gameModes"`
	RequiresFeatures []string             `json:"requiresFeatures,omitempty"`
	RequiresSystems  []string             `json:"requiresSystems,omitempty"`
	Tags             []CardTag            `json:"tags,omitempty"`
	Requirements     *CardRequirementsDto `json:"requirements,omitempty"`
	Behaviors        []CardBehaviorDto    `json:"behaviors,omitempty"`
	ResourceStorage  *ResourceStorageDto  `json:"resourceStorage,omitempty"`
	VPConditions     []VPConditionDto     `json:"vpConditions,omitempty"`

	StartingResources  *ResourceSet `json:"startingResources,omitempty"`
	StartingProduction *ResourceSet `json:"startingProduction,omitempty"`
}

// SelectCorporationPhaseDto represents corporation selection state for the current player
type SelectCorporationPhaseDto struct {
	AvailableCorporations []CardDto `json:"availableCorporations"`
}

// SelectCorporationOtherPlayerDto represents corporation selection state for other players
type SelectCorporationOtherPlayerDto struct{}

type SelectStartingCardsPhaseDto struct {
	AvailableCards []CardDto `json:"availableCards"`
}

type SelectStartingCardsOtherPlayerDto struct{}

// SelectPreludeCardsPhaseDto represents prelude card selection state for the current player
type SelectPreludeCardsPhaseDto struct {
	AvailablePreludes []CardDto `json:"availablePreludes"`
	MaxSelectable     int       `json:"maxSelectable"`
}

// SelectPreludeCardsOtherPlayerDto represents prelude card selection state for other players
type SelectPreludeCardsOtherPlayerDto struct {
	// Empty - other players don't see selection details
}

// ProductionPhaseDto represents card selection and production phase state for a player
type ProductionPhaseDto struct {
	ProductionUsed    ProductionDto `json:"productionUsed"`
	AvailableCards    []CardDto     `json:"availableCards"`    // Cards available for selection
	SelectionComplete bool          `json:"selectionComplete"` // Whether player completed card selection
	BeforeResources   ResourcesDto  `json:"beforeResources"`
	AfterResources    ResourcesDto  `json:"afterResources"`
	ResourceDelta     ResourcesDto  `json:"resourceDelta"`
	EnergyConverted   int           `json:"energyConverted"`
	CreditsIncome     int           `json:"creditsIncome"`
}

type ProductionPhaseOtherPlayerDto struct {
	ProductionUsed    ProductionDto `json:"productionUsed"`
	SelectionComplete bool          `json:"selectionComplete"` // Whether player completed card selection
	BeforeResources   ResourcesDto  `json:"beforeResources"`
	AfterResources    ResourcesDto  `json:"afterResources"`
	ResourceDelta     ResourcesDto  `json:"resourceDelta"`
	EnergyConverted   int           `json:"energyConverted"`
	CreditsIncome     int           `json:"creditsIncome"`
}

// GameSettingsDto contains configurable game parameters
type GameSettingsDto struct {
	GameModeID            string         `json:"gameModeId"`
	GameModeParameters    map[string]int `json:"gameModeParameters"`
	Features              []string       `json:"features"`
	Systems               []string       `json:"systems"`
	AvailableGameModes    []GameModeDto  `json:"availableGameModes"`
	MaxPlayers            int            `json:"maxPlayers"`
	MapID                 string         `json:"mapId"`
	DevelopmentMode       bool           `json:"developmentMode"`
	DemoGame              bool           `json:"demoGame"`
	AllowRandomBuy        bool           `json:"allowRandomBuy"`
	HasClaudeOAuthToken   bool           `json:"hasClaudeOAuthToken"`
	BotSpendCapUSD        float64        `json:"botSpendCapUsd"`
	BotSpendUSD           float64        `json:"botSpendUsd"`
	AvailablePlayerColors []string       `json:"availablePlayerColors"`
	AvailableMaps         []MapInfoDto   `json:"availableMaps"`
	Temperature           *int           `json:"temperature,omitempty"`
	Oxygen                *int           `json:"oxygen,omitempty"`
	Oceans                *int           `json:"oceans,omitempty"`
	Generation            *int           `json:"generation,omitempty"`
}

// MapInfoDto contains basic map info for the frontend
type MapInfoDto struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Tiles       []MapPreviewTile `json:"tiles"`
}

// MapPreviewTile contains minimal tile data for map preview
type MapPreviewTile struct {
	Q        int      `json:"q"`
	R        int      `json:"r"`
	S        int      `json:"s"`
	Type     string   `json:"type"`
	Volcanic bool     `json:"volcanic,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

// PendingDemoChoicesDto contains a player's demo lobby card selections
type PendingDemoChoicesDto struct {
	CorporationID   string        `json:"corporationId"`
	PreludeIDs      []string      `json:"preludeIds"`
	CardIDs         []string      `json:"cardIds"`
	Resources       ResourcesDto  `json:"resources"`
	Production      ProductionDto `json:"production"`
	TerraformRating int           `json:"terraformRating"`
}

// GlobalParameterBonusDto describes a bonus step on a global parameter track
type GlobalParameterBonusDto struct {
	Parameter    string `json:"parameter"`
	Threshold    int    `json:"threshold"`
	RewardType   string `json:"rewardType"`
	RewardAmount int    `json:"rewardAmount"`
}

// GlobalParametersDto represents the terraforming progress
type GlobalParametersDto struct {
	Temperature int                       `json:"temperature"` // Range: -30 to +8°C
	Oxygen      int                       `json:"oxygen"`      // Range: 0-14%
	Oceans      int                       `json:"oceans"`      // Range: 0-9
	MaxOceans   int                       `json:"maxOceans"`   // Dynamic max, starts at 9
	Venus       int                       `json:"venus"`       // Range: 0-30%
	Bonuses     []GlobalParameterBonusDto `json:"bonuses"`
}

// ResourcesDto represents a player's resources
type ResourcesDto struct {
	Credits  int `json:"credits"`
	Steel    int `json:"steel"`
	Titanium int `json:"titanium"`
	Plants   int `json:"plants"`
	Energy   int `json:"energy"`
	Heat     int `json:"heat"`
}

// ProductionDto represents a player's production values
type ProductionDto struct {
	Credits  int `json:"credits"`
	Steel    int `json:"steel"`
	Titanium int `json:"titanium"`
	Plants   int `json:"plants"`
	Energy   int `json:"energy"`
	Heat     int `json:"heat"`
}

type PaymentSourceDto struct {
	Target   string       `json:"target" tstype:"'self-player' | 'self-card'"`
	Resource ResourceType `json:"resource"`
	CardID   string       `json:"cardId,omitempty"`
}

type PaymentSubstituteDto struct {
	Source          PaymentSourceDto `json:"source"`
	TargetResource  ResourceType     `json:"targetResource"`
	ConversionRate  int              `json:"conversionRate"`
	GrantedByCardID string           `json:"grantedByCardId,omitempty"`
	Selectors       []SelectorDto    `json:"selectors"`
}

type PaymentAllocationDto struct {
	Source         PaymentSourceDto `json:"source"`
	TargetResource ResourceType     `json:"targetResource"`
	Amount         int              `json:"amount"`
}

type PaymentDto struct {
	Allocations []PaymentAllocationDto `json:"allocations"`
}

type PaymentOptionDto struct {
	Source         PaymentSourceDto `json:"source"`
	TargetResource ResourceType     `json:"targetResource"`
	ConversionRate int              `json:"conversionRate"`
	Available      int              `json:"available"`
}

type PaymentQuoteDto struct {
	Costs   map[ResourceType]int `json:"costs"`
	Options []PaymentOptionDto   `json:"options"`
}

// StateErrorCode represents error codes for entity state validation.
// All codes use kebab-case for consistency with JSON serialization.
type StateErrorCode string

const (
	ErrorCodeNotYourTurn StateErrorCode = "not-your-turn"
	ErrorCodeWrongPhase  StateErrorCode = "wrong-phase"

	ErrorCodeInsufficientCredits StateErrorCode = "insufficient-credits"

	ErrorCodeInsufficientResources StateErrorCode = "insufficient-resources"
	ErrorCodeTooManyResources      StateErrorCode = "too-many-resources"

	ErrorCodeTemperatureTooLow  StateErrorCode = "temperature-too-low"
	ErrorCodeTemperatureTooHigh StateErrorCode = "temperature-too-high"
	ErrorCodeOxygenTooLow       StateErrorCode = "oxygen-too-low"
	ErrorCodeOxygenTooHigh      StateErrorCode = "oxygen-too-high"
	ErrorCodeOceansTooLow       StateErrorCode = "oceans-too-low"
	ErrorCodeOceansTooHigh      StateErrorCode = "oceans-too-high"
	ErrorCodeTRTooLow           StateErrorCode = "tr-too-low"
	ErrorCodeTRTooHigh          StateErrorCode = "tr-too-high"

	ErrorCodeInsufficientTags       StateErrorCode = "insufficient-tags"
	ErrorCodeTooManyTags            StateErrorCode = "too-many-tags"
	ErrorCodeInsufficientProduction StateErrorCode = "insufficient-production"

	ErrorCodeNoOceanTiles         StateErrorCode = "no-ocean-tiles"
	ErrorCodeNoCityPlacements     StateErrorCode = "no-city-placements"
	ErrorCodeNoGreeneryPlacements StateErrorCode = "no-greenery-placements"
	ErrorCodeNoCardsInHand        StateErrorCode = "no-cards-in-hand"
	ErrorCodeInvalidProjectType   StateErrorCode = "invalid-project-type"
	ErrorCodeInvalidRequirement   StateErrorCode = "invalid-requirement"

	ErrorCodeInvalidCardType StateErrorCode = "invalid-card-type"
)

// StateErrorCategory represents categories for error grouping.
// Categories enable UI filtering and display organization.
type StateErrorCategory string

const (
	ErrorCategoryTurn          StateErrorCategory = "turn"
	ErrorCategoryPhase         StateErrorCategory = "phase"
	ErrorCategoryCost          StateErrorCategory = "cost"
	ErrorCategoryInput         StateErrorCategory = "input"
	ErrorCategoryRequirement   StateErrorCategory = "requirement"
	ErrorCategoryAvailability  StateErrorCategory = "availability"
	ErrorCategoryConfiguration StateErrorCategory = "configuration"
	ErrorCategoryInternal      StateErrorCategory = "internal"
)

// StateErrorDto represents a specific reason why an entity (card, action, project) is unavailable
// Part of the Player-Scoped Card Architecture for rich error information
type StateErrorDto struct {
	Code     StateErrorCode     `json:"code"`     // Error code (e.g., ErrorCodeInsufficientCredits)
	Category StateErrorCategory `json:"category"` // Error category (e.g., ErrorCategoryCost)
	Message  string             `json:"message"`  // Human-readable error message
}

// StateWarningCode represents warning codes for entity state validation.
// All codes use kebab-case for consistency with JSON serialization.
type StateWarningCode string

// StateWarningDto represents a non-blocking warning about an action
// Warnings inform the player of potential issues without preventing the action
type StateWarningDto struct {
	Code    StateWarningCode `json:"code"`
	Message string           `json:"message"`
}

// PlayerCardDto represents a card in a player's hand with calculated playability state
// Part of the Player-Scoped Card Architecture
type PlayerCardDto struct {
	DependencyTargets []string                    `json:"dependencyTargets,omitempty"`
	ID                string                      `json:"id"`
	Name              string                      `json:"name"`
	Type              CardType                    `json:"type"`
	Cost              int                         `json:"cost"` // Original card cost (same as CardDto.Cost)
	Description       []CardDescriptionSectionDto `json:"description"`

	GameModes        []string             `json:"gameModes"`
	RequiresFeatures []string             `json:"requiresFeatures,omitempty"`
	RequiresSystems  []string             `json:"requiresSystems,omitempty"`
	Tags             []CardTag            `json:"tags,omitempty"`
	Requirements     *CardRequirementsDto `json:"requirements,omitempty"`
	Behaviors        []CardBehaviorDto    `json:"behaviors,omitempty"`
	ResourceStorage  *ResourceStorageDto  `json:"resourceStorage,omitempty"`
	VPConditions     []VPConditionDto     `json:"vpConditions,omitempty"`

	Available      bool                       `json:"available"`                // Computed: len(Errors) == 0
	Errors         []StateErrorDto            `json:"errors"`                   // Single source of truth for availability
	Warnings       []StateWarningDto          `json:"warnings,omitempty"`       // Non-blocking warnings
	EffectiveCost  int                        `json:"effectiveCost"`            // Effective cost after discounts (credits)
	Discounts      map[string]int             `json:"discounts,omitempty"`      // Discount amounts per resource type (if any)
	ComputedValues []ComputedBehaviorValueDto `json:"computedValues,omitempty"` // Pre-computed per-condition values
}

// PlayerEffectDto represents ongoing effects that a player has active for client consumption
// Aligned with PlayerActionDto structure for consistent behavior handling
type PlayerEffectDto struct {
	CardID         string                     `json:"cardId"`                   // ID of the card that provides this effect
	CardName       string                     `json:"cardName"`                 // Name of the card for display purposes
	BehaviorIndex  int                        `json:"behaviorIndex"`            // Which behavior on the card this effect represents
	Behavior       CardBehaviorDto            `json:"behavior"`                 // The actual behavior definition with inputs/outputs
	ComputedValues []ComputedBehaviorValueDto `json:"computedValues,omitempty"` // Pre-computed per-condition values
}

// ActionReuseOptionDto identifies a candidate and its current reuse availability.
type ActionReuseOptionDto struct {
	CardID        string          `json:"cardId"`
	BehaviorIndex int             `json:"behaviorIndex"`
	Available     bool            `json:"available"`
	Errors        []StateErrorDto `json:"errors"`
}

// PlayerActionDto represents an owned action with calculated usability.
type PlayerActionDto struct {
	DependencyTargets []string               `json:"dependencyTargets,omitempty"`
	ReuseOptions      []ActionReuseOptionDto `json:"reuseOptions,omitempty"`

	CardID        string          `json:"cardId"`        // ID of the card that provides this action
	CardName      string          `json:"cardName"`      // Name of the card for display purposes
	BehaviorIndex int             `json:"behaviorIndex"` // Which behavior on the card this action represents
	Behavior      CardBehaviorDto `json:"behavior"`      // The actual behavior definition with inputs/outputs

	TimesUsedThisTurn       int `json:"timesUsedThisTurn"`       // Times used this turn
	TimesUsedThisGeneration int `json:"timesUsedThisGeneration"` // Times used this generation

	Available      bool                       `json:"available"`                // Computed: action is usable
	Errors         []StateErrorDto            `json:"errors"`                   // Reasons why action is not usable
	Warnings       []StateWarningDto          `json:"warnings,omitempty"`       // Non-blocking warnings
	ComputedValues []ComputedBehaviorValueDto `json:"computedValues,omitempty"` // Pre-computed per-condition values
}

// PlayerStandardProjectDto represents a standard project with availability state
type PlayerStandardProjectDto struct {
	ProjectType string            `json:"projectType"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Behaviors   []CardBehaviorDto `json:"behaviors"`
	Style       *StyleDto         `json:"style,omitempty"`
	BaseCost    map[string]int    `json:"baseCost"`

	Available     bool                   `json:"available"`
	Errors        []StateErrorDto        `json:"errors"`
	Warnings      []StateWarningDto      `json:"warnings,omitempty"`
	EffectiveCost map[string]int         `json:"effectiveCost"`
	Discounts     map[string]int         `json:"discounts,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// PlayerResourceConversionDto is the availability state of a plants or heat conversion.
type PlayerResourceConversionDto struct {
	ProjectType   StandardProject   `json:"projectType"`
	Available     bool              `json:"available"`
	Errors        []StateErrorDto   `json:"errors"`
	Warnings      []StateWarningDto `json:"warnings,omitempty"`
	EffectiveCost map[string]int    `json:"effectiveCost"`
}

// StyleDto provides visual hints for the frontend
type StyleDto struct {
	Color string `json:"color"`
	Icon  string `json:"icon"`
}

// ForcedFirstActionDto represents an action that must be completed as the player's first turn action
type ForcedFirstActionDto struct {
	CorporationID string `json:"corporationId"`
	State         string `json:"state" tstype:"'queued' | 'resolving'"`
	Description   string `json:"description"`
}

// PendingTileSelectionDto represents a pending tile placement action for client consumption
type PendingTileSelectionDto struct {
	TileType       string   `json:"tileType"`       // "city", "greenery", "ocean"
	AvailableHexes []string `json:"availableHexes"` // Backend-calculated valid hex coordinates
	Source         string   `json:"source"`         // What triggered this selection (card ID, standard project, etc.)
}

// PendingCardSelectionDto represents a pending card selection action (e.g., sell patents, card effects)
type PendingCardSelectionDto struct {
	AvailableCards []PlayerCardDto `json:"availableCards"` // Cards with playability state
	CardCosts      map[string]int  `json:"cardCosts"`      // Card ID -> cost to select (0 for sell patents, 3 for buying cards)
	CardRewards    map[string]int  `json:"cardRewards"`    // Card ID -> reward for selecting (1 MC for sell patents)
	Source         string          `json:"source"`         // What triggered this selection ("sell-patents", card ID, etc.)
	MinCards       int             `json:"minCards"`       // Minimum cards to select (0 for sell patents)
	MaxCards       int             `json:"maxCards"`       // Maximum cards to select (hand size for sell patents)
}

// PendingCardDrawSelectionDto represents a pending card draw/peek/take/buy action from card effects
type PendingCardDrawSelectionDto struct {
	MinFreeTakeCount int             `json:"minFreeTakeCount"`
	AvailableCards   []PlayerCardDto `json:"availableCards"` // Cards with playability state
	FreeTakeCount    int             `json:"freeTakeCount"`  // Maximum number of cards to take for free
	MaxBuyCount      int             `json:"maxBuyCount"`    // Maximum cards to buy (optional, 0 = no buying allowed)
	CardBuyCost      int             `json:"cardBuyCost"`    // Cost per card when buying (typically 3 MC, 0 if no buying)
	Source           string          `json:"source"`         // Card ID or action that triggered this
	PlayAsPrelude    bool            `json:"playAsPrelude"`  // When true, selected card is played as prelude
}

// PendingBehaviorResolutionDto describes one independently resolvable decision.
type PendingBehaviorResolutionDto struct {
	ID                  string      `json:"id"`
	Kind                string      `json:"kind" tstype:"'card-discard' | 'choice'"`
	Source              string      `json:"source"`
	SourceCardID        string      `json:"sourceCardId"`
	SourceBehaviorIndex int         `json:"sourceBehaviorIndex"`
	TriggeringCardID    string      `json:"triggeringCardId,omitempty"`
	TriggeringCardName  string      `json:"triggeringCardName,omitempty"`
	MinCards            int         `json:"minCards"`
	MaxCards            int         `json:"maxCards"`
	Outputs             []any       `json:"outputs,omitempty" tstype:"ResourceCondition[]"`
	Choices             []ChoiceDto `json:"choices,omitempty"`
}

// PendingResourceRemovalSelectionDto represents a pending optional resource removal
type PendingResourceRemovalSelectionDto struct {
	ID                string         `json:"id"`
	MaxAmounts        map[string]int `json:"maxAmounts"`
	EligiblePlayerIDs []string       `json:"eligiblePlayerIds"`
	ResourceType      string         `json:"resourceType"`
	Amount            int            `json:"amount"`
	Source            string         `json:"source"`
	SourceCardID      string         `json:"sourceCardId"`
}

// PendingColonyResourceSelectionDto represents a pending card storage selection for colony resources
// ColonyResourceReason represents why a colony resource selection is pending
type ColonyResourceReason string

const (
	ColonyResourceReasonTrade       ColonyResourceReason = "trade"
	ColonyResourceReasonColonyTax   ColonyResourceReason = "colony-tax"
	ColonyResourceReasonBuild       ColonyResourceReason = "build"
	ColonyResourceReasonColonyBonus ColonyResourceReason = "colony-bonus"
)

// PendingColonyResourceSelectionDto represents a pending card storage selection for colony resources
type PendingColonyResourceSelectionDto struct {
	ResourceType string               `json:"resourceType"`
	Amount       int                  `json:"amount"`
	Source       string               `json:"source"`
	ColonyID     string               `json:"colonyId"`
	Reason       ColonyResourceReason `json:"reason"`
}

// PendingAwardFundSelectionDto represents a pending award fund selection for client consumption
type PendingAwardFundSelectionDto struct {
	AvailableAwards []string `json:"availableAwards"`
	Source          string   `json:"source"`
}

// PendingColonySelectionDto represents a pending colony selection from a card effect
type PendingColonySelectionDto struct {
	AddTile                    bool        `json:"addTile"`
	TileOptions                []ColonyDto `json:"tileOptions,omitempty"`
	AvailableColonyIDs         []string    `json:"availableColonyIds"`
	AllowDuplicatePlayerColony bool        `json:"allowDuplicatePlayerColony"`
	Source                     string      `json:"source"`
	SourceCardID               string      `json:"sourceCardId"`
}

// PendingFreeTradeSelectionDto represents a pending free trade colony selection
type PendingFreeTradeSelectionDto struct {
	AvailableColonyIDs []string `json:"availableColonyIds"`
	Source             string   `json:"source"`
	SourceCardID       string   `json:"sourceCardId"`
}

// PlayerStatus represents the current status of a player in the game
type PlayerStatus string

const (
	PlayerStatusSelectingStartingCards   PlayerStatus = "selecting-starting-cards"
	PlayerStatusSelectingProductionCards PlayerStatus = "selecting-production-cards"
	PlayerStatusWaiting                  PlayerStatus = "waiting"
	PlayerStatusActive                   PlayerStatus = "active"
	PlayerStatusSelection                PlayerStatus = "selection"
	PlayerStatusTile                     PlayerStatus = "tile"
	PlayerStatusExited                   PlayerStatus = "exited"
)

// PlayerDto represents a player in the game for client consumption
type ResourceRemovalTargetDto struct {
	PlayerID     string       `json:"playerId"`
	CardID       string       `json:"cardId,omitempty"`
	ResourceType ResourceType `json:"resourceType"`
	Amount       int          `json:"amount"`
}

type CardReceiptDto struct {
	ID           string    `json:"id"`
	Source       string    `json:"source"`
	SourceCardID string    `json:"sourceCardId"`
	Cards        []CardDto `json:"cards"`
}

type PlayerDto struct {
	PermanentProduction    ProductionDto                 `json:"permanentProduction"`
	CardReceipts           []CardReceiptDto              `json:"cardReceipts"`
	ResourceRemovalTargets []ResourceRemovalTargetDto    `json:"resourceRemovalTargets"`
	PendingCardReveal      *PendingCardRevealDto         `json:"pendingCardReveal,omitempty"`
	PendingEffectSelection *PendingEffectSelectionDto    `json:"pendingEffectSelection,omitempty"`
	ID                     string                        `json:"id"`
	Name                   string                        `json:"name"`
	PlayerType             string                        `json:"playerType"`
	BotStatus              string                        `json:"botStatus,omitempty" tstype:"'loading' | 'ready' | 'failed' | 'thinking'"`
	BotPersona             string                        `json:"botPersona,omitempty"`
	BotError               string                        `json:"botError,omitempty"`
	Color                  string                        `json:"color"`
	Status                 PlayerStatus                  `json:"status"`
	Corporation            *CardDto                      `json:"corporation"`
	Cards                  []PlayerCardDto               `json:"cards"` // Hand cards with playability state (Player-Scoped Architecture)
	Resources              ResourcesDto                  `json:"resources"`
	Production             ProductionDto                 `json:"production"`
	TerraformRating        int                           `json:"terraformRating"`
	PlayedCards            []CardDto                     `json:"playedCards"` // Full card details for all played cards
	Passed                 bool                          `json:"passed"`
	AvailableActions       int                           `json:"availableActions"`
	TotalActions           int                           `json:"totalActions"`
	IsConnected            bool                          `json:"isConnected"`
	IsExited               bool                          `json:"isExited"`
	Effects                []PlayerEffectDto             `json:"effects"`          // Active ongoing effects (discounts, special abilities, etc.)
	Actions                []PlayerActionDto             `json:"actions"`          // Available actions from played cards with manual triggers
	StandardProjects       []PlayerStandardProjectDto    `json:"standardProjects"` // Standard projects with availability state (Player-Scoped Architecture)
	ResourceConversions    []PlayerResourceConversionDto `json:"resourceConversions"`
	Milestones             []PlayerMilestoneDto          `json:"milestones"` // Milestones with player eligibility state
	Awards                 []PlayerAwardDto              `json:"awards"`     // Awards with player eligibility state

	DemoReady          bool                   `json:"demoReady"`
	PendingDemoChoices *PendingDemoChoicesDto `json:"pendingDemoChoices,omitempty"`

	SelectCorporationPhase          *SelectCorporationPhaseDto          `json:"selectCorporationPhase"`
	SelectStartingCardsPhase        *SelectStartingCardsPhaseDto        `json:"selectStartingCardsPhase"`
	SelectPreludeCardsPhase         *SelectPreludeCardsPhaseDto         `json:"selectPreludeCardsPhase"`
	ProductionPhase                 *ProductionPhaseDto                 `json:"productionPhase"`
	StartingCards                   []CardDto                           `json:"startingCards"`
	PendingTileSelection            *PendingTileSelectionDto            `json:"pendingTileSelection"`
	PendingCardSelection            *PendingCardSelectionDto            `json:"pendingCardSelection"`
	PendingCardDrawSelection        *PendingCardDrawSelectionDto        `json:"pendingCardDrawSelection"`
	PendingBehaviorResolutions      []PendingBehaviorResolutionDto      `json:"pendingBehaviorResolutions"`
	PendingResourceRemovalSelection *PendingResourceRemovalSelectionDto `json:"pendingResourceRemovalSelection"`
	PendingColonyResourceSelection  *PendingColonyResourceSelectionDto  `json:"pendingColonyResourceSelection"`
	PendingAwardFundSelection       *PendingAwardFundSelectionDto       `json:"pendingAwardFundSelection"`
	PendingColonySelection          *PendingColonySelectionDto          `json:"pendingColonySelection"`
	PendingFreeTradeSelection       *PendingFreeTradeSelectionDto       `json:"pendingFreeTradeSelection"`
	ForcedFirstAction               *ForcedFirstActionDto               `json:"forcedFirstAction"`
	ResourceStorage                 map[string]int                      `json:"resourceStorage"`
	PaymentSubstitutes              []PaymentSubstituteDto              `json:"paymentSubstitutes"`
	GenerationalEvents              []PlayerGenerationalEventEntryDto   `json:"generationalEvents"`
	VPGranters                      []VPGranterDto                      `json:"vpGranters"`
	BonusTags                       map[string]int                      `json:"bonusTags"`
	ActionCosts                     []ActionCostDto                     `json:"actionCosts"`

	// HasAvailableActions reports whether the player has any legal move this turn.
	// Populated only on the WebSocket broadcast path where every action registry is
	// present; omitted (nil) on the HTTP path that lacks registries, so the field is
	// an explicit "unknown" rather than a misleading false.
	HasAvailableActions *bool `json:"hasAvailableActions,omitempty"`
}

// ActionCostDto represents the costs for a specific action type (e.g., card-buying, colony-trade)
type ActionCostDto struct {
	ActionType string               `json:"actionType"`
	Costs      []ActionCostEntryDto `json:"costs"`
}

// ActionCostEntryDto represents a single resource cost for an action
type ActionCostEntryDto struct {
	PaymentCapacity int    `json:"paymentCapacity"`
	Resource        string `json:"resource"`
	BaseCost        int    `json:"baseCost"`
	EffectiveCost   int    `json:"effectiveCost"`
	Discount        int    `json:"discount"`
}

// OtherPlayerDto represents another player from the viewing player's perspective (limited data)
type OtherPlayerDto struct {
	PermanentProduction ProductionDto     `json:"permanentProduction"`
	ID                  string            `json:"id"`
	Name                string            `json:"name"`
	PlayerType          string            `json:"playerType"`
	BotStatus           string            `json:"botStatus,omitempty" tstype:"'loading' | 'ready' | 'failed' | 'thinking'"`
	BotPersona          string            `json:"botPersona,omitempty"`
	BotError            string            `json:"botError,omitempty"`
	Color               string            `json:"color"`
	Status              PlayerStatus      `json:"status"`
	Corporation         *CardDto          `json:"corporation"`
	HandCardCount       int               `json:"handCardCount"` // Number of cards in hand (private)
	Resources           ResourcesDto      `json:"resources"`
	Production          ProductionDto     `json:"production"`
	TerraformRating     int               `json:"terraformRating"`
	PlayedCards         []CardDto         `json:"playedCards"` // Played cards are public - full card details
	Passed              bool              `json:"passed"`
	AvailableActions    int               `json:"availableActions"`
	TotalActions        int               `json:"totalActions"`
	IsConnected         bool              `json:"isConnected"`
	IsExited            bool              `json:"isExited"`
	Effects             []PlayerEffectDto `json:"effects"`
	Actions             []PlayerActionDto `json:"actions"`

	DemoReady bool `json:"demoReady"`

	SelectCorporationPhase   *SelectCorporationOtherPlayerDto   `json:"selectCorporationPhase"`
	SelectStartingCardsPhase *SelectStartingCardsOtherPlayerDto `json:"selectStartingCardsPhase"`
	SelectPreludeCardsPhase  *SelectPreludeCardsOtherPlayerDto  `json:"selectPreludeCardsPhase"`
	ProductionPhase          *ProductionPhaseOtherPlayerDto     `json:"productionPhase"`
	ResourceStorage          map[string]int                     `json:"resourceStorage"`
	PaymentSubstitutes       []PaymentSubstituteDto             `json:"paymentSubstitutes"`
	VPGranters               []VPGranterDto                     `json:"vpGranters"`
	BonusTags                map[string]int                     `json:"bonusTags"`
}

// GameDto represents a game for client consumption (clean architecture)
type GameDto struct {
	ResumeLobby        *ResumeLobbyDto          `json:"resumeLobby,omitempty"`
	ModeRuntime        *ModeRuntimeDto          `json:"modeRuntime,omitempty"`
	ID                 string                   `json:"id"`
	Status             GameStatus               `json:"status"`
	Settings           GameSettingsDto          `json:"settings"`
	HostPlayerID       string                   `json:"hostPlayerId"`
	CurrentPhase       GamePhase                `json:"currentPhase"`
	GlobalParameters   GlobalParametersDto      `json:"globalParameters"`
	CurrentPlayer      PlayerDto                `json:"currentPlayer"`   // Viewing player's full data
	OtherPlayers       []OtherPlayerDto         `json:"otherPlayers"`    // Other players' limited data
	ViewingPlayerID    string                   `json:"viewingPlayerId"` // The player viewing this game state
	CurrentTurn        *string                  `json:"currentTurn"`     // Whose turn it is (nullable)
	Generation         int                      `json:"generation"`
	PlayerOrder        []string                 `json:"playerOrder"`                // Player IDs in join order
	TurnOrder          []string                 `json:"turnOrder"`                  // Turn order of all players in game
	Board              BoardDto                 `json:"board"`                      // Game board with tiles and occupancy state
	PaymentConstants   PaymentConstantsDto      `json:"paymentConstants"`           // Conversion rates for alternative payments
	Milestones         []MilestoneDto           `json:"milestones"`                 // All milestones with claim status
	Awards             []AwardDto               `json:"awards"`                     // All awards with funding status
	AwardResults       []AwardResultDto         `json:"awardResults"`               // Current award placements (1st/2nd place per award)
	FinalScores        []FinalScoreDto          `json:"finalScores,omitempty"`      // Final scores (only when game completed)
	TriggeredEffects   []TriggeredEffectDto     `json:"triggeredEffects,omitempty"` // Recently triggered passive effects
	PlaceableTileTypes []PlaceableTileTypeDto   `json:"placeableTileTypes"`         // Available tile types for the demo tile picker
	InitPhase          *InitPhaseDto            `json:"initPhase,omitempty"`
	Spectators         []SpectatorDto           `json:"spectators"`
	ChatMessages       []ChatMessageDto         `json:"chatMessages"`
	IsSpectator        bool                     `json:"isSpectator"`
	Colonies           []ColonyDto              `json:"colonies,omitempty"`
	TradeFleets        map[string]TradeFleetDto `json:"tradeFleets,omitempty"`
	ProjectFunding     []ProjectFundingDto      `json:"projectFunding,omitempty"`
	IsLastRound        bool                     `json:"isLastRound"`
}

// SpectatorDto represents a spectator visible to all clients.
type SpectatorDto struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// ChatMessageDto represents a chat message sent by a player or spectator.
type ChatMessageDto struct {
	SenderID    string `json:"senderId"`
	SenderName  string `json:"senderName"`
	SenderColor string `json:"senderColor"`
	Message     string `json:"message"`
	Timestamp   string `json:"timestamp"`
	IsSpectator bool   `json:"isSpectator"`
	Kind        string `json:"kind" tstype:"'chat' | 'recap' | 'system'"`
}

// PlaceableTileTypeDto represents a tile type available for placement in the demo tile picker
type PlaceableTileTypeDto struct {
	Type  string `json:"type"`
	Label string `json:"label"`
	Group string `json:"group"`
}

// InitPhaseStage is the showcase stage of the current init player
type InitPhaseStage string

const (
	InitPhaseStageReveal  InitPhaseStage = "reveal"
	InitPhaseStageApplied InitPhaseStage = "applied"
	InitPhaseStageRoster  InitPhaseStage = "roster"
)

// InitPhaseDto represents the state of the init_apply_corp or init_apply_prelude phase
type InitPhaseDto struct {
	CurrentPlayerID     string         `json:"currentPlayerId"`
	CurrentPlayerIndex  int            `json:"currentPlayerIndex"`
	TotalPlayers        int            `json:"totalPlayers"`
	WaitingForConfirm   bool           `json:"waitingForConfirm"`
	ConfirmVersion      int            `json:"confirmVersion"`
	HasPreludePhase     bool           `json:"hasPreludePhase"`
	Stage               InitPhaseStage `json:"stage" tstype:"'reveal' | 'applied' | 'roster'"`
	Preludes            []CardDto      `json:"preludes"`
	PreludesPlayed      int            `json:"preludesPlayed"`
	HasPendingSelection bool           `json:"hasPendingSelection"`
	PendingSourceCardID string         `json:"pendingSourceCardId"`
}

// Colony-related DTOs

// ColonyDto represents a colony in the game
type ColonyDto struct {
	Active         bool                   `json:"active"`
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Location       string                 `json:"location"`
	Steps          []ColonyStepDto        `json:"steps"`
	ColonyBonus    []ColonyOutputDto      `json:"colonyBonus"`
	Colonies       []ColonySlotDto        `json:"colonies"`
	MarkerPosition int                    `json:"markerPosition"`
	PlayerColonies []string               `json:"playerColonies"`
	TradedThisGen  bool                   `json:"tradedThisGen"`
	TraderID       string                 `json:"traderId"`
	Style          StyleDto               `json:"style"`
	TradeOptions   []ColonyTradeOptionDto `json:"tradeOptions"`
	TradeAvailable bool                   `json:"tradeAvailable"`
	BuildAvailable bool                   `json:"buildAvailable"`
	TradeErrors    []StateErrorDto        `json:"tradeErrors"`
	BuildErrors    []StateErrorDto        `json:"buildErrors"`
}

// ColonyStepDto represents one position on the trade track
type ColonyStepDto struct {
	Outputs []ColonyOutputDto `json:"outputs"`
}

// ColonyOutputDto represents a resource output from a colony
type ColonyOutputDto struct {
	Type   string `json:"type"`
	Amount int    `json:"amount"`
}

// ColonySlotDto represents a colony placement slot
type ColonySlotDto struct {
	Reward []ColonyOutputDto `json:"reward"`
}

// Project Funding DTOs

// ProjectFundingDto represents a project funding tile in the game
type ProjectFundingDto struct {
	ID                 string                     `json:"id"`
	Name               string                     `json:"name"`
	Description        string                     `json:"description"`
	Seats              []ProjectSeatDto           `json:"seats"`
	SeatOwners         []ProjectSeatOwnerDto      `json:"seatOwners"`
	IsCompleted        bool                       `json:"isCompleted"`
	NextSeatIndex      int                        `json:"nextSeatIndex"`
	NextSeatCost       int                        `json:"nextSeatCost"`
	CanBuySeat         bool                       `json:"canBuySeat"`
	BuyErrors          []StateErrorDto            `json:"buyErrors"`
	CurrentPlayerSeats int                        `json:"currentPlayerSeats"`
	CurrentPlayerTier  *ProjectRewardTierDto      `json:"currentPlayerTier,omitempty"`
	PaymentSubstitutes []ProjectPaymentSubDto     `json:"paymentSubstitutes"`
	RewardTiers        []ProjectRewardTierDto     `json:"rewardTiers"`
	CompletionEffect   ProjectCompletionEffectDto `json:"completionEffect"`
	Style              StyleDto                   `json:"style"`
}

// ProjectSeatDto represents a seat definition
type ProjectSeatDto struct {
	Cost               int                    `json:"cost"`
	PaymentSubstitutes []ProjectPaymentSubDto `json:"paymentSubstitutes"`
	OwnerID            string                 `json:"ownerId"`
	OwnerName          string                 `json:"ownerName"`
	OwnerColor         string                 `json:"ownerColor"`
	IsFilled           bool                   `json:"isFilled"`
}

// ProjectSeatOwnerDto represents a seat owner entry
type ProjectSeatOwnerDto struct {
	PlayerID string `json:"playerId"`
	Name     string `json:"name"`
	Color    string `json:"color"`
}

// ProjectRewardTierDto represents a reward tier
type ProjectRewardTierDto struct {
	SeatsOwned int               `json:"seatsOwned"`
	Rewards    []ColonyOutputDto `json:"rewards"`
}

// ProjectCompletionEffectDto represents the completion effect
type ProjectCompletionEffectDto struct {
	Description   string                   `json:"description"`
	Rewards       []ColonyOutputDto        `json:"rewards"`
	GlobalEffects []ProjectGlobalOutputDto `json:"globalEffects,omitempty"`
}

// ProjectGlobalOutputDto represents a one-time game-wide effect on project completion
type ProjectGlobalOutputDto struct {
	Type   string `json:"type"`
	Amount int    `json:"amount"`
}

// ProjectPaymentSubDto represents a payment substitute for a seat
type ProjectPaymentSubDto struct {
	ResourceType   string `json:"resourceType"`
	ConversionRate int    `json:"conversionRate"`
}

// Board-related DTOs for tygo generation

// TileBonusDto represents a resource bonus provided by a tile when occupied
type TileBonusDto struct {
	Type   string `json:"type"`
	Amount int    `json:"amount"`
}

// TileOccupantDto represents what currently occupies a tile
type TileOccupantDto struct {
	DisplayName string         `json:"displayName,omitempty"`
	Visual      *TileVisualDto `json:"visual,omitempty"`
	Type        string         `json:"type"`
	Tags        []string       `json:"tags"`
}

// TileDto represents a single hexagonal tile on the game board
type TileDto struct {
	Coordinates HexPositionDto   `json:"coordinates"`
	Tags        []string         `json:"tags"`
	Type        string           `json:"type"`
	Location    string           `json:"location"`
	DisplayName *string          `json:"displayName,omitempty"`
	Bonuses     []TileBonusDto   `json:"bonuses"`
	OccupiedBy  *TileOccupantDto `json:"occupiedBy,omitempty"`
	OwnerID     *string          `json:"ownerId,omitempty"`
	ReservedBy  *string          `json:"reservedBy,omitempty"`
}

// BoardDto represents the game board containing all tiles
type BoardDto struct {
	Tiles []TileDto `json:"tiles"`
}

// MilestoneDto represents a milestone for client consumption
type MilestoneDto struct {
	Type           string           `json:"type"`
	Name           string           `json:"name"`
	Description    string           `json:"description"`
	IsClaimed      bool             `json:"isClaimed"`
	ClaimedBy      *string          `json:"claimedBy"`
	ClaimCost      int              `json:"claimCost"`
	Required       int              `json:"required"`
	PlayerProgress map[string]int   `json:"playerProgress"`
	Reward         []AwardRewardDto `json:"rewards"`
	Style          *StyleDto        `json:"style,omitempty"`
}

// AwardDto represents an award for client consumption
type AwardDto struct {
	Type           string           `json:"type"`
	Name           string           `json:"name"`
	Description    string           `json:"description"`
	IsFunded       bool             `json:"isFunded"`
	FundedBy       *string          `json:"fundedBy"`
	FundingCost    int              `json:"fundingCost"`
	PlayerProgress map[string]int   `json:"playerProgress"`
	Rewards        []AwardRewardDto `json:"rewards"`
	Style          *StyleDto        `json:"style,omitempty"`
}

// AwardRewardDto represents a placement reward
type AwardRewardDto struct {
	Place   int   `json:"place"`
	Outputs []any `json:"outputs" tstype:"ResourceCondition[]"`
}

// AwardResultDto represents the placement results for a single funded award
type AwardResultDto struct {
	AwardType      string   `json:"awardType"`
	FirstPlaceIds  []string `json:"firstPlaceIds"`
	SecondPlaceIds []string `json:"secondPlaceIds"`
}

// PlayerMilestoneDto represents a milestone with player-specific eligibility state
type PlayerMilestoneDto struct {
	Type        string           `json:"type"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	ClaimCost   int              `json:"claimCost"`
	IsClaimed   bool             `json:"isClaimed"`
	ClaimedBy   *string          `json:"claimedBy"`
	Available   bool             `json:"available"` // Can this player claim this milestone?
	Progress    int              `json:"progress"`  // Current progress towards requirement
	Required    int              `json:"required"`  // Requirement threshold
	Errors      []StateErrorDto  `json:"errors"`    // Reasons why not available
	Reward      []AwardRewardDto `json:"rewards"`
	Style       *StyleDto        `json:"style,omitempty"`
}

// PlayerAwardDto represents an award with player-specific eligibility state
type PlayerAwardDto struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	FundingCost int             `json:"fundingCost"` // Current cost to fund (increases as more are funded)
	IsFunded    bool            `json:"isFunded"`
	FundedBy    *string         `json:"fundedBy"`
	Available   bool            `json:"available"` // Can this player fund this award?
	Errors      []StateErrorDto `json:"errors"`    // Reasons why not available
	Style       *StyleDto       `json:"style,omitempty"`
}

// VPGranterConditionDto represents a single VP condition's computed breakdown for client consumption
type VPGranterConditionDto struct {
	Amount             int     `json:"amount"`
	ConditionType      string  `json:"conditionType"`
	PerType            *string `json:"perType,omitempty"`
	PerAmount          *int    `json:"perAmount,omitempty"`
	AdjacentToSelfTile bool    `json:"adjacentToSelfTile"`
	Count              int     `json:"count"`
	ComputedVP         int     `json:"computedVP"`
	Explanation        string  `json:"explanation"`
}

// VPGranterDto represents a VP source from a played card or corporation for client consumption
type VPGranterDto struct {
	CardID        string                  `json:"cardId"`
	CardName      string                  `json:"cardName"`
	Description   string                  `json:"description"`
	ComputedValue int                     `json:"computedValue"`
	Conditions    []VPGranterConditionDto `json:"conditions"`
}

// CardVPConditionDetailDto represents the detailed calculation of a single VP condition
type CardVPConditionDetailDto struct {
	ConditionType  string `json:"conditionType"` // "fixed", "per", "once"
	Amount         int    `json:"amount"`        // VP amount per trigger or fixed amount
	Count          int    `json:"count"`         // Items counted (for "per" conditions)
	MaxTrigger     *int   `json:"maxTrigger"`
	ActualTriggers int    `json:"actualTriggers"` // Actual triggers after applying max
	TotalVP        int    `json:"totalVP"`        // Final VP from this condition
	Explanation    string `json:"explanation"`    // Human-readable breakdown
}

// CardVPDetailDto represents VP calculation for a single card
type CardVPDetailDto struct {
	CardID     string                     `json:"cardId"`
	CardName   string                     `json:"cardName"`
	Conditions []CardVPConditionDetailDto `json:"conditions"`
	TotalVP    int                        `json:"totalVP"`
}

// GreeneryVPDetailDto represents VP from a single greenery tile
type GreeneryVPDetailDto struct {
	Coordinate string `json:"coordinate"` // Format: "q,r,s"
	VP         int    `json:"vp"`         // Always 1 per greenery
}

// CityVPDetailDto represents VP from a single city tile and its adjacent greeneries
type CityVPDetailDto struct {
	CityCoordinate     string   `json:"cityCoordinate"`     // Format: "q,r,s"
	AdjacentGreeneries []string `json:"adjacentGreeneries"` // Coordinates of adjacent greenery tiles
	VP                 int      `json:"vp"`                 // Number of adjacent greeneries
}

// VPBreakdownDto represents a breakdown of victory points for client consumption
type VPBreakdownDto struct {
	TerraformRating   int                   `json:"terraformRating"`
	CardVP            int                   `json:"cardVP"`
	CardVPDetails     []CardVPDetailDto     `json:"cardVPDetails"` // Per-card VP breakdown
	MilestoneVP       int                   `json:"milestoneVP"`
	AwardVP           int                   `json:"awardVP"`
	GreeneryVP        int                   `json:"greeneryVP"`
	GreeneryVPDetails []GreeneryVPDetailDto `json:"greeneryVPDetails"` // Per-greenery VP breakdown
	CityVP            int                   `json:"cityVP"`
	CityVPDetails     []CityVPDetailDto     `json:"cityVPDetails"` // Per-city VP breakdown with adjacencies
	TotalVP           int                   `json:"totalVP"`
}

// FinalScoreDto represents a player's final score for client consumption
type FinalScoreDto struct {
	PlayerID    string         `json:"playerId"`
	PlayerName  string         `json:"playerName"`
	VPBreakdown VPBreakdownDto `json:"vpBreakdown"`
	IsWinner    bool           `json:"isWinner"`
	Placement   int            `json:"placement"`
}

// TriggeredEffectDto represents a card effect that was triggered for client notification
type TriggeredEffectDto struct {
	CardName          string                `json:"cardName"`
	PlayerID          string                `json:"playerId"`
	SourceType        string                `json:"sourceType"`
	Outputs           []any                 `json:"outputs" tstype:"ResourceCondition[]"`
	CalculatedOutputs []CalculatedOutputDto `json:"calculatedOutputs,omitempty"`
	Behaviors         []CardBehaviorDto     `json:"behaviors,omitempty"`
	VPConditions      []VPConditionDto      `json:"vpConditions,omitempty"`
}

// GenerationalEvent represents events tracked within a generation for conditional card behaviors
type GenerationalEvent string

const (
	GenerationalEventTRRaise           GenerationalEvent = "tr-raise"
	GenerationalEventOceanPlacement    GenerationalEvent = "ocean-placement"
	GenerationalEventCityPlacement     GenerationalEvent = "city-placement"
	GenerationalEventGreeneryPlacement GenerationalEvent = "greenery-placement"
)

// PlayerGenerationalEventEntryDto represents a player's tracked generational event for client consumption
type PlayerGenerationalEventEntryDto struct {
	Event GenerationalEvent `json:"event"`
	Count int               `json:"count"`
}

// GenerationalEventRequirementDto represents a requirement based on generational events for card behaviors
type GenerationalEventRequirementDto struct {
	Event  GenerationalEvent `json:"event"`
	Count  *MinMaxValueDto   `json:"count,omitempty"`
	Target *TargetType       `json:"target,omitempty"`
}

// CityStyleRequestDto contains optional visual requests, independent of placement rules.
type CityStyleRequestDto struct {
	Plan          string   `json:"plan,omitempty" tstype:"'clustered' | 'radial' | 'grid' | 'courtyard'"`
	Density       string   `json:"density,omitempty" tstype:"'sparse' | 'balanced' | 'dense'"`
	Heights       string   `json:"heights,omitempty" tstype:"'low' | 'mixed' | 'tall'"`
	Ground        string   `json:"ground,omitempty" tstype:"'level' | 'recessed'"`
	Exposure      string   `json:"exposure,omitempty" tstype:"'surface' | 'mostly-buried'"`
	Perimeter     string   `json:"perimeter,omitempty" tstype:"'open' | 'low-wall' | 'high-wall'"`
	Cover         string   `json:"cover,omitempty" tstype:"'none' | 'dome' | 'flat-glass'"`
	Landmark      string   `json:"landmark,omitempty" tstype:"'none' | 'hall' | 'observatory'"`
	Connections   string   `json:"connections,omitempty" tstype:"'paths' | 'skybridges' | 'enclosed'"`
	Landscaping   string   `json:"landscaping,omitempty" tstype:"'sparse' | 'parks' | 'lush'"`
	Lighting      string   `json:"lighting,omitempty" tstype:"'normal' | 'bright'"`
	PerimeterRoad string   `json:"perimeterRoad,omitempty" tstype:"'none' | 'ring'"`
	EntranceMin   int      `json:"entranceMin,omitempty"`
	EntranceMax   int      `json:"entranceMax,omitempty"`
	Details       []string `json:"details,omitempty"`
}

// CardStyleDto contains card tile appearance requests.
type CardStyleDto struct {
	Tile *CityStyleRequestDto `json:"tile,omitempty"`
}

// TileVisualDto is the stable visual input shared by every viewer.
type TileVisualDto struct {
	Seed uint32               `json:"seed"`
	City *CityStyleRequestDto `json:"city,omitempty"`
}

// CopyConditionDto describes the source and scope of a copy operation.
type CopyConditionDto struct {
	Type      string        `json:"type" tstype:"'copy'"`
	Amount    int           `json:"amount"`
	Target    TargetType    `json:"target"`
	Scope     string        `json:"scope" tstype:"'production-box'"`
	Zone      string        `json:"zone" tstype:"'played'"`
	Selectors []SelectorDto `json:"selectors,omitempty"`
}

func (d CopyConditionDto) GetConditionType() string { return d.Type }
func (d CopyConditionDto) GetConditionAmount() int  { return d.Amount }

type PendingEffectSelectionDto struct {
	Source       string                     `json:"source"`
	SourceCardID string                     `json:"sourceCardId"`
	Outputs      []any                      `json:"outputs" tstype:"ResourceCondition[]"`
	Options      []EffectSelectionOptionDto `json:"options"`
}
type EffectSelectionOptionDto struct {
	CardID         string   `json:"cardId,omitempty"`
	TargetPlayerID string   `json:"targetPlayerId,omitempty"`
	ColonyIDs      []string `json:"colonyIds,omitempty"`
	Outputs        []any    `json:"outputs,omitempty" tstype:"ResourceCondition[]"`
}

// CardRevealConditionDto defines a public project-card reveal.
type CardRevealConditionDto struct {
	Type        string          `json:"type" tstype:"'card-reveal'"`
	Amount      int             `json:"amount"`
	Target      TargetType      `json:"target"`
	Destination string          `json:"destination" tstype:"'discard'"`
	OnMatch     *RevealMatchDto `json:"onMatch,omitempty"`
}

func (d CardRevealConditionDto) GetConditionType() string { return d.Type }
func (d CardRevealConditionDto) GetConditionAmount() int  { return d.Amount }

type RevealMatchDto struct {
	Selectors []SelectorDto `json:"selectors"`
	Outputs   []any         `json:"outputs" tstype:"ResourceCondition[]"`
}

// RevealedCardDto is public information from a resolved reveal.
type RevealedCardDto struct {
	CardID  string `json:"cardId"`
	Name    string `json:"name"`
	Matched bool   `json:"matched"`
}

// PendingCardRevealDto presents the resolved result for acknowledgement.
type PendingCardRevealDto struct {
	Source       string                `json:"source"`
	SourceCardID string                `json:"sourceCardId"`
	Cards        []CardDto             `json:"cards"`
	Results      []RevealedCardDto     `json:"results"`
	Rewards      []CalculatedOutputDto `json:"rewards"`
}

// TradeFleetDto reports permanent fleet capacity and remaining trades.
type TradeFleetDto struct {
	Total     int `json:"total"`
	Available int `json:"available"`
}

// ColonyTradeOptionDto previews a legal increase and all gains for the viewing player.
type ColonyTradeOptionDto struct {
	TrackSteps     int               `json:"trackSteps"`
	MarkerPosition int               `json:"markerPosition"`
	Outputs        []ColonyOutputDto `json:"outputs"`
}

// ColonyTradeRequest supplies an explicit track choice for a paid trade.
type ColonyTradeRequest struct {
	Payment     PaymentDto `json:"payment"`
	ColonyID    string     `json:"colonyId"`
	PaymentType string     `json:"paymentType"`
	TrackSteps  *int       `json:"trackSteps" tstype:"number"`
}

// FreeTradeRequest supplies an explicit track choice for a pending free trade.
type FreeTradeRequest struct {
	ColonyID   string `json:"colonyId"`
	TrackSteps *int   `json:"trackSteps" tstype:"number"`
}

// PaymentIntentDto identifies the operation to quote.
type PaymentIntentDto struct {
	Action             string   `json:"action"`
	CardID             string   `json:"cardId,omitempty"`
	BehaviorIndex      int      `json:"behaviorIndex,omitempty"`
	ChoiceIndex        *int     `json:"choiceIndex,omitempty"`
	SelectedAmount     *int     `json:"selectedAmount,omitempty"`
	CardStorageSources []string `json:"cardStorageSources,omitempty"`
	ProjectID          string   `json:"projectId,omitempty"`
	CorporationID      string   `json:"corporationId,omitempty"`
	CardIDs            []string `json:"cardIds,omitempty"`
	CardsToBuy         []string `json:"cardsToBuy,omitempty"`
	RandomBuy          bool     `json:"randomBuy,omitempty"`
	PaymentType        string   `json:"paymentType,omitempty"`
	MilestoneType      string   `json:"milestoneType,omitempty"`
	AwardType          string   `json:"awardType,omitempty"`
}

// ResumeSeatDto describes a saved seat without exposing private game state.
type ResumeSeatDto struct {
	CorporationName string `json:"corporationName"`
	ID              string `json:"id"`
	Name            string `json:"name"`
	SavedName       string `json:"savedName"`
	CorporationID   string `json:"corporationId"`
	Color           string `json:"color"`
	PlayerType      string `json:"playerType"`
	Claimed         bool   `json:"claimed"`
	Connected       bool   `json:"connected"`
	Exited          bool   `json:"exited"`
	BotStatus       string `json:"botStatus"`
	BotError        string `json:"botError"`
}

type ResumeLobbyDto struct {
	Seats []ResumeSeatDto `json:"seats"`
}

type GameSaveSummaryDto struct {
	Generation     int             `json:"generation"`
	Phase          GamePhase       `json:"phase"`
	MapID          string          `json:"mapId"`
	SavedAt        string          `json:"savedAt"`
	Seats          []ResumeSeatDto `json:"seats"`
	HistoryEntries int             `json:"historyEntries"`
	LogEntries     int             `json:"logEntries"`
}

type ImportGameSaveResponse struct {
	GameID     string `json:"gameId"`
	PlayerID   string `json:"playerId"`
	PlayerName string `json:"playerName"`
}

type ResumeGameRequest struct {
	GameID     string `json:"gameId"`
	SeatID     string `json:"seatId,omitempty"`
	PlayerName string `json:"playerName,omitempty"`
	BotToken   string `json:"botToken,omitempty"`
}

// GameSaveErrorResponse contains a stable save failure code and a safe player message.
type GameSaveErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
