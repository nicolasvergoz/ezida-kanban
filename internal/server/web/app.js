// Ezida viewer — Alpine component.
// V1 fetched /api/board once and rendered read-only. V2 wires
// Sortable.js so cards can be dragged across and within columns; on
// drop, a POST /api/cards/<id>/move request fires. On failure, the
// component refetches the board so the rendered DOM matches disk
// (ADR 0002 §D3 — server is source of truth, no manual revert).

function board() {
  return {
    loaded: false,
    schema_version: 0,
    columns: [],
    priorities: [],
    cards: [],
    // _sortables holds the Sortable instances mounted on each column
    // so they can be destroyed before being remounted after a refetch
    // (Alpine reuses the <ul> nodes across renders).
    _sortables: [],
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
        // Defer until Alpine has flushed the new DOM, then attach
        // Sortable to each freshly-rendered column.
        this.$nextTick(() => this.mountSortable());
      } catch (e) {
        console.error('failed to fetch /api/board', e);
      }
    },
    cardsByColumn(name) {
      return this.cards.filter(function (c) { return c.column === name; });
    },
    mountSortable() {
      // Tear down any previous instances so re-renders do not leak
      // listeners.
      while (this._sortables.length) {
        const s = this._sortables.pop();
        if (s && typeof s.destroy === 'function') s.destroy();
      }
      if (typeof Sortable === 'undefined') return;
      const self = this;
      document.querySelectorAll('.cards').forEach((ul) => {
        const s = Sortable.create(ul, {
          group: 'cards',
          animation: 0,
          ghostClass: 'sortable-ghost',
          onEnd: (evt) => self.handleDrop(evt),
        });
        self._sortables.push(s);
      });
    },
    async handleDrop(evt) {
      const id = evt.item && evt.item.dataset ? evt.item.dataset.cardId : '';
      const column = evt.to && evt.to.dataset ? evt.to.dataset.column : '';
      const position = typeof evt.newIndex === 'number' ? evt.newIndex : 0;
      if (!id || !column) {
        console.error('drop missing id/column', { id, column });
        await this.load();
        return;
      }
      try {
        const res = await fetch('/api/cards/' + encodeURIComponent(id) + '/move', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ column: column, position: position }),
        });
        if (!res.ok) {
          const err = await res.json().catch(() => ({}));
          console.error('move failed, refetching', err);
          await this.load();
          return;
        }
        // Success — refresh local state from the server so the
        // in-memory cards array reflects updated_at and any other
        // server-set fields. (Sortable already moved the DOM node;
        // this reconciles the JS model.)
        await this.load();
      } catch (e) {
        console.error('move request errored, refetching', e);
        await this.load();
      }
    },
  };
}

if (window) window.board = board;
