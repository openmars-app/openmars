# Assets - Static Game Data

Contains card definitions and static game data loaded by the backend at startup.

## Card Database

Authoritative source for all card definitions: corporations, project cards, and prelude cards with behaviors, costs, requirements, and effects.

### Card Structure

```json
{
  "id": "068",
  "name": "Sponsors",
  "type": "automated",
  "cost": 6,
  "description": [{ "type": "generic", "text": "Increase your M€ production 2 steps." }],
  "gameModes": ["classic", "earth-withdrawal"],
  "tags": ["earth"],
  "behaviors": [{
    "id": "sponsor-income",
    "triggers": [{ "type": "auto" }],
    "outputs": [{ "type": "credit-production", "amount": 2, "target": "self-player" }],
    "description": "Increase your M€ production 2 steps."
  }]
}
```

**Card Types:** `corporation`, `active`, `automated`, `event`, `prelude`

**Membership:** `gameModes` lists the modes that can select the card. An explicit empty list keeps a card in the library only. Optional `requiresFeatures` (for example `["prelude"]`) and `requiresSystems` must also be satisfied. Every prelude requires the `prelude` feature; other card types can require it too. Standard projects, awards, and milestones use the same membership fields.

### Mode Modifiers

A shared card keeps its ordinary definition and applies mode-specific absolute changes before the game uses its catalog:

```json
{
  "when": { "gameModes": ["earth-withdrawal"] },
  "changes": [
    { "type": "set-cost", "amount": 3 },
    { "type": "append-description", "description": [{ "type": "generic", "text": "The ongoing benefit needs Earth contact." }] }
  ]
}
```

Place these objects in the card's `modifiers` array. Supported operations are `set-cost` (`amount`), `set-description` / `append-description` (`description`), `set-requirements` (`requirements`), `set-dependency-groups` (`dependencyGroups`), `add-behavior` (`behavior`), and `replace-behavior` / `remove-behavior` (`behaviorId`, plus the complete `behavior` for replacement). Replace complete behaviors rather than patching array offsets. A replacement retains the target ID. Unknown targets, duplicate IDs, and conflicting changes are errors. Modifier modes must also appear in the card's membership.

Keep scenario-only dependency groups and rules in modifiers when the base card is also available in Classic. The resolved catalog is isolated per game.

### Behavior System

Each card has a `behaviors` array. Each behavior contains:
- `id`: Stable identifier, unique within the card; retain it when editing an operation
- `description`: Concise rules text for this operation, including its conditions
- `triggers`: When the behavior activates
- `inputs`: Resources consumed (costs)
- `outputs`: Resources produced (effects)
- `choices`: Alternative options (A OR B)

The JSON uses a flat format with a `type` field that determines which fields are valid. At load time, each condition is categorized into a typed Go struct (see `internal/game/shared/CLAUDE.md`).

Card descriptions use structured `requirement`, `action`, `effect`, and `generic` sections. Do not include generated prefixes such as "Requires" or "Action:" in section text. Give requirements a standalone readable `description`, and give stored resources a player-facing label.

Optional behavior controls are `stateRequirements: [{"state":"earth-contact","equals":1}]`, `resourceRequirements` (`type`, `min`/`max`, optional `target`, `location`, `permanent`), `maxUsesPerGeneration`, `maxUsesPerGame`, and `usageGroup`. Behaviors sharing a usage group share their limits. Choices also support state and resource requirements. Card-level requirements prevent playing a card; an immediate behavior's state requirements only skip that operation when unmet.

### Production Boxes

Set `productionBox: "evaluate"` only on unconditional, pure production outputs. Copying evaluates that authored production box. Use `"resolved"` to record only production actually granted by a one-shot operation, including choices, mixed initial gains, or a later conditional expansion. It does not copy the operation's other outputs or replay its trigger. Recurring manual production cannot be a production box. Copies retain source dependency metadata, including signed upkeep; importing the original does not import the copy.

### Trigger Types

| Trigger | Description |
|---------|-------------|
| `auto` | Applies immediately when card is played |
| `auto-corporation-start` | Applies once when corporation is selected |
| `manual` | Player-activated action (blue cards) |
| `auto` + `condition` | Passive effect triggered by game events |

Mode events include `condition: {"type":"production"}` and `{"type":"state-changed","state":"withdrawal-occurred","to":1}`. A state-change trigger may filter `direction: "increase"` / `"decrease"` and `target: "self-player"` to match the actor. First-withdrawal rewards use the historical state, not repeated contact closures.

### Output Type Categories

Each output `type` belongs to a category that determines which fields are valid:

