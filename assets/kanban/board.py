#!/usr/bin/env python3
"""gogo done board -- an interactive terminal kanban for /gogo:done.

Reads a work-index (records classified by the gogo-status Step A classifier),
shows four columns -- unfinished, in-progress, ready-to-ship, shipped -- lets the
user select ready-to-ship cards and confirm "ship", then writes the chosen slugs
to a result file as {"ship": ["<slug>", ...]}.

Exit-code contract (the gogo-done skill branches on these):
  0  confirmed  -- result file written (possibly {"ship": []} for an empty pick).
  1  cancel     -- user quit (q/ESC); no result file written.
  2  error      -- cannot read the work-index (missing/malformed --index or stdin),
                   or the board cannot start. No result file; a one-line reason on
                   stderr and NO traceback. The skill treats 2 like a launch
                   failure -> its status-table fallback, never a cancel.

This is a selector/visualizer only: it never copies, archives, or mutates any
gogo state. The actual shipping stays single-sourced in the gogo-done skill,
which reads the result file and runs its existing archive+link flow per slug.

Pure stdlib (curses, json, sys, argparse, tempfile, os). No network. Offline.

Interactive curses needs a tty, so a headless self-test path (--selftest)
exercises the classify -> select -> emit logic WITHOUT curses, for automated
verification. A --headless --ship <slugs> path emits a result file without curses
too (used by the skill's degradation and by smoke tests).
"""

import argparse
import json
import os
import sys
import tempfile

# The four work-index classes, left-to-right in pipeline order. Only cards in
# SHIPPABLE are selectable to ship; the rest are shown read-only for context.
CLASSES = ["unfinished", "in-progress", "ready-to-ship", "shipped"]
SHIPPABLE = "ready-to-ship"


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


