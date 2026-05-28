## Context

Priorities are user-defined (`[board].priorities` is a free-form list), so the viewer cannot hard-code a fixed palette in CSS. Today the priority badge uses `.badge` (single neutral skin) regardless of value. We want a TOML-driven mapping the viewer can read at runtime, with sensible defaults for the common `low`/`medium`/`high` triad most boards use.

## Goals / Non-Goals

**Goals:**
- One source of truth for priority → color, in `kanban.toml`.
- Server-resolved defaults so viewer + demo + future clients see the same final palette without each reimplementing the default logic.
- Fail closed at load time on bad config (unknown key, bad hex) — same Validate pass as other rules.
- Additive wire change, zero schema bump.

**Non-Goals:**
- CLI surface for editing colors (TOML hand-edit only for now — matches the card scope).
- Background gradients, dark-mode-specific palettes, or contrast computation. Single hex per priority; existing badge contrast (white text on color) is the default skin.
- Per-column or per-tag colors.

## Decisions

### 1. Schema: `[board.priority_colors]` as a TOML inline table (map)

```toml
[board.priority_colors]
low = "#22c55e"
medium = "#f59e0b"
high = "#ef4444"
```

Modeled in Go as `PriorityColors map[string]string \`toml:"priority_colors,omitempty"\`` on `BoardConfig`. `omitempty` so an absent map round-trips as absent (no empty `[board.priority_colors]` written back on save).

**Alternative considered:** parallel arrays `priority_colors = ["#22c55e", "#f59e0b", "#ef4444"]` indexed by `[board].priorities`. Rejected — fragile to reorders, no obvious binding to the priority name.

### 2. Validation rule (rule 10)

- Each key MUST be a value in `[board].priorities` → `UNKNOWN_PRIORITY` style violation otherwise.
- Each value MUST match `^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$` → invalid format violation otherwise.
- Map MAY be absent or empty → both legal.
- Map MAY be a subset of priorities → priorities without a color simply render with the existing neutral badge skin.

Implemented as new branch in `Validate`. Number is `10` to extend the existing 1..9 sequence.

### 3. Defaults resolved server-side, not in the viewer

`/api/board` returns `priority_colors` as a fully resolved map. Algorithm:

1. Start from `b.Board.PriorityColors` (user values).
2. For each conventional default name in `{"low": "#22c55e", "medium": "#f59e0b", "high": "#ef4444"}`, if the priority is declared in `[board].priorities` AND the user did not supply a color for it, fill in the default.
3. Drop any entry whose key is not in declared priorities (defense in depth; validation already rejects it).

User values always win — the defaults only fill in gaps for the conventional names. Priorities outside `low/medium/high` (e.g. `urgent`) without a user color stay absent from the map, and render with the neutral badge skin.

**Alternative considered:** ship defaults in the viewer JS. Rejected — duplicates the constant in two places (main viewer + demo) and any future client would re-implement it.

### 4. Wire shape

```jsonc
{
  "schema_version": 1,
  "columns": [...],
  "priorities": [...],
  "priority_colors": { "low": "#22c55e", "medium": "#f59e0b", "high": "#ef4444" },
  "cards_per_column": {...},
  "cards": [...],
  "project_name": "..."
}
```

Field is always present; empty `{}` when no defaults applied (no `low`/`medium`/`high` priorities declared) and no user values supplied.

### 5. Viewer rendering

The card-meta badge is currently:
```html
<li class="badge t-tag" x-text="card.priority"></li>
```

Becomes:
```html
<li class="badge t-tag"
    :style="priorityColors[card.priority] ? ('background-color:' + priorityColors[card.priority] + ';border-color:' + priorityColors[card.priority]) : ''"
    x-text="card.priority"></li>
```

`priorityColors` is set from `data.priority_colors` in the `load()` callback. No CSS-level changes; foreground stays white (current `var(--surface)`) because all default colors have sufficient contrast.

Demo (`site/demo/`) mirrors the same change against the baked `board.json`.

## Risks / Trade-offs

- **Contrast on custom colors** → user can pick a near-white color that disappears on a light background. Mitigation: documentation note. Out of scope to auto-compute foreground.
- **Demo `board.json` drift** → the static demo bakes a snapshot. Regenerated as part of this change; future schema additions still require a manual refresh (existing constraint).
- **Schema_version unchanged** → consumers parsing the TOML themselves and not tolerating unknown keys will choke. The Go `toml` library and current viewer flow are tolerant; documented in `docs/` if a downstream parser is mentioned.

## Migration Plan

- No data migration. New field is optional; existing boards continue to load unchanged.
- Existing demo board snapshot regenerated in the same commit.
- Rollback: revert the change — the wire field becomes unknown, viewer falls back to neutral badge (current behavior). TOML files containing `[board.priority_colors]` would then fail to load on the old binary because `Validate` would not yet know the field — acceptable, since this only matters if a user writes the field and then downgrades the binary.

## Open Questions

None — scope is contained to the card description.
