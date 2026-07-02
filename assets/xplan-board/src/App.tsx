import { useCallback, useEffect, useRef, useState } from 'react';
import { TONE } from './tokens';

// gogo board -- a React port of xplan's ScreenTasks kanban (BoardColumns / Card /
// LabelChip look, native HTML5 drag-drop via a single dragId, the TONE dark
// palette + the .wf-panel card shell). Adapted to gogo's derived model: four
// fixed columns fed by the shared gogo-status classifier, a text filter, live
// polling of GET /api/board, "view" opens a pre-built page, and shipping a ready
// card (drag -> changelog, or select + mark done) POSTs a schema-v2 ship intent.
// Dropped from xplan: GitHub, Mongo, card create/edit/delete, assignees, the
// move-to Select -- gogo columns are derived, only the ship transition exists.

type BoardColumn = { id: string; name: string };
type BoardItem = {
  slug: string;
  title: string;
  class: string; // shipped | ready-to-ship | in-progress | unfinished
  column: string; // plan | in-progress | ready | changelog
  view_url: string;
  report_path?: string | null;
  changelog_path?: string | null;
  iterations?: string;
  status?: string;
};
type Board = {
  repo: string;
  generated: string;
  columns: BoardColumn[];
  items: BoardItem[];
};
type Toast = { msg: string; kind: 'info' | 'hint' | 'error' | 'warn'; dismissible?: boolean } | null;

// Per-column accent dot (muted, not the saturated state palette) -- mirrors
// xplan's COLUMN_ACCENT, keyed by gogo's derived column ids.
const COLUMN_ACCENT: Record<string, string> = {
  plan: TONE.muted,
  'in-progress': TONE.extra,
  ready: TONE.accent,
  changelog: TONE.done,
};

// Chip color follows the item's classifier class.
const CLASS_COLOR: Record<string, string> = {
  unfinished: TONE.todoText,
  'in-progress': TONE.extra,
  'ready-to-ship': TONE.accent,
  shipped: TONE.done,
};

const POLL_MS = 3000;
// If a ship is accepted but the card hasn't reconciled to `changelog` after this
// long, downgrade the persistent "shipping..." toast to a dismissible warning.
const SHIP_WATCHDOG_MS = 180000; // ~3 minutes

