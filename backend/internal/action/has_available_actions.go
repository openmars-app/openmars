package action

import (
	"openmars/internal/game"
	"openmars/internal/game/award"
	"openmars/internal/game/cards"
	"openmars/internal/game/milestone"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
	"openmars/internal/game/standardproject"
)

// ConversionStandardProjects are the two resource-conversion "standard projects"
// that are not part of the standard-project registry (they are surfaced as
// resource buttons), but still constitute legal action-phase moves.
var ConversionStandardProjects = []shared.StandardProject{
	shared.StandardProjectConvertHeatToTemperature,
	shared.StandardProjectConvertPlantsToGreenery,
}

// HasAvailableActions reports whether the player has at least one legal move on
// their turn. It is the single aggregate "can this player act at all?" signal,
// built entirely from the existing per-surface state calculators so it can never
// diverge from what the UI shows as available.
//
// A player who has a pending selection or an incomplete forced first action is
// NOT stuck: they have a mandatory thing to do, so the function returns true.
//
// The function short-circuits on the first available surface. The caller is
// responsible for only invoking it once every registry is present.
func HasAvailableActions(
	g *game.Game,
	p *player.Player,
	cardRegistry cards.CardRegistry,
	stdProjRegistry standardproject.StandardProjectRegistry,
	milestoneRegistry milestone.MilestoneRegistry,
	awardRegistry award.AwardRegistry,
) bool {
	cardRegistry = cards.ForGame(cardRegistry, g)
	if g.HasAnyPendingSelection(p.ID()) {
		return true
	}
	if forced := g.GetForcedFirstAction(p.ID()); forced != nil {
		return true
	}

	for _, cardID := range p.Hand().Cards() {
		card, err := cardRegistry.GetByID(cardID)
		if err != nil {
			continue
		}
		if CalculatePlayerCardState(card, p, g, cardRegistry).Available() {
			return true
		}
	}

	for _, act := range p.Actions().List() {
		state := CalculatePlayerCardActionState(
			act.CardID, act.Behavior, act.TimesUsedThisGeneration, p, g, cardRegistry,
		)
		if state.Available() {
			return true
		}
	}

	if hasAvailableStandardProject(g, p, cardRegistry, stdProjRegistry) {
		return true
	}

	for _, def := range FilterMilestones(milestoneRegistry.GetAll(), g.SelectedMilestones(), g.Settings()) {
		state := CalculateMilestoneState(
			shared.MilestoneType(def.ID), p, g, cardRegistry, milestoneRegistry,
		)
		if state.Available() {
			return true
		}
	}

	for _, def := range FilterAwards(awardRegistry.GetAll(), g.SelectedAwards(), g.Settings()) {
		if CalculateAwardState(shared.AwardType(def.ID), p, g, awardRegistry, cardRegistry).Available() {
			return true
		}
	}

	if g.HasColonies() {
		if CalculateColonyTradeState(p, g, cardRegistry).Available() {
			return true
		}
	}

	return false
}

// hasAvailableStandardProject reports whether any standard project the player can
// reach this game is available, covering the registry-backed projects (filtered to
// the selected mode, mirroring the DTO mapper) plus the two resource conversions
// and sell-patents, which the per-project calculator already validates.
func hasAvailableStandardProject(
	g *game.Game,
	p *player.Player,
	cardRegistry cards.CardRegistry,
	stdProjRegistry standardproject.StandardProjectRegistry,
) bool {
	settings := g.Settings()

	for _, def := range stdProjRegistry.GetAll() {
		if !settings.AllowsContent(def.GameModes, def.RequiresSystems, def.RequiresFeatures) {
			continue
		}
		if CalculatePlayerStandardProjectState(shared.StandardProject(def.ID), p, g, cardRegistry).Available() {
			return true
		}
	}

	for _, projectType := range ConversionStandardProjects {
		if CalculatePlayerStandardProjectState(projectType, p, g, cardRegistry).Available() {
			return true
		}
	}

	return false
}

// FilterMilestones reduces the full milestone definition set to the ones actually
// in play this game: the explicitly selected set when one exists, otherwise every
// definition eligible for the selected mode. It is the single source of truth for "which
// milestones does this game expose", shared by the DTO mapper and the
// HasAvailableActions aggregator so the two can never diverge.
func FilterMilestones(allDefs []milestone.MilestoneDefinition, selectedIDs []string, settings shared.GameSettings) []milestone.MilestoneDefinition {
	if len(selectedIDs) > 0 {
		selectedSet := make(map[string]bool, len(selectedIDs))
		for _, id := range selectedIDs {
			selectedSet[id] = true
		}
		filtered := make([]milestone.MilestoneDefinition, 0, len(selectedIDs))
		for _, def := range allDefs {
			if selectedSet[def.ID] && settings.AllowsContent(def.GameModes, def.RequiresSystems, def.RequiresFeatures) {
				filtered = append(filtered, def)
			}
		}
		return filtered
	}
	var filtered []milestone.MilestoneDefinition
	for _, def := range allDefs {
		if !settings.AllowsContent(def.GameModes, def.RequiresSystems, def.RequiresFeatures) {
			continue
		}
		filtered = append(filtered, def)
	}
	return filtered
}

// FilterAwards reduces the full award definition set to the ones actually in play
// this game, mirroring FilterMilestones. It is shared by the DTO mapper and the
// HasAvailableActions aggregator so the two can never diverge.
func FilterAwards(allDefs []award.AwardDefinition, selectedIDs []string, settings shared.GameSettings) []award.AwardDefinition {
	if len(selectedIDs) > 0 {
		selectedSet := make(map[string]bool, len(selectedIDs))
		for _, id := range selectedIDs {
			selectedSet[id] = true
		}
		filtered := make([]award.AwardDefinition, 0, len(selectedIDs))
		for _, def := range allDefs {
			if selectedSet[def.ID] && settings.AllowsContent(def.GameModes, def.RequiresSystems, def.RequiresFeatures) {
				filtered = append(filtered, def)
			}
		}
		return filtered
	}
	var filtered []award.AwardDefinition
	for _, def := range allDefs {
		if !settings.AllowsContent(def.GameModes, def.RequiresSystems, def.RequiresFeatures) {
			continue
		}
		filtered = append(filtered, def)
	}
	return filtered
}
