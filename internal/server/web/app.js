// Ezida viewer — Alpine component (v1, read-only).
// Fetches /api/board once on init, groups cards by column. No event
// wiring yet (move/edit/SSE land in later phases).

function board() {
  return {
    loaded: false,
    schema_version: 0,
    columns: [],
    priorities: [],
    cards: [],
    async load() {
      try {
        const res = await fetch('/api/board');
        if (!res.ok) {
          const err = await res.json().catch(() => ({}));
          console.error('failed to load board', err);
          return;
        }
        const data = await res.json();
        this.schema_version = data.schema_version;
        this.columns = data.columns || [];
        this.priorities = data.priorities || [];
        this.cards = data.cards || [];
        this.loaded = true;
      } catch (e) {
        console.error('failed to fetch /api/board', e);
      }
    },
    cardsByColumn(name) {
      return this.cards.filter(function (c) { return c.column === name; });
    },
  };
}

if (window) window.board = board;
