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
    // V3 edit-modal state. `editing` toggles the overlay; `draft` is
    // a shallow-cloned card under edit; `tagInput` is the chip-input
    // buffer; `error` carries the last server-side validation
    // message (or fetch error) so it can render inside the modal.
    editing: false,
    draft: null,
    tagInput: '',
    error: '',
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
    // openCard clones the clicked card into `draft` and flips the
    // modal open. The shallow clone (plus an explicit tags-slice
    // copy) means edits in the modal cannot mutate the rendered
    // board state until Save commits via PATCH.
    openCard(card) {
      this.draft = Object.assign({}, card, { tags: (card.tags || []).slice() });
      this.tagInput = '';
      this.error = '';
      this.editing = true;
    },
    closeCard() {
      this.editing = false;
      this.draft = null;
      this.tagInput = '';
      this.error = '';
    },
    addTag() {
      if (!this.draft) return;
      const t = (this.tagInput || '').trim();
      if (!t) return;
      if (!this.draft.tags.includes(t)) this.draft.tags.push(t);
      this.tagInput = '';
    },
    removeTag(t) {
      if (!this.draft) return;
      this.draft.tags = this.draft.tags.filter(function (x) { return x !== t; });
    },
    // saveCard sends the full editable field set to the server.
    // Spec D8 still allows partial patches; the UI sends all four
    // fields because it always has the full state in `draft`.
    async saveCard() {
      if (!this.draft) return;
      this.error = '';
      const body = {
        title: this.draft.title,
        description: this.draft.description,
        tags: this.draft.tags,
        priority: this.draft.priority || '',
      };
      try {
        const res = await fetch('/api/cards/' + encodeURIComponent(this.draft.id), {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body),
        });
        if (!res.ok) {
          const err = await res.json().catch(function () { return {}; });
          this.error = (err && err.error && err.error.message) || ('HTTP ' + res.status);
          return;
        }
        this.closeCard();
        await this.load();
      } catch (e) {
        this.error = e.message || String(e);
      }
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