| Category | Types | Valid Fields |
|----------|-------|-------------|
| Basic Resource | `credit`, `steel`, `titanium`, `plant`, `energy`, `heat` | `per`, `variableAmount`, `targetRestriction`, `maxTrigger` |
| Production | `credit-production` ... `heat-production`, `any-production` | `per`, `variableAmount` |
| Tile Placement | `city-placement`, `greenery-placement`, `ocean-placement`, `volcano-placement`, `tile-placement`, `land-claim` | `tileRestrictions`, `tileType` |
| Global Parameter | `temperature`, `oxygen`, `ocean`, `venus`, `tr`, `global-parameter` | `per` |
| Card Operation | `card-draw`, `card-take`, `card-peek`, `card-buy`, `card-discard` | `selectors`, `variableAmount` |
| Card Storage | `microbe`, `animal`, `floater`, `science`, `asteroid`, `fighter`, `disease`, `card-resource` | `selectors`, `per`, `variableAmount` |
| Payment substitute | `payment-substitute` | `source` (`target`, `resource`), `targetResource`, `selectors` |
| Effect | `discount`, `value-modifier`, `defense`, `global-parameter-lenience`, `ignore-global-requirements`, `ocean-adjacency-bonus`, `action-reuse`, `effect`, `tag` | `selectors`, `temporary` |
| Colony | `colony`, `colony-count`, `colony-bonus`, `colony-track-step` | `allowDuplicatePlayerColony` |
| Tile Modification | `tile-destruction`, `tile-replacement` | `tileType` |
| Misc | `extra-actions`, `bonus-tags`, `world-tree-tile`, `award-fund`, `trade` | `per`, `selectors` |

All types share: `type`, `amount`, `target`.

## Dependencies and State Operations

An ongoing operation can declare `dependency: "earth-support"` referencing a card dependency group:

```json
{
  "id": "earth-support",
  "label": "Earth support",
  "state": "earth-contact",
  "equals": 1,
  "importable": true
}
```

Put groups in `dependencyGroups`. Optional `retireOnState: {"state":"withdrawal-occurred","equals":1}` ends an operation permanently. Group all ongoing benefits and upkeep together. Initial grants remain separate. Conditional production is source-tracked and cannot pay permanent production reductions; reconnecting restores the retained contribution without granting it twice. Only explicitly importable project groups can be imported. Tags alone never imply a dependency.

State outputs share `type`, `amount`, and `target`:

| Type | Additional fields and meaning |
|------|-------------------------------|
| `state-change` | `state`; signed `amount` adjusts a declared deadline; `target: "game"` |
| `remove-dependency` | `dependency`; `amount: 1`, `target: "any-card"` imports one eligible owned project's whole ongoing operation |
| `retire-dependency` | `dependency`; `target: "any-card"` or `"self-card"`; optional `minimumProduction: {"type":"credit-production","amount":2}` filters eligible production |
| `schedule-state-window` | `state`, `value`, `start`, `end`; `amount: 1`, `target: "game"`; boundaries contain `generationOffset` and `boundary` |

Supported window boundaries are `research-start`, `production-start`, and `production-end`. A full next-generation window starts at offset 1 research and ends at offset 1 production end. A production-only pulse uses offset 0 production start/end. The engine rejects overlapping windows. State outputs can carry a readable `description` for the UI.

## Game Modes

`gamemodes.json` is an array of mode definitions. Each declares:

- Identity and UI: `id`, `name`, `summary`, `scenario`, `rules: [{title, text}]`, `minPlayers`, `maxPlayers`.
- Configuration: `parameters: [{id, label, description, default, values}]`, with integer values; `features: [{id, name, description, experimental, default}]`; `systems` lists enabled system IDs.
- Status presentation: `statuses: [{state, label, kind, labels?, from?, to?}]`. Use `kind: "counter"` or `"connection"`; `labels` maps integer strings to readable values.
- Executable `runtime`: initial integer `state`, `deadlines`, shared `behaviors`, and optional `presentationRules`. Mode behaviors use the same behavior model with stable IDs and `scope: "each-player"` or `"game"`.

Each deadline declares `state`, `parameter`, `activeState`, `occurredState`, `activeValue`, `inactiveValue`, `maxParameterAdjustment`, `minGenerationOffset`, and `perPlayerGenerationLimit`. Its parameter supplies the initial deadline; do not duplicate that deadline key in initial `runtime.state`. Expiry follows the generation's production. The active state can have temporary windows; the occurred state records irreversible history.

Presentation rules contain `state`, optional `to`, `title`, optional `description`, and optional `presentation: {kind, from, to, connected}`. They produce readable status events and optional scene hints. Use the existing Earth Withdrawal definition as the complete example. Turn order and normal Mars scoring remain engine rules.

## Adding New Cards

1. Add the definition, explicit memberships, stable behavior IDs, and readable descriptions to `cards.json`.
2. Use existing behavior primitives. New primitives need engine implementation and backend tests; JSON does not invent executable semantics.
3. Register artwork in `assets/catalog.json`. Corporation entries also need `corporation: {key, name, color}` so the generated logo registry can resolve them.
4. Run `just test` from the repository root and `bun run assets:check` from `frontend/`. Startup validates base and effective catalogs, including references, modifiers, mode state, and content dependencies.

See `internal/game/shared/CLAUDE.md` for typed condition categories. Existing primitives allow new cards and modes to be authored in JSON without card-specific engine code.
