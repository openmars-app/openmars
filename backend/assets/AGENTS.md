# Card and mode authoring

Definitions here are loaded at startup. Use [cards.json](cards.json) and [gamemodes.json](gamemodes.json) as current authored examples; use [card validation](../internal/game/cards/card_json_validator.go), [mode modifiers](../internal/game/cards/mode_modifiers.go), and [shared condition rules](../internal/game/shared/resource_condition_rules.go) as the schema authority.

## Membership and modifiers

- Declare explicit `gameModes`; an empty list keeps a card in the library only. `requiresFeatures` and `requiresSystems` further restrict selection. Every prelude requires the `prelude` feature. Standard projects, awards, and milestones use the same membership rules.
- Give every behavior a stable ID unique within its card. Retain IDs when editing operations.
- Keep a shared card's ordinary definition intact and express scenario-specific changes through `modifiers`. Modifier modes must appear in the card's membership.
- Use the operations supported by [mode_modifiers.go](../internal/game/cards/mode_modifiers.go), including `set-required-systems` when changing system dependencies. Replace whole behaviors by ID, not array offsets; replacements retain the target ID.
- Keep scenario-only dependency groups and rules in modifiers when the base card is also available in Classic. Resolved catalogs must remain isolated per game.
- Validate both base and effective catalogs: unknown references, duplicate IDs, and conflicting changes are errors.

## Descriptions and requirements

Card descriptions contain structured `requirement`, `action`, `effect`, and `generic` sections. Omit generated prefixes such as “Requires”, “Requirement:”, and “Action:” from section text.

For every card with requirements, keep `requirements.description` exactly equal to its single `requirement` section's text, including punctuation. Use concise condition wording, trimmed and ending with a period. Mode modifiers must keep both copies aligned. The [catalog description test](../test/game/cards/card_description_test.go) checks effective catalogs.

Behavior descriptions must include their conditions. Give stored resources a player-facing label. Card-level requirements block playing the card; an immediate behavior's state requirements skip that operation when unmet. Behaviors sharing a `usageGroup` share usage limits; choices can have their own requirements.

## Production, dependencies, and history

- Use `productionBox: "evaluate"` only for unconditional pure production outputs. Copying evaluates the authored production box.
- Use `"resolved"` to record production actually granted by a one-shot operation, including choices, mixed initial gains, and later conditional expansions. Copies do not replay triggers or other outputs. Recurring manual production cannot be a production box.
- Copies retain source dependency metadata and signed upkeep. Importing the original does not import its copy.
- Group ongoing benefits and upkeep in `dependencyGroups`; keep initial grants separate. Tags alone never imply a dependency.
- Conditional production is source-tracked and cannot pay permanent production reductions. Reconnection restores retained contributions without granting them twice.
- Only explicitly importable project groups can be imported. `retireOnState` permanently ends the operation when its condition is met.
- Use historical state such as `withdrawal-occurred` for first-withdrawal rewards, rather than rewarding every later contact closure.
- `resourceStorage.icon` and `label` control presentation while `type` remains the engine resource type. `capacity` displays a limit; behavior requirements must also enforce it.

## Mode runtime

Use the existing Earth Withdrawal definition as an example and [mode_runtime.go](../internal/game/shared/mode_runtime.go) for state-operation fields.

- Deadline parameters supply initial deadlines; do not duplicate the deadline key in initial runtime state. Expiry follows the generation's production. Active contact state can change temporarily; occurred state records irreversible history.
- Window boundaries are `research-start`, `production-start`, and `production-end`. A full next-generation window runs from offset 1 research start to offset 1 production end; a production-only pulse uses offset 0 production start/end. Overlapping windows are rejected.
- Use `remove-dependency` to import an eligible project's whole ongoing operation; use `retire-dependency` for retirement. Do not simulate either by removing isolated positive outputs.
- `stateVisuals` supplies reusable card symbols for declared states, not extra status rows.
- Presentation rules produce announcements and optional renderer capabilities through `effects`. A `link-transition` carries `link: {from, to, connected}`. Unsupported capabilities are skipped while the announcement remains.
- Turn order and normal Mars scoring remain engine rules. JSON authoring reuses implemented primitives; a new primitive needs engine code and backend tests.

## Artwork and tile appearance

Register source artwork through the root [asset catalog](../../assets/catalog.json); corporation entries also need `corporation: {key, name, color}` for the logo registry. Follow [asset tooling](../../frontend/scripts/README.md) rather than editing generated runtime files.

`style.tile` requests best-effort visual features for the shared Structure renderer without changing placement, tile identity, or scoring. Placement copies the request into `occupiedBy.visual.structure` and freezes a deterministic seed.

Reuse shared geometry and layout; never implement a renderer for one card. Extend reusable feature requests when needed. Factory halls, smokestacks, tanks, and pipes must fit around roads, entrances, and neighboring structures. Covered or buried layouts omit exposed industrial fixtures; smoke uses generated stack positions. Follow the [board instructions](../../frontend/src/components/game/board/AGENTS.md) for rendering and mandatory visual review.

## Verification

Read the [shared-condition checklist](../internal/game/shared/AGENTS.md) when extending types or fields. Run affected catalog, description, and gameplay tests; for catalog changes use `just backend test ./test/game/cards/...` from the root. For artwork changes use `just frontend assets-check`. Startup validation supplements tests; remember that Air does not reload JSON assets.
