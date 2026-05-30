const { useState, useEffect, useRef, useCallback, useMemo } = React;

/* =========================================================
   Wire-shape ↔ UI-shape adapter
   Server JSON:  { columns[], cards[{ id, title, column, priority,
                   tags[], description, created_at, updated_at }],
                   priorities[], priority_colors{}, project_name }
   UI tree:      { title, lists:[{ id=columnName, title=DISPLAY,
                   cards:[{ id, text, tags, priority, description,
                   createdAt, updatedAt }] }],
                   priorities[], priorityColors{} }
========================================================= */
function toUiBoard(server) {
  const cardsByCol = {};
  for (const c of server.cards || []) {
    (cardsByCol[c.column] = cardsByCol[c.column] || []).push({
      id: c.id,
      text: c.title || "",
      tags: c.tags || [],
      priority: c.priority || "",
      description: c.description || "",
      createdAt: c.created_at,
      updatedAt: c.updated_at,
    });
  }
  return {
    title: server.project_name || "",
    lists: (server.columns || []).map((name) => ({
      id: name,
      title: String(name).toUpperCase(),
      cards: cardsByCol[name] || [],
    })),
    priorities: server.priorities || [],
    priorityColors: server.priority_colors || {},
  };
}

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
    let detail = "";
    try { detail = JSON.stringify(await r.json()); } catch (_) {}
    throw new Error(`${method} ${path} → ${r.status} ${detail}`);
  }
  if (r.status === 204) return null;
  const ct = r.headers.get("content-type") || "";
  return ct.includes("application/json") ? r.json() : null;
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
};

function filterIsActive(f) {
  return (f.query && f.query.trim().length > 0) || (f.priorities && f.priorities.length > 0);
}

