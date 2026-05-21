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
    // UI-3 filter state. `filter` is the current substring query
    // (trimmed/lowercased per-match). `filterOpen` toggles the
    // popover. Both are transient — never written to localStorage
    // (ADR 0003 §D8). A page reload clears both.
    filter: '',
    filterOpen: false,
    // UI-2 theme controller state. `theme` is the user's *choice*
    // ('light' | 'system' | 'dark') and drives the toggle's
    // aria-pressed state. `_mediaQuery` is the cached MediaQueryList
    // for `(prefers-color-scheme: dark)`; the change listener wired
    // in initTheme() re-applies the effective theme when the OS
    // flips while the user is in system mode (ADR 0003 §D7).
    theme: 'system',
    _mediaQuery: null,
    // init wires component-level $watch handlers that need the Alpine
    // proxy in scope (and so cannot be declared as inline expressions
    // on x-init). The filter watch retriggers mountSortable() after
    // the DOM has reflowed (ADR 0003 §D9 — Sortable must re-bind to
    // the visible <ul> children when the filter changes).
    init() {
      const self = this;
      this.$watch('filter', function () {
        self.$nextTick(function () { self.mountSortable(); });
      });
    },
    // initTheme reads localStorage["ezida.theme"], validates against
    // the whitelist, wires the matchMedia listener, and paints the
    // initial effective theme. Guarded so localStorage exceptions
    // (private browsing / blocked storage) do not break the page —
    // the toggle still works in-session.
    initTheme() {
      let stored = null;
      try {
        stored = localStorage.getItem('ezida.theme');
      } catch (_) {
        stored = null;
      }
      const whitelist = ['light', 'system', 'dark'];
      this.theme = whitelist.indexOf(stored) >= 0 ? stored : 'system';
      try {
        this._mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
        const self = this;
        const onChange = () => { self.applyTheme(); };
        if (typeof this._mediaQuery.addEventListener === 'function') {
          this._mediaQuery.addEventListener('change', onChange);
        } else if (typeof this._mediaQuery.addListener === 'function') {
          // Safari < 14 fallback.
          this._mediaQuery.addListener(onChange);
        }
      } catch (_) {
        this._mediaQuery = null;
      }
      this.applyTheme();
    },
    // applyTheme resolves the effective theme from `this.theme`
    // (and matchMedia when in system mode) and writes it to
    // <html data-theme="...">, which triggers the CSS variable swap.
    applyTheme() {
      let effective;
      if (this.theme === 'system') {
        effective = (this._mediaQuery && this._mediaQuery.matches) ? 'dark' : 'light';
      } else {
        effective = this.theme;
      }
      document.documentElement.setAttribute('data-theme', effective);
    },
    // setTheme is the click handler for the 3 toggle segments.
    // Validates against the whitelist (no-op on invalid), persists
    // the choice to localStorage (swallowing throws), and re-applies
    // the effective theme.
    setTheme(choice) {
      const whitelist = ['light', 'system', 'dark'];
      if (whitelist.indexOf(choice) < 0) return;
      this.theme = choice;
      try {
        localStorage.setItem('ezida.theme', choice);
      } catch (_) { /* swallow — session-only choice */ }
      this.applyTheme();
    },
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
    // filterMatches returns true when the current filter is empty
    // (after trim+lowercase), otherwise tests a single lowercased
    // haystack built from card.title + description + tags. indexOf
    // is used over .includes for parity with the rest of app.js
    // (ADR 0003 §D2: no ES2015+ features beyond what Alpine needs).
    filterMatches(card) {
      const q = (this.filter || '').trim().toLowerCase();
      if (!q) return true;
      const hay = (
        (card.title || '') + ' ' +
        (card.description || '') + ' ' +
        ((card.tags || []).join(' '))
      ).toLowerCase();
      return hay.indexOf(q) !== -1;
    },
    // filteredCardsByColumn returns cardsByColumn(name) verbatim when
    // the filter is empty/whitespace; otherwise the same list reduced
    // to cards matching filterMatches. Column templates iterate this
    // accessor; cardsByColumn(name) stays the source of truth for the
    // list-count badge (ADR 0003 §D8 / design.md §"List").
    filteredCardsByColumn(name) {
      const all = this.cardsByColumn(name);
      if (!(this.filter || '').trim()) return all;
      const self = this;
      return all.filter(function (c) { return self.filterMatches(c); });
    },
    // openFilter flips filterOpen true and focuses the input on the
    // next Alpine tick so x-show has rendered the popover by the time
    // focus() runs.
    openFilter() {
      this.filterOpen = true;
      const self = this;
      this.$nextTick(function () {
        if (self.$refs && self.$refs.filterInput) {
          self.$refs.filterInput.focus();
        }
      });
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
