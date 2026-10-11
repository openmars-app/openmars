# Card behavior rendering

Render backend behavior data using the existing classifier, merger, layout handlers, and display primitives. Preserve gameplay meaning when optimizing space.

## Visual semantics

- Arrow (`→`): manual-action inputs to outputs.
- Colon (`:`): trigger condition to effect.
- Slash (`/`): alternative resources.
- Minus/plus (`−/+`): negative/positive production.
- Keep action, production, and immediate-effect styling distinct; preserve attack and affordability indicators.

Do not merge away trigger conditions, state requirements, dependencies, choices, or other qualifiers. Reuse [behaviorMerger](utils/behaviorMerger.ts) and its eligibility rules instead of grouping effects just because their icons match.

Use [ResourceDisplay](components/ResourceDisplay.tsx), [CardIcon](components/CardIcon.tsx), and the existing qualifier/layout components. Keep resource counts consistent within a layout, including compact numeric presentation. Read sizing and row limits from the current implementation instead of duplicating constants.

## Changes and verification

When extending a behavior presentation, inspect [behaviorClassifier](utils/behaviorClassifier.ts), routing in [BehaviorSection](BehaviorSection.tsx), and the relevant layout handler. Update only the layers needed for the new semantics.

Use backend-provided state for availability and existing affordability helpers for presentation; do not add a separate frontend rule engine. Check both affordable and unaffordable presentations where applicable.

Follow the [frontend visual and mobile requirements](../../../../../AGENTS.md). Inspect affected card contexts at normal and compact sizes, including long descriptions, choices, and qualifiers. For card-data changes, also read [asset instructions](../../../../../../backend/assets/AGENTS.md).
