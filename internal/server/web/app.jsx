const { useState, useEffect, useRef, useCallback, useMemo } = React;

/* =========================================================
   Wire-shape ↔ UI-shape adapter
   Server JSON:  { columns[], done_columns[], cards[{ id, title,
                   column, priority, tags[], description, epic, color,
                   created_at, updated_at }],
                   priorities[], priority_colors{}, project_name }
   UI tree:      { title, lists:[{ id=columnName, title=DISPLAY, done,
                   cards:[{ id, text, tags, priority, description,
                   epic, color, createdAt, updatedAt }] }],
                   priorities[], priorityColors{}, epics }

   The wire carries no denormalized relation data — no parent title,
   no children, no progress — because the payload already holds every
   card on the board. This adapter is therefore the single place that
   resolves a relation: it builds one id → card index per load and
   exposes parent / children / progress off it, so the resolution cost
   is paid once per load rather than once per rendered card.
========================================================= */
function toUiBoard(server) {
  const doneColumns = server.done_columns || [];
  const doneSet = new Set(doneColumns);
  const cardsByCol = {};
  const allCards = [];
  for (const c of server.cards || []) {
    const ui = {
      id: c.id,
      text: c.title || "",
      column: c.column,
      tags: c.tags || [],
      priority: c.priority || "",
      description: c.description || "",
      epic: c.epic || "",
      color: c.color || "",
      createdAt: c.created_at,
      updatedAt: c.updated_at,
    };
    allCards.push(ui);
    (cardsByCol[c.column] = cardsByCol[c.column] || []).push(ui);
  }
  return {
    title: server.project_name || "",
    version: server.version || "",
    lists: (server.columns || []).map((name) => ({
      id: name,
      title: String(name).toUpperCase(),
      done: doneSet.has(name),
      cards: cardsByCol[name] || [],
    })),
    priorities: server.priorities || [],
    priorityColors: server.priority_colors || {},
    doneColumns,
    epics: buildEpicIndex(allCards, doneSet),
  };
}

/* Epic index — built once per board load over the full payload, so a
   parent's counts report the board and never the active filter. An
   `epic` naming a card that is not in the payload resolves to no
   parent rather than throwing: validation forbids a dangling
   reference on disk, but a board rewritten between fetch and render
   can still produce one. */
function buildEpicIndex(cards, doneSet) {
  const byId = new Map();
  for (const c of cards) byId.set(c.id, c);

  const kids = new Map(); // parent id → children, in payload order
  for (const c of cards) {
    if (!c.epic || !byId.has(c.epic)) continue;
    const list = kids.get(c.epic);
    if (list) list.push(c);
    else kids.set(c.epic, [c]);
  }

  const progress = new Map(); // parent id → { done, total }
  for (const [id, children] of kids) {
    let done = 0;
    for (const c of children) if (doneSet.has(c.column)) done++;
    progress.set(id, { done, total: children.length });
  }

  // The epics themselves, in payload order — walked over `cards` rather
  // than over `kids`, whose insertion order is the order each parent's
  // *first child* appears, not the parent's own position. A card
  // carrying a color but referenced by nobody is not an epic and stays
  // out.
  const all = cards.filter((c) => kids.has(c.id));

  return {
    parentOf: (card) => (card && card.epic ? byId.get(card.epic) || null : null),
    childrenOf: (id) => kids.get(id) || [],
    progressOf: (id) => progress.get(id) || { done: 0, total: 0 },
    isEpic: (id) => kids.has(id),
    all: () => all,
  };
}

/* An empty index keeps components total while the board is loading or
   when an optimistic update rebuilds a list without one. */
const EMPTY_EPIC_INDEX = buildEpicIndex([], new Set());

/* =========================================================
   REST client — all mutations call existing endpoints.
   On non-2xx, throws; callers refetch via fetchBoard().
========================================================= */
async function apiGet(path) {
  const r = await fetch(path, { headers: { Accept: "application/json" } });
  if (!r.ok) throw new Error(`${r.status} ${r.statusText}`);
  return r.json();
}
async function apiSend(method, path, body) {
  const r = await fetch(path, {
    method,
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: body == null ? undefined : JSON.stringify(body),
  });
  if (!r.ok) {
    /* The server's error envelope is the message a user reads —
       "no card on this board carries that id" — so it travels on the
       Error rather than being stringified into it. `expected` marks a
       refusal the UI is equipped to display: a 4xx that arrived in the
       documented shape. Anything else is a bug or an outage and still
       belongs in the console. */
    let envelope = null;
    try { envelope = (await r.json()).error || null; } catch (_) {}
    const err = new Error(
      envelope && envelope.message
        ? envelope.message
        : `${method} ${path} → ${r.status}`);
    err.status = r.status;
    err.envelope = envelope;
    err.expected = r.status >= 400 && r.status < 500 && !!envelope;
    throw err;
  }
  if (r.status === 204) return null;
  const ct = r.headers.get("content-type") || "";
  return ct.includes("application/json") ? r.json() : null;
}

/* Hand a failure to whoever asked to display it, and log only what
   nothing else records. A handled 4xx that also reached console.error
   is noise — and the browser tests fail any page that logs one, so an
   asserted refusal would fail on the error it is asserting. */
function reportFailure(e, onFail) {
  if (onFail) onFail(e && e.message ? e.message : String(e));
  if (!(e && e.expected)) console.error(e);
}

/* =========================================================
   Filter helpers
========================================================= */
const DEFAULT_FILTER = {
  query: "",
  inTitle: true,
  inDescription: true,
  inTags: true,
  inId: true,
  priorities: [], // empty = all pass
  epics: [],      // parent card ids; NO_EPIC for "belongs to none"
};

/* The `No epic` pseudo-scope rides in the same array as the real epic
   ids. A card id is always six characters, so the empty string can
   never collide with one — no second field, no fourth state. */
const NO_EPIC = "";

/* The query, the priority set, and the epic set are independent
   dimensions: a card must satisfy every dimension that is set. */
function filterIsActive(f) {
  return (f.query && f.query.trim().length > 0) ||
    (f.priorities && f.priorities.length > 0) ||
    (f.epics && f.epics.length > 0);
}

function matchCard(card, f, epics) {
  if (f.priorities && f.priorities.length > 0) {
    const p = card.priority || "";
    if (!f.priorities.includes(p)) return false;
  }
  if (f.epics && f.epics.length > 0 && !matchesEpicScope(card, f.epics, epics)) return false;
  const q = (f.query || "").trim().toLowerCase();
  if (!q) return true;
  if (!f.inTitle && !f.inDescription && !f.inTags && !f.inId) return false;
  if (f.inTitle && (card.text || "").toLowerCase().includes(q)) return true;
  if (f.inDescription && (card.description || "").toLowerCase().includes(q)) return true;
  if (f.inTags && (card.tags || []).some((t) => String(t).toLowerCase().includes(q))) return true;
  if (f.inId && (card.id || "").toLowerCase().includes(q)) return true;
  return false;
}

/* OR across the selected ids. A card passes an id either by belonging
   to it or by BEING it: an epic's parent carries no `epic` field, so
   matching on `card.epic` alone would hide the parent — and with it
   the glyph, the tinted border and the progress bar that are the whole
   reason to focus an epic.

   NO_EPIC means "unrelated to any epic", which is stricter than
   "carries no epic": a parent card carries none either, and a card six
   others point at is the least epic-less card on the board. */
function matchesEpicScope(card, selected, epics) {
  for (const id of selected) {
    if (id === NO_EPIC) {
      if (!card.epic && !(epics && epics.isEpic(card.id))) return true;
      continue;
    }
    if (card.epic === id || card.id === id) return true;
  }
  return false;
}

/* =========================================================
   Icons
========================================================= */
const Icon = ({ d, size = 16, stroke = 1.6 }) =>
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={stroke} strokeLinecap="round" strokeLinejoin="round" style={{ width: size, height: size }}>{d}</svg>;
const IconPlus = (p) => <Icon {...p} d={<><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></>} />;
const IconFilter = (p) => <Icon {...p} d={<polygon points="3,5 21,5 14,13 14,20 10,18 10,13" />} />;
const IconSun = (p) => <Icon {...p} d={<><circle cx="12" cy="12" r="4" /><line x1="12" y1="2" x2="12" y2="4" /><line x1="12" y1="20" x2="12" y2="22" /><line x1="4.93" y1="4.93" x2="6.34" y2="6.34" /><line x1="17.66" y1="17.66" x2="19.07" y2="19.07" /><line x1="2" y1="12" x2="4" y2="12" /><line x1="20" y1="12" x2="22" y2="12" /><line x1="4.93" y1="19.07" x2="6.34" y2="17.66" /><line x1="17.66" y1="6.34" x2="19.07" y2="4.93" /></>} />;
const IconMoon = (p) => <Icon {...p} d={<path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />} />;
const IconMonitor = (p) => <Icon {...p} d={<><rect x="2" y="4" width="20" height="13" rx="2" /><line x1="8" y1="21" x2="16" y2="21" /><line x1="12" y1="17" x2="12" y2="21" /></>} />;
const IconClose = (p) => <Icon {...p} d={<><line x1="6" y1="6" x2="18" y2="18" /><line x1="6" y1="18" x2="18" y2="6" /></>} />;
const IconDots = (p) => <Icon {...p} d={<><circle cx="5" cy="12" r="1.2" /><circle cx="12" cy="12" r="1.2" /><circle cx="19" cy="12" r="1.2" /></>} stroke={2.4} />;
const IconCheck = (p) => <Icon {...p} d={<polyline points="20 6 9 17 4 12" />} />;
/* Four squares — one epic holding its children. Marks both the chip on
   a child card and the title of the parent it points at. */
