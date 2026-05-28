// demo-shim.js — make app.js work without the Go server.
//
// Loads board.json once, intercepts every fetch('/api/...') call, and
// routes it against an in-memory copy of the board. Mutations are
// applied to the in-memory copy and never persisted. EventSource is
// stubbed with a custom EventTarget so app.js's board-changed
// listener still fires after each mutation, giving the UI the same
// refresh behaviour it would get from the real SSE stream.
//
// All routes mirror the response shapes documented in
// internal/server/handlers.go so app.js consumes them through the
// exact same code path as the live server.

(function () {
  'use strict';

  var realFetch = window.fetch.bind(window);
  var RealEventSource = window.EventSource;

  // state holds the in-memory copy of board.json. Lazy-loaded on the
  // first /api/* call so the load is co-located with the code that
  // depends on it (app.js's first call is GET /api/board).
  var state = null;
  var loadPromise = null;
  var eventTargets = new Set();

  function loadState() {
    if (loadPromise) return loadPromise;
    loadPromise = realFetch('board.json').then(function (res) {
      if (!res.ok) throw new Error('board.json HTTP ' + res.status);
      return res.json();
    }).then(function (data) {
      state = data;
      // Defensive: app.js expects arrays, not null.
      if (!Array.isArray(state.cards)) state.cards = [];
      if (!Array.isArray(state.columns)) state.columns = [];
      if (!Array.isArray(state.priorities)) state.priorities = [];
      if (state.priority_colors === null || typeof state.priority_colors !== 'object' || Array.isArray(state.priority_colors)) {
        state.priority_colors = {};
      }
      state.cards.forEach(function (c) {
        if (!Array.isArray(c.tags)) c.tags = [];
        if (typeof c.description !== 'string') c.description = '';
      });
      return state;
    }).catch(function (e) {
      console.error('[demo-shim] failed to load board.json:', e);
      // Empty fallback keeps the UI usable rather than blank-screening.
      state = {
        schema_version: 1,
        project_name: 'Ezida (demo)',
        columns: ['todo', 'ongoing', 'done'],
        priorities: ['low', 'medium', 'high'],
        priority_colors: { low: '#22c55e', medium: '#f59e0b', high: '#ef4444' },
        cards_per_column: { todo: 0, ongoing: 0, done: 0 },
        cards: [],
      };
      return state;
    });
    return loadPromise;
  }

  function recomputeCounts() {
    var counts = {};
    state.columns.forEach(function (c) { counts[c] = 0; });
    state.cards.forEach(function (c) { counts[c.column] = (counts[c.column] || 0) + 1; });
    state.cards_per_column = counts;
  }

  function nowISO() { return new Date().toISOString(); }

  function newCardID() {
    // 6 chars from [0-9a-z] — matches the schema's id pattern.
    var alphabet = '0123456789abcdefghijklmnopqrstuvwxyz';
    for (var attempt = 0; attempt < 64; attempt++) {
      var s = '';
      for (var i = 0; i < 6; i++) {
        s += alphabet[Math.floor(Math.random() * alphabet.length)];
      }
      if (!state.cards.some(function (c) { return c.id === s; })) return s;
    }
    return 'demo' + Math.floor(Math.random() * 100); // pragmatic fallback
  }

  function fireBoardChanged() {
    eventTargets.forEach(function (t) {
      try {
        t.dispatchEvent(new MessageEvent('board-changed', { data: '' }));
      } catch (_) { /* swallow — demo SSE is best-effort */ }
    });
  }

  function jsonResponse(body, status) {
    return new Response(JSON.stringify(body), {
      status: status || 200,
      headers: { 'Content-Type': 'application/json' },
    });
  }

  function errorResponse(code, message, status) {
    return jsonResponse({ error: { code: code, message: message } }, status);
  }

  function emptyOK() { return new Response(null, { status: 204 }); }

  // route handles the in-memory mutation and returns a Response. Each
  // case mirrors the equivalent server handler in
  // internal/server/handlers.go (same response status + shape).
  function route(url, method, bodyText) {
    var path = url.replace(/^.*\/api\//, '/api/');
    var body = null;
    if (bodyText) {
      try { body = JSON.parse(bodyText); } catch (_) { body = null; }
    }

    // GET /api/board → full snapshot.
    if (method === 'GET' && path === '/api/board') {
      return jsonResponse(state);
    }

    // POST /api/cards → add card.
    if (method === 'POST' && path === '/api/cards') {
      var addTitle = (body && typeof body.title === 'string') ? body.title.trim() : '';
      var addCol = (body && typeof body.column === 'string') ? body.column : '';
      if (!addTitle) return errorResponse('INVALID_BODY', 'title required', 400);
      if (!state.columns.includes(addCol)) return errorResponse('INVALID_COLUMN', 'unknown column ' + addCol, 400);
      var card = {
        id: newCardID(),
        title: addTitle,
        column: addCol,
        priority: '',
        tags: [],
        description: '',
        created_at: nowISO(),
        updated_at: nowISO(),
      };
      state.cards.push(card);
      recomputeCounts();
      fireBoardChanged();
      return jsonResponse({ card: card }, 201);
    }

    // POST /api/cards/<id>/move
    var moveMatch = path.match(/^\/api\/cards\/([^/]+)\/move$/);
    if (method === 'POST' && moveMatch) {
      var moveID = decodeURIComponent(moveMatch[1]);
      var mc = state.cards.find(function (c) { return c.id === moveID; });
      if (!mc) return errorResponse('CARD_NOT_FOUND', 'no card ' + moveID, 404);
      var targetCol = body && body.column;
      var targetPos = (body && typeof body.position === 'number') ? body.position : 0;
      if (!state.columns.includes(targetCol)) return errorResponse('INVALID_COLUMN', 'unknown column ' + targetCol, 400);
      // Remove from current position then splice into target column at index.
      state.cards = state.cards.filter(function (c) { return c.id !== moveID; });
      mc.column = targetCol;
      mc.updated_at = nowISO();
      // Insert at the target position among cards of the target column.
      var insertAt = 0;
      var seen = 0;
      for (var i = 0; i < state.cards.length; i++) {
        if (state.cards[i].column === targetCol) {
          if (seen === targetPos) { insertAt = i; break; }
          seen++;
        }
        insertAt = i + 1;
      }
      state.cards.splice(insertAt, 0, mc);
      recomputeCounts();
      fireBoardChanged();
      return emptyOK();
    }

    // PATCH /api/cards/<id> → edit field(s) on a card.
    var patchMatch = path.match(/^\/api\/cards\/([^/]+)$/);
    if (method === 'PATCH' && patchMatch) {
      var pid = decodeURIComponent(patchMatch[1]);
      var pc = state.cards.find(function (c) { return c.id === pid; });
      if (!pc) return errorResponse('CARD_NOT_FOUND', 'no card ' + pid, 404);
      if (body) {
        if (typeof body.title === 'string') {
          var t = body.title.trim();
          if (!t) return errorResponse('INVALID_BODY', 'title cannot be empty', 400);
          pc.title = t;
        }
        if (typeof body.description === 'string') pc.description = body.description;
        if (typeof body.priority === 'string') {
          if (body.priority !== '' && !state.priorities.includes(body.priority)) {
            return errorResponse('INVALID_PRIORITY', 'unknown priority ' + body.priority, 400);
          }
          pc.priority = body.priority;
        }
        if (Array.isArray(body.tags)) pc.tags = body.tags.slice();
      }
      pc.updated_at = nowISO();
      fireBoardChanged();
      return jsonResponse({ card: pc });
    }

    // DELETE /api/cards/<id>
    if (method === 'DELETE' && patchMatch) {
      var dpid = decodeURIComponent(patchMatch[1]);
      var idx = state.cards.findIndex(function (c) { return c.id === dpid; });
      if (idx < 0) return errorResponse('CARD_NOT_FOUND', 'no card ' + dpid, 404);
      state.cards.splice(idx, 1);
      recomputeCounts();
      fireBoardChanged();
      return emptyOK();
    }

    // POST /api/columns → add column.
    if (method === 'POST' && path === '/api/columns') {
      var newName = body && typeof body.name === 'string' ? body.name.trim() : '';
      if (!newName) return errorResponse('INVALID_BODY', 'name required', 400);
      if (state.columns.includes(newName)) return errorResponse('COLUMN_EXISTS', 'column already exists', 409);
      state.columns.push(newName);
      recomputeCounts();
      fireBoardChanged();
      return jsonResponse({ name: newName }, 201);
    }

    // POST /api/columns/move → reorder columns.
    if (method === 'POST' && path === '/api/columns/move') {
      var mvName = body && body.name;
      var mvPos = (body && typeof body.position === 'number') ? body.position : 0;
      var mvIdx = state.columns.indexOf(mvName);
      if (mvIdx < 0) return errorResponse('COLUMN_NOT_FOUND', 'no column ' + mvName, 404);
      state.columns.splice(mvIdx, 1);
      var clamped = Math.max(0, Math.min(state.columns.length, mvPos));
      state.columns.splice(clamped, 0, mvName);
      fireBoardChanged();
      return emptyOK();
    }

    // PATCH /api/columns/<old> → rename column (cascades to cards).
    var colPatchMatch = path.match(/^\/api\/columns\/([^/]+)$/);
    if (method === 'PATCH' && colPatchMatch) {
      var oldName = decodeURIComponent(colPatchMatch[1]);
      var rawNew = body && typeof body.name === 'string' ? body.name.trim() : '';
      if (!rawNew) return errorResponse('INVALID_BODY', 'name required', 400);
      var oldIdx = state.columns.indexOf(oldName);
      if (oldIdx < 0) return errorResponse('COLUMN_NOT_FOUND', 'no column ' + oldName, 404);
      if (rawNew !== oldName && state.columns.includes(rawNew)) {
        return errorResponse('COLUMN_EXISTS', 'column already exists', 409);
      }
      state.columns[oldIdx] = rawNew;
      state.cards.forEach(function (c) { if (c.column === oldName) c.column = rawNew; });
      recomputeCounts();
      fireBoardChanged();
      return jsonResponse({ name: rawNew });
    }

    // DELETE /api/columns/<col> → remove column (refused if non-empty).
    if (method === 'DELETE' && colPatchMatch) {
      var rmName = decodeURIComponent(colPatchMatch[1]);
      var rmIdx = state.columns.indexOf(rmName);
      if (rmIdx < 0) return errorResponse('COLUMN_NOT_FOUND', 'no column ' + rmName, 404);
      if (state.columns.length <= 1) return errorResponse('CANNOT_DELETE_LAST_COLUMN', 'cannot delete the last column', 409);
      var has = state.cards.some(function (c) { return c.column === rmName; });
      if (has) return errorResponse('COLUMN_HAS_CARDS', 'column still has cards', 409);
      state.columns.splice(rmIdx, 1);
      recomputeCounts();
      fireBoardChanged();
      return emptyOK();
    }

    // Unmatched route — return 404; app.js refetches /api/board.
    return errorResponse('NOT_FOUND', 'demo shim does not handle ' + method + ' ' + path, 404);
  }

  // Override fetch: any /api/* call goes through the in-memory router;
  // everything else passes through to the real fetch.
  window.fetch = async function (input, init) {
    var url = (typeof input === 'string') ? input : input.url;
    if (!url || url.indexOf('/api/') < 0) return realFetch(input, init);
    var method = (init && init.method) ? init.method.toUpperCase() : 'GET';
    var bodyText = (init && init.body && typeof init.body === 'string') ? init.body : null;
    await loadState();
    try {
      return route(url, method, bodyText);
    } catch (e) {
      console.error('[demo-shim] router error:', e);
      return errorResponse('INTERNAL', e.message || String(e), 500);
    }
  };

  // Override EventSource with a no-op that participates in
  // board-changed dispatch so mutations refresh the UI. The real
  // EventSource is preserved on window._RealEventSource for forensics.
  window._RealEventSource = RealEventSource;
  function DemoEventSource(url) {
    var target = new EventTarget();
    eventTargets.add(target);
    var self = this;
    self.readyState = 0;
    self.url = url;
    self.onopen = null;
    self.onerror = null;
    self.onmessage = null;
    self.addEventListener = target.addEventListener.bind(target);
    self.removeEventListener = target.removeEventListener.bind(target);
    self.dispatchEvent = target.dispatchEvent.bind(target);
    self.close = function () {
      self.readyState = 2;
      eventTargets.delete(target);
    };
    // Fire 'open' on the next microtask so listeners registered after
    // construction (the common pattern) still see it.
    queueMicrotask(function () {
      self.readyState = 1;
      var openEvt = new Event('open');
      target.dispatchEvent(openEvt);
      if (typeof self.onopen === 'function') self.onopen(openEvt);
    });
  }
  DemoEventSource.CONNECTING = 0;
  DemoEventSource.OPEN = 1;
  DemoEventSource.CLOSED = 2;
  window.EventSource = DemoEventSource;
})();
