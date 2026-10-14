package shared

// BasicResourceCondition covers credit, steel, titanium, plant, energy, heat.
type BasicResourceCondition struct {
	ConditionBase
	Per               *PerCondition      `json:"per,omitempty"`
	VariableAmount    bool               `json:"variableAmount,omitempty"`
	MaxTrigger        *int               `json:"maxTrigger,omitempty"`
	Optional          bool               `json:"optional,omitempty"`
	TargetRestriction *TargetRestriction `json:"targetRestriction,omitempty"`
	PaymentAllowed    []ResourceType     `json:"paymentAllowed,omitempty"`
}

func NewBasicResourceCondition(rt ResourceType, amount int, target string) *BasicResourceCondition {
	return &BasicResourceCondition{ConditionBase: ConditionBase{ResourceType: rt, Amount: amount, Target: target}}
}

func (c *BasicResourceCondition) isBehaviorCondition() {}
func (c *BasicResourceCondition) deepCopyCondition() BehaviorCondition {
	cp := *c
	if c.Per != nil {
		cp.Per = c.Per.Clone()
	}
	if c.MaxTrigger != nil {
		v := *c.MaxTrigger
		cp.MaxTrigger = &v
	}
	if c.TargetRestriction != nil {
		tr := *c.TargetRestriction
		tr.Selectors = CloneSelectors(tr.Selectors)
		cp.TargetRestriction = &tr
	}
	if c.PaymentAllowed != nil {
		pa := make([]ResourceType, len(c.PaymentAllowed))
		copy(pa, c.PaymentAllowed)
		cp.PaymentAllowed = pa
	}
	return &cp
}

// ProductionCondition covers credit-production through any-production.
type ProductionCondition struct {
	Dependency *DependencyGroup `json:"dependencyGroup,omitempty"`
	ConditionBase
	Per            *PerCondition `json:"per,omitempty"`
	VariableAmount bool          `json:"variableAmount,omitempty"`
}

func NewProductionCondition(rt ResourceType, amount int, target string) *ProductionCondition {
	return &ProductionCondition{ConditionBase: ConditionBase{ResourceType: rt, Amount: amount, Target: target}}
}

func (c *ProductionCondition) isBehaviorCondition() {}
func (c *ProductionCondition) deepCopyCondition() BehaviorCondition {
	cp := *c
	if c.Dependency != nil {
		d := *c.Dependency
		if d.RetireOnState != nil {
			r := *d.RetireOnState
			d.RetireOnState = &r
		}
		cp.Dependency = &d
	}
	if c.Per != nil {
		cp.Per = c.Per.Clone()
	}
	return &cp
}

// TilePlacementCondition covers city/greenery/ocean/volcano/tile-placement and land-claim.
type TilePlacementCondition struct {
	ConditionBase
	TileRestrictions *TileRestrictions `json:"tileRestrictions,omitempty"`
	TileType         string            `json:"tileType,omitempty"`
	Optional         bool              `json:"optional,omitempty"`
}

func NewTilePlacementCondition(rt ResourceType, amount int, target string) *TilePlacementCondition {
	return &TilePlacementCondition{ConditionBase: ConditionBase{ResourceType: rt, Amount: amount, Target: target}}
}

func (c *TilePlacementCondition) isBehaviorCondition() {}
func (c *TilePlacementCondition) deepCopyCondition() BehaviorCondition {
	cp := *c
	cp.TileRestrictions = c.TileRestrictions.Clone()
	return &cp
}

// GlobalParameterCondition covers temperature, oxygen, ocean, venus, tr, global-parameter.
type GlobalParameterCondition struct {
	ConditionBase
	Per *PerCondition `json:"per,omitempty"`
}

func NewGlobalParameterCondition(rt ResourceType, amount int, target string) *GlobalParameterCondition {
	return &GlobalParameterCondition{ConditionBase: ConditionBase{ResourceType: rt, Amount: amount, Target: target}}
}

func (c *GlobalParameterCondition) isBehaviorCondition() {}
func (c *GlobalParameterCondition) deepCopyCondition() BehaviorCondition {
	cp := *c
	if c.Per != nil {
		cp.Per = c.Per.Clone()
	}
	return &cp
}