const IconEpic = (p) => <Icon {...p} d={<><rect x="3" y="3" width="7.5" height="7.5" rx="1.5" /><rect x="13.5" y="3" width="7.5" height="7.5" rx="1.5" /><rect x="3" y="13.5" width="7.5" height="7.5" rx="1.5" /><rect x="13.5" y="13.5" width="7.5" height="7.5" rx="1.5" /></>} />;
const IconRefresh = (p) => <Icon {...p} d={<><polyline points="23 4 23 10 17 10" /><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10" /></>} />;

/* =========================================================
   Copyable ID — click to copy, brief "Copied" feedback.
   Used by the card list ID strip and the modal header ID.
========================================================= */
function CopyableId({ value, className }) {
  const [copied, setCopied] = useState(false);
  const timerRef = useRef(null);
  useEffect(() => () => { if (timerRef.current) clearTimeout(timerRef.current); }, []);
  const onClick = (e) => {
    e.stopPropagation();
    const txt = String(value || "");
    const finish = () => {
      setCopied(true);
      if (timerRef.current) clearTimeout(timerRef.current);
      timerRef.current = setTimeout(() => setCopied(false), 1100);
    };
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(txt).then(finish).catch(finish);
    } else {
      // Fallback: hidden textarea + execCommand.
      const ta = document.createElement("textarea");
      ta.value = txt;
      ta.setAttribute("readonly", "");
      ta.style.position = "fixed";
      ta.style.opacity = "0";
      document.body.appendChild(ta);
      ta.select();
      try { document.execCommand("copy"); } catch (_) {}
      document.body.removeChild(ta);
      finish();
    }
  };
  return (
    <button
      type="button"
      className={(className || "") + (copied ? " copied" : "")}
      onClick={onClick}
      title={copied ? "Copied" : "Click to copy ID"}>
      {copied ? "Copied" : value}
    </button>);
}

/* =========================================================
   Theme management
========================================================= */
function useTheme() {
  const [pref, setPref] = useState(() => localStorage.getItem("kanban.theme") || "system");
  const [systemDark, setSystemDark] = useState(() =>
    window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches
  );

  useEffect(() => {
    if (!window.matchMedia) return;
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const handler = (e) => setSystemDark(e.matches);
    mq.addEventListener("change", handler);
    return () => mq.removeEventListener("change", handler);
  }, []);

  const isDark = pref === "dark" || (pref === "system" && systemDark);

  useEffect(() => {
    document.documentElement.dataset.theme = isDark ? "dark" : "light";
  }, [isDark]);

  useEffect(() => {
    localStorage.setItem("kanban.theme", pref);
  }, [pref]);

  return { pref, setPref, isDark };
}

/* =========================================================
   App
========================================================= */
function App() {
  const theme = useTheme();

  const [board, setBoard] = useState(null);
  const [loadError, setLoadError] = useState(null);
  const [filter, setFilter] = useState(DEFAULT_FILTER);
  const [filterOpen, setFilterOpen] = useState(false);
  const [openCardId, setOpenCardId] = useState(null);
  const [sseStatus, setSseStatus] = useState("connecting"); // 'connecting'|'online'|'offline'
  const [refreshing, setRefreshing] = useState(false);
  const refreshingRef = useRef(false);
  const refetchTimer = useRef(null);

  const fetchBoard = useCallback(async () => {
    try {
      const data = await apiGet("/api/board");
      setBoard(toUiBoard(data));
      setLoadError(null);
    } catch (e) {
      console.error("fetch /api/board failed:", e);
      setLoadError(String(e));
    }
  }, []);

  /* The refresh button's in-flight guard. A ref (not the state) keeps
     the guard atomic against double-clicks within the same tick; the
     state drives the busy UI. Same load path, errors and re-render as
     the initial load and the SSE refetch. */
  const refreshBoard = useCallback(async () => {
    if (refreshingRef.current) return;
    refreshingRef.current = true;
    setRefreshing(true);
    try { await fetchBoard(); }
    finally {
      refreshingRef.current = false;
      setRefreshing(false);
    }
  }, [fetchBoard]);

  // Initial load
  useEffect(() => { fetchBoard(); }, [fetchBoard]);

  // SSE — refetch on board-changed (debounced)
  useEffect(() => {
    const es = new EventSource("/api/events");
    es.onopen = () => setSseStatus("online");
    es.onerror = () => setSseStatus("offline");
    const onChange = () => {
      if (refetchTimer.current) clearTimeout(refetchTimer.current);
      refetchTimer.current = setTimeout(() => { fetchBoard(); }, 50);
    };
    es.addEventListener("board-changed", onChange);
    es.addEventListener("message", onChange);
    return () => {
      if (refetchTimer.current) clearTimeout(refetchTimer.current);
      es.close();
    };
  }, [fetchBoard]);

  /* ---------- Mutations ---------- */
  const addCard = async (listId, text) => {
    const t = text.trim();
    if (!t) return;
    try {
      await apiSend("POST", "/api/cards", { column: listId, title: t });
    } catch (e) { console.error(e); }
    fetchBoard();
  };

  /* onFail, when given, receives the server's sentence so a caller
     with somewhere to show it can. Callers that pass nothing keep the
     previous behaviour exactly: the failure is logged and the refetch
     puts the field back. */
  const patchCard = async (cardId, patch, onFail) => {
    // Translate UI keys → server keys. The empty string is the clear
    // for `epic` and `color`, which is what the server understands.
    const body = {};
    if ("text" in patch) body.title = patch.text;
    if ("description" in patch) body.description = patch.description;
    if ("priority" in patch) body.priority = patch.priority;
    if ("tags" in patch) body.tags = patch.tags;
    if ("epic" in patch) body.epic = patch.epic;
    if ("color" in patch) body.color = patch.color;
    try { await apiSend("PATCH", `/api/cards/${encodeURIComponent(cardId)}`, body); }
    catch (e) { reportFailure(e, onFail); }
    fetchBoard();
  };

  const removeCard = async (cardId, onFail) => {
    try { await apiSend("DELETE", `/api/cards/${encodeURIComponent(cardId)}`); }
    catch (e) { reportFailure(e, onFail); }
    fetchBoard();
  };

  const toggleTag = async (cardId, tag, onFail) => {
    if (!board) return;
    const card = board.lists.flatMap((l) => l.cards).find((c) => c.id === cardId);
    if (!card) return;
    const tags = card.tags.includes(tag) ? card.tags.filter((t) => t !== tag) : [...card.tags, tag];
    try { await apiSend("PATCH", `/api/cards/${encodeURIComponent(cardId)}`, { tags }); }
    catch (e) { reportFailure(e, onFail); }
    fetchBoard();
  };

  const moveCard = async (fromListId, cardId, toListId, toIndex, onFail) => {
    // Optimistic local update so the UI is snappy.
    setBoard((b) => {
      if (!b) return b;
      const newLists = b.lists.map((l) => ({ ...l, cards: [...l.cards] }));
      const from = newLists.find((l) => l.id === fromListId);
      const to = newLists.find((l) => l.id === toListId);
      if (!from || !to) return b;
      const idx = from.cards.findIndex((c) => c.id === cardId);
      if (idx < 0) return b;
      const [card] = from.cards.splice(idx, 1);
      let insertAt = toIndex;
      if (from === to && idx < toIndex) insertAt -= 1;
      if (insertAt < 0) insertAt = 0;
      if (insertAt > to.cards.length) insertAt = to.cards.length;
      to.cards.splice(insertAt, 0, card);
      return { ...b, lists: newLists };
    });
    let serverIdx = toIndex;
    if (fromListId === toListId) {
      const list = board?.lists.find((l) => l.id === fromListId);
      if (list) {
        const cur = list.cards.findIndex((c) => c.id === cardId);
        if (cur >= 0 && cur < toIndex) serverIdx -= 1;
      }
    }
    try {
      await apiSend("POST", `/api/cards/${encodeURIComponent(cardId)}/move`, {
        column: toListId, position: serverIdx,
      });
    } catch (e) { reportFailure(e, onFail); fetchBoard(); }
  };

  const addList = async (title) => {
    const name = title.trim();
    if (!name) return;
    try { await apiSend("POST", "/api/columns", { name }); }
    catch (e) { alert(`Cannot add list: ${e.message}`); }
    fetchBoard();
  };

  // patch carries whichever of `name` / `done` the caller changed; the
  // caller has already decided there is something to send.
  const patchList = async (from, patch) => {
    if (!patch || !Object.keys(patch).length) return;
    try { await apiSend("PATCH", `/api/columns/${encodeURIComponent(from)}`, patch); }
    catch (e) { alert(`Cannot update list: ${e.message}`); fetchBoard(); }
  };

  const removeList = async (name) => {
    try { await apiSend("DELETE", `/api/columns/${encodeURIComponent(name)}`); }
    catch (e) { alert(`Cannot delete list: ${e.message}`); }
    fetchBoard();
  };

  const moveList = async (fromIdx, toIdx) => {
    if (!board) return;
    const name = board.lists[fromIdx]?.id;
    if (!name) return;
    setBoard((b) => {
      if (!b) return b;
      const lists = [...b.lists];
      const [m] = lists.splice(fromIdx, 1);
      lists.splice(toIdx, 0, m);
      return { ...b, lists };
    });
    try { await apiSend("POST", "/api/columns/move", { name, position: toIdx }); }
    catch (e) { console.error(e); fetchBoard(); }
  };

  // Clicking a card's epic chip toggles that epic's scope — adding to
  // the filter rather than replacing it, like the tag chip.
  const focusEpic = (id) => setFilter((f) => {
    const set = new Set(f.epics || []);
    if (set.has(id)) set.delete(id); else set.add(id);
    return { ...f, epics: Array.from(set) };
  });

  const filterActive = filterIsActive(filter);
  const filteredCount = useMemo(() => {
    if (!board || !filterActive) return 0;
    return board.lists.reduce(
      (acc, l) => acc + l.cards.filter((c) => matchCard(c, filter, board.epics)).length,
      0
    );
  }, [board, filter, filterActive]);

  if (!board) {
    return (
      <div className="app loading-screen">
        <div className="bg-layers" aria-hidden="true">
          <div className="bg-base" />
          <div className="bg-grain" />
        </div>
        <div className="loading t-body-md" style={{ color: "var(--text-muted)", padding: 28 }}>
          {loadError ? "Could not load board. Retrying via SSE…" : "Loading…"}
        </div>
      </div>
    );
  }

  return (
    <div className="app" data-screen-label="Kanban Board">
      <div className="bg-layers" aria-hidden="true">
        <div className="bg-base" />
        <div className="bg-grain" />
        <div className="bg-topshade" />
      </div>

      <TopBar
        title={board.title}
        version={board.version}
        filter={filter}
        onFilterChange={setFilter}
        filterActive={filterActive}
        filterOpen={filterOpen}
        setFilterOpen={setFilterOpen}
        filteredCount={filteredCount}
        priorities={board.priorities}
        priorityColors={board.priorityColors}
        epics={board.epics}
        theme={theme}
        sseStatus={sseStatus}
        onRefresh={refreshBoard}
        refreshing={refreshing}
      />

      <Board
        board={board}
        filter={filter}
        filterActive={filterActive}
        priorityColors={board.priorityColors}
        epics={board.epics}
        onAddList={addList}
        onPatchList={patchList}
        onRemoveList={removeList}
        onAddCard={addCard}
        onRemoveCard={removeCard}
        onToggleTag={toggleTag}
        onMoveCard={moveCard}
        onMoveList={moveList}
        onOpenCard={(cardId) => setOpenCardId(cardId)}
        onFocusEpic={focusEpic}
      />

      {openCardId && (() => {
        const list = board.lists.find((l) => l.cards.some((c) => c.id === openCardId));
        const card = list?.cards.find((c) => c.id === openCardId);
        if (!card) { setOpenCardId(null); return null; }
        return (
          <CardDetailModal
            card={card}
            list={list}
            allLists={board.lists}
            priorities={board.priorities}
            priorityColors={board.priorityColors}
            epics={board.epics}
            onClose={() => setOpenCardId(null)}
            onPatch={(patch, onFail) => patchCard(card.id, patch, onFail)}
            onPatchCard={(cardId, patch, onFail) => patchCard(cardId, patch, onFail)}
            onMoveColumn={(toListId, onFail) => moveCard(list.id, card.id, toListId, board.lists.find((l) => l.id === toListId).cards.length, onFail)}
            onToggleTag={(tag, onFail) => toggleTag(card.id, tag, onFail)}
            onRemove={() => { removeCard(card.id); setOpenCardId(null); }}
          />
        );
      })()}
    </div>
  );
}