function matchCard(card, f) {
  if (f.priorities && f.priorities.length > 0) {
    const p = card.priority || "";
    if (!f.priorities.includes(p)) return false;
  }
  const q = (f.query || "").trim().toLowerCase();
  if (!q) return true;
  if (!f.inTitle && !f.inDescription && !f.inTags && !f.inId) return false;
  if (f.inTitle && (card.text || "").toLowerCase().includes(q)) return true;
  if (f.inDescription && (card.description || "").toLowerCase().includes(q)) return true;
  if (f.inTags && (card.tags || []).some((t) => String(t).toLowerCase().includes(q))) return true;
  if (f.inId && (card.id || "").toLowerCase().includes(q)) return true;
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

  const editCard = async (cardId, text) => {
    try { await apiSend("PATCH", `/api/cards/${encodeURIComponent(cardId)}`, { title: text }); }
    catch (e) { console.error(e); }
    fetchBoard();
  };

  const patchCard = async (cardId, patch) => {
    // Translate UI keys → server keys.
    const body = {};
    if ("text" in patch) body.title = patch.text;
    if ("description" in patch) body.description = patch.description;
    if ("priority" in patch) body.priority = patch.priority;
    if ("tags" in patch) body.tags = patch.tags;
    try { await apiSend("PATCH", `/api/cards/${encodeURIComponent(cardId)}`, body); }
    catch (e) { console.error(e); }
    fetchBoard();
  };

  const removeCard = async (cardId) => {
    try { await apiSend("DELETE", `/api/cards/${encodeURIComponent(cardId)}`); }
    catch (e) { console.error(e); }
    fetchBoard();
  };

  const toggleTag = async (cardId, tag) => {
    if (!board) return;
    const card = board.lists.flatMap((l) => l.cards).find((c) => c.id === cardId);
    if (!card) return;
    const tags = card.tags.includes(tag) ? card.tags.filter((t) => t !== tag) : [...card.tags, tag];
    try { await apiSend("PATCH", `/api/cards/${encodeURIComponent(cardId)}`, { tags }); }
    catch (e) { console.error(e); }
    fetchBoard();
  };

  const moveCard = async (fromListId, cardId, toListId, toIndex) => {
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
    } catch (e) { console.error(e); fetchBoard(); }
  };

  const addList = async (title) => {
    const name = title.trim();
    if (!name) return;
    try { await apiSend("POST", "/api/columns", { name }); }
    catch (e) { alert(`Cannot add list: ${e.message}`); }
    fetchBoard();
  };

  const renameList = async (from, to) => {
    const name = to.trim();
    if (!name || name === from) return;
    try { await apiSend("PATCH", `/api/columns/${encodeURIComponent(from)}`, { name }); }
    catch (e) { alert(`Cannot rename list: ${e.message}`); fetchBoard(); }
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

  const filterActive = filterIsActive(filter);
  const filteredCount = useMemo(() => {
    if (!board || !filterActive) return 0;
    return board.lists.reduce(
      (acc, l) => acc + l.cards.filter((c) => matchCard(c, filter)).length,
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
        filter={filter}
        onFilterChange={setFilter}
        filterActive={filterActive}
        filterOpen={filterOpen}
        setFilterOpen={setFilterOpen}
        filteredCount={filteredCount}
        priorities={board.priorities}
        priorityColors={board.priorityColors}
        theme={theme}
        sseStatus={sseStatus}
      />

      <Board
        board={board}
        filter={filter}
        filterActive={filterActive}
        priorityColors={board.priorityColors}
        onAddList={addList}
        onRenameList={renameList}
        onRemoveList={removeList}
        onAddCard={addCard}
        onEditCard={editCard}
        onRemoveCard={removeCard}
        onToggleTag={toggleTag}
        onMoveCard={moveCard}
        onMoveList={moveList}
        onOpenCard={(cardId) => setOpenCardId(cardId)}
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
            onClose={() => setOpenCardId(null)}
            onPatch={(patch) => patchCard(card.id, patch)}
            onMoveColumn={(toListId) => moveCard(list.id, card.id, toListId, board.lists.find((l) => l.id === toListId).cards.length)}
            onToggleTag={(tag) => toggleTag(card.id, tag)}
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
function TopBar({ title, filter, onFilterChange, filterActive, filterOpen, setFilterOpen, filteredCount, priorities, priorityColors, theme, sseStatus }) {
  const popRef = useRef(null);
  useClickOutside(popRef, () => setFilterOpen(false), filterOpen);

  const toggleScope = (key) => onFilterChange({ ...filter, [key]: !filter[key] });
  const togglePriority = (id) => {
    const set = new Set(filter.priorities || []);
    if (set.has(id)) set.delete(id); else set.add(id);
    onFilterChange({ ...filter, priorities: Array.from(set) });
  };
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
              {filterActive &&
                <button className="filter-clear" onClick={clearFilter}>
                  <IconClose size={12} /> Clear all
                </button>}
            </div>}
        </div>

        <span className="topbar-divider" />

        <ThemeToggle theme={theme} />

        <ServerStatus status={sseStatus} />
      </div>
    </header>);
}

function ServerStatus({ status }) {
  const [open, setOpen] = useState(false);
  const popRef = useRef(null);
  useClickOutside(popRef, () => setOpen(false), open);

  const statusMeta = {
    connecting: { label: "Connecting…", color: "oklch(0.72 0.13 85)" },
    online:     { label: "Online",      color: "oklch(0.68 0.16 145)" },
    offline:    { label: "Offline",     color: "oklch(0.6 0.2 25)"    },
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
function Board({ board, filter, filterActive, priorityColors, onAddList, onRenameList, onRemoveList, onAddCard, onEditCard, onRemoveCard, onToggleTag, onMoveCard, onMoveList, onOpenCard }) {
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
            dragRef={dragRef}
            onRename={(t) => onRenameList(list.id, t)}
            onRemove={() => onRemoveList(list.id)}
            onAddCard={(t) => onAddCard(list.id, t)}
            onEditCard={(cid, t) => onEditCard(cid, t)}
            onRemoveCard={(cid) => onRemoveCard(cid)}
            onToggleTag={(cid, tag) => onToggleTag(cid, tag)}
            onMoveCard={onMoveCard}
            onMoveList={onMoveList}
            onOpenCard={(cid) => onOpenCard(cid)}
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
function ListColumn({ list, index, filter, filterActive, priorityColors, dragRef, onRename, onRemove, onAddCard, onEditCard, onRemoveCard, onToggleTag, onMoveCard, onMoveList, onOpenCard }) {
  const [adding, setAdding] = useState(false);
  const [isOver, setIsOver] = useState(false);
  const [draggingSelf, setDraggingSelf] = useState(false);
  const counterRef = useRef(0);

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
    return list.cards.filter((c) => matchCard(c, filter));
  }, [list.cards, filter, filterActive]);

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
      onMoveCard(fromListId, cardId, list.id, list.cards.length);
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
          original={list.id}
          onChange={(t) => onRename(t)}
          uppercase />
        <span className="list-count" title={`${list.cards.length} cards`}>{list.cards.length}</span>
        <ListMenu onRemove={onRemove} />
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
            onEdit={(t) => onEditCard(card.id, t)}
            onRemove={() => onRemoveCard(card.id)}
            onToggleTag={(tag) => onToggleTag(card.id, tag)}
            onMoveCard={onMoveCard}
            onOpen={() => onOpenCard(card.id)} />)}
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

function ListMenu({ onRemove }) {
  const [open, setOpen] = useState(false);
  const ref = useRef(null);
  useClickOutside(ref, () => setOpen(false), open);
  return (
    <div ref={ref} style={{ position: "relative" }}>
      <button className="list-menu" onClick={() => setOpen((v) => !v)} aria-label="More options">
        <IconDots />
      </button>
      {open &&
        <div className="popover" style={{ right: 0, top: "calc(100% + 4px)", minWidth: 180, padding: 6 }}>
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
function CardItem({ card, listId, index, dragRef, priorityColors, onEdit, onRemove, onToggleTag, onMoveCard, onOpen }) {
  const [editing, setEditing] = useState(false);
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

  if (editing) {
    return (
      <CardComposer
        initial={card.text}
        onAdd={(t) => { onEdit(t); setEditing(false); }}
        onClose={() => setEditing(false)}
        submitLabel="Save" />);
  }

  const prioColor = card.priority ? (priorityColors[card.priority] || "var(--text-muted)") : null;
  const hasDesc = !!(card.description && card.description.trim());

  return (
    <article
      className={"card" + (dragging ? " dragging" : "") + (dropPos ? " drop-" + dropPos : "")}
      data-card-id={card.id}
      draggable
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
      onDragOver={onDragOver}
      onDragLeave={onDragLeave}
      onDrop={onDrop}
      onClick={(e) => {
        if (e.target.closest('.card-tag-chip, .card-tag-add, .card-tag-input, .card-delete')) return;
        onOpen?.();
      }}
      onDoubleClick={(e) => {
        if (e.target.closest('.card-title')) { e.stopPropagation(); setEditing(true); }
      }}>
      <CopyableId className="card-id" value={card.id} />
      <div className="card-title">{card.text}</div>
      <div className="card-foot">
        {prioColor && <span className="card-prio-pill" style={{ background: prioColor }} aria-label={"Priority " + card.priority} />}
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
function CardComposer({ onAdd, onClose, initial = "", submitLabel = "Add" }) {
  const [text, setText] = useState(initial);
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
        <button className="btn-primary" onMouseDown={(e) => e.preventDefault()} onClick={submit}>{submitLabel}</button>
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
   Editable text — emits trimmed original column name on commit
========================================================= */
function EditableText({ value, original, onChange, className, placeholder, uppercase }) {
  const ref = useRef(null);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);

  useEffect(() => { setDraft(value); }, [value]);

  if (editing) {
    return (
      <input
        ref={ref}
        className={className}
        value={draft}
        onChange={(e) => setDraft(uppercase ? e.target.value.toUpperCase() : e.target.value)}
        onBlur={() => {
          const t = (draft || "").trim();
          if (t) onChange(t.toLowerCase());
          setEditing(false);
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter") { e.preventDefault(); e.currentTarget.blur(); }
          if (e.key === "Escape") { setDraft(value); setEditing(false); }
        }}
        autoFocus
        onFocus={(e) => e.currentTarget.select()} />);
  }
  return (
    <span
      className={className}
      onClick={() => setEditing(true)}
      tabIndex={0}
      role="textbox"
      onKeyDown={(e) => { if (e.key === "Enter") setEditing(true); }}
      style={{ display: "inline-block", cursor: "text", fontWeight: "700" }}>
      {value || placeholder}
    </span>);
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

function CardDetailModal({ card, list, allLists, priorities, priorityColors, onClose, onPatch, onMoveColumn, onToggleTag, onRemove }) {
  const overlayRef = useRef(null);
  const [descDraft, setDescDraft] = useState(card.description || "");
  const [editingDesc, setEditingDesc] = useState(false);
  const [editingTitle, setEditingTitle] = useState(false);
  const [titleDraft, setTitleDraft] = useState(card.text);
  const [prioOpen, setPrioOpen] = useState(false);
  const [colOpen, setColOpen] = useState(false);
  const [addingTag, setAddingTag] = useState(false);
  const [tagDraft, setTagDraft] = useState("");
  const prioRef = useRef(null);
  const colRef = useRef(null);
  const descRef = useRef(null);
  const tagInputRef = useRef(null);

  useClickOutside(prioRef, () => setPrioOpen(false), prioOpen);
  useClickOutside(colRef, () => setColOpen(false), colOpen);

  useEffect(() => {
    const onKey = (e) => {
      if (e.key === "Escape") {
        if (editingDesc || editingTitle || addingTag || prioOpen || colOpen) return;
        onClose();
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [editingDesc, editingTitle, addingTag, prioOpen, colOpen, onClose]);

  useEffect(() => { setDescDraft(card.description || ""); }, [card.id]);
  useEffect(() => { setTitleDraft(card.text); }, [card.id, card.text]);
  useEffect(() => { if (editingDesc && descRef.current) { descRef.current.focus(); descRef.current.setSelectionRange(descDraft.length, descDraft.length); } }, [editingDesc]);
  useEffect(() => { if (addingTag) tagInputRef.current?.focus(); }, [addingTag]);

  const commitDesc = () => { onPatch({ description: descDraft }); setEditingDesc(false); };
  const cancelDesc = () => { setDescDraft(card.description || ""); setEditingDesc(false); };
  const commitTitle = () => {
    const t = titleDraft.trim();
    if (t && t !== card.text) onPatch({ text: t });
    else setTitleDraft(card.text);
    setEditingTitle(false);
  };
  const commitTag = () => {
    const t = tagDraft.trim().slice(0, 15);
    if (t && !(card.tags || []).includes(t)) onToggleTag(t);
    setTagDraft("");
    setAddingTag(false);
  };

  const prio = card.priority;
  const prioColor = prio ? (priorityColors[prio] || "var(--text-muted)") : null;
  const tags = card.tags || [];

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
                        onClick={() => { if (l.id !== list.id) onMoveColumn(l.id); setColOpen(false); }}>
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

          <section className="modal-section">
            <div className="modal-section-head"><label className="modal-label">Tags</label></div>
            <div className="modal-tags">
              {tags.map((t) =>
                <button key={t} className="modal-tag-chip" onClick={() => onToggleTag(t)} title="Remove this tag">
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
