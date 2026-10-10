package cards

import (
	"cmp"
	"fmt"
	"slices"

	"openmars/internal/game"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
)

// IsResourceProtected evaluates the owner's persistent defenses from the actor's perspective.
// An empty cardID denotes a basic resource pool, never card storage.
func IsResourceProtected(actor, owner *player.Player, rt shared.ResourceType, cardID string) bool {
	if actor == nil || owner == nil {
		return false
	}
	for _, effect := range owner.Effects().List() {
		for _, output := range effect.Behavior.Outputs {
			defense, ok := output.(*shared.EffectCondition)
			if !ok || defense.ResourceType != shared.ResourceDefense || !MatchesAnyResourceSelector(string(rt), defense.Selectors) {
				continue
			}
			if defense.Against == "opponents" && actor.ID() == owner.ID() {
				continue
			}
			if defense.Against != "any-player" && defense.Against != "opponents" {
				continue
			}
			if defense.Target == "self-player" || (defense.Target == "self-card" && cardID != "" && effect.CardID == cardID) {
				return true
			}
		}
	}
	return false
}

type ResourceRemovalTarget struct {
	PlayerID     string
	CardID       string
	ResourceType shared.ResourceType
	Amount       int
}

func ownedStorageCards(p *player.Player) []string {
	ids := append([]string(nil), p.PlayedCards().Cards()...)
	if id := p.CorporationID(); id != "" && !slices.Contains(ids, id) {
		ids = append(ids, id)
	}
	return ids
}

// AvailableResourceRemovalTargets is shared by action availability and client target metadata.
func AvailableResourceRemovalTargets(g *game.Game, actor *player.Player, registry CardRegistryInterface) []ResourceRemovalTarget {
	targets := make([]ResourceRemovalTarget, 0)
	if g == nil || actor == nil {
		return targets
	}
	for _, owner := range g.GetAllPlayers() {
		if owner.HasExited() {
			continue
		}
		for _, rt := range []shared.ResourceType{shared.ResourceCredit, shared.ResourceSteel, shared.ResourceTitanium, shared.ResourcePlant, shared.ResourceEnergy, shared.ResourceHeat} {
			if amount := owner.Resources().Get().GetAmount(rt); amount > 0 && !IsResourceProtected(actor, owner, rt, "") {
				targets = append(targets, ResourceRemovalTarget{PlayerID: owner.ID(), ResourceType: rt, Amount: amount})
			}
		}
		if registry == nil {
			continue
		}
		for _, id := range ownedStorageCards(owner) {
			card, err := registry.GetByID(id)
			if err != nil || card.ResourceStorage == nil {
				continue
			}
			rt := card.ResourceStorage.Type
			if amount := owner.Resources().GetCardStorage(id); amount > 0 && !IsResourceProtected(actor, owner, rt, id) {
				targets = append(targets, ResourceRemovalTarget{PlayerID: owner.ID(), CardID: id, ResourceType: rt, Amount: amount})
			}
		}
	}
	slices.SortFunc(targets, func(a, b ResourceRemovalTarget) int {
		if order := cmp.Compare(a.PlayerID, b.PlayerID); order != 0 {
			return order
		}
		if order := cmp.Compare(a.CardID, b.CardID); order != 0 {
			return order
		}
		return cmp.Compare(a.ResourceType, b.ResourceType)
	})
	return targets
}

// removalStorageOwner validates ownership, storage type, protection and a nonempty source.
// Transfers require the full amount; destructive project effects can remove up to that amount.
func (a *BehaviorApplier) removalStorageOwner(id string, rt shared.ResourceType, amount int, transfer bool) (*player.Player, error) {
	if a.game == nil || a.cardRegistry == nil {
		return nil, fmt.Errorf("card removal requires game and card registry")
	}
	card, err := a.cardRegistry.GetByID(id)
	if err != nil || card.ResourceStorage == nil {
		return nil, fmt.Errorf("invalid storage card %s", id)
	}
	if rt != shared.ResourceCardResource && card.ResourceStorage.Type != rt {
		return nil, fmt.Errorf("card %s does not store %s", id, rt)
	}
	for _, owner := range a.game.GetAllPlayers() {
		if owner.HasExited() || !slices.Contains(ownedStorageCards(owner), id) {
			continue
		}
		if IsResourceProtected(a.player, owner, card.ResourceStorage.Type, id) {
			return nil, fmt.Errorf("%s on card %s are protected", card.ResourceStorage.Type, id)
		}
		available := owner.Resources().GetCardStorage(id)
		if available <= 0 || (transfer && available < amount) {
			return nil, fmt.Errorf("insufficient resources on card %s", id)
		}
		return owner, nil
	}
	return nil, fmt.Errorf("storage card %s is not owned by an active player", id)
}