/* =========================================================
   TopBar
========================================================= */
function TopBar({ title, version, filter, onFilterChange, filterActive, filterOpen, setFilterOpen, filteredCount, priorities, priorityColors, epics, theme, sseStatus, onRefresh, refreshing }) {
  const popRef = useRef(null);
  useClickOutside(popRef, () => setFilterOpen(false), filterOpen);

  const toggleScope = (key) => onFilterChange({ ...filter, [key]: !filter[key] });
  const togglePriority = (id) => {
    const set = new Set(filter.priorities || []);
    if (set.has(id)) set.delete(id); else set.add(id);
    onFilterChange({ ...filter, priorities: Array.from(set) });
  };
  const toggleEpic = (id) => {
    const set = new Set(filter.epics || []);
    if (set.has(id)) set.delete(id); else set.add(id);
    onFilterChange({ ...filter, epics: Array.from(set) });
  };
  // Payload order, straight from the adapter: the same order the modal
  // lists children in, and the only order the board gives them.
  const epicList = (epics || EMPTY_EPIC_INDEX).all();
  const selectedEpics = filter.epics || [];
  const clearFilter = () => onFilterChange({ ...DEFAULT_FILTER });

  return (
    <header className="topbar">
      <div className="topbar-left">
        <span className="brand">{(title || "").toUpperCase()}</span>
      </div>

      <div className="topbar-right">
        <div ref={popRef} style={{ position: "relative" }}>
          <button
            className={"iconbtn" + (filterActive ? " active" : "")}
            onClick={() => setFilterOpen((v) => !v)}
            aria-label="Filter">
            <IconFilter />
            <span className="iconbtn-label">Filter</span>
            {filterActive && <span className="iconbtn-badge">{filteredCount}</span>}
          </button>
          {filterOpen &&
            <div className="popover filter-popover" role="dialog">
              <p className="popover-title">Filter cards</p>
              <input
                className="filter-input"
                placeholder="Search…"
                value={filter.query}
                onChange={(e) => onFilterChange({ ...filter, query: e.target.value })}
                autoFocus />
              <p className="popover-sub">Search in</p>
              <div className="filter-pills">
                <button type="button" className={"filter-pill" + (filter.inTitle ? " on" : "")} onClick={() => toggleScope("inTitle")} aria-pressed={filter.inTitle}>Title</button>
                <button type="button" className={"filter-pill" + (filter.inDescription ? " on" : "")} onClick={() => toggleScope("inDescription")} aria-pressed={filter.inDescription}>Description</button>
                <button type="button" className={"filter-pill" + (filter.inTags ? " on" : "")} onClick={() => toggleScope("inTags")} aria-pressed={filter.inTags}>Tags</button>
                <button type="button" className={"filter-pill" + (filter.inId ? " on" : "")} onClick={() => toggleScope("inId")} aria-pressed={filter.inId}>ID</button>
              </div>
              {priorities.length > 0 && <>
                <p className="popover-sub">Priority</p>
                <div className="filter-pills">
                  {priorities.map((p) => {
                    const on = (filter.priorities || []).includes(p);
                    return (
                      <button
                        key={p}
                        type="button"
                        className={"filter-pill filter-pill-prio" + (on ? " on" : "")}
                        onClick={() => togglePriority(p)}
                        aria-pressed={on}>
                        <span className="filter-pill-dot" style={{ background: priorityColors[p] || "var(--text-muted)" }} aria-hidden="true" />
                        {p[0].toUpperCase() + p.slice(1)}
                      </button>);
                  })}
                </div>
              </>}
              {epicList.length > 0 && <>
                <p className="popover-sub">Epic</p>
                <div className="filter-pills">
                  {epicList.map((e) => {
                    const on = selectedEpics.includes(e.id);
                    return (
                      <button
                        key={e.id}
                        type="button"
                        className={"filter-pill filter-pill-epic" + (on ? " on" : "")}
                        onClick={() => toggleEpic(e.id)}
                        aria-pressed={on}
                        title={e.text}>
                        <span className="filter-pill-dot" style={{ background: e.color || "var(--text-muted)" }} aria-hidden="true" />
                        <span className="filter-pill-label">{e.text}</span>
                      </button>);
                  })}
                  {(() => {
                    const on = selectedEpics.includes(NO_EPIC);
                    return (
                      <button
                        type="button"
                        className={"filter-pill" + (on ? " on" : "")}
                        onClick={() => toggleEpic(NO_EPIC)}
                        aria-pressed={on}>
                        No epic
                      </button>);
                  })()}
                </div>
              </>}
              {filterActive &&
                <button className="filter-clear" onClick={clearFilter}>
                  <IconClose size={12} /> Clear all
                </button>}
            </div>}
        </div>

        <span className="topbar-divider" />

        <ThemeToggle theme={theme} />

        <ServerStatus status={sseStatus} version={version} />

        <button
          className={"iconbtn" + (refreshing ? " refreshing" : "")}
          onClick={onRefresh}
          disabled={refreshing}
          aria-label="Refresh"
          title="Refresh">
          <IconRefresh />
          <span className="iconbtn-label">Refresh</span>
        </button>
      </div>
    </header>);
}