export function App() {
  const [board, setBoard] = useState<Board | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState('');
  const [dragId, setDragId] = useState<string | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [shipping, setShipping] = useState<string[]>([]);
  const [toast, setToast] = useState<Toast>(null);
  const [refreshedAt, setRefreshedAt] = useState<number | null>(null);

  // Refs so the poll timer reads current values without re-subscribing.
  const draggingRef = useRef(false);
  const shippingRef = useRef<string[]>([]);
  shippingRef.current = shipping;
  // When the current ship was accepted, and whether we've already downgraded its
  // toast to a warning (so a user's dismiss sticks instead of re-appearing).
  const shippedAtRef = useRef<number | null>(null);
  const warnedRef = useRef(false);

  const loadBoard = useCallback(async () => {
    try {
      const res = await fetch('api/board', { cache: 'no-store' });
      if (!res.ok) throw new Error('HTTP ' + res.status);
      const data = (await res.json()) as Board;
      setBoard(data);
      setError(null);
      setRefreshedAt(Date.now());
      // Reconcile the in-flight ship set: a shipped card lands in `changelog`.
      const pending = shippingRef.current;
      if (pending.length) {
        const done = (slug: string) =>
          data.items.some((i) => i.slug === slug && i.column === 'changelog');
        const still = pending.filter((s) => !done(s));
        if (still.length !== pending.length) setShipping(still);
        if (still.length === 0) {
          // Everything reconciled -- clear the ship state + its toast.
          shippedAtRef.current = null;
          warnedRef.current = false;
          setToast(null);
        } else if (
          !warnedRef.current &&
          shippedAtRef.current &&
          Date.now() - shippedAtRef.current > SHIP_WATCHDOG_MS
        ) {
          // Accepted but not reconciled after ~3 min: downgrade to a dismissible
          // warning once (polling continues, so a late finish still self-clears).
          warnedRef.current = true;
          setToast({
            msg: 'still waiting on gogo -- check the chat; this can be safely dismissed',
            kind: 'warn',
            dismissible: true,
          });
        }
      }
    } catch (e) {
      setError((e as Error).message);
    }
  }, []);

  // Initial load + poll every POLL_MS, paused while a drag is in progress.
  useEffect(() => {
    loadBoard();
    const id = window.setInterval(() => {
      if (!draggingRef.current) loadBoard();
    }, POLL_MS);
    return () => window.clearInterval(id);
  }, [loadBoard]);

  // Auto-dismiss transient toasts; the "shipping..." info toast persists until
  // the ship reconciles, and the watchdog 'warn' persists until dismissed by hand.
  useEffect(() => {
    if (toast && toast.kind !== 'info' && toast.kind !== 'warn') {
      const id = window.setTimeout(() => setToast(null), 4000);
      return () => window.clearTimeout(id);
    }
    return undefined;
  }, [toast]);

  // Keep the selection to cards that are still ready (one may ship out from under
  // us on a refresh).
  useEffect(() => {
    if (!board) return;
    const ready = new Set(board.items.filter((i) => i.column === 'ready').map((i) => i.slug));
    setSelected((prev) => {
      let changed = false;
      const next = new Set<string>();
      for (const s of prev) {
        if (ready.has(s)) next.add(s);
        else changed = true;
      }
      return changed ? next : prev;
    });
  }, [board]);

  const ship = useCallback(async (slugs: string[]) => {
    if (!slugs.length) return;
    const action = slugs.length > 1 ? 'ship-merged' : 'ship';
    try {
      const res = await fetch('api/ship', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ schema: 2, action, items: slugs }),
      });
      if (res.status === 202) {
        setShipping(slugs);
        shippedAtRef.current = Date.now();
        warnedRef.current = false;
        setSelected(new Set());
        setToast({
          msg: `shipping ${slugs.length} ${slugs.length > 1 ? 'items' : 'item'}... waiting for gogo`,
          kind: 'info',
        });
      } else {
        // Any non-202 (400 rejected / 409 already pending / 500) -- surface the
        // server's own message and clear this attempt's shipping state so a
        // rejected ship never wedges the surface.
        const body = (await res.json().catch(() => ({}))) as { error?: string };
        setShipping((prev) => prev.filter((s) => !slugs.includes(s)));
        setToast({
          msg: body.error ? `ship rejected: ${body.error}` : `ship rejected (HTTP ${res.status})`,
          kind: 'error',
        });
      }
    } catch (e) {
      setToast({ msg: `ship failed: ${(e as Error).message}`, kind: 'error' });
    }
  }, []);

  const onDragStart = (slug: string) => {
    setDragId(slug);
    draggingRef.current = true;
  };
  const onDragEnd = () => {
    setDragId(null);
    draggingRef.current = false;
  };
  const onDrop = (columnId: string) => {
    const slug = dragId;
    setDragId(null);
    draggingRef.current = false;
    if (!slug || !board) return;
    const item = board.items.find((i) => i.slug === slug);
    if (!item) return;
    if (item.column === 'ready' && columnId === 'changelog') {
      // Dragging a selected card ships the whole selection; otherwise just it.
      const slugs = selected.has(slug) ? [...selected] : [slug];
      ship(slugs);
    } else if (item.column !== columnId) {
      // Any other cross-column move is illegal -- columns are derived state.
      // Never clobber a persistent shipping toast with the transient hint (TEST-001):
      // the "shipping..." info toast must survive until the ship resolves.
      if (shippingRef.current.length === 0) {
        setToast({ msg: 'only ready -> changelog is allowed - columns follow the work state', kind: 'hint' });
      }
    }
    // Dropping a card back on its own column is a silent no-op.
  };

  const toggleSelect = (slug: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(slug)) next.delete(slug);
      else next.add(slug);
      return next;
    });
  };

  const q = filter.trim().toLowerCase();
  const matches = (i: BoardItem) => !q || `${i.slug} ${i.title}`.toLowerCase().includes(q);

  const columns = board?.columns ?? [];
  const items = board?.items ?? [];

  return (
    <div className="app">
      <header className="board-header wf-row">
        <span className="brand">
          <span className="brand-dot" /> gogo board
        </span>
        <span className="wf-mono repo">{board?.repo || '(repo)'}</span>
        <input
          className="filter-input"
          type="text"
          placeholder="filter by slug or title..."
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          aria-label="Filter cards"
        />
        <span className="spacer" />
        <span className={`refresh${error ? ' is-error' : ''}`} title={error ? `offline: ${error}` : 'auto-refresh every 3s'}>
          <span className="pulse" key={refreshedAt ?? 0} />
          {error ? 'reconnecting...' : 'live'}
        </span>
      </header>

      {!board && !error && <div className="board-empty">connecting to the gogo board server...</div>}
      {!board && error && (
        <div className="board-empty">can't reach the board server - retrying... ({error})</div>
      )}

      {board && (
        <div className="board-scroll">
          {columns.map((col) => {
            const all = items.filter((i) => i.column === col.id);
            const cards = all.filter(matches);
            return (
              <div
                key={col.id}
                className="board-col"
                onDragOver={(e) => e.preventDefault()}
                onDrop={() => onDrop(col.id)}
              >
                <div className="col-head wf-row">
                  <span className="col-dot" style={{ background: COLUMN_ACCENT[col.id] ?? TONE.muted }} />
                  <span className="col-name">{col.name}</span>
                  <span className="wf-mono col-count">{q ? `${cards.length}/${all.length}` : all.length}</span>
                </div>
                <div className="col-cards">
                  {cards.map((item) => (
                    <Card
                      key={item.slug}
                      item={item}
                      selectable={col.id === 'ready'}
                      selected={selected.has(item.slug)}
                      dragging={dragId === item.slug}
                      shipping={shipping.includes(item.slug)}
                      onToggle={toggleSelect}
                      onDragStart={onDragStart}
                      onDragEnd={onDragEnd}
                    />
                  ))}
                  {cards.length === 0 && <div className="col-empty">{q && all.length ? 'no match' : 'empty'}</div>}
                </div>
              </div>
            );
          })}
        </div>
      )}

      {selected.size > 0 && (
        <div className="footer-bar">
          <span className="count">{selected.size} selected</span>
          <button type="button" className="wf-btn wf-btn-primary" onClick={() => ship([...selected])}>
            Mark done ({selected.size}){selected.size > 1 ? ' -> one merged entry' : ''}
          </button>
          <button type="button" className="wf-btn" onClick={() => setSelected(new Set())}>
            clear
          </button>
        </div>
      )}

      {toast && (
        <div className={`toast toast-${toast.kind}`}>
          <span className="toast-msg">{toast.msg}</span>
          {toast.dismissible && (
            <button
              type="button"
              className="toast-dismiss"
              onClick={() => setToast(null)}
              aria-label="Dismiss"
              title="dismiss"
            >
              x
            </button>
          )}
        </div>
      )}
    </div>
  );
}