// CardOperationCondition covers card-draw, card-take, card-peek, card-buy, card-discard.
type CardOperationCondition struct {
	CostReduction int `json:"costReduction,omitempty"`
	ConditionBase
	Per            *PerCondition  `json:"per,omitempty"`
	Selectors      []Selector     `json:"selectors,omitempty"`
	Optional       bool           `json:"optional,omitempty"`
	PaymentAllowed []ResourceType `json:"paymentAllowed,omitempty"`
	VariableAmount bool           `json:"variableAmount,omitempty"`
}

func NewCardOperationCondition(rt ResourceType, amount int, target string) *CardOperationCondition {
	return &CardOperationCondition{ConditionBase: ConditionBase{ResourceType: rt, Amount: amount, Target: target}}
}

func (c *CardOperationCondition) isBehaviorCondition() {}
func (c *CardOperationCondition) deepCopyCondition() BehaviorCondition {
	cp := *c
	cp.Per = c.Per.Clone()
	if c.Selectors != nil {
		cp.Selectors = CloneSelectors(c.Selectors)
	}
	if c.PaymentAllowed != nil {
		pa := make([]ResourceType, len(c.PaymentAllowed))
		copy(pa, c.PaymentAllowed)
		cp.PaymentAllowed = pa
	}
	return &cp
}

// CardStorageCondition covers microbe, animal, floater, science, asteroid, fighter, disease, card-resource.
type CardStorageCondition struct {
	ConditionBase
	Selectors      []Selector    `json:"selectors,omitempty"`
	Per            *PerCondition `json:"per,omitempty"`
	Optional       bool          `json:"optional,omitempty"`
	VariableAmount bool          `json:"variableAmount,omitempty"`
}

func NewCardStorageCondition(rt ResourceType, amount int, target string) *CardStorageCondition {
	return &CardStorageCondition{ConditionBase: ConditionBase{ResourceType: rt, Amount: amount, Target: target}}
}

func (c *CardStorageCondition) isBehaviorCondition() {}
func (c *CardStorageCondition) deepCopyCondition() BehaviorCondition {
	cp := *c
	if c.Selectors != nil {
		cp.Selectors = CloneSelectors(c.Selectors)
	}
	if c.Per != nil {
		cp.Per = c.Per.Clone()
	}
	return &cp
}

// PaymentSubstituteCondition registers a payment capability without transferring resources.
type PaymentSubstituteCondition struct {
	ConditionBase
	Source         PaymentSource `json:"source"`
	TargetResource ResourceType  `json:"targetResource"`
	Selectors      []Selector    `json:"selectors,omitempty"`
}

func (c *PaymentSubstituteCondition) isBehaviorCondition() {}
func (c *PaymentSubstituteCondition) deepCopyCondition() BehaviorCondition {
	cp := *c
	cp.Selectors = CloneSelectors(c.Selectors)
	return &cp
}

// EffectCondition covers discount, value-modifier,
// global-parameter-lenience, ignore-global-requirements, ocean-adjacency-bonus, defense, action-reuse, effect, tag.
type EffectCondition struct {
	ConditionBase
	Selectors []Selector `json:"selectors,omitempty"`
	Against   string     `json:"against,omitempty"`
	Temporary string     `json:"temporary,omitempty"`
}

func NewEffectCondition(rt ResourceType, amount int, target string) *EffectCondition {
	return &EffectCondition{ConditionBase: ConditionBase{ResourceType: rt, Amount: amount, Target: target}}
}

func (c *EffectCondition) isBehaviorCondition() {}
func (c *EffectCondition) deepCopyCondition() BehaviorCondition {
	cp := *c
	if c.Selectors != nil {
		cp.Selectors = CloneSelectors(c.Selectors)
	}
	return &cp
}

// ColonyCondition covers colony, colony-count, colony-bonus, colony-track-step.
type ColonyCondition struct {
	Optional       bool   `json:"optional,omitempty"`
	SelectionGroup string `json:"selectionGroup,omitempty"`
	ConditionBase
	AllowDuplicatePlayerColony bool `json:"allowDuplicatePlayerColony,omitempty"`
}