function ServerStatus({ status, version }) {
  const [open, setOpen] = useState(false);
  const popRef = useRef(null);
  useClickOutside(popRef, () => setOpen(false), open);

  // Escape closes the overlay, matching every other popover on the page.
  useEffect(() => {
    if (!open) return;
    const onKey = (e) => { if (e.key === "Escape") setOpen(false); };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open]);

  const statusMeta = {
    connecting: { label: "Connecting…", color: "oklch(0.72 0.13 85)" },
    online:     { label: "Online",      color: "oklch(0.68 0.16 145)" },
    offline:    { label: "Offline",     color: "oklch(0.6 0.2 25)" },
  }[status] || { label: status, color: "var(--text-muted)" };

  return (
    <div ref={popRef} className="server-status" data-status={status}>
      <button
        className="server-dot-btn"
        onClick={() => setOpen((v) => !v)}
        aria-label={"Server status: " + statusMeta.label}
        title={statusMeta.label}>
        <span className="server-dot" style={{ background: statusMeta.color, "--dot-color": statusMeta.color }} />
      </button>
      {open &&
        <div className="popover server-popover" role="dialog">
          <div className="server-row">
            <span className="server-row-label">Status</span>
            <span className="server-row-value">
              <span className="server-dot-sm" style={{ background: statusMeta.color }} />
              {statusMeta.label}
            </span>
          </div>
          <div className="server-row">
            <span className="server-row-label">Storage</span>
            <span className="server-row-value mono">kanban.toml</span>
          </div>
          <div className="server-row">
            <span className="server-row-label">Version</span>
            <span className="server-row-value mono">{version || "dev"}</span>
          </div>
        </div>}
    </div>);
}

function ThemeToggle({ theme }) {
  const opts = [
    { id: "light", Icon: IconSun, label: "Light" },
    { id: "system", Icon: IconMonitor, label: "System" },
    { id: "dark", Icon: IconMoon, label: "Dark" },
  ];
  return (
    <div className="theme-toggle" role="group" aria-label="Theme">
      {opts.map((o) =>
        <button
          key={o.id}
          className={theme.pref === o.id ? "active" : ""}
          onClick={() => theme.setPref(o.id)}
          aria-label={o.label}
          title={o.label}>
          <o.Icon />
        </button>)}
    </div>);
}

/* =========================================================
   Board
========================================================= */
function Board({ board, filter, filterActive, priorityColors, epics, onAddList, onPatchList, onRemoveList, onAddCard, onRemoveCard, onToggleTag, onMoveCard, onMoveList, onOpenCard, onFocusEpic }) {
  const [addingList, setAddingList] = useState(false);
  const dragRef = useRef({ kind: null, cardId: null, fromListId: null, listIdx: null });
  const wrapRef = useRef(null);

  useEffect(() => {
    const wrap = wrapRef.current;
    if (!wrap) return;
    let dragging = false;
    let startX = 0;
    let startScroll = 0;
    let moved = false;
    const onMouseDown = (e) => {
      if (e.button !== 0) return;
      const t = e.target;
      if (t.closest('input, textarea, button, a, .card, .list-header, .card-composer, .add-list-composer, [draggable="true"]')) return;
      dragging = true;
      moved = false;
      startX = e.pageX;
      startScroll = wrap.scrollLeft;
      wrap.classList.add('dragging-scroll');
    };
    const onMouseMove = (e) => {
      if (!dragging) return;
      const dx = e.pageX - startX;
      if (Math.abs(dx) > 3) moved = true;
      wrap.scrollLeft = startScroll - dx;
    };
    const onMouseUp = () => {
      if (!dragging) return;
      dragging = false;
      requestAnimationFrame(() => wrap.classList.remove('dragging-scroll'));
      moved = false;
    };
    wrap.addEventListener('mousedown', onMouseDown);
    window.addEventListener('mousemove', onMouseMove);
    window.addEventListener('mouseup', onMouseUp);
    return () => {
      wrap.removeEventListener('mousedown', onMouseDown);
      window.removeEventListener('mousemove', onMouseMove);
      window.removeEventListener('mouseup', onMouseUp);
    };
  }, []);

  return (
    <main className="board-wrap" ref={wrapRef} style={{ padding: "28px" }}>
      <div className="board">
        {board.lists.map((list, idx) =>
          <ListColumn
            key={list.id}
            list={list}
            index={idx}
            filter={filter}
            filterActive={filterActive}
            priorityColors={priorityColors}
            epics={epics}
            dragRef={dragRef}
            onPatch={(patch) => onPatchList(list.id, patch)}
            onRemove={() => onRemoveList(list.id)}
            onAddCard={(t) => onAddCard(list.id, t)}
            onRemoveCard={(cid) => onRemoveCard(cid)}
            onToggleTag={(cid, tag) => onToggleTag(cid, tag)}
            onMoveCard={onMoveCard}
            onMoveList={onMoveList}
            onOpenCard={(cid) => onOpenCard(cid)}
            onFocusEpic={onFocusEpic}
          />)}

        {addingList ?
          <AddListComposer
            onAdd={(t) => { onAddList(t); setAddingList(false); }}
            onCancel={() => setAddingList(false)} /> :
          <button className="add-list" onClick={() => setAddingList(true)}>
            <IconPlus /> Add a list
          </button>}
      </div>
    </main>);
}

/* =========================================================
   List column
========================================================= */
function ListColumn({ list, index, filter, filterActive, priorityColors, epics, dragRef, onPatch, onRemove, onAddCard, onRemoveCard, onToggleTag, onMoveCard, onMoveList, onOpenCard, onFocusEpic }) {
  const [adding, setAdding] = useState(false);
  const [isOver, setIsOver] = useState(false);
  const [draggingSelf, setDraggingSelf] = useState(false);
  // null while the header is not being renamed; the staged terminal
  // value once it is.
  const [stagedDone, setStagedDone] = useState(null);
  const counterRef = useRef(0);

  // One request carries both halves of the edit, so the name can never
  // land without the marker it was committed with. Only the keys the
  // user actually moved are sent — a commit must not assert a value
  // they never touched.
  const commitHeaderEdit = (name) => {
    const done = stagedDone;
    setStagedDone(null);
    // An empty name leaves the field in an invalid state; there is no
    // committing half of it, so the staged marker goes with it.
    if (name === null) return;
    const patch = {};
    if (name !== list.id) patch.name = name;
    if (done !== null && done !== list.done) patch.done = done;
    if (Object.keys(patch).length) onPatch(patch);
  };

  useEffect(() => {
    const onHover = (e) => setIsOver(e.detail === list.id);
    const reset = () => { counterRef.current = 0; setIsOver(false); };
    window.addEventListener("kanban:hover-list", onHover);
    window.addEventListener("dragend", reset, true);
    window.addEventListener("drop", reset, true);
    window.addEventListener("kanban:drag-cleanup", reset);
    return () => {
      window.removeEventListener("kanban:hover-list", onHover);
      window.removeEventListener("dragend", reset, true);
      window.removeEventListener("drop", reset, true);
      window.removeEventListener("kanban:drag-cleanup", reset);
    };
  }, [list.id]);

  const visibleCards = useMemo(() => {
    if (!filterActive) return list.cards;
    return list.cards.filter((c) => matchCard(c, filter, epics));
  }, [list.cards, filter, filterActive, epics]);

  const onDragOver = (e) => {
    if (dragRef.current.kind === "card") {
      e.preventDefault();
      e.dataTransfer.dropEffect = "move";
      window.dispatchEvent(new CustomEvent("kanban:hover-list", { detail: list.id }));
    } else if (dragRef.current.kind === "list") {
      e.preventDefault();
    }
  };
  const onDrop = (e) => {
    e.preventDefault();
    counterRef.current = 0;
    setIsOver(false);
    if (dragRef.current.kind === "card") {
      const { cardId, fromListId } = dragRef.current;
      onMoveCard(fromListId, cardId, list.id, 0);
      dragRef.current = { kind: null };
    } else if (dragRef.current.kind === "list") {
      onMoveList(dragRef.current.listIdx, index);
      dragRef.current = { kind: null };
    }
  };

  const onListDragStart = (e) => {
    dragRef.current = { kind: "list", listIdx: index };
    e.dataTransfer.effectAllowed = "move";
    e.dataTransfer.setData("text/plain", "list:" + list.id);
    setTimeout(() => setDraggingSelf(true), 0);
  };
  const onListDragEnd = () => {
    setDraggingSelf(false);
    dragRef.current = { kind: null };
  };

  return (
    <section
      className={"list" + (isOver ? " drop-target" : "") + (draggingSelf ? " dragging" : "")}
      data-column={list.id}
      onDragOver={onDragOver}
      onDrop={onDrop}>
      <header
        className="list-header"
        draggable
        onDragStart={onListDragStart}
        onDragEnd={onListDragEnd}>
        <EditableText
          className="list-title"
          value={list.title}
          uppercase
          onOpen={() => setStagedDone(list.done)}
          onCancel={() => setStagedDone(null)}
          onCommit={commitHeaderEdit}
          accessory={
            <TerminalCheck
              checked={stagedDone === null ? list.done : stagedDone}
              columnName={list.id}
              onToggle={() => setStagedDone((v) => !(v === null ? list.done : v))} />
          } />
        {/* The resting mark stands down while the editor is open: the
            staged check reports the same fact, editably, right beside
            it. Two marks for one fact reads as two settings. */}
        {list.done && stagedDone === null &&
          <span
            className="list-done-mark"
            title="Cards in this column count as done"
            aria-label={`${list.title} is a terminal column`}
            role="img">
            <IconCheck size={12} />
          </span>}
        <span className="list-count" title={`${list.cards.length} cards`}>{list.cards.length}</span>
        <ListMenu
          done={list.done}
          onToggleDone={() => onPatch({ done: !list.done })}
          onRemove={onRemove} />
      </header>

      <div className={"cards" + (visibleCards.length === 0 && !adding ? " empty" : "")}>
        {visibleCards.length === 0 && !adding && filterActive &&
          <div className="list-empty">No matches</div>}
        {visibleCards.map((card, i) =>
          <CardItem
            key={card.id}
            card={card}
            listId={list.id}
            index={i}
            dragRef={dragRef}
            priorityColors={priorityColors}
            epics={epics}
            onRemove={() => onRemoveCard(card.id)}
            onToggleTag={(tag) => onToggleTag(card.id, tag)}
            onMoveCard={onMoveCard}
            onOpen={() => onOpenCard(card.id)}
            onFocusEpic={onFocusEpic} />)}
        {adding &&
          <CardComposer
            onAdd={(t) => { onAddCard(t); }}
            onClose={() => setAdding(false)} />}
      </div>

      {!adding &&
        <button className="add-card" onClick={() => setAdding(true)}>
          <IconPlus /> Add a card
        </button>}
    </section>);
}

