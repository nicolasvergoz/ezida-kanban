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
    // project_name comes from /api/board (filepath.Base of parent dir,
    // computed once on the server at boot). Bound to the topbar brand
    // via x-text; CSS handles uppercasing.
    project_name: '',
    columns: [],
    priorities: [],
    cards: [],
    // _dragScrollMounted guards setupDragScroll() so the pointer
    // listeners attach exactly once even though load() runs on every
    // SSE-triggered refetch.
    _dragScrollMounted: false,
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
    // V4 SSE state. `connected` drives the topbar status dot;
    // `eventSource` holds the active EventSource handle so a future
    // reconnect can tear down the previous one (the browser's
    // built-in retry handles transient disconnects, but a hard
    // reconnect after server restart re-uses the same handle).
    connected: false,
    eventSource: null,
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
        this.project_name = data.project_name || 'Ezida';
        this.columns = data.columns || [];
        this.priorities = data.priorities || [];
        this.cards = data.cards || [];
        this.loaded = true;
        // Defer until Alpine has flushed the new DOM, then attach
        // Sortable to each freshly-rendered column and wire the
        // drag-scroll affordance on the .board element (one-shot).
        this.$nextTick(() => {
          this.mountSortable();
          this.setupDragScroll();
        });
        // V4: open the SSE connection once, after the very first
        // successful board fetch, so initial render and live updates
        // share the same code path.
        if (!this.eventSource) this.connectEvents();
      } catch (e) {
        console.error('failed to fetch /api/board', e);
      }
    },
    // connectEvents subscribes to the server's SSE stream. Browsers
    // auto-reconnect on disconnect honoring the server's
    // `retry: 2000` directive, so we only need to track the open
    // state (`connected`) and react to `board-changed` events.
    connectEvents() {
      const es = new EventSource('/api/events');
      const self = this;
      es.addEventListener('board-changed', function () { self.handleExternalChange(); });
      es.onopen = function () { self.connected = true; };
      es.onerror = function () { self.connected = false; };
      this.eventSource = es;
    },
    // handleExternalChange is invoked when the server fires a
    // `board-changed` SSE event (e.g. CLI write, manual save). The
    // open modal — if any — closes without prompting (spec D9, V4);
    // then we refetch the board so the rendered DOM reflects disk.
    handleExternalChange() {
      if (this.editing) this.closeCard();
      this.load();
    },
    cardsByColumn(name) {
      return this.cards.filter(function (c) { return c.column === name; });
    },
    // setupDragScroll wires pointerdown/pointermove/pointerup listeners
    // on the .board element so a user can drag the empty board surface
    // horizontally. Guarded by _dragScrollMounted so the listeners
    // attach exactly once. Skips interactive descendants
    // (.card, .column-header, button, form controls, .modal) so
    // Sortable's card drags and click handlers keep working.
    //
    // While a drag is active, body.is-scrolling is set; CSS uses that
    // class to disable pointer-events on .card so the gesture isn't
    // hijacked by a child click.
    setupDragScroll() {
      if (this._dragScrollMounted) return;
      const board = this.$el.querySelector('.board');
      if (!board) return;
      this._dragScrollMounted = true;
      let isDragging = false;
      let startX = 0;
      let startScroll = 0;
      board.addEventListener('pointerdown', function (e) {
        if (e.button !== 0) return;
        const t = e.target;
        if (t && t.closest && t.closest('.card, .column-header, button, input, textarea, select, .modal, .modal-overlay')) {
          return;
        }
        isDragging = true;
        startX = e.clientX;
        startScroll = board.scrollLeft;
        document.body.classList.add('is-scrolling');
        try { board.setPointerCapture(e.pointerId); } catch (_) {}
      });
      board.addEventListener('pointermove', function (e) {
        if (!isDragging) return;
        board.scrollLeft = startScroll - (e.clientX - startX);
      });
      const endDrag = function (e) {
        if (!isDragging) return;
        isDragging = false;
        document.body.classList.remove('is-scrolling');
        try { board.releasePointerCapture(e.pointerId); } catch (_) {}
      };
      board.addEventListener('pointerup', endDrag);
      board.addEventListener('pointercancel', endDrag);
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
