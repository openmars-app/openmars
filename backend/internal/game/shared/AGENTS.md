# Shared behavior-condition instructions

Card JSON uses flat conditions. [card_behavior_types.go](card_behavior_types.go) defines that JSON shape; [behavior_condition_categories.go](behavior_condition_categories.go) categorizes it into typed conditions and flattens it for serialization. Use the typed `BehaviorCondition` interface in domain code.

## Adding a type or field

- For a new resource type, add its constant in [resource_type.go](resource_type.go), register it in `AllResourceTypes` in [resource_condition_rules.go](resource_condition_rules.go), and update its `classifyResourceType` mapping.
- Add fields to the flat JSON representation and the relevant typed condition. Follow existing dedicated condition files when extending those categories.
- Update both `categorizeCondition` and `flattenCondition`; a field must survive JSON round trips.
- Update `deepCopyCondition` for reference-valued fields so copies do not share mutable pointers, slices, or maps.
- Update input/output validation profiles in [resource_condition_rules.go](resource_condition_rules.go).
- For a new category, implement the interface and its copy behavior, and extend the execution dispatch in [cards](../cards).
- If the client needs the new data, update the DTO, `toResourceConditionDto`, and generated union together. Follow [DTO instructions](../../delivery/dto/AGENTS.md).
- Update applicable frontend guards in [resourceConditions.ts](../../../../frontend/src/types/resourceConditions.ts) when changing the client condition union.
- Add backend tests for execution and affected conversion, validation, or copy behavior. Use constructed conditions in [behavior tests](../../../test/action/behavior).

Use `just generate` when the client contract changes and run the affected backend tests. Do not assume that adding a JSON field alone gives it executable behavior.

## Access and authoring

Use interface methods for common fields and the accessor helpers in [behavior_condition.go](behavior_condition.go) for optional category fields. Use a type assertion when the category is known; do not add irrelevant fields to every category.

Read [asset instructions](../../../assets/AGENTS.md) when changing card or mode definitions. Keep authored conditions, dependencies, and descriptions consistent with what the engine actually executes.