/* useClickOutside is correct here: its bubble-phase document listener
   only fails inside the card modal, which stops mousedown at its own
   container. This menu is not in the modal. */
function ListMenu({ done, onToggleDone, onRemove }) {
  const [open, setOpen] = useState(false);
  const ref = useRef(null);
  useClickOutside(ref, () => setOpen(false), open);
  return (
    <div ref={ref} style={{ position: "relative" }}>
      <button className="list-menu" onClick={() => setOpen((v) => !v)} aria-label="More options">
        <IconDots />
      </button>
      {open &&
        <div className="popover" style={{ right: 0, top: "calc(100% + 4px)", minWidth: 200, padding: 6 }}>
          {/* No name to coordinate with, so nothing to stage: this
              writes `done` alone and never sends a `name` the client
              may already have stale. */}
          <button
            className="add-card menu-toggle"
            role="menuitemcheckbox"
            aria-checked={done}
            onClick={() => { onToggleDone(); setOpen(false); }}>
            <span className={"menu-check" + (done ? " on" : "")}><IconCheck size={12} /></span>
            Terminal column
          </button>
          <button
            className="add-card"
            style={{ color: "oklch(0.55 0.18 25)" }}
            onClick={() => { onRemove(); setOpen(false); }}>
            <IconClose /> Delete list
          </button>
        </div>}
    </div>);
}

/* =========================================================
   Card
========================================================= */
function CardItem({ card, listId, index, dragRef, priorityColors, epics, onRemove, onToggleTag, onMoveCard, onOpen, onFocusEpic }) {
  const [dragging, setDragging] = useState(false);
  const [dropPos, setDropPos] = useState(null);

  const onDragStart = (e) => {
    dragRef.current = { kind: "card", cardId: card.id, fromListId: listId };
    e.dataTransfer.effectAllowed = "move";
    e.dataTransfer.setData("text/plain", "card:" + card.id);
    document.body.classList.add("is-dragging");
    setTimeout(() => setDragging(true), 0);
  };
  const onDragEnd = () => {
    setDragging(false);
    setDropPos(null);
    dragRef.current = { kind: null };
    document.body.classList.remove("is-dragging");
    window.dispatchEvent(new Event("kanban:drag-cleanup"));
  };
  const onDragOver = (e) => {
    if (dragRef.current.kind !== "card") return;
    if (dragRef.current.cardId === card.id) return;
    e.preventDefault();
    e.stopPropagation();
    e.dataTransfer.dropEffect = "move";
    const rect = e.currentTarget.getBoundingClientRect();
    const midpoint = rect.top + rect.height / 2;
    setDropPos(e.clientY < midpoint ? "above" : "below");
  };
  const onDragLeave = () => setDropPos(null);
  const onDrop = (e) => {
    if (dragRef.current.kind !== "card") return;
    e.preventDefault();
    e.stopPropagation();
    const pos = dropPos;
    setDropPos(null);
    const { cardId, fromListId } = dragRef.current;
    if (cardId === card.id) return;
    const insertIdx = pos === "above" ? index : index + 1;
    onMoveCard(fromListId, cardId, listId, insertIdx);
    dragRef.current = { kind: null };
  };


  const prioColor = card.priority ? (priorityColors[card.priority] || "var(--text-muted)") : null;
  const hasDesc = !!(card.description && card.description.trim());
  const idx = epics || EMPTY_EPIC_INDEX;
  const parent = idx.parentOf(card);
  // The parent signals key off actually having children, not off
  // carrying a color: a card whose last child was reassigned keeps its
  // color and must render as an ordinary card again.
  const isEpic = idx.isEpic(card.id);
  const progress = isEpic ? idx.progressOf(card.id) : null;

  return (
    <article
      className={"card" + (isEpic ? " is-epic" : "") + (dragging ? " dragging" : "") + (dropPos ? " drop-" + dropPos : "")}
      style={isEpic ? { "--epic-color": card.color || "var(--text-muted)" } : undefined}
      data-card-id={card.id}
      draggable
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
      onDragOver={onDragOver}
      onDragLeave={onDragLeave}
      onDrop={onDrop}
      onClick={(e) => {
        if (e.target.closest('.card-tag-chip, .card-tag-add, .card-tag-input, .card-delete, .card-epic-chip')) return;
        onOpen?.();
      }}>
      <CopyableId className="card-id" value={card.id} />
      <div className="card-title">
        {isEpic && <span className="card-epic-glyph" aria-label="Epic"><IconEpic size={12} /></span>}
        {card.text}
      </div>
      {progress && <EpicProgress done={progress.done} total={progress.total} />}
      <div className="card-foot">
        {prioColor && <span className="card-prio-pill" style={{ background: prioColor }} aria-label={"Priority " + card.priority} />}
        {parent && <EpicChip card={parent} onFocus={onFocusEpic} />}
        {hasDesc &&
          <span className="card-desc-icon" title="This card has a description" aria-hidden="true">
            <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
              <line x1="4" y1="7" x2="16" y2="7" />
              <line x1="4" y1="12" x2="20" y2="12" />
              <line x1="4" y1="17" x2="14" y2="17" />
            </svg>
          </span>}
        <CardTags tags={card.tags || []} onToggle={onToggleTag} />
      </div>
      <button className="card-delete" onClick={(e) => { e.stopPropagation(); onRemove(); }} title="Delete">
        <IconClose />
      </button>
    </article>);
}

/* =========================================================
   Epic presentation — the chip a child wears, and the progress a
   parent reports. Both are inert: assigning or recoloring an epic is
   the CLI's job in this version.
========================================================= */
/* onFocus toggles this epic in the filter, mirroring the tag chip,
   which already edits the filter on click. Passed only on the board:
   the chip in the modal's parent row stays inert, because it sits over
   a board the user cannot see, so a scope set from it would have no
   observable effect until the modal closes. */
function EpicChip({ card, onFocus }) {
  const style = { "--epic-color": card.color || "var(--text-muted)" };
  const body = <><IconEpic size={10} /><span className="card-epic-text">{card.text}</span></>;
  if (!onFocus) {
    return <span className="card-epic-chip" style={style} title={card.text}>{body}</span>;
  }
  return (
    <button
      type="button"
      className="card-epic-chip is-clickable"
      style={style}
      title={`Focus ${card.text}`}
      onClick={(e) => { e.stopPropagation(); onFocus(card.id); }}>
      {body}
    </button>);
}

function EpicProgress({ done, total, className }) {
  const pct = total > 0 ? Math.round((done / total) * 100) : 0;
  return (
    <div className={"epic-progress" + (className ? " " + className : "")}>
      <div
        className="epic-bar"
        role="progressbar"
        aria-valuenow={done}
        aria-valuemin={0}
        aria-valuemax={total}
        aria-label={`${done} of ${total} children done`}>
        <span className="epic-bar-fill" style={{ width: pct + "%" }} />
      </div>
      <span className="epic-count">{done}/{total}</span>
    </div>);
}

/* The palette, mirrored from board.EpicPalette. Eight constants that
   change when the Go source changes is a smaller price than a second
   endpoint or a top-level `palette` array on every board fetch. Order
   is chromatic distance, not hue — see colors.go. */
const EPIC_PALETTE = [
  { name: "violet", hex: "#8b5cf6" },
  { name: "emerald", hex: "#10b981" },
  { name: "orange", hex: "#f97316" },
  { name: "blue", hex: "#3b82f6" },
  { name: "pink", hex: "#ec4899" },
  { name: "lime", hex: "#84cc16" },
  { name: "cyan", hex: "#06b6d4" },
  { name: "fuchsia", hex: "#d946ef" },
];

