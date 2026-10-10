package shared

// Resources represents a player's resources
type Resources struct {
	Credits  int
	Steel    int
	Titanium int
	Plants   int
	Energy   int
	Heat     int
}

// IsZero returns true if all resource values are zero
func (r Resources) IsZero() bool {
	return r.Credits == 0 && r.Steel == 0 && r.Titanium == 0 &&
		r.Plants == 0 && r.Energy == 0 && r.Heat == 0
}

// DeepCopy creates a deep copy of the Resources struct
func (r Resources) DeepCopy() Resources {
	return Resources{
		Credits:  r.Credits,
		Steel:    r.Steel,
		Titanium: r.Titanium,
		Plants:   r.Plants,
		Energy:   r.Energy,
		Heat:     r.Heat,
	}
}

// GetAmount returns the amount of a specific resource type.
func (r Resources) GetAmount(resourceType ResourceType) int {
	switch resourceType {
	case ResourceCredit:
		return r.Credits
	case ResourceSteel:
		return r.Steel
	case ResourceTitanium:
		return r.Titanium
	case ResourcePlant:
		return r.Plants
	case ResourceEnergy:
		return r.Energy
	case ResourceHeat:
		return r.Heat
	default:
		return 0
	}
}

// PositiveTypes counts resource types with production greater than zero.
func (p Production) PositiveTypes() int {
	count := 0
	for _, amount := range []int{p.Credits, p.Steel, p.Titanium, p.Plants, p.Energy, p.Heat} {
		if amount > 0 {
			count++
		}
	}
	return count
}
