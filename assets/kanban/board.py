#!/usr/bin/env python3
"""gogo done board -- an interactive terminal kanban cockpit for /gogo:done.

Reads a work-index (records classified by the gogo-status Step A classifier) and
shows four columns -- unfinished, in-progress, ready-to-ship, shipped. It is the
pipeline's cockpit: from the same table the user can VIEW any card's page, SHIP
ready cards (separately or merged), GO (run/resume the pipeline) on an unbuilt
card, and FILTER the board by text. It is ONE mode with action keys -- no
view/manage toggle.

Action keys (guards enforce legality per class):
  arrows/hjkl  move the cursor            space/enter  toggle a ready-to-ship card
  v  view the focused card (any class)    s  ship the selection (separately)
  m  ship the selection merged (>=2)      g  go/resume the focused card
  /  filter (Esc clears)                  q  cancel

This is a selector/visualizer ONLY: it never copies, archives, or mutates any gogo
state. Every action key just writes a single-shot INTENT to the result file and
exits; the gogo-done orchestrator executes the intent (view build / ship writer /
pipeline handoff) and relaunches the board. All execution stays single-sourced in
the skill -- the board has no LLM and never ships anything itself.

Intent result schema (v2), written on any action, exit 0:
  {"schema": 2, "action": "view|ship|ship-merged|go", "items": ["<slug>", ...]}
    - view / go  carry the single focused slug.
    - ship / ship-merged  carry the selected ready-to-ship slugs (record order).
gogo-done also accepts the legacy {"ship": [...]} shape as action "ship"
(back-compat); see normalize_intent().

Exit-code contract (the gogo-done skill branches on these):
  0  action    -- result file written with a schema-v2 intent.
  1  cancel    -- user quit (q); no result file written.
  2  error     -- cannot read the work-index (missing/malformed --index or stdin),
                  a headless guard violation, or the board cannot start. No result
                  file; a one-line reason on stderr and NO traceback. The skill
                  treats 2 like a launch failure -> its status-table fallback,
                  never a cancel.

Pure stdlib (curses, json, sys, argparse, tempfile, os). No network. Offline.

Interactive curses needs a tty, so a headless self-test path (--selftest) exercises
the classify -> guard -> intent logic WITHOUT curses, for automated verification. A
--headless --action <a> [--ship <slugs>] path emits an intent file without curses
(used by the skill's degradation and by smoke tests); --action defaults to "ship"
and --ship still maps to the shipped items, for back-compat.
"""

import argparse
import json
import os
import sys
import tempfile

# The four work-index classes, left-to-right in pipeline order. Only cards in
# SHIPPABLE are selectable to ship; GOABLE cards are the ones `g` can run/resume.
CLASSES = ["unfinished", "in-progress", "ready-to-ship", "shipped"]
SHIPPABLE = "ready-to-ship"
GOABLE = ("unfinished", "in-progress")
INTENT_SCHEMA = 2
ACTIONS = ["view", "ship", "ship-merged", "go", "cancel"]


# --------------------------------------------------------------------------- #
# Core logic (curses-free, so --selftest can exercise it headlessly).
# --------------------------------------------------------------------------- #

def load_index(source):
    """Load raw work-index records from a file path, '-' for stdin, or a list.

    Accepts a bare JSON array of records, or an object wrapping them under
    "records" / "index". Returns a list (possibly empty)."""
    if isinstance(source, list):
        raw = source
    else:
        if source in (None, "-"):
            text = sys.stdin.read()
        else:
            with open(source, "r", encoding="utf-8") as fh:
                text = fh.read()
        data = json.loads(text) if text.strip() else []
        raw = data
    if isinstance(raw, dict):
        raw = raw.get("records") or raw.get("index") or []
    if not isinstance(raw, list):
        return []
    return raw


def normalize_records(raw):
    """Keep only records with a string slug; default an unknown class to
    'unfinished'. Preserves input order (the caller sorts newest-first)."""
    out = []
    for rec in raw:
        if not isinstance(rec, dict):
            continue
        slug = rec.get("slug")
        if not isinstance(slug, str) or not slug.strip():
            continue
        cls = rec.get("class")
        if cls not in CLASSES:
            cls = "unfinished"
        out.append({
            "slug": slug.strip(),
            "title": (rec.get("title") or "").strip(),
            "status": (rec.get("status") or "").strip(),
            "class": cls,
        })
    return out