/* Card-search combobox — the first entity picker in Ezida, written to
   outlive epics: dependencies and linked files need the same control.

   It searches the board the client already holds. GET /api/board
   returns every card on every fetch, so a request here would be asking
   the server for an array we are holding.

   The candidate list is filtered by the caller, which knows which side
   of the relation is being filled: an epic target may have children, a
   candidate child may not. Filtering is a courtesy — the server stays
   the authority, and a refusal lands in the modal's error region. */
function EpicPicker({ candidates, columnTitle, placeholder, onPick, onClose }) {
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const boxRef = useRef(null);
  const inputRef = useRef(null);

  /* Not useClickOutside: that hook listens on document during the
     bubble phase, and the modal stops mousedown at its own container,
     so a click on any other part of the modal never reaches it. The
     capture phase runs top-down and arrives before anything can stop
     it. */
  useEffect(() => {
    const handler = (e) => {
      if (boxRef.current && !boxRef.current.contains(e.target)) onClose();
    };
    document.addEventListener("mousedown", handler, true);
    return () => document.removeEventListener("mousedown", handler, true);
  }, [onClose]);
  useEffect(() => { inputRef.current?.focus(); }, []);

  const matches = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return candidates;
    return candidates.filter((c) =>
      (c.text || "").toLowerCase().includes(q) || c.id.toLowerCase().startsWith(q));
  }, [candidates, query]);

  // A shrinking result set must never leave the highlight past the end.
  const at = matches.length ? Math.min(active, matches.length - 1) : 0;
  const activeOptionId = matches.length ? `epic-option-${matches[at].id}` : undefined;

  const onKeyDown = (e) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActive(matches.length ? Math.min(at + 1, matches.length - 1) : 0);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive(Math.max(at - 1, 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      if (matches[at]) onPick(matches[at].id);
    } else if (e.key === "Escape") {
      // The modal closes on Escape from a document-level listener, so
      // the native event must stop here or abandoning a search closes
      // the whole modal.
      e.preventDefault();
      e.stopPropagation();
      onClose();
    }
  };

  return (
    <div className="epic-picker" ref={boxRef}>
      <input
        ref={inputRef}
        className="epic-picker-input"
        type="text"
        role="combobox"
        aria-expanded="true"
        aria-controls="epic-picker-list"
        aria-autocomplete="list"
        aria-activedescendant={activeOptionId}
        placeholder={placeholder}
        value={query}
        onChange={(e) => { setQuery(e.target.value); setActive(0); }}
        onKeyDown={onKeyDown} />
      <ul className="modal-dropdown epic-picker-list" id="epic-picker-list" role="listbox">
        {matches.length === 0 ?
          <li className="epic-picker-empty" role="presentation">No card matches</li> :
          matches.map((c, i) =>
            <li key={c.id} id={`epic-option-${c.id}`} role="option" aria-selected={i === at}>
              <button
                type="button"
                className={"modal-dropdown-item epic-picker-item" + (i === at ? " active" : "")}
                onMouseEnter={() => setActive(i)}
                onClick={() => onPick(c.id)}>
                <span className="epic-picker-title" title={c.text}>{c.text}</span>
                <span className="epic-picker-meta">
                  <span className="epic-picker-id">{c.id}</span>
                  <span className="epic-picker-col">{columnTitle(c.column)}</span>
                </span>
              </button>
            </li>)}
      </ul>
    </div>);
}

function CardTags({ tags, onToggle }) {
  const [adding, setAdding] = useState(false);
  const [draft, setDraft] = useState("");
  const inputRef = useRef(null);

  useEffect(() => { if (adding) inputRef.current?.focus(); }, [adding]);

  const commit = () => {
    const t = draft.trim().slice(0, 15);
    if (t && !tags.includes(t)) onToggle(t);
    setDraft("");
    setAdding(false);
  };

  const hasTags = tags.length > 0;

  return (
    <div className="card-tags">
      {tags.map((t) =>
        <button key={t} className="card-tag-chip" title="Remove this tag" onClick={(e) => { e.stopPropagation(); onToggle(t); }}>
          <span className="card-tag-text">{t}</span>
          <IconClose size={9} />
        </button>)}
      {adding ?
        <input
          ref={inputRef}
          className="card-tag-input"
          value={draft}
          maxLength={15}
          placeholder="tag…"
          onChange={(e) => setDraft(e.target.value.slice(0, 15))}
          onClick={(e) => e.stopPropagation()}
          onKeyDown={(e) => {
            e.stopPropagation();
            if (e.key === "Enter") { e.preventDefault(); commit(); }
            if (e.key === "Escape") { setDraft(""); setAdding(false); }
          }}
          onBlur={commit} /> :
        <button
          className={"card-tag-add" + (hasTags ? "" : " standalone")}
          title="Add a tag"
          onClick={(e) => { e.stopPropagation(); setAdding(true); }}>
          <IconPlus size={10} />
        </button>}
    </div>);
}

/* =========================================================
   Composers
========================================================= */
/* Only ever composes a new card now: the edit-an-existing-title caller
   went with the dead double-click, taking `initial` and `submitLabel`
   with it. */
function CardComposer({ onAdd, onClose }) {
  const [text, setText] = useState("");
  const ref = useRef(null);
  useEffect(() => {
    ref.current?.focus();
    ref.current?.setSelectionRange(text.length, text.length);
    autoSize(ref.current);
  }, []);
  const submit = () => {
    if (!text.trim()) { onClose(); return; }
    onAdd(text);
    setText("");
  };
  return (
    <div className="card-composer">
      <textarea
        ref={ref}
        value={text}
        placeholder="Enter card title…"
        onChange={(e) => { setText(e.target.value); autoSize(e.target); }}
        onKeyDown={(e) => {
          if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); submit(); }
          if (e.key === "Escape") onClose();
        }}
        onBlur={() => {
          setTimeout(() => {
            if (!ref.current) return;
            if (!ref.current.parentElement.contains(document.activeElement)) onClose();
          }, 100);
        }} />
      <div className="composer-actions">
        <button className="btn-primary" onMouseDown={(e) => e.preventDefault()} onClick={submit}>Add</button>
        <button className="btn-ghost" onMouseDown={(e) => e.preventDefault()} onClick={onClose}>Cancel</button>
      </div>
    </div>);
}

function autoSize(el) {
  if (!el) return;
  el.style.height = "auto";
  el.style.height = el.scrollHeight + "px";
}

function AddListComposer({ onAdd, onCancel }) {
  const [title, setTitle] = useState("");
  const ref = useRef(null);
  useEffect(() => { ref.current?.focus(); }, []);
  const submit = () => {
    if (!title.trim()) { onCancel(); return; }
    onAdd(title.trim());
    setTitle("");
  };
  return (
    <div className="add-list-composer">
      <input
        ref={ref}
        value={title}
        placeholder="List name"
        onChange={(e) => setTitle(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") { e.preventDefault(); submit(); }
          if (e.key === "Escape") onCancel();
        }} />
      <div className="composer-actions">
        <button className="btn-primary" onMouseDown={(e) => e.preventDefault()} onClick={submit}>Add</button>
        <button className="btn-ghost" onMouseDown={(e) => e.preventDefault()} onClick={onCancel}>Cancel</button>
      </div>
    </div>);
}

/* =========================================================
   Editable text — reports the end of an inline edit
========================================================= */
/* EditableText reports the end of an edit; it does not decide whether
   that edit is worth a request. The list header commits a name and a
   terminal marker together, and only the caller knows both values —
   an editor that reverted on an unchanged name would silently drop a
   staged marker.

   onCommit receives the trimmed, lowercased name, or null when the
   field was left empty. onCancel fires on Escape. onOpen fires when
   the input appears, so the caller can seed whatever it stages
   alongside. `accessory` renders beside the input while editing. */
function EditableText({ value, onCommit, onCancel, onOpen, accessory, className, placeholder, uppercase }) {
  const ref = useRef(null);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);

  useEffect(() => { setDraft(value); }, [value]);

  const open = () => { setEditing(true); if (onOpen) onOpen(); };

  if (editing) {
    const input = (
      <input
        ref={ref}
        className={className}
        value={draft}
        onChange={(e) => setDraft(uppercase ? e.target.value.toUpperCase() : e.target.value)}
        onBlur={() => {
          const t = (draft || "").trim();
          setEditing(false);
          onCommit(t ? t.toLowerCase() : null);
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter") { e.preventDefault(); e.currentTarget.blur(); }
          if (e.key === "Escape") {
            setDraft(value);
            setEditing(false);
            if (onCancel) onCancel();
          }
        }}
        autoFocus
        onFocus={(e) => e.currentTarget.select()} />);
    if (!accessory) return input;
    return (
      <span className="editable-with-accessory">
        {input}
        {accessory}
      </span>);
  }
  return (
    <span
      className={className}
      onClick={open}
      tabIndex={0}
      role="textbox"
      onKeyDown={(e) => { if (e.key === "Enter") open(); }}
      style={{ display: "inline-block", cursor: "text", fontWeight: "700" }}>
      {value || placeholder}
    </span>);
}

/* The terminal-column check that rides along with an inline rename.

   It acts on mousedown and kills the default focus shift, because a
   click would blur the input first and blur is what commits the
   rename: the marker would arrive after its own commit, or split the
   write in two. dragstart is stopped as well — the header this sits in
   is the column-reorder drag handle. */