function Card({
  item,
  selectable,
  selected,
  dragging,
  shipping,
  onToggle,
  onDragStart,
  onDragEnd,
}: {
  item: BoardItem;
  selectable: boolean;
  selected: boolean;
  dragging: boolean;
  shipping: boolean;
  onToggle: (slug: string) => void;
  onDragStart: (slug: string) => void;
  onDragEnd: () => void;
}) {
  const accent = CLASS_COLOR[item.class] ?? TONE.muted;
  const cls = ['wf-panel', 'card'];
  if (dragging) cls.push('is-dragging');
  if (shipping) cls.push('is-shipping');
  return (
    <div
      className={cls.join(' ')}
      draggable
      onDragStart={(e) => {
        e.dataTransfer.effectAllowed = 'move';
        onDragStart(item.slug);
      }}
      onDragEnd={onDragEnd}
      style={selected ? { boxShadow: `0 0 0 2px ${TONE.accent}, 0 0 0 4px ${TONE.accentFill}` } : undefined}
    >
      <div className="card-top wf-row">
        <span className="wf-row card-slug" title={item.slug}>
          <span className="card-dot" style={{ background: accent }} />
          <span className="wf-mono">{item.slug}</span>
        </span>
        {selectable && (
          <label className="card-check" title="select to ship" onClick={(e) => e.stopPropagation()}>
            <input
              type="checkbox"
              checked={selected}
              onChange={() => onToggle(item.slug)}
              aria-label={`Select ${item.slug}`}
            />
          </label>
        )}
      </div>

      <div className="card-title">{item.title}</div>

      {item.iterations && (
        <div className="card-chips wf-row">
          <span className="chip wf-mono" style={{ color: accent, borderColor: accent }}>
            {item.iterations}
          </span>
        </div>
      )}

      <div className="card-foot wf-row">
        <a
          className="wf-btn view-btn"
          href={`view/${item.view_url}`}
          target="_blank"
          rel="noreferrer"
          title="open the built page in a new tab"
          onClick={(e) => e.stopPropagation()}
        >
          view
        </a>
        {shipping && <span className="wf-mono shipping-tag">shipping...</span>}
      </div>
    </div>
  );
}
