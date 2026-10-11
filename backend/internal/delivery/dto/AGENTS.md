# Client DTO instructions

DTOs define the client contract. Keep domain models internal; only this package belongs in [tygo.yaml](../../../tygo.yaml).

## Contract changes

1. Add or update the relevant DTO alongside related definitions in this package.
2. Update its `mapper_*.go` conversion and affected consumers together.
3. Run `just generate` from the repository root and inspect the generated TypeScript change.
4. Verify mapping and visibility with affected backend tests and frontend type checking.

An internal model field does not automatically belong in a DTO. Expose only information required by the client contract. Preserve the distinction between a player's private view, opponent summaries, and spectator views; see [mapper_player.go](mapper_player.go), [mapper_game.go](mapper_game.go), and [mapper_spectator.go](mapper_spectator.go).

When removing or renaming a public field, update mappings and consumers without retaining a compatibility field.

## Type generation

- `json:"fieldName"` controls the serialized and generated field name; `omitempty` makes the TypeScript field optional.
- Let tygo infer types. Use `tstype:` only for overrides such as literal unions.
- `ts:` tags are ignored; do not add them.
- Use `//tygo:emit` for required TypeScript union declarations, following existing definitions.
- Use the root `just generate` recipe, which also formats the output. Do not hand-edit generated types.

## Behavior conditions

Conditions map to category-specific DTOs and a discriminated TypeScript union. For a client-visible condition change, update its DTO, the union where necessary, and `toResourceConditionDto` in [mapper_card.go](mapper_card.go). Follow the [shared-condition checklist](../../game/shared/AGENTS.md) so JSON conversion, validation, copying, and execution remain aligned.