def columns(records):
    """Group records into the four ordered columns."""
    return {cls: [r for r in records if r["class"] == cls] for cls in CLASSES}


def selectable_slugs(records):
    """Slugs that are ready-to-ship, in record order -- the only shippable set."""
    return [r["slug"] for r in records if r["class"] == SHIPPABLE]


def filter_shippable(records, requested):
    """Intersect a requested ship list with what is actually ready-to-ship,
    preserving the requested order and dropping duplicates. This is the guard:
    a slug that is not ready-to-ship can never be shipped."""
    ready = set(selectable_slugs(records))
    seen = set()
    out = []
    for slug in requested:
        slug = slug.strip()
        if slug in ready and slug not in seen:
            out.append(slug)
            seen.add(slug)
    return out


def can_select(rec):
    """A card is selectable (for ship/merge) only when it is ready-to-ship."""
    return rec is not None and rec["class"] == SHIPPABLE


def can_go(rec):
    """`g` (run/resume the pipeline) applies only to unfinished / in-progress."""
    return rec is not None and rec["class"] in GOABLE


def match_record(rec, text):
    """Case-insensitive substring match on slug + title. Empty text matches all."""
    if not text:
        return True
    t = text.lower()
    return t in rec["slug"].lower() or t in rec["title"].lower()


def filtered_columns(records, text):
    """Group into the four columns, keeping only records that match `text`."""
    return {cls: [r for r in records if r["class"] == cls and match_record(r, text)]
            for cls in CLASSES}


def clamp_index(idx, n):
    """Clamp a cursor row into [0, n-1]; 0 when the column is empty (so a filter
    that narrows a column always re-clamps the cursor onto a visible card)."""
    if n <= 0:
        return 0
    return max(0, min(idx, n - 1))


def build_intent(action, items):
    """The schema-v2 intent the board writes on any action."""
    return {"schema": INTENT_SCHEMA, "action": action, "items": list(items)}


def normalize_intent(data):
    """Read either a schema-v2 intent or the legacy {"ship":[...]} shape and
    return a normalized {"action", "items"} dict (back-compat for gogo-done)."""
    if isinstance(data, dict) and "ship" in data and "action" not in data:
        items = data.get("ship") or []
        return {"action": "ship", "items": list(items)}
    action = (data or {}).get("action", "ship")
    items = (data or {}).get("items") or []
    return {"action": action, "items": list(items)}


def resolve_action(action, focus_rec, selected, records):
    """Apply the per-class guards for an action and return (intent, hint): exactly
    one is non-None. A legal action -> (schema-v2 intent, None); a guard violation
    -> (None, "<one-line hint>"). This is the single source of truth the curses
    loop, the headless path, and the self-test all share.

    - view: any focused card.
    - go:   the focused card, only if unfinished / in-progress.
    - ship / ship-merged: the SELECTED ready-to-ship slugs (record order); the
      selection is independent of the current filter, so it survives filtering.
      merge needs >= 2.
    """
    ready = selectable_slugs(records)
    sel_ready = [s for s in ready if s in selected]  # record order, ready only
    if action == "view":
        if focus_rec is None:
            return None, "view: no card here"
        return build_intent("view", [focus_rec["slug"]]), None
    if action == "go":
        if focus_rec is None:
            return None, "go: no card here"
        if not can_go(focus_rec):
            return None, "go: only unfinished / in-progress cards"
        return build_intent("go", [focus_rec["slug"]]), None
    if action == "ship":
        if not sel_ready:
            return None, "ship: select ready-to-ship cards first (space)"
        return build_intent("ship", sel_ready), None
    if action == "ship-merged":
        if len(sel_ready) < 2:
            return None, "merge: select >= 2 ready-to-ship cards"
        return build_intent("ship-merged", sel_ready), None
    return None, "unknown action: %s" % action


def emit_intent(path, intent):
    """Write the schema-v2 intent to the result file (the skill's input)."""
    with open(path, "w", encoding="utf-8") as fh:
        json.dump(intent, fh, indent=2)
        fh.write("\n")


# --------------------------------------------------------------------------- #
# Interactive curses board.
# --------------------------------------------------------------------------- #

