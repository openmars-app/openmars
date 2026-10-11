# Card-family integration tests

Use package `card_packs_test`. Keep related card and corporation cases together in the existing family files, such as `play_card_base_test.go` and `corporations_colonies_test.go`.

- Locate related card tests to select the family file; the current card database has no `pack` field.
- Add cases to an existing matching family file instead of creating one file per card. Create a family file only when no existing one fits.
- This convention applies to this directory. Condition, passive-effect, and mode-lifecycle tests remain in their own suites.
- Use the parent [test guide](../../AGENTS.md) for setup and synchronization, and [asset instructions](../../../assets/AGENTS.md) for card semantics.