// ValidateResourceOutputs must run before paying costs or applying any part of an action.
// It preserves positional target state so the same applier can subsequently execute.
func (a *BehaviorApplier) ValidateResourceOutputs(outputs []shared.BehaviorCondition) error {
	index := a.anyCardTargetIdx
	for _, output := range outputs {
		amount := output.GetAmount()
		if per := shared.GetPerCondition(output); per != nil && per.Amount > 0 && a.game != nil && a.player != nil {
			projected := *per
			if a.sourceType == shared.SourceTypeCardPlay && a.player.Hand().HasCard(a.sourceCardID) && a.cardRegistry != nil {
				source, err := a.cardRegistry.GetByID(a.sourceCardID)
				if err == nil && source.Type != CardTypeEvent {
					projected.IncludeSource = true
				}
			}
			amount = shared.CalculateScaledAmount(output, a.countPerCondition(&projected))
		}
		if shared.IsVariableAmount(output) {
			amount = output.GetAmount() * a.selectedAmount
		}
		switch o := output.(type) {
		case *shared.ProductionCondition:
			if amount >= 0 || a.player == nil || o.Target == "any-player" || o.Dependency != nil || (a.behavior != nil && a.behavior.Dependency != "") {
				continue
			}
			if a.player.Resources().Production().GetAmount(o.ResourceType) > a.player.Resources().PermanentProduction().GetAmount(o.ResourceType) && a.player.Resources().PermanentProduction().GetAmount(o.ResourceType)+amount < shared.ProductionMinimum(o.ResourceType) {
				return fmt.Errorf("insufficient permanent %s", o.ResourceType)
			}
		case *shared.BasicResourceCondition:
			// Deferred selection revalidates after placement.
			if o.TargetRestriction != nil {
				continue
			}
			if o.Target == "any-player" || o.Target == "steal-any-player" {
				if a.targetPlayerID == "" {
					continue
				}
				if a.game == nil {
					return fmt.Errorf("resource removal requires game")
				}
				owner, err := a.game.GetPlayer(a.targetPlayerID)
				if err != nil || owner.HasExited() {
					return fmt.Errorf("invalid target player")
				}
				if IsResourceProtected(a.player, owner, o.ResourceType, "") {
					return fmt.Errorf("%s are protected", o.ResourceType)
				}
			} else if amount < 0 && IsResourceProtected(a.player, a.player, o.ResourceType, "") {
				return fmt.Errorf("%s are protected", o.ResourceType)
			}
		case *shared.CardStorageCondition:
			id := a.sourceCardID
			if o.Target == "any-card" || (o.ResourceType == shared.ResourceCardResource && o.Target != "self-card") {
				id = ""
				if index < len(a.targetCardIDs) {
					id = a.targetCardIDs[index]
				}
				index++
			}
			if amount > 0 && (o.Target == "any-card" || (o.ResourceType == shared.ResourceCardResource && o.Target != "self-card")) {
				enteringID := ""
				if a.player != nil && a.player.Hand().HasCard(a.sourceCardID) {
					enteringID = a.sourceCardID
				}
				if id != "" {
					if err := ValidateStorageDestination(a.player, a.cardRegistry, o, id, enteringID); err != nil {
						return err
					}
				} else if len(StorageDestinations(a.player, a.cardRegistry, o, enteringID)) > 0 {
					return fmt.Errorf("select a card for resource storage")
				}
			}
			if o.Target == "steal-from-any-card" {
				if _, err := a.removalStorageOwner(a.stealSourceCardID, o.ResourceType, amount, true); err != nil {
					return err
				}
			} else if amount < 0 {
				// Omitting an optional attack target skips the removal.
				if id == "" && o.Target == "any-card" {
					continue
				}
				if o.Target == "triggering-card" {
					owner, card, err := ResolveTriggeringCard(a.game, a.cardRegistry, a.triggeringCardID, a.triggeringPlayerID, o)
					if err != nil {
						return err
					}
					if IsResourceProtected(a.player, owner, card.ResourceStorage.Type, card.ID) {
						return fmt.Errorf("card resources are protected")
					}
				} else if o.Target == "any-card" || (o.ResourceType == shared.ResourceCardResource && o.Target != "self-card") {
					if _, err := a.removalStorageOwner(id, o.ResourceType, -amount, false); err != nil {
						return err
					}
				} else if IsResourceProtected(a.player, a.player, o.ResourceType, id) {
					return fmt.Errorf("card resources are protected")
				}
			}
		}
	}
	return nil
}