func NewColonyCondition(rt ResourceType, amount int, target string) *ColonyCondition {
	return &ColonyCondition{ConditionBase: ConditionBase{ResourceType: rt, Amount: amount, Target: target}}
}

func (c *ColonyCondition) isBehaviorCondition() {}
func (c *ColonyCondition) deepCopyCondition() BehaviorCondition {
	cp := *c
	return &cp
}

// TileModificationCondition covers tile-destruction and tile-replacement.
type TileModificationCondition struct {
	ConditionBase
	TileType string `json:"tileType,omitempty"`
}

func NewTileModificationCondition(rt ResourceType, amount int, target string) *TileModificationCondition {
	return &TileModificationCondition{ConditionBase: ConditionBase{ResourceType: rt, Amount: amount, Target: target}}
}

func (c *TileModificationCondition) isBehaviorCondition() {}
func (c *TileModificationCondition) deepCopyCondition() BehaviorCondition {
	cp := *c
	return &cp
}

// MiscCondition covers extra-actions, bonus-tags, world-tree-tile, award-fund, trade.
type MiscCondition struct {
	ConditionBase
	Per       *PerCondition `json:"per,omitempty"`
	Selectors []Selector    `json:"selectors,omitempty"`
}

func NewMiscCondition(rt ResourceType, amount int, target string) *MiscCondition {
	return &MiscCondition{ConditionBase: ConditionBase{ResourceType: rt, Amount: amount, Target: target}}
}

func (c *MiscCondition) isBehaviorCondition() {}
func (c *MiscCondition) deepCopyCondition() BehaviorCondition {
	cp := *c
	if c.Per != nil {
		cp.Per = c.Per.Clone()
	}
	if c.Selectors != nil {
		cp.Selectors = CloneSelectors(c.Selectors)
	}
	return &cp
}

// classifyResourceType maps a ResourceType to its category string.
func classifyResourceType(rt ResourceType) string {
	switch rt {
	case ResourceStateChange, ResourceRemoveDependency, ResourceRetireDependency, ResourceScheduleStateWindow:
		return "state"
	case ResourceCardReveal:
		return "card-reveal"
	case ResourceCopy:
		return "copy"
	case ResourceCredit, ResourceSteel, ResourceTitanium, ResourcePlant, ResourceEnergy, ResourceHeat:
		return "basic-resource"
	case ResourceCreditProduction, ResourceSteelProduction, ResourceTitaniumProduction,
		ResourcePlantProduction, ResourceEnergyProduction, ResourceHeatProduction, ResourceAnyProduction:
		return "production"
	case ResourceCityPlacement, ResourceOceanPlacement, ResourceGreeneryPlacement,
		ResourceVolcanoPlacement, ResourceTilePlacement, ResourceLandClaim:
		return "tile-placement"
	case ResourceTemperature, ResourceOxygen, ResourceOcean, ResourceVenus, ResourceTR, ResourceGlobalParameter:
		return "global-parameter"
	case ResourceCardPlay, ResourceCardDraw, ResourceCardTake, ResourceCardPeek, ResourceCardBuy, ResourceCardDiscard:
		return "card-operation"
	case ResourceMicrobe, ResourceAnimal, ResourceFloater, ResourceScience, ResourceAsteroid,
		ResourceFighter, ResourceDisease, ResourceCamp, ResourceCardResource:
		return "card-storage"
	case ResourcePaymentSubstitute:
		return "payment-substitute"
	case ResourceDiscount,
		ResourceValueModifier, ResourceGlobalParameterLenience, ResourceIgnoreGlobalRequirements,
		ResourceOceanAdjacencyBonus, ResourceDefense, ResourceActionReuse, ResourceEffect, ResourceTag:
		return "effect"
	case ResourceColonyTileAdd, ResourceColony, ResourceColonyCount, ResourceColonyBonus, ResourceColonyTrackStep, ResourceTradeFleet:
		return "colony"
	case ResourceTileDestruction, ResourceTileReplacement:
		return "tile-modification"
	case ResourceExtraActions, ResourceBonusTags, ResourceWorldTreeTile, ResourceAwardFund, ResourceFreeTrade:
		return "misc"
	default:
		return "misc"
	}
}

