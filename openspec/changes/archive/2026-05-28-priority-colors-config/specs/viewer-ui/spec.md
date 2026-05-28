## MODIFIED Requirements

### Requirement: Priorities map to distinguishable visual styles

The page SHALL apply a CSS class `priority-<value>` to each card
whose `priority` field is set, AND SHALL apply the per-priority
color resolved from the server-provided `priority_colors` map as
an inline `background-color` and matching `border-color` on the
priority badge element. Each value present in `[board].priorities`
MUST produce a visually distinguishable treatment: when the badge
has a color in `priority_colors`, the inline style takes effect;
when no color is configured, the badge falls back to the default
neutral chip skin defined by `:root` design tokens. Implementations
SHALL NOT hard-code per-priority hex colors in CSS.

#### Scenario: All three priorities present with default colors

- **WHEN** the board's response `priority_colors` equals
  `{"low":"#22c55e","medium":"#f59e0b","high":"#ef4444"}` and three
  cards with these priorities are rendered
- **THEN** each rendered card carries the matching `priority-*`
  class
- **AND** the priority badge of each card has its `background-color`
  computed-style equal to the corresponding hex

#### Scenario: Card without priority

- **WHEN** a card has no `priority` field
- **THEN** the rendered `.card` MUST NOT carry any `priority-*`
  class
- **AND** no priority badge is rendered

#### Scenario: Priority declared without a color

- **WHEN** a card's priority is declared in `[board].priorities` but
  `priority_colors` has no entry for it
- **THEN** the priority badge is rendered
- **AND** its inline `background-color` style is not set, so it
  falls back to the default badge skin