def emit_result(path, ship_slugs):
    """Write the result file as {"ship": ["<slug>", ...]} (the skill's input)."""
    with open(path, "w", encoding="utf-8") as fh:
        json.dump({"ship": list(ship_slugs)}, fh, indent=2)
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
    """Run the curses event loop. Returns the chosen ship-slug list on confirm,
    or None on cancel."""
    import curses

    curses.curs_set(0)
    stdscr.keypad(True)

    cols = columns(records)
    order = CLASSES
    selected = set()

    cur_col = 2 if cols[SHIPPABLE] else 0  # start on ready-to-ship if any
    cur_row = 0

    CARD_TOP = 5

    def clamp_row():
        nonlocal cur_row
        n = len(cols[order[cur_col]])
        cur_row = 0 if n == 0 else max(0, min(cur_row, n - 1))

    clamp_row()

    while True:
        stdscr.erase()
        maxy, maxx = stdscr.getmaxyx()
        col_w = max(8, maxx // 4)
        card_rows = max(1, maxy - CARD_TOP - 1)

        _addstr(stdscr, 0, 0, "gogo done -- work board", curses.A_BOLD)
        _addstr(stdscr, 1, 0,
                "move: arrows/hjkl   select: space/enter   ship: s   cancel: q",
                curses.A_DIM)

        for ci, cls in enumerate(order):
            x0 = ci * col_w
            items = cols[cls]
            head = "%s (%d)" % (cls, len(items))
            _addstr(stdscr, 3, x0, head, curses.A_BOLD)
            _addstr(stdscr, 4, x0, "-" * (col_w - 1))

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

        footer = "selected to ship: %d" % len(selected)
        _addstr(stdscr, maxy - 1, 0, footer, curses.A_BOLD)

        stdscr.refresh()
        ch = stdscr.getch()

        if ch in (ord("q"), ord("Q"), 27):            # cancel
            return None
        if ch in (curses.KEY_LEFT, ord("h")):
            cur_col = max(0, cur_col - 1)
            clamp_row()
        elif ch in (curses.KEY_RIGHT, ord("l")):
            cur_col = min(len(order) - 1, cur_col + 1)
            clamp_row()
        elif ch in (curses.KEY_UP, ord("k")):
            cur_row = max(0, cur_row - 1)
        elif ch in (curses.KEY_DOWN, ord("j")):
            n = len(cols[order[cur_col]])
            cur_row = min(max(0, n - 1), cur_row + 1)
        elif ch in (ord(" "), curses.KEY_ENTER, 10, 13):  # toggle-select
            items = cols[order[cur_col]]
            if order[cur_col] == SHIPPABLE and items:
                slug = items[cur_row]["slug"]
                if slug in selected:
                    selected.discard(slug)
                else:
                    selected.add(slug)
        elif ch in (ord("s"), ord("S")):             # confirm ship
            return [r["slug"] for r in cols[SHIPPABLE] if r["slug"] in selected]


def run_board(records, result_path):
    """Launch the curses board. On confirm, emit the result file and return 0;
    on cancel, write nothing and return 1."""
    import curses

    chosen = curses.wrapper(_board_loop, records)
    if chosen is None:
        return 1
    emit_result(result_path, chosen)
    return 0


# --------------------------------------------------------------------------- #
# Headless self-test (no curses, no tty) -- the auto-testable path.
# --------------------------------------------------------------------------- #

def selftest():
    """Exercise classify -> select -> emit without curses. Prints PASS/FAIL per
    check and returns 0 if all pass, 1 otherwise."""
    fixture = [
        {"slug": "alpha", "class": "shipped", "title": "Alpha"},
        {"slug": "bravo", "class": "ready-to-ship", "title": "Bravo"},
        {"slug": "charlie", "class": "in-progress", "title": "Charlie"},
        {"slug": "delta", "class": "ready-to-ship", "title": "Delta"},
        {"slug": "echo", "class": "unfinished", "title": "Echo"},
        {"slug": "foxtrot", "class": "bogus-class"},  # unknown -> unfinished
    ]
    recs = normalize_records(load_index(fixture))
    checks = []

    checks.append(("normalize keeps 6 records with valid slugs", len(recs) == 6))
    checks.append(("unknown class defaults to unfinished",
                   next(r for r in recs if r["slug"] == "foxtrot")["class"] == "unfinished"))
    checks.append(("selectable == [bravo, delta]",
                   selectable_slugs(recs) == ["bravo", "delta"]))

    # guard: a non-ready slug (charlie/alpha) can never be shipped
    shipped = filter_shippable(recs, ["bravo", "charlie", "alpha", "delta"])
    checks.append(("guard filters non-ready slugs -> [bravo, delta]",
                   shipped == ["bravo", "delta"]))
    checks.append(("guard drops duplicates",
                   filter_shippable(recs, ["bravo", "bravo"]) == ["bravo"]))

    tmpdir = tempfile.mkdtemp(prefix="gogo-board-selftest-")
    try:
        p1 = os.path.join(tmpdir, "result.json")
        emit_result(p1, shipped)
        with open(p1, encoding="utf-8") as fh:
            data = json.load(fh)
        checks.append(('emit shape {"ship":[...]}',
                       data == {"ship": ["bravo", "delta"]}))

        p2 = os.path.join(tmpdir, "empty.json")
        emit_result(p2, [])
        with open(p2, encoding="utf-8") as fh:
            empty = json.load(fh)
        checks.append(('empty selection -> {"ship": []}', empty == {"ship": []}))
    finally:
        for name in ("result.json", "empty.json"):
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
    print("selftest: " + ("PASS" if ok else "FAIL"))
    return 0 if ok else 1


# --------------------------------------------------------------------------- #
# CLI.
# --------------------------------------------------------------------------- #

def main(argv=None):
    parser = argparse.ArgumentParser(
        description="Interactive terminal kanban for /gogo:done (selector only).")
    parser.add_argument("--index", default="-",
                        help="work-index JSON path, or '-' for stdin (default)")
    parser.add_argument("--result",
                        help="path to write the {\"ship\":[...]} result file")
    parser.add_argument("--ship", default=None,
                        help="comma-separated slugs to ship headlessly (no curses)")
    parser.add_argument("--headless", action="store_true",
                        help="emit the result file without curses (needs --ship)")
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
        if not args.result:
            parser.error("--result is required to write the ship list")
        requested = [s for s in (args.ship or "").split(",") if s.strip()]
        emit_result(args.result, filter_shippable(records, requested))
        return 0

    if not args.result:
        parser.error("--result is required for the interactive board")
    return run_board(records, args.result)


if __name__ == "__main__":
    sys.exit(main())