// categorizeCondition converts a flat resourceConditionJSON to the appropriate typed category struct.
func categorizeCondition(rc resourceConditionJSON) BehaviorCondition {
	base := ConditionBase{
		ResourceType: rc.ResourceType,
		Amount:       rc.Amount,
		Target:       rc.Target,
	}

	switch classifyResourceType(rc.ResourceType) {
	case "state":
		return &StateCondition{Description: rc.Description, ConditionBase: base, MinimumProduction: rc.MinimumProduction, State: rc.State, Value: rc.Value, Dependency: rc.Dependency, Start: rc.Start, End: rc.End}
	case "payment-substitute":
		source := PaymentSource{}
		if rc.Source != nil {
			source = *rc.Source
		}
		return &PaymentSubstituteCondition{ConditionBase: base, Source: source, TargetResource: rc.TargetResource, Selectors: rc.Selectors}
	case "card-reveal":
		return &CardRevealCondition{ConditionBase: base, Destination: rc.Destination, OnMatch: rc.OnMatch}
	case "copy":
		return &CopyCondition{ConditionBase: base, Scope: rc.Scope, Zone: rc.Zone, Selectors: rc.Selectors}
	case "basic-resource":
		return &BasicResourceCondition{
			ConditionBase:     base,
			Per:               rc.Per,
			VariableAmount:    rc.VariableAmount,
			MaxTrigger:        rc.MaxTrigger,
			Optional:          rc.Optional,
			TargetRestriction: rc.TargetRestriction,
			PaymentAllowed:    rc.PaymentAllowed,
		}
	case "production":
		return &ProductionCondition{
			Dependency:     rc.DependencyGroup,
			ConditionBase:  base,
			Per:            rc.Per,
			VariableAmount: rc.VariableAmount,
		}
	case "tile-placement":
		return &TilePlacementCondition{
			ConditionBase:    base,
			TileRestrictions: rc.TileRestrictions,
			TileType:         rc.TileType,
			Optional:         rc.Optional,
		}
	case "global-parameter":
		return &GlobalParameterCondition{
			ConditionBase: base,
			Per:           rc.Per,
		}
	case "card-operation":
		return &CardOperationCondition{
			CostReduction:  rc.CostReduction,
			Per:            rc.Per,
			ConditionBase:  base,
			Selectors:      rc.Selectors,
			Optional:       rc.Optional,
			PaymentAllowed: rc.PaymentAllowed,
			VariableAmount: rc.VariableAmount,
		}
	case "card-storage":
		return &CardStorageCondition{
			ConditionBase:  base,
			Selectors:      rc.Selectors,
			Per:            rc.Per,
			Optional:       rc.Optional,
			VariableAmount: rc.VariableAmount,
		}
	case "effect":
		return &EffectCondition{
			ConditionBase: base,
			Selectors:     rc.Selectors,
			Temporary:     rc.Temporary,
			Against:       rc.Against,
		}
	case "colony":
		return &ColonyCondition{
			SelectionGroup:             rc.SelectionGroup,
			Optional:                   rc.Optional,
			ConditionBase:              base,
			AllowDuplicatePlayerColony: rc.AllowDuplicatePlayerColony,
		}
	case "tile-modification":
		return &TileModificationCondition{
			ConditionBase: base,
			TileType:      rc.TileType,
		}
	default:
		return &MiscCondition{
			ConditionBase: base,
			Per:           rc.Per,
			Selectors:     rc.Selectors,
		}
	}
}

