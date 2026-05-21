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
    // _dragJustEnded is a transient flag set by Sortable's onEnd and
    // cleared on the next tick. deleteCard() consults it so a
    // drag-end mouseup that happens to land on the .card-delete
    // button does NOT fire a stray DELETE (UI-4 design.md §D5).
    _dragJustEnded: false,
    // UI-5 detail-modal state. The modal renders each editable field
    // as rendered text by default; clicking a field flips its
    // `editing.<name>` flag and swaps in the matching inline editor.
    // `saving.<name>` and `errors.<name>` are sibling maps. `drafts`
    // holds the in-flight value while a field is in editing mode;
    // tags has no draft slot because the chip editor mutates
    // `openCardData.tags` directly via PATCH (per-action commit).
    // `openCardData` is the source-of-truth card object, replaced
    // wholesale from the server response after each successful PATCH.
    open: false,
    openCardData: null,
    editing: { title: false, description: false, priority: false, tags: false },
    saving: { title: false, description: false, priority: false, tags: false },
    errors: { title: '', description: '', priority: '', tags: '' },
    drafts: { title: '', description: '', priority: '' },
    tagInput: '',
    // V4 SSE state. `connected` drives the topbar status dot;
    // `eventSource` holds the active EventSource handle so a future
    // reconnect can tear down the previous one (the browser's
    // built-in retry handles transient disconnects, but a hard
    // reconnect after server restart re-uses the same handle).
    connected: false,
    eventSource: null,
    // Own-write suppression: any write we issue (PATCH/POST/DELETE)
    // also triggers the fsnotify watcher and a board-changed SSE
    // event. We don't want our own writes to close the open modal or
    // tear down an in-flight rename — they would interrupt the user
    // mid-edit. _lastSelfWriteAt is bumped right before every write;
    // handleExternalChange treats events within ~1500ms as own.
    _lastSelfWriteAt: 0,
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
    // UI-6 inline-column-ops state. `renamingColumn` is the column
    // currently being inline-renamed (single rename at a time across
    // the page) or null. `renameDraft` holds the in-flight value;
    // `renameError` carries the server's error message when a PATCH
    // refuses the rename (keeps the input open per design TD10).
    renamingColumn: null,
    renameDraft: '',
    renameError: '',
    // List-menu state (3-dots delete). Single menu open at a time.
    openMenuColumn: null,
    menuError: '',
    // Add-list composer state (placeholder/composer pair at the end
    // of the column strip, design TD9).
    composingList: false,
    listDraft: '',
    listError: '',
    // _listSortable holds the single Sortable instance bound to the
    // .columns container; tear-down + re-mount happens on every
    // load() refetch so Alpine's re-rendered DOM gets re-bound.
    _listSortable: null,
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
          this.mountListSortable();
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
      const isOwn = (Date.now() - this._lastSelfWriteAt) < 1500;
      if (this.open && !isOwn) this.closeModal();
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
    // --- UI-6 inline column ops --------------------------------------------
    // startRename flips the list-header span into its input variant for
    // the clicked column. The Alpine x-show toggles do the swap; we
    // seed the draft and clear any error so the input opens clean.
    startRename(col) {
      this.renamingColumn = col;
      this.renameDraft = col;
      this.renameError = '';
    },
    // cancelRename reverts without firing a PATCH (Esc or blur with
    // unchanged/empty value).
    cancelRename() {
      this.renamingColumn = null;
      this.renameDraft = '';
      this.renameError = '';
    },
    // commitRename trims the draft, no-ops on same-name/empty, else
    // PATCH /api/columns/<old>. On 2xx clears all three fields (SSE
    // refetch updates the rendered name). On non-2xx leaves the input
    // open with the server's error message visible.
    async commitRename() {
      if (this.renamingColumn === null) return;
      const from = this.renamingColumn;
      const trimmed = (this.renameDraft || '').trim();
      if (trimmed === from || trimmed === '') {
        this.cancelRename();
        return;
      }
      try {
        this._lastSelfWriteAt = Date.now();
        const res = await fetch('/api/columns/' + encodeURIComponent(from), {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ name: trimmed }),
        });
        if (!res.ok) {
          const err = await res.json().catch(() => ({}));
          this.renameError = (err && err.error && err.error.message) || ('HTTP ' + res.status);
          return;
        }
        this.renamingColumn = null;
        this.renameDraft = '';
        this.renameError = '';
      } catch (e) {
        this.renameError = e.message || String(e);
      }
    },
    // toggleListMenu opens the menu for col, or closes it if already
    // open. Clears any prior menuError so a fresh open never reuses a
    // stale message.
    toggleListMenu(col) {
      if (this.openMenuColumn === col) {
        this.openMenuColumn = null;
      } else {
        this.openMenuColumn = col;
      }
      this.menuError = '';
    },
    closeListMenu() {
      this.openMenuColumn = null;
      this.menuError = '';
    },
    // deleteList issues DELETE /api/columns/<col>. On 2xx closes the
    // menu (SSE refetch picks up the deletion). On non-2xx (e.g.
    // COLUMN_HAS_CARDS, CANNOT_DELETE_LAST_COLUMN) keeps the menu open
    // and surfaces the server's message inline.
    async deleteList(col) {
      this.menuError = '';
      try {
        this._lastSelfWriteAt = Date.now();
        const res = await fetch('/api/columns/' + encodeURIComponent(col), {
          method: 'DELETE',
        });
        if (!res.ok) {
          const err = await res.json().catch(() => ({}));
          this.menuError = (err && err.error && err.error.message) || ('HTTP ' + res.status);
          return;
        }
        this.closeListMenu();
      } catch (e) {
        this.menuError = e.message || String(e);
      }
    },
    // openListComposer / cancelNewList / submitNewList drive the
    // dashed Add-list placeholder ↔ composer state machine (design
    // TD9).
    openListComposer() {
      this.composingList = true;
      this.listDraft = '';
      this.listError = '';
    },
    cancelNewList() {
      this.composingList = false;
      this.listDraft = '';
      this.listError = '';
    },
    async submitNewList() {
      const trimmed = (this.listDraft || '').trim();
      if (!trimmed) {
        this.listError = 'name required';
        return;
      }
      this.listError = '';
      try {
        this._lastSelfWriteAt = Date.now();
        const res = await fetch('/api/columns', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ name: trimmed }),
        });
        if (!res.ok) {
          const err = await res.json().catch(() => ({}));
          this.listError = (err && err.error && err.error.message) || ('HTTP ' + res.status);
          return;
        }
        this.cancelNewList();
      } catch (e) {
        this.listError = e.message || String(e);
      }
    },
    // mountListSortable binds the column-strip Sortable instance to
    // .columns with a distinct group from the cards instance (TD8) so
    // the two cannot interfere. Re-mounts on every load() refetch.
    mountListSortable() {
      if (this._listSortable && typeof this._listSortable.destroy === 'function') {
        this._listSortable.destroy();
      }
      this._listSortable = null;
      if (typeof Sortable === 'undefined') return;
      const container = document.querySelector('.columns');
      if (!container) return;
      const self = this;
      this._listSortable = Sortable.create(container, {
        group: 'lists',
        handle: '.list-header',
        filter: '.is-renaming, .list-menu-btn, .list-menu, .column-name--input, .add-list-placeholder, .list-composer',
        // Without preventOnFilter: false, Sortable calls
        // preventDefault on mousedown for filter-matched elements,
        // which kills native focus on the rename <input> — the field
        // appears open but is not editable.
        preventOnFilter: false,
        animation: 0,
        ghostClass: 'sortable-ghost-list',
        draggable: '.column',
        onEnd: (evt) => self.handleListDrop(evt),
      });
    },
    // handleListDrop posts the move. On success refetch; on failure
    // log + refetch so the server stays source of truth (ADR 0002 §D3).
    async handleListDrop(evt) {
      const name = evt.item && evt.item.dataset ? evt.item.dataset.column : '';
      const position = typeof evt.newIndex === 'number' ? evt.newIndex : 0;
      if (!name) {
        console.error('list drop missing name', evt);
        await this.load();
        return;
      }
      try {
        this._lastSelfWriteAt = Date.now();
        const res = await fetch('/api/columns/move', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ name: name, position: position }),
        });
        if (!res.ok) {
          const err = await res.json().catch(() => ({}));
          console.error('list move failed, refetching', err);
          await this.load();
          return;
        }
        await this.load();
      } catch (e) {
        console.error('list move errored, refetching', e);
        await this.load();
      }
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
          onEnd: (evt) => {
            self._dragJustEnded = true;
            setTimeout(() => { self._dragJustEnded = false; }, 0);
            self.handleDrop(evt);
          },
        });
        self._sortables.push(s);
      });
    },
    // deleteCard issues DELETE /api/cards/<id> per UI-4 design.md §D5.
    // - stopPropagation prevents the card's click handler from also
    //   firing openCard(card) and popping the modal.
    // - _dragJustEnded guards against a drag-end mouseup that lands
    //   on the delete button region — Sortable's onEnd handler sets
    //   the flag in the same macrotask, so a 0 ms timeout is enough
    //   to skip the immediate-following synthetic click and re-arm.
    // - On 404 (or any non-2xx) the page refetches /api/board so the
    //   client recovers from a CLI race that already deleted the card.
    // - On success, the SSE board-changed event drives the refetch;
    //   no optimistic local mutation here.
    async deleteCard(id, evt) {
      if (evt) evt.stopPropagation();
      if (this._dragJustEnded) return;
      try {
        this._lastSelfWriteAt = Date.now();
        const res = await fetch('/api/cards/' + encodeURIComponent(id), {
          method: 'DELETE',
        });
        if (!res.ok) {
          console.warn('delete failed', res.status);
          await this.load();
          return;
        }
        // SSE will refetch; no manual reload here.
      } catch (e) {
        console.error('delete request errored, refetching', e);
        await this.load();
      }
    },
    // UI-5: openCard seeds `openCardData` from the clicked card and
    // flips the modal open. Per-field `editing` / `saving` flags reset
    // to false; `errors` and `drafts` reset to empty strings; tags
    // array is cloned so removeTag's PATCH (which sends a new array)
    // cannot accidentally alias the rendered board state.
    openCard(card) {
      this.openCardData = Object.assign({}, card, { tags: (card.tags || []).slice() });
      this.editing = { title: false, description: false, priority: false, tags: false };
      this.saving = { title: false, description: false, priority: false, tags: false };
      this.errors = { title: '', description: '', priority: '', tags: '' };
      this.drafts = { title: '', description: '', priority: '' };
      this.tagInput = '';
      this.open = true;
    },
    // closeModal tears down the per-field state and hides the modal.
    // No prompt, no in-flight cancellation — overlay-click / Esc
    // (with no active field) reach this path; field-revert lives in
    // revertField. SSE board-changed also routes through here.
    closeModal() {
      this.open = false;
      this.openCardData = null;
      this.editing = { title: false, description: false, priority: false, tags: false };
      this.saving = { title: false, description: false, priority: false, tags: false };
      this.errors = { title: '', description: '', priority: '', tags: '' };
      this.drafts = { title: '', description: '', priority: '' };
      this.tagInput = '';
    },
    // startEdit flips a single field into edit mode (design MD2: one
    // field at a time). Any other currently-editing field is committed
    // (blur-style) first. The current rendered value is copied into
    // `drafts[name]` and focus moves to the swapped-in editor on the
    // next tick.
    async startEdit(name) {
      if (!this.openCardData) return;
      // Commit any other field that is currently in editing.
      const names = ['title', 'description', 'priority'];
      for (let i = 0; i < names.length; i++) {
        const other = names[i];
        if (other !== name && this.editing[other]) {
          await this.commitField(other);
        }
      }
      const current = this.openCardData[name];
      this.drafts[name] = (current === undefined || current === null) ? '' : current;
      this.errors[name] = '';
      this.editing[name] = true;
      const self = this;
      const refMap = {
        title: 'titleInput',
        description: 'descriptionInput',
        priority: 'priorityInput',
      };
      this.$nextTick(function () {
        const ref = refMap[name];
        if (ref && self.$refs && self.$refs[ref]) {
          // $nextTick is a microtask, which Safari treats as a
          // continuation of the originating user gesture (the field
          // click). setTimeout(0) would be a macrotask and break the
          // gesture chain — Safari then refuses programmatic focus.
          self.$refs[ref].focus();
        }
      });
    },
    // commitField is the thin wrapper bound to blur / Enter / change
    // events on the inline editors — it forwards the current draft to
    // saveField. Declarative bindings stay readable; the network
    // logic lives in saveField.
    async commitField(name) {
      if (!this.editing[name]) return;
      await this.saveField(name, this.drafts[name]);
    },
    // revertField discards the in-flight draft and clears any field
    // error without firing a PATCH (design MD5 Esc semantics). The
    // last-saved value is the rendered span's source, so flipping
    // editing[name] back to false restores it visually.
    revertField(name) {
      this.drafts[name] = '';
      this.errors[name] = '';
      this.editing[name] = false;
    },
    // saveField issues a single-key PATCH per design MD3/MD4. On 2xx
    // the server response card replaces `openCardData` and the field
    // flips back to rendered mode. On non-2xx the editor stays open
    // with its in-flight value preserved and the server message
    // renders in `.field-error` under the field. On any path
    // `saving[name]` flips back to false in finally.
    async saveField(name, value) {
      if (!this.openCardData) return;
      const id = this.openCardData.id;
      this.errors[name] = '';
      this.saving[name] = true;
      try {
        const body = {};
        body[name] = value;
        this._lastSelfWriteAt = Date.now();
        const res = await fetch('/api/cards/' + encodeURIComponent(id), {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body),
        });
        if (!res.ok) {
          const err = await res.json().catch(function () { return {}; });
          this.errors[name] = (err && err.error && err.error.message) || ('HTTP ' + res.status);
          return; // leave editing[name] true so the user can fix and retry
        }
        const data = await res.json().catch(function () { return {}; });
        if (data && data.card) {
          this.openCardData = Object.assign({}, data.card, {
            tags: (data.card.tags || []).slice(),
          });
        }
        this.editing[name] = false;
        if (name === 'tags') {
          this.tagInput = '';
        } else {
          this.drafts[name] = '';
        }
        await this.load();
      } catch (e) {
        this.errors[name] = e.message || String(e);
      } finally {
        this.saving[name] = false;
      }
    },
    // onEscape is the modal-overlay Esc handler — context-sensitive
    // per design MD5: if any field is in editing, revert that one;
    // otherwise close the modal. Inline editors also stop.prevent on
    // their own Esc (defense in depth) so the bubbled handler is
    // only reached when no field is active.
    onEscape() {
      if (!this.open) return;
      const names = ['title', 'description', 'priority'];
      for (let i = 0; i < names.length; i++) {
        if (this.editing[names[i]]) {
          this.revertField(names[i]);
          return;
        }
      }
      this.closeModal();
    },
    // addTag commits a per-action PATCH with the new tags array.
    // Trims, dedups, and clears the input on no-op. Server response
    // (via saveField) replaces openCardData, keeping the chip list
    // and tag input in sync.
    async addTag() {
      if (!this.openCardData) return;
      const t = (this.tagInput || '').trim();
      if (!t) return;
      const existing = this.openCardData.tags || [];
      if (existing.indexOf(t) >= 0) {
        this.tagInput = '';
        return; // dedup, no PATCH
      }
      const next = existing.concat([t]);
      this.tagInput = '';
      await this.saveField('tags', next);
    },
    // removeTag commits a per-action PATCH with the tag filtered out.
    // The chip list reflects the server response after the PATCH
    // settles (no optimistic update).
    async removeTag(t) {
      if (!this.openCardData) return;
      const next = (this.openCardData.tags || []).filter(function (x) { return x !== t; });
      await this.saveField('tags', next);
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
        this._lastSelfWriteAt = Date.now();
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
