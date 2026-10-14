Highlights since v8.1.0.

## Added

- Card-play outputs store a pending card play with a cost reduction.
  `pendingCardPlaySelection` lists the discounted hand options, and the
  card can be played outside the normal turn checks.
- Players can skip colony placement, free trade, award funding and
  effect selection when no legal choice remains. The server rejects a
  skip while a legal choice exists, and the bot autopilot uses it.
- Selection state is revalidated and flags when no choices remain.
  Tile restrictions expose `placementLocation`.
- A lone legal branch of a forced passive choice is applied
  automatically; a choice with no legal branch is dropped.
- Colony requirements accept the `any-player` target.
- Behavior, award, milestone and colony icons use dedicated artwork
  registered in the asset catalog.
- Behavior icons carry location, adjacency, tag and VP-scoring badges.
- Card selection overlays show playability as error and warning badges.
- The UI Debug window in the debug menu previews every dialog and prompt
  with local sample data in an isolated overlay environment.
- `just run` serves the built frontend without hot reload.

## Changed

- An unplayable Prelude is discarded for 15 M€ instead of committed.
- Free trade requires the current turn.
- `Hand.ReceiveCards` adds cards and their private receipt in one
  update. Colony, trade, research, project funding, Venus bonus, discard
  and tile bonus draws use it; `Selection.AddCardReceipt` is removed.
- Unconditional auto behaviors with inputs render as one immediate
  exchange row. Resource amounts appear inline before their icons.
- Production boxes wrap their icons to the card width.
- Behavior descriptions describe only their own operation. Payments
  move out of requirement text on Air Raid, Potatoes, Moss, Nitrophilic
  Moss and Stratospheric Birds.
- Off-Mars sites are generated from one reserved celestial site list.
- Game panels, session dialogs, behavior resolution and resource removal
  are split into controllers and callback-driven views.

## Fixed

- Oceans no longer count as the placer's owned tiles, so they do not
  satisfy adjacency for greenery and other placements.
- Rad-Suits, Sister Planet Support and Urban Decomposers enforce their
  city, tag and colony requirements.
- Excentric Sponsor plays a hand card with a 25 M€ discount instead of
  granting 25 M€.
- Manutech's M€ production trigger used the wrong resource type and
  never fired.
- Colony track selection excludes inactive colonies, and the frontend
  hides their markers.
- Trading is disabled when the selected payment cannot be afforded.
- Skipped storage gains name the resource, and amount controls no
  longer shift as the value changes.
- The built frontend is read per request, so rebuilt files are served
  without restarting. Symlinks outside the build are refused.
- Card fitting is batched per animation frame, keeping virtual rows
  aligned while scrolling.

## Build

- Playwright browsers run with audio muted.