def _addstr(win, y, x, text, attr=0):
    """Clipping addstr -- never raises on small terminals or the bottom-right
    cell."""
    maxy, maxx = win.getmaxyx()
    if y < 0 or y >= maxy or x >= maxx:
        return
    if x < 0:
        text = text[-x:]
        x = 0
    avail = maxx - x
    if avail <= 0:
        return
    try:
        win.addstr(y, x, text[:avail], attr)
    except Exception:
        pass


def _board_loop(stdscr, records):
    """Run the curses event loop. Returns a schema-v2 intent dict on an action
    key, or None on cancel (q)."""
    import curses

    curses.curs_set(0)
    stdscr.keypad(True)

    order = CLASSES
    all_cols = columns(records)
    selected = set()                              # slugs (only ever ready-to-ship)
    filter_text = ""
    filter_edit = False
    status_hint = ""

    cur_col = 2 if all_cols[SHIPPABLE] else 0     # start on ready-to-ship if any
    cur_row = 0

    CARD_TOP = 6

    while True:
        fcols = filtered_columns(records, filter_text)
        cur_col = max(0, min(cur_col, len(order) - 1))
        cur_row = clamp_index(cur_row, len(fcols[order[cur_col]]))
        items_here = fcols[order[cur_col]]
        focus = items_here[cur_row] if items_here else None
        shown = sum(len(fcols[c]) for c in order)
        total = len(records)

        stdscr.erase()
        maxy, maxx = stdscr.getmaxyx()
        col_w = max(8, maxx // 4)
        card_rows = max(1, maxy - CARD_TOP - 1)

        _addstr(stdscr, 0, 0, "gogo done -- work board (cockpit)", curses.A_BOLD)
        _addstr(stdscr, 1, 0,
                "move: arrows/hjkl   select: space   "
                "view:v  ship:s  merge:m  go:g  filter:/  quit:q",
                curses.A_DIM)

        if filter_edit:
            fline = "filter (type; Enter=apply, Esc=clear): " + filter_text + "_"
        elif filter_text:
            fline = "filter: %s  (%d/%d shown)   Esc=clear" % (
                filter_text, shown, total)
        else:
            fline = ""
        _addstr(stdscr, 2, 0, fline, curses.A_DIM)

        for ci, cls in enumerate(order):
            x0 = ci * col_w
            items = fcols[cls]
            head = "%s (%d)" % (cls, len(items))
            _addstr(stdscr, 4, x0, head, curses.A_BOLD)
            _addstr(stdscr, 5, x0, "-" * (col_w - 1))

            start = 0
            if ci == cur_col and cur_row >= card_rows:
                start = cur_row - card_rows + 1

            for vi, rec in enumerate(items[start:start + card_rows]):
                idx = start + vi
                y = CARD_TOP + vi
                is_cursor = (ci == cur_col and idx == cur_row)
                if cls == SHIPPABLE:
                    box = "[x] " if rec["slug"] in selected else "[ ] "
                else:
                    box = "    "
                label = rec["slug"]
                if rec["title"]:
                    label = "%s -- %s" % (rec["slug"], rec["title"])
                cursor = "> " if is_cursor else "  "
                line = cursor + box + label
                attr = curses.A_REVERSE if is_cursor else 0
                if cls == SHIPPABLE and rec["slug"] in selected:
                    attr |= curses.A_BOLD
                _addstr(stdscr, y, x0, line[:col_w - 1], attr)

        count = "selected: %d" % len(selected)
        footer = ("%s   %s" % (count, status_hint)) if status_hint else count
        _addstr(stdscr, maxy - 1, 0, footer, curses.A_BOLD)

        stdscr.refresh()
        ch = stdscr.getch()
        status_hint = ""                          # transient: cleared on next key

        # ----- filter input mode: keys are literal text ---------------------- #
        if filter_edit:
            if ch in (10, 13, curses.KEY_ENTER):
                filter_edit = False
            elif ch == 27:                        # Esc: clear + leave edit
                filter_text = ""
                filter_edit = False
            elif ch in (curses.KEY_BACKSPACE, 127, 8):
                filter_text = filter_text[:-1]
            elif 32 <= ch <= 126:
                filter_text += chr(ch)
            continue

        # ----- command mode -------------------------------------------------- #
        if ch in (ord("q"), ord("Q")):            # cancel
            return None
        elif ch == ord("/"):                      # open the filter input line
            filter_edit = True
        elif ch == 27:                            # Esc clears an active filter
            if filter_text:
                filter_text = ""
        elif ch in (curses.KEY_LEFT, ord("h")):
            cur_col = max(0, cur_col - 1)
        elif ch in (curses.KEY_RIGHT, ord("l")):
            cur_col = min(len(order) - 1, cur_col + 1)
        elif ch in (curses.KEY_UP, ord("k")):
            cur_row = max(0, cur_row - 1)
        elif ch in (curses.KEY_DOWN, ord("j")):
            cur_row = min(max(0, len(items_here) - 1), cur_row + 1)
        elif ch in (ord(" "), curses.KEY_ENTER, 10, 13):  # toggle-select
            if can_select(focus):
                slug = focus["slug"]
                selected.discard(slug) if slug in selected else selected.add(slug)
            elif focus is not None:
                status_hint = "only ready-to-ship cards can be selected"
            else:
                status_hint = "no card here"
        elif ch in (ord("v"), ord("V")):
            intent, hint = resolve_action("view", focus, selected, records)
            if intent is not None:
                return intent
            status_hint = hint
        elif ch in (ord("s"), ord("S")):
            intent, hint = resolve_action("ship", focus, selected, records)
            if intent is not None:
                return intent
            status_hint = hint
        elif ch in (ord("m"), ord("M")):
            intent, hint = resolve_action("ship-merged", focus, selected, records)
            if intent is not None:
                return intent
            status_hint = hint
        elif ch in (ord("g"), ord("G")):
            intent, hint = resolve_action("go", focus, selected, records)
            if intent is not None:
                return intent
            status_hint = hint
        elif 32 <= ch <= 126:
            status_hint = "unknown key '%s' -- keys: v s m g space / q" % chr(ch)


def run_board(records, result_path):
    """Launch the curses board. On an action, emit the intent file and return 0;
    on cancel, write nothing and return 1; on any curses/loop failure, print one
    stderr line and return 2 (a crash is an ERROR, never a cancel -- exit-code
    contract: 0=confirmed, 1=cancel, 2=error)."""
    import curses

    try:
        intent = curses.wrapper(_board_loop, records)
    except Exception as exc:  # curses startup (no tty, bad TERM) or loop failure
        sys.stderr.write("board: cannot run the TUI: %s\n" % exc)
        return 2
    if intent is None:
        return 1
    emit_intent(result_path, intent)
    return 0


# --------------------------------------------------------------------------- #
# Headless self-test (no curses, no tty) -- the auto-testable path.
# --------------------------------------------------------------------------- #

def selftest():
    """Exercise classify -> guard -> intent (and the filter logic) without curses.
    Prints PASS/FAIL per check and returns 0 if all pass, 1 otherwise."""
    fixture = [
        {"slug": "alpha", "class": "shipped", "title": "Alpha"},
        {"slug": "bravo", "class": "ready-to-ship", "title": "Bravo"},
        {"slug": "charlie", "class": "in-progress", "title": "Charlie"},
        {"slug": "delta", "class": "ready-to-ship", "title": "Delta"},
        {"slug": "echo", "class": "unfinished", "title": "Echo"},
        {"slug": "foxtrot", "class": "bogus-class"},  # unknown -> unfinished
    ]
    recs = normalize_records(load_index(fixture))
    by = {r["slug"]: r for r in recs}
    checks = []

    def chk(name, cond):
        checks.append((name, bool(cond)))

    # -- classify / normalize (unchanged behaviour) -------------------------- #
    chk("normalize keeps 6 records with valid slugs", len(recs) == 6)
    chk("unknown class defaults to unfinished", by["foxtrot"]["class"] == "unfinished")
    chk("selectable == [bravo, delta]", selectable_slugs(recs) == ["bravo", "delta"])
    chk("guard filters non-ready slugs -> [bravo, delta]",
        filter_shippable(recs, ["bravo", "charlie", "alpha", "delta"]) == ["bravo", "delta"])
    chk("guard drops duplicates",
        filter_shippable(recs, ["bravo", "bravo"]) == ["bravo"])

    # -- per-class action guards (resolve_action) ---------------------------- #
    i, h = resolve_action("view", by["alpha"], set(), recs)
    chk("view on a shipped card -> intent view [alpha]",
        h is None and i == build_intent("view", ["alpha"]))
    i, h = resolve_action("view", by["echo"], set(), recs)
    chk("view on an unfinished card -> intent view [echo]",
        h is None and i == build_intent("view", ["echo"]))

    i, h = resolve_action("go", by["charlie"], set(), recs)
    chk("go on in-progress -> intent go [charlie]",
        h is None and i == build_intent("go", ["charlie"]))
    i, h = resolve_action("go", by["echo"], set(), recs)
    chk("go on unfinished -> intent go [echo]",
        h is None and i == build_intent("go", ["echo"]))
    i, h = resolve_action("go", by["bravo"], set(), recs)
    chk("go rejected on ready-to-ship (hint, no intent)", i is None and bool(h))
    i, h = resolve_action("go", by["alpha"], set(), recs)
    chk("go rejected on shipped (hint, no intent)", i is None and bool(h))

    i, h = resolve_action("ship", None, {"bravo"}, recs)
    chk("ship with a ready selection -> intent ship [bravo]",
        h is None and i == build_intent("ship", ["bravo"]))
    i, h = resolve_action("ship", None, set(), recs)
    chk("ship rejected on empty selection (hint, no intent)", i is None and bool(h))
    i, h = resolve_action("ship", None, {"bravo", "charlie"}, recs)
    chk("ship ignores a non-ready slug in the selection -> [bravo]",
        h is None and i == build_intent("ship", ["bravo"]))

    i, h = resolve_action("ship-merged", None, {"bravo"}, recs)
    chk("merge rejected with < 2 selected (hint, no intent)", i is None and bool(h))
    i, h = resolve_action("ship-merged", None, {"bravo", "delta"}, recs)
    chk("merge with >= 2 -> intent ship-merged [bravo, delta]",
        h is None and i == build_intent("ship-merged", ["bravo", "delta"]))
    i, h = resolve_action("ship-merged", None, {"bravo", "delta", "charlie"}, recs)
    chk("merge ignores a non-ready slug -> [bravo, delta]",
        h is None and i == build_intent("ship-merged", ["bravo", "delta"]))

    chk("selection survives filtering (ship uses full records, not the filter)",
        resolve_action("ship", None, {"bravo", "delta"}, recs)[0]
        == build_intent("ship", ["bravo", "delta"]))

    # -- filter logic -------------------------------------------------------- #
    narrowed = filtered_columns(recs, "bra")
    chk("filter 'bra' narrows ready-to-ship to [bravo]",
        [r["slug"] for r in narrowed[SHIPPABLE]] == ["bravo"])
    chk("filter 'bra' shows exactly 1 card overall",
        sum(len(narrowed[c]) for c in CLASSES) == 1)
    chk("filter is case-insensitive ('BRA' matches bravo)",
        match_record(by["bravo"], "BRA"))
    chk("filter matches on title ('harl' matches Charlie)",
        match_record(by["charlie"], "harl"))
    chk("clearing the filter ('') shows all 6",
        sum(len(filtered_columns(recs, "")[c]) for c in CLASSES) == 6)
    chk("cursor re-clamps when a filter narrows a column", clamp_index(5, 1) == 0)
    chk("cursor clamps to 0 on an empty column", clamp_index(3, 0) == 0)

    # -- intent emission shape (schema v2) + legacy back-compat -------------- #
    chk("build_intent shape is schema v2",
        build_intent("ship", ["bravo", "delta"])
        == {"schema": 2, "action": "ship", "items": ["bravo", "delta"]})
    chk("empty ship intent -> items []",
        build_intent("ship", []) == {"schema": 2, "action": "ship", "items": []})
    chk("legacy {'ship':[...]} parses as action ship",
        normalize_intent({"ship": ["bravo", "delta"]})
        == {"action": "ship", "items": ["bravo", "delta"]})
    chk("schema-v2 intent normalizes to itself",
        normalize_intent(build_intent("ship-merged", ["bravo", "delta"]))
        == {"action": "ship-merged", "items": ["bravo", "delta"]})

    tmpdir = tempfile.mkdtemp(prefix="gogo-board-selftest-")
    try:
        p1 = os.path.join(tmpdir, "result.json")
        emit_intent(p1, build_intent("ship-merged", ["bravo", "delta"]))
        with open(p1, encoding="utf-8") as fh:
            data = json.load(fh)
        chk("emit_intent round-trips a schema-v2 intent",
            data == {"schema": 2, "action": "ship-merged", "items": ["bravo", "delta"]})

        p2 = os.path.join(tmpdir, "view.json")
        emit_intent(p2, build_intent("view", ["alpha"]))
        with open(p2, encoding="utf-8") as fh:
            vdata = json.load(fh)
        chk("emit_intent writes a view intent",
            vdata == {"schema": 2, "action": "view", "items": ["alpha"]})
    finally:
        for name in ("result.json", "view.json"):
            try:
                os.remove(os.path.join(tmpdir, name))
            except OSError:
                pass
        try:
            os.rmdir(tmpdir)
        except OSError:
            pass

    ok = all(passed for _, passed in checks)
    for name, passed in checks:
        print(("PASS" if passed else "FAIL") + " - " + name)
    print("selftest: %s (%d/%d)" % (
        "PASS" if ok else "FAIL",
        sum(1 for _, p in checks if p), len(checks)))
    return 0 if ok else 1


# --------------------------------------------------------------------------- #
# CLI.
# --------------------------------------------------------------------------- #

def _headless(records, result_path, action, requested):
    """Emit a schema-v2 intent without curses, applying the same per-class guards.
    Returns the process exit code (0 action / 1 cancel / 2 guard violation)."""
    if action == "cancel":
        return 1
    if action in ("ship", "ship-merged"):
        items = filter_shippable(records, requested)   # ready-to-ship guard
        if action == "ship-merged" and len(items) < 2:
            print("board: ship-merged needs >= 2 ready-to-ship slugs",
                  file=sys.stderr)
            return 2
        emit_intent(result_path, build_intent(action, items))
        return 0
    # view / go are single-focus: take the first requested slug.
    if not requested:
        print("board: --action %s needs a slug in --ship" % action, file=sys.stderr)
        return 2
    slug = requested[0].strip()
    rec = next((r for r in records if r["slug"] == slug), None)
    if rec is None:
        print("board: unknown slug '%s'" % slug, file=sys.stderr)
        return 2
    if action == "go" and not can_go(rec):
        print("board: go only applies to unfinished / in-progress (%s is %s)"
              % (slug, rec["class"]), file=sys.stderr)
        return 2
    emit_intent(result_path, build_intent(action, [slug]))
    return 0


def main(argv=None):
    parser = argparse.ArgumentParser(
        description="Interactive terminal kanban cockpit for /gogo:done (selector only).")
    parser.add_argument("--index", default="-",
                        help="work-index JSON path, or '-' for stdin (default)")
    parser.add_argument("--result",
                        help="path to write the schema-v2 intent result file")
    parser.add_argument("--action", default="ship", choices=ACTIONS,
                        help="headless intent action (default: ship, for back-compat)")
    parser.add_argument("--ship", default=None,
                        help="comma-separated slugs for the headless intent items")
    parser.add_argument("--headless", action="store_true",
                        help="emit the intent file without curses (uses --action/--ship)")
    parser.add_argument("--selftest", action="store_true",
                        help="run the headless self-test and exit")
    args = parser.parse_args(argv)

    if args.selftest:
        return selftest()

    # Guard the load: a missing/malformed --index (or stdin) must not dump a
    # traceback. FileNotFoundError is an OSError; json.JSONDecodeError is a
    # ValueError -- so (OSError, ValueError) covers both. Exit 2 (a distinct code
    # the skill tells apart from a cancel's 1) with a one-line reason on stderr.
    try:
        records = normalize_records(load_index(args.index))
    except (OSError, ValueError) as exc:
        src = args.index if args.index not in (None, "-") else "<stdin>"
        print("board: cannot read work-index %s: %s" % (src, exc), file=sys.stderr)
        return 2

    if args.headless or args.ship is not None:
        if args.action != "cancel" and not args.result:
            parser.error("--result is required to write the intent")
        requested = [s for s in (args.ship or "").split(",") if s.strip()]
        return _headless(records, args.result, args.action, requested)

    if not args.result:
        parser.error("--result is required for the interactive board")
    return run_board(records, args.result)


if __name__ == "__main__":
    sys.exit(main())