// flattenCondition converts any typed condition back to a flat resourceConditionJSON.
func flattenCondition(bc BehaviorCondition) resourceConditionJSON {
	switch c := bc.(type) {
	case *StateCondition:
		return resourceConditionJSON{Description: c.Description, ResourceType: c.ResourceType, Amount: c.Amount, Target: c.Target, MinimumProduction: c.MinimumProduction, State: c.State, Value: c.Value, Dependency: c.Dependency, Start: c.Start, End: c.End}
	case *PaymentSubstituteCondition:
		return resourceConditionJSON{ResourceType: c.ResourceType, Amount: c.Amount, Target: c.Target, Source: &c.Source, TargetResource: c.TargetResource, Selectors: c.Selectors}
	case *CardRevealCondition:
		return resourceConditionJSON{ResourceType: c.ResourceType, Amount: c.Amount, Target: c.Target, Destination: c.Destination, OnMatch: c.OnMatch}
	case *CopyCondition:
		return resourceConditionJSON{ResourceType: c.ResourceType, Amount: c.Amount, Target: c.Target, Scope: c.Scope, Zone: c.Zone, Selectors: c.Selectors}
	case *BasicResourceCondition:
		return resourceConditionJSON{
			ResourceType:      c.ResourceType,
			Amount:            c.Amount,
			Target:            c.Target,
			Per:               c.Per,
			VariableAmount:    c.VariableAmount,
			MaxTrigger:        c.MaxTrigger,
			Optional:          c.Optional,
			TargetRestriction: c.TargetRestriction,
			PaymentAllowed:    c.PaymentAllowed,
		}
	case *ProductionCondition:
		return resourceConditionJSON{
			DependencyGroup: c.Dependency,
			ResourceType:    c.ResourceType,
			Amount:          c.Amount,
			Target:          c.Target,
			Per:             c.Per,
			VariableAmount:  c.VariableAmount,
		}
	case *TilePlacementCondition:
		return resourceConditionJSON{
			ResourceType:     c.ResourceType,
			Amount:           c.Amount,
			Target:           c.Target,
			TileRestrictions: c.TileRestrictions,
			TileType:         c.TileType,
			Optional:         c.Optional,
		}
	case *GlobalParameterCondition:
		return resourceConditionJSON{
			ResourceType: c.ResourceType,
			Amount:       c.Amount,
			Target:       c.Target,
			Per:          c.Per,
		}
	case *CardOperationCondition:
		return resourceConditionJSON{
			CostReduction:  c.CostReduction,
			Per:            c.Per,
			ResourceType:   c.ResourceType,
			Amount:         c.Amount,
			Target:         c.Target,
			Selectors:      c.Selectors,
			Optional:       c.Optional,
			PaymentAllowed: c.PaymentAllowed,
			VariableAmount: c.VariableAmount,
		}
	case *CardStorageCondition:
		return resourceConditionJSON{
			ResourceType:   c.ResourceType,
			Amount:         c.Amount,
			Target:         c.Target,
			Selectors:      c.Selectors,
			Per:            c.Per,
			Optional:       c.Optional,
			VariableAmount: c.VariableAmount,
		}
	case *EffectCondition:
		return resourceConditionJSON{
			ResourceType: c.ResourceType,
			Amount:       c.Amount,
			Target:       c.Target,
			Selectors:    c.Selectors,
			Temporary:    c.Temporary,
			Against:      c.Against,
		}
	case *ColonyCondition:
		return resourceConditionJSON{
			SelectionGroup:             c.SelectionGroup,
			Optional:                   c.Optional,
			ResourceType:               c.ResourceType,
			Amount:                     c.Amount,
			Target:                     c.Target,
			AllowDuplicatePlayerColony: c.AllowDuplicatePlayerColony,
		}
	case *TileModificationCondition:
		return resourceConditionJSON{
			ResourceType: c.ResourceType,
			Amount:       c.Amount,
			Target:       c.Target,
			TileType:     c.TileType,
		}
	case *MiscCondition:
		return resourceConditionJSON{
			ResourceType: c.ResourceType,
			Amount:       c.Amount,
			Target:       c.Target,
			Per:          c.Per,
			Selectors:    c.Selectors,
		}
	default:
		return resourceConditionJSON{
			ResourceType: bc.GetResourceType(),
			Amount:       bc.GetAmount(),
			Target:       bc.GetTarget(),
		}
	}
}

// CopyCondition selects one source and applies the specified part of its behavior.
type CopyCondition struct {
	ConditionBase
	Scope     string     `json:"scope"`
	Zone      string     `json:"zone"`
	Selectors []Selector `json:"selectors,omitempty"`
}

func (c *CopyCondition) isBehaviorCondition() {}
func (c *CopyCondition) deepCopyCondition() BehaviorCondition {
	cp := *c
	cp.Selectors = CloneSelectors(c.Selectors)
	return &cp
}