function TerminalCheck({ checked, onToggle, columnName }) {
  return (
    <button
      type="button"
      className={"terminal-check" + (checked ? " on" : "")}
      role="checkbox"
      aria-checked={checked}
      aria-label={`Terminal column — cards in ${columnName} count as done`}
      title="Terminal column — its cards count as done for epic progress"
      draggable={false}
      onDragStart={(e) => { e.preventDefault(); e.stopPropagation(); }}
      onMouseDown={(e) => { e.preventDefault(); e.stopPropagation(); onToggle(); }}
      onClick={(e) => e.preventDefault()}>
      <IconCheck size={12} />
    </button>);
}

/* =========================================================
   Hooks
========================================================= */
function useClickOutside(ref, onOutside, when = true) {
  useEffect(() => {
    if (!when) return;
    const handler = (e) => {
      if (ref.current && !ref.current.contains(e.target)) onOutside();
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [when, onOutside]);
}

/* =========================================================
   Card detail modal
========================================================= */
function formatRelative(iso) {
  if (!iso) return "—";
  const d = new Date(iso);
  const diff = Date.now() - d.getTime();
  const m = Math.round(diff / 60000);
  if (m < 1) return "just now";
  if (m < 60) return `${m}m ago`;
  const h = Math.round(m / 60);
  if (h < 24) return `${h}h ago`;
  const dd = Math.round(h / 24);
  if (dd < 7) return `${dd}d ago`;
  return d.toLocaleDateString("en-US", { day: "numeric", month: "short", year: "numeric" });
}

function formatAbsolute(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  return d.toLocaleString("en-US", { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" });
}

function CardDetailModal({ card, list, allLists, priorities, priorityColors, epics, onClose, onPatch, onPatchCard, onMoveColumn, onToggleTag, onRemove }) {
  const overlayRef = useRef(null);
  const [descDraft, setDescDraft] = useState(card.description || "");
  const [editingDesc, setEditingDesc] = useState(false);
  const [editingTitle, setEditingTitle] = useState(false);
  const [titleDraft, setTitleDraft] = useState(card.text);
  const [prioOpen, setPrioOpen] = useState(false);
  const [colOpen, setColOpen] = useState(false);
  const [addingTag, setAddingTag] = useState(false);
  const [tagDraft, setTagDraft] = useState("");
  const [picker, setPicker] = useState(null); // null | "parent" | "child"
  /* { at, message } — `at` names the section that issued the failed
     mutation, so the sentence renders where the cause is rather than
     floating at the top of the modal. */
  const [error, setError] = useState(null);
  const prioRef = useRef(null);
  const colRef = useRef(null);
  const descRef = useRef(null);
  const tagInputRef = useRef(null);

  useClickOutside(prioRef, () => setPrioOpen(false), prioOpen);
  useClickOutside(colRef, () => setColOpen(false), colOpen);

  useEffect(() => {
    const onKey = (e) => {
      if (e.key === "Escape") {
        if (editingDesc || editingTitle || addingTag || prioOpen || colOpen || picker) return;
        onClose();
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [editingDesc, editingTitle, addingTag, prioOpen, colOpen, picker, onClose]);

  useEffect(() => { setDescDraft(card.description || ""); }, [card.id]);
  // A message belongs to the card that produced it, and so does an
  // open picker.
  useEffect(() => { setError(null); setPicker(null); }, [card.id]);
  useEffect(() => { setTitleDraft(card.text); }, [card.id, card.text]);
  useEffect(() => { if (editingDesc && descRef.current) { descRef.current.focus(); descRef.current.setSelectionRange(descDraft.length, descDraft.length); } }, [editingDesc]);
  useEffect(() => { if (addingTag) tagInputRef.current?.focus(); }, [addingTag]);

  const commitDesc = () => { onPatch({ description: descDraft }, failAt("card")); setEditingDesc(false); };
  const cancelDesc = () => { setDescDraft(card.description || ""); setEditingDesc(false); };
  const commitTitle = () => {
    const t = titleDraft.trim();
    if (t && t !== card.text) onPatch({ text: t }, failAt("card"));
    else setTitleDraft(card.text);
    setEditingTitle(false);
  };
  const commitTag = () => {
    const t = tagDraft.trim().slice(0, 15);
    if (t && !(card.tags || []).includes(t)) onToggleTag(t, failAt("card"));
    setTagDraft("");
    setAddingTag(false);
  };

  const prio = card.priority;
  const prioColor = prio ? (priorityColors[prio] || "var(--text-muted)") : null;
  const tags = card.tags || [];

  // One-level nesting makes "child and parent at once" unrepresentable,
  // so at most one of these two sections ever renders.
  const idx = epics || EMPTY_EPIC_INDEX;
  const parent = idx.parentOf(card);
  const children = idx.childrenOf(card.id);
  const isParent = children.length > 0;
  const progress = isParent ? idx.progressOf(card.id) : null;
  const columnTitle = (name) => (allLists.find((l) => l.id === name) || {}).title || name;
  const allCards = useMemo(() => allLists.flatMap((l) => l.cards), [allLists]);

  /* The two candidate sets are not the same exclusions, because the
     two sides of the relation are not symmetric on the server. An epic
     target must carry no epic of its own — but may well have children,
     which is the ordinary case. A candidate child must have no
     children, and there is no point offering one already attached
     here. A card belonging to another epic stays listed: reassigning
     is legal. */
  const epicCandidates = useMemo(
    () => allCards.filter((c) => c.id !== card.id && !c.epic),
    [allCards, card.id]);
  const childCandidates = useMemo(
    () => allCards.filter((c) => c.id !== card.id && c.epic !== card.id && !idx.isEpic(c.id)),
    [allCards, card.id, idx]);

  /* An off-palette hex is shown as a ninth, selected swatch rather
     than as no selection at all: a hand-edited color is legal, and a
     UI that cannot represent the current value invites overwriting it
     by accident. */
  const swatches = useMemo(() => {
    if (!card.color || EPIC_PALETTE.some((p) => p.hex === card.color)) return EPIC_PALETTE;
    return [...EPIC_PALETTE, { name: card.color, hex: card.color }];
  }, [card.color]);

  /* Clearing before the call is what makes "cleared by the next
     successful mutation" true: only a failure writes a message back. */
  const failAt = (at) => { setError(null); return (message) => setError({ at, message }); };
  const openPicker = (which) => { setError(null); setPicker(which); };
  // Every relation write is a PATCH on the child, whichever side of
  // the modal it was issued from.
  const commitRelation = (childId, epicId, at) => {
    setPicker(null);
    onPatchCard(childId, { epic: epicId }, failAt(at));
  };
  const commitColor = (hex) => onPatch({ color: hex }, failAt("children"));

  return (
    <div
      className="modal-overlay"
      ref={overlayRef}
      onMouseDown={(e) => { if (e.target === overlayRef.current) onClose(); }}
      role="dialog"
      aria-modal="true">
      <div className="modal" onMouseDown={(e) => e.stopPropagation()}>
        <header className="modal-head">
          <div className="modal-id">
            <CopyableId className="modal-id-value" value={card.id} />
          </div>
          <div className="modal-actions">
            <button className="modal-action danger" onClick={() => { if (window.confirm("Delete this card?")) onRemove(); }} title="Delete card">
              <Icon d={<><polyline points="3 6 5 6 21 6" /><path d="M19 6l-2 14a2 2 0 0 1-2 2H9a2 2 0 0 1-2-2L5 6" /><path d="M10 11v6M14 11v6" /><path d="M9 6V4a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2v2" /></>} size={14} />
            </button>
            <button className="modal-action" onClick={onClose} title="Close">
              <IconClose size={14} />
            </button>
          </div>
        </header>

        <div className="modal-body">
          {editingTitle ?
            <input
              className="modal-title-input"
              value={titleDraft}
              autoFocus
              onChange={(e) => setTitleDraft(e.target.value)}
              onBlur={commitTitle}
              onKeyDown={(e) => {
                if (e.key === "Enter") { e.preventDefault(); e.currentTarget.blur(); }
                if (e.key === "Escape") { setTitleDraft(card.text); setEditingTitle(false); }
              }} /> :
            <h2 className="modal-title" onClick={() => setEditingTitle(true)} title="Click to edit">{card.text}</h2>}

          <div className="modal-meta">
            {priorities.length > 0 &&
              <div className="modal-field" ref={prioRef}>
                <label className="modal-label">Priority</label>
                <button
                  className={"modal-select modal-prio" + (prio ? " prio-" + prio : "")}
                  onClick={() => setPrioOpen((v) => !v)}
                  aria-haspopup="listbox"
                  aria-expanded={prioOpen}>
                  <span className="prio-dot" style={{ background: prioColor || "transparent", border: prioColor ? "none" : "1px dashed var(--text-faint)" }} aria-hidden="true" />
                  <span className="modal-select-text">{prio ? prio[0].toUpperCase() + prio.slice(1) : "None"}</span>
                  <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" className="modal-select-chev"><polyline points="6 9 12 15 18 9" /></svg>
                </button>
                {prioOpen &&
                  <ul className="modal-dropdown" role="listbox">
                    <li key="__none">
                      <button
                        className={"modal-dropdown-item" + (!prio ? " selected" : "")}
                        onClick={() => { onPatch({ priority: "" }); setPrioOpen(false); }}>
                        <span className="prio-dot" style={{ background: "transparent", border: "1px dashed var(--text-faint)" }} aria-hidden="true" />
                        <span>None</span>
                        {!prio && <IconCheck size={12} />}
                      </button>
                    </li>
                    {priorities.map((p) =>
                      <li key={p}>
                        <button
                          className={"modal-dropdown-item" + (p === prio ? " selected" : "")}
                          onClick={() => { onPatch({ priority: p }); setPrioOpen(false); }}>
                          <span className="prio-dot" style={{ background: priorityColors[p] || "var(--text-muted)" }} aria-hidden="true" />
                          <span>{p[0].toUpperCase() + p.slice(1)}</span>
                          {p === prio && <IconCheck size={12} />}
                        </button>
                      </li>)}
                  </ul>}
              </div>}

            <div className="modal-field" ref={colRef}>
              <label className="modal-label">Column</label>
              <button
                className="modal-select"
                onClick={() => setColOpen((v) => !v)}
                aria-haspopup="listbox"
                aria-expanded={colOpen}>
                <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><rect x="3" y="4" width="5" height="16" rx="1" /><rect x="10" y="4" width="5" height="10" rx="1" /><rect x="17" y="4" width="4" height="7" rx="1" /></svg>
                <span className="modal-select-text">{list.title}</span>
                <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" className="modal-select-chev"><polyline points="6 9 12 15 18 9" /></svg>
              </button>
              {colOpen &&
                <ul className="modal-dropdown" role="listbox">
                  {allLists.map((l) =>
                    <li key={l.id}>
                      <button
                        className={"modal-dropdown-item" + (l.id === list.id ? " selected" : "")}
                        onClick={() => { if (l.id !== list.id) onMoveColumn(l.id, failAt("card")); setColOpen(false); }}>
                        <span className="col-tick" />
                        <span>{l.title}</span>
                        {l.id === list.id && <IconCheck size={12} />}
                      </button>
                    </li>)}
                </ul>}
            </div>
          </div>

          <section className="modal-section">
            <div className="modal-section-head"><label className="modal-label">Description</label></div>
            {editingDesc ?
              <div className="modal-desc-edit">
                <textarea
                  ref={descRef}
                  className="modal-textarea"
                  value={descDraft}
                  onChange={(e) => setDescDraft(e.target.value)}
                  placeholder="Add more details about this card…"
                  onKeyDown={(e) => {
                    if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) { e.preventDefault(); commitDesc(); }
                    if (e.key === "Escape") cancelDesc();
                  }} />
                <div className="modal-desc-actions">
                  <button className="btn-primary" onClick={commitDesc}>Save</button>
                  <button className="btn-ghost" onClick={cancelDesc}>Cancel</button>
                  <span className="modal-hint">⌘↵ to save</span>
                </div>
              </div> :
              card.description ?
                <p className="modal-desc" onClick={() => setEditingDesc(true)}>{card.description}</p> :
                <button className="modal-desc-empty" onClick={() => setEditingDesc(true)}>
                  No description. Click to add one.
                </button>}
          </section>

          {/* The Epic section renders on every card that could carry
              one: a card with no relation is precisely the card that
              needs the attach control, and without it a board with no
              epics has no path to its first. A card with children is
              the one exception — one-level nesting forbids giving it a
              parent, so it gets the Children section instead. */}
          {!isParent &&
            <section className="modal-section modal-epic">
              <div className="modal-section-head"><label className="modal-label">Epic</label></div>
              {parent ?
                <div className="modal-epic-parent">
                  <EpicChip card={parent} />
                  <span className="modal-epic-id">{parent.id}</span>
                  <div className="modal-epic-parent-actions">
                    <button
                      type="button"
                      className="btn-ghost modal-epic-action"
                      onClick={() => openPicker("parent")}>
                      Change
                    </button>
                    <button
                      type="button"
                      className="btn-ghost modal-epic-action"
                      onClick={() => commitRelation(card.id, "", "epic")}>
                      Detach
                    </button>
                  </div>
                </div> :
                picker !== "parent" &&
                  <button
                    type="button"
                    className="modal-desc-empty modal-epic-empty"
                    onClick={() => openPicker("parent")}>
                    No epic. Add this card to one.
                  </button>}
              {picker === "parent" &&
                <EpicPicker
                  candidates={epicCandidates}
                  columnTitle={columnTitle}
                  placeholder="Search a card by title or id…"
                  onPick={(id) => commitRelation(card.id, id, "epic")}
                  onClose={() => setPicker(null)} />}
              {error && error.at === "epic" &&
                <p className="modal-error" role="alert">{error.message}</p>}
            </section>}

          {progress &&
            <section
              className="modal-section modal-epic"
              style={{ "--epic-color": card.color || "var(--text-muted)" }}>
              <div className="modal-section-head">
                <label className="modal-label">Children</label>
                {picker !== "child" &&
                  <button
                    type="button"
                    className="btn-ghost modal-epic-action"
                    onClick={() => openPicker("child")}>
                    <IconPlus size={11} /><span>Add</span>
                  </button>}
              </div>
              <EpicProgress done={progress.done} total={progress.total} className="modal-epic-progress" />
              <ul className="modal-epic-children">
                {children.map((c) =>
                  <li key={c.id} className="modal-epic-child">
                    <span className="modal-epic-child-title" title={c.text}>{c.text}</span>
                    <span className="modal-epic-child-col">{columnTitle(c.column)}</span>
                    {/* Removing a child is a PATCH on the child: the
                        relation lives in its `epic` field and nowhere
                        else, so the parent is never written. */}
                    <button
                      type="button"
                      className="modal-epic-child-remove"
                      title={`Remove ${c.text} from this epic`}
                      aria-label={`Remove ${c.text} from this epic`}
                      onClick={() => commitRelation(c.id, "", "children")}>
                      <IconClose size={11} />
                    </button>
                  </li>)}
              </ul>
              {picker === "child" &&
                <EpicPicker
                  candidates={childCandidates}
                  columnTitle={columnTitle}
                  placeholder="Search a card by title or id…"
                  onPick={(id) => commitRelation(id, card.id, "children")}
                  onClose={() => setPicker(null)} />}

              <div className="modal-section-head modal-epic-color-head">
                <label className="modal-label">Color</label>
              </div>
              <div className="modal-epic-swatches">
                {swatches.map((s) =>
                  <button
                    key={s.hex}
                    type="button"
                    className={"modal-epic-swatch" + (s.hex === card.color ? " selected" : "")}
                    style={{ "--swatch": s.hex }}
                    title={s.name}
                    aria-label={s.name}
                    aria-pressed={s.hex === card.color}
                    onClick={() => commitColor(s.hex)} />)}
                <button
                  type="button"
                  className={"modal-epic-swatch is-clear" + (card.color ? "" : " selected")}
                  title="No color"
                  aria-label="No color"
                  aria-pressed={!card.color}
                  onClick={() => commitColor("")} />
              </div>
              {error && error.at === "children" &&
                <p className="modal-error" role="alert">{error.message}</p>}
            </section>}

          <section className="modal-section">
            <div className="modal-section-head"><label className="modal-label">Tags</label></div>
            <div className="modal-tags">
              {tags.map((t) =>
                <button key={t} className="modal-tag-chip" onClick={() => onToggleTag(t, failAt("card"))} title="Remove this tag">
                  <span>{t}</span>
                  <IconClose size={10} />
                </button>)}
              {addingTag ?
                <input
                  ref={tagInputRef}
                  className="modal-tag-input"
                  value={tagDraft}
                  maxLength={15}
                  placeholder="new tag…"
                  onChange={(e) => setTagDraft(e.target.value.slice(0, 15))}
                  onBlur={commitTag}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") { e.preventDefault(); commitTag(); }
                    if (e.key === "Escape") { setTagDraft(""); setAddingTag(false); }
                  }} /> :
                <button className="modal-tag-add" onClick={() => setAddingTag(true)}>
                  <IconPlus size={11} /><span>Add</span>
                </button>}
            </div>
          </section>

          {/* Failures from the fields that have no section of their own
              — title, description, priority, column, tags — land here
              rather than in the console alone. */}
          {error && error.at === "card" &&
            <p className="modal-error" role="alert">{error.message}</p>}
        </div>

        <footer className="modal-foot">
          <div className="modal-foot-item">
            <span className="modal-foot-label">Created</span>
            <span className="modal-foot-value" title={formatAbsolute(card.createdAt)}>{formatRelative(card.createdAt)}</span>
          </div>
          <span className="modal-foot-sep" />
          <div className="modal-foot-item">
            <span className="modal-foot-label">Modified</span>
            <span className="modal-foot-value" title={formatAbsolute(card.updatedAt)}>{formatRelative(card.updatedAt)}</span>
          </div>
        </footer>
      </div>
    </div>);
}

/* =========================================================
   Mount
========================================================= */
const root = ReactDOM.createRoot(document.getElementById("root"));
root.render(<App />);
