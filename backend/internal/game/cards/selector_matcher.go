package cards

import (
	"slices"

	"openmars/internal/game/shared"
)

// MatchesSelector checks if a card matches a single selector.
// Tags: ALL must be present (AND logic)
// CardTypes: ANY can match (OR logic - card has one type)
// Resources: card must have the resource (in storage, behavior outputs, or behavior inputs)
// RequiredOriginalCost: card cost must satisfy min/max constraints
// A selector must have at least one card-relevant criterion to match.
func MatchesSelector(card *Card, selector shared.Selector) bool {
	if selector.Dependency != "" {
		found := false
		for _, d := range card.DependencyGroups {
			if d.ID == selector.Dependency {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	if selector.Dependency == "" && len(selector.Tags) == 0 && len(selector.CardTypes) == 0 && selector.TagCount == nil && selector.RequiredOriginalCost == nil && selector.VP == nil && len(selector.Resources) == 0 {
		return false
	}

	if selector.TagCount != nil {
		count := 0
		for _, tag := range card.Tags {
			if tag != shared.TagWild {
				count++
			}
		}
		if selector.TagCount.Min != nil && count < *selector.TagCount.Min {
			return false
		}
		if selector.TagCount.Max != nil && count > *selector.TagCount.Max {
			return false
		}
	}

	if len(selector.Tags) > 0 {
		for _, requiredTag := range selector.Tags {
			if !slices.Contains(card.Tags, requiredTag) {
				return false
			}
		}
	}

	if len(selector.CardTypes) > 0 {
		if !slices.Contains(selector.CardTypes, string(card.Type)) {
			return false
		}
	}

	if selector.RequiredOriginalCost != nil {
		if selector.RequiredOriginalCost.Min != nil && card.Cost < *selector.RequiredOriginalCost.Min {
			return false
		}
		if selector.RequiredOriginalCost.Max != nil && card.Cost > *selector.RequiredOriginalCost.Max {
			return false
		}
	}

	if selector.VP != nil {
		hasVP := false
		for _, vp := range card.VPConditions {
			if selector.VP.Min != nil && vp.Amount < *selector.VP.Min {
				continue
			}
			if selector.VP.Max != nil && vp.Amount > *selector.VP.Max {
				continue
			}
			hasVP = true
			break
		}
		if !hasVP {
			return false
		}
	}

	if len(selector.Resources) > 0 {
		if !cardHasAnyResource(card, selector.Resources) {
			return false
		}
	}

	return true
}

// cardHasAnyResource matches printed resource references, not storage capacity alone.
func cardHasAnyResource(card *Card, resources []string) bool {
	for _, res := range resources {
		rt := shared.ResourceType(res)
		if card.ResourceStorage != nil && card.ResourceStorage.Type == rt {
			return true
		}
		if card.Requirements != nil {
			for _, req := range card.Requirements.Items {
				if req.Resource != nil && *req.Resource == rt {
					return true
				}
			}
		}
		for _, vp := range card.VPConditions {
			if vp.Per != nil && vp.Per.ResourceType == rt {
				return true
			}
		}
		for _, b := range card.Behaviors {
			if conditionsReferenceResource(b.Inputs, rt) || conditionsReferenceResource(b.Outputs, rt) {
				return true
			}
			for _, choice := range b.Choices {
				if conditionsReferenceResource(choice.Inputs, rt) || conditionsReferenceResource(choice.Outputs, rt) {
					return true
				}
				if choice.Requirements != nil {
					for _, req := range choice.Requirements.Items {
						if req.Resource != nil && *req.Resource == rt {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

func conditionsReferenceResource(conditions []shared.BehaviorCondition, rt shared.ResourceType) bool {
	for _, condition := range conditions {
		if condition.GetResourceType() == rt {
			return true
		}
		if per := shared.GetPerCondition(condition); per != nil && per.ResourceType == rt {
			return true
		}
		if reveal, ok := condition.(*shared.CardRevealCondition); ok && reveal.OnMatch != nil && conditionsReferenceResource(reveal.OnMatch.Outputs, rt) {
			return true
		}
		if payment, ok := condition.(*shared.PaymentSubstituteCondition); ok && payment.Source.Resource == rt {
			return true
		}
	}
	return false
}

// MatchesAnySelector tests OR alternatives, each of which combines its fields with AND.

func MatchesAnySelector(card *Card, selectors []shared.Selector) bool {
	if len(selectors) == 0 {
		return false
	}
	for _, sel := range selectors {
		if MatchesSelector(card, sel) {
			return true
		}
	}
	return false
}

// MatchesStandardProjectSelector checks if a project matches a selector
func MatchesStandardProjectSelector(project shared.StandardProject, selector shared.Selector) bool {
	return slices.Contains(selector.StandardProjects, project)
}

// MatchesAnyStandardProjectSelector checks OR between selectors for projects
func MatchesAnyStandardProjectSelector(project shared.StandardProject, selectors []shared.Selector) bool {
	for _, sel := range selectors {
		if MatchesStandardProjectSelector(project, sel) {
			return true
		}
	}
	return false
}

// HasCardSelectors returns true if any selector targets cards
func HasCardSelectors(selectors []shared.Selector) bool {
	for _, sel := range selectors {
		if sel.Dependency != "" || len(sel.Tags) > 0 || len(sel.CardTypes) > 0 || sel.TagCount != nil || sel.RequiredOriginalCost != nil || sel.VP != nil || len(sel.Resources) > 0 {
			return true
		}
	}
	return false
}

// HasCardSelectorsExcludingResources returns true if any selector targets cards via tags, types, cost, or VP.
// Unlike HasCardSelectors, this ignores the Resources field which on discount selectors
// indicates the affected resource type rather than a card filter.
func HasCardSelectorsExcludingResources(selectors []shared.Selector) bool {
	for _, sel := range selectors {
		if sel.Dependency != "" || len(sel.Tags) > 0 || len(sel.CardTypes) > 0 || sel.TagCount != nil || sel.RequiredOriginalCost != nil || sel.VP != nil {
			return true
		}
	}
	return false
}

// MatchesSelectorExcludingResources checks if a card matches a selector, ignoring the Resources field.
// Used for discount calculations where Resources indicates the discounted resource type, not a card filter.
func MatchesSelectorExcludingResources(card *Card, selector shared.Selector) bool {
	if selector.Dependency != "" {
		found := false
		for _, d := range card.DependencyGroups {
			if d.ID == selector.Dependency {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	if selector.Dependency == "" && len(selector.Tags) == 0 && len(selector.CardTypes) == 0 && selector.TagCount == nil && selector.RequiredOriginalCost == nil && selector.VP == nil {
		return false
	}

	if selector.TagCount != nil {
		count := 0
		for _, tag := range card.Tags {
			if tag != shared.TagWild {
				count++
			}
		}
		if selector.TagCount.Min != nil && count < *selector.TagCount.Min {
			return false
		}
		if selector.TagCount.Max != nil && count > *selector.TagCount.Max {
			return false
		}
	}

	if len(selector.Tags) > 0 {
		for _, requiredTag := range selector.Tags {
			if !slices.Contains(card.Tags, requiredTag) {
				return false
			}
		}
	}

	if len(selector.CardTypes) > 0 {
		if !slices.Contains(selector.CardTypes, string(card.Type)) {
			return false
		}
	}

	if selector.RequiredOriginalCost != nil {
		if selector.RequiredOriginalCost.Min != nil && card.Cost < *selector.RequiredOriginalCost.Min {
			return false
		}
		if selector.RequiredOriginalCost.Max != nil && card.Cost > *selector.RequiredOriginalCost.Max {
			return false
		}
	}

	if selector.VP != nil {
		hasVP := false
		for _, vp := range card.VPConditions {
			if selector.VP.Min != nil && vp.Amount < *selector.VP.Min {
				continue
			}
			if selector.VP.Max != nil && vp.Amount > *selector.VP.Max {
				continue
			}
			hasVP = true
			break
		}
		if !hasVP {
			return false
		}
	}

	return true
}

// MatchesAnySelectorExcludingResources checks if a card matches any selector, ignoring Resources field.
func MatchesAnySelectorExcludingResources(card *Card, selectors []shared.Selector) bool {
	for _, sel := range selectors {
		if MatchesSelectorExcludingResources(card, sel) {
			return true
		}
	}
	return false
}

// HasStandardProjectSelectors returns true if any selector targets standard projects
func HasStandardProjectSelectors(selectors []shared.Selector) bool {
	for _, sel := range selectors {
		if len(sel.StandardProjects) > 0 {
			return true
		}
	}
	return false
}

// HasResourceSelectors returns true if any selector targets resources
func HasResourceSelectors(selectors []shared.Selector) bool {
	for _, sel := range selectors {
		if len(sel.Resources) > 0 {
			return true
		}
	}
	return false
}

// GetResourcesFromSelectors collects all resources from selectors into a single slice
func GetResourcesFromSelectors(selectors []shared.Selector) []string {
	var resources []string
	seen := make(map[string]bool)
	for _, sel := range selectors {
		for _, r := range sel.Resources {
			if !seen[r] {
				seen[r] = true
				resources = append(resources, r)
			}
		}
	}
	return resources
}

// hasPreludeCardType returns true if any selector contains "prelude" in its CardTypes
func hasPreludeCardType(selectors []shared.Selector) bool {
	for _, sel := range selectors {
		if slices.Contains(sel.CardTypes, "prelude") {
			return true
		}
	}
	return false
}

// HasActionSelectors returns true if any selector targets actions
func HasActionSelectors(selectors []shared.Selector) bool {
	for _, sel := range selectors {
		if len(sel.Actions) > 0 {
			return true
		}
	}
	return false
}

// MatchesAnyActionSelector checks if an action type matches any selector (OR between selectors)
func MatchesAnyActionSelector(actionType string, selectors []shared.Selector) bool {
	for _, sel := range selectors {
		if slices.Contains(sel.Actions, actionType) {
			return true
		}
	}
	return false
}

// MatchesResourceSelector checks if a resource type matches a selector
func MatchesResourceSelector(resourceType string, selector shared.Selector) bool {
	return slices.Contains(selector.Resources, resourceType)
}

// MatchesAnyResourceSelector checks OR between selectors for resources
func MatchesAnyResourceSelector(resourceType string, selectors []shared.Selector) bool {
	for _, sel := range selectors {
		if MatchesResourceSelector(resourceType, sel) {
			return true
		}
	}
	return false
}
