#!/usr/bin/env python3
"""gogo xplan board server -- python3 standard library only, localhost only.

Serves the committed React board (dist/) plus two API routes, and the pre-built
/gogo:view pages so the board's "view" links open offline. It is a vendored
executable per gogo's coding-rules: pure stdlib (no pip / no network), pure
ASCII, ships a --selftest, and exposes a documented exit-code contract the
calling skill (skills/gogo-xplan) branches on.

Routes:
  GET  /                     -> dist/index.html
  GET  /<asset>              -> static file from --dist (path-traversal safe)
  GET  /view/<name>.html     -> a pre-built page from <view-root>
  GET  /viewer/<file>        -> from <view-root>'s parent (so a page's ../viewer/*)
  GET  /mermaid.min.js       -> from <view-root>'s parent (so a page's ../mermaid.min.js)
  GET  /api/board            -> the JSON file <data>/board.json (re-read each request)
  POST /api/ship             -> validate + write <data>/ship-intent.json (atomic)

/api/ship responses: 202 accepted (intent written) / 400 bad shape or a slug
that is not ready-to-ship / 409 an unprocessed intent already exists / 500 write
failed. The board polls /api/board; the orchestrator watches ship-intent.json,
ships via the gogo-done writer, rebuilds board.json, and the card moves live.

Exit codes:
  0  normal shutdown, or --selftest passed
  2  bad args (e.g. --dist not a built board) or --selftest failed
Errors print a single line to stderr.
"""

import argparse
import json
import os
import posixpath
import re
import signal
import sys
import tempfile
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import unquote, urlparse

HOST = "127.0.0.1"
SLUG_RE = re.compile(r"^[a-z0-9]+(-[a-z0-9]+)*$")
VALID_ACTIONS = ("ship", "ship-merged")
MAX_BODY = 1000000
# Only these host-names may address the server (DNS-rebinding guard). It binds
# 127.0.0.1 only, so its own origin is always one of these.
ALLOWED_HOSTS = ("127.0.0.1", "localhost")

CONTENT_TYPES = {
    ".html": "text/html; charset=utf-8",
    ".js": "text/javascript; charset=utf-8",
    ".mjs": "text/javascript; charset=utf-8",
    ".css": "text/css; charset=utf-8",
    ".json": "application/json; charset=utf-8",
    ".map": "application/json; charset=utf-8",
    ".svg": "image/svg+xml",
    ".png": "image/png",
    ".jpg": "image/jpeg",
    ".jpeg": "image/jpeg",
    ".gif": "image/gif",
    ".ico": "image/x-icon",
    ".webp": "image/webp",
    ".woff": "font/woff",
    ".woff2": "font/woff2",
    ".ttf": "font/ttf",
    ".txt": "text/plain; charset=utf-8",
}


def content_type(path):
    return CONTENT_TYPES.get(Path(path).suffix.lower(), "application/octet-stream")


def _unlink_quiet(path):
    try:
        os.unlink(path)
    except OSError:
        pass


def host_allowed(host_header):
    """True if the Host header's host-part is a localhost name (DNS-rebind guard).

    Tolerates an optional :port ("127.0.0.1:4173") but not IPv6 literals -- the
    server binds 127.0.0.1 only, so a real browser Host is 127.0.0.1/localhost.
    """
    if not host_header:
        return False
    host = host_header.strip()
    if host.count(":") == 1:  # drop a trailing :port; leave IPv6 (many colons)
        host = host.split(":", 1)[0]
    return host.lower() in ALLOWED_HOSTS


def origin_allowed(origin_header):
    """True if the Origin is absent OR a localhost origin (same-origin as us).

    Blocks a cross-site page from POSTing a ship (CSRF): its Origin host would be
    e.g. evil.example, which is not in the localhost family the server serves.
    """
    if not origin_header:
        return True
    o_host = urlparse(origin_header).hostname
    if not o_host:
        return False
    return o_host.lower() in ALLOWED_HOSTS


def safe_path(root, urlpath):
    """Map a URL path to a file under root, refusing traversal.

    Returns a resolved Path guaranteed to sit inside root, or None if the request
    escapes it. Both %2e-encoded and literal ../ sequences are neutralized.
    """
    raw = unquote(urlpath)
    norm = posixpath.normpath("/" + raw.lstrip("/"))
    rel = norm.lstrip("/")
    base = Path(root)
    candidate = base if rel in ("", ".") else base / rel
    try:
        root_resolved = base.resolve()
        cand_resolved = candidate.resolve()
    except (OSError, RuntimeError, ValueError):
        return None
    if cand_resolved == root_resolved or root_resolved in cand_resolved.parents:
        return cand_resolved
    return None


def load_board_map(board_file):
    """Read board.json -> {slug: item}. None if unreadable/invalid JSON."""
    try:
        data = json.loads(Path(board_file).read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return None
    items = data.get("items") if isinstance(data, dict) else None
    out = {}
    if isinstance(items, list):
        for it in items:
            if isinstance(it, dict) and isinstance(it.get("slug"), str):
                out[it["slug"]] = it
    return out


def ready_slugs(board_map):
    """Slugs the board considers shippable (ready-to-ship / ready column)."""
    return set(
        s
        for s, it in board_map.items()
        if it.get("column") == "ready" or it.get("class") == "ready-to-ship"
    )


def validate_intent(raw, board_file):
    """Validate a POST /api/ship body against board.json.

    Returns (True, intent_dict) on success or (False, error_message). The
    intent is only accepted when every slug is currently ready-to-ship.
    """
    try:
        obj = json.loads(raw)
    except ValueError:
        return False, "body is not valid JSON"
    if not isinstance(obj, dict):
        return False, "intent must be a JSON object"
    if obj.get("schema") != 2:
        return False, "schema must be 2"
    action = obj.get("action")
    if action not in VALID_ACTIONS:
        return False, "action must be one of " + ", ".join(VALID_ACTIONS)
    items = obj.get("items")
    if not isinstance(items, list) or not items:
        return False, "items must be a non-empty array"
    if not all(isinstance(s, str) and SLUG_RE.match(s) for s in items):
        return False, "items must be kebab-case slug strings"
    if len(set(items)) != len(items):
        return False, "items must be unique"
    if action == "ship" and len(items) != 1:
        return False, "ship needs exactly one item"
    if action == "ship-merged" and len(items) < 2:
        return False, "ship-merged needs at least two items"
    board_map = load_board_map(board_file)
    if board_map is None:
        return False, "board.json is unavailable"
    ready = ready_slugs(board_map)
    not_ready = [s for s in items if s not in ready]
    if not_ready:
        return False, "not ready-to-ship: " + ", ".join(not_ready)
    return True, {"schema": 2, "action": action, "items": list(items)}


class BoardHandler(BaseHTTPRequestHandler):
    # Set as class attributes before serve_forever (all resolved absolute paths).
    dist_dir = ""
    resources_root = ""
    data_dir = ""
    server_version = "gogo-xplan/1.0"

    def log_message(self, fmt, *args):  # quiet -- errors go to stderr explicitly
        return

    # ---- helpers -------------------------------------------------------
    def _send_json(self, code, obj):
        body = json.dumps(obj).encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        if self.command != "HEAD":
            self.wfile.write(body)

    def _send_bytes(self, code, body, ctype):
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        if self.command != "HEAD":
            self.wfile.write(body)

    # ---- routing -------------------------------------------------------
    def _host_ok(self):
        """Reject a non-localhost Host with 403 (DNS-rebind guard). True == pass."""
        if host_allowed(self.headers.get("Host", "")):
            return True
        self._send_json(403, {"error": "forbidden host"})
        return False

    def do_GET(self):
        if not self._host_ok():
            return
        path = urlparse(self.path).path
        if path == "/api/board":
            return self._serve_board()
        if not self._serve_static(path):
            self._send_json(404, {"error": "not found"})

    def do_HEAD(self):
        self.do_GET()

    def do_POST(self):
        if not self._host_ok():
            return
        if urlparse(self.path).path == "/api/ship":
            # A ship additionally requires a same-origin (or absent) Origin so a
            # cross-site page can't drive it (CSRF).
            if not origin_allowed(self.headers.get("Origin", "")):
                return self._send_json(403, {"error": "forbidden origin"})
            return self._serve_ship()
        self._send_json(404, {"error": "not found"})

    # ---- handlers ------------------------------------------------------
    def _serve_board(self):
        board_file = Path(self.data_dir) / "board.json"
        try:
            data = board_file.read_bytes()
        except OSError:
            return self._send_json(404, {"error": "no board.json yet"})
        self._send_bytes(200, data, "application/json; charset=utf-8")

    def _serve_static(self, path):
        # /view/*, /viewer/*, and /mermaid.min.js resolve against the resources
        # root (the parent of --view-root) so a page opened under /view/ can reach
        # its ../mermaid.min.js and ../viewer/* siblings. Everything else is the
        # React dist. Returns True if it produced a response (incl. 403).
        if (
            path == "/view"
            or path.startswith("/view/")
            or path == "/viewer"
            or path.startswith("/viewer/")
            or path == "/mermaid.min.js"
        ):
            root = self.resources_root
        else:
            root = self.dist_dir
        target = safe_path(root, path)
        if target is None:
            self._send_json(403, {"error": "forbidden"})
            return True
        if target.is_dir():
            target = target / "index.html"
        if not target.is_file():
            if root == self.dist_dir and path in ("/", ""):
                target = Path(self.dist_dir) / "index.html"
            if not target.is_file():
                return False
        try:
            body = target.read_bytes()
        except OSError:
            return False
        self._send_bytes(200, body, content_type(str(target)))
        return True

    def _serve_ship(self):
        try:
            length = int(self.headers.get("Content-Length", 0) or 0)
        except ValueError:
            length = 0
        if length <= 0 or length > MAX_BODY:
            return self._send_json(400, {"error": "empty or oversized body"})
        raw = self.rfile.read(length)
        ok, result = validate_intent(raw, Path(self.data_dir) / "board.json")
        if not ok:
            return self._send_json(400, {"error": result})
        intent = result
        intent_file = Path(self.data_dir) / "ship-intent.json"
        intent["received"] = time.strftime("%Y-%m-%dT%H:%M:%S")
        # Write the payload to a UNIQUE tmp first (mkstemp), so two concurrent
        # writers never share or clobber a fixed tmp name.
        try:
            os.makedirs(self.data_dir, exist_ok=True)
            fd, tmp_name = tempfile.mkstemp(prefix="ship-intent-", suffix=".tmp", dir=self.data_dir)
            with os.fdopen(fd, "w", encoding="ascii") as fh:
                fh.write(json.dumps(intent) + "\n")
        except OSError as exc:
            sys.stderr.write("gogo-board: cannot write intent: %s\n" % exc)
            return self._send_json(500, {"error": "cannot write intent"})
        # Atomic "already pending" gate: O_CREAT|O_EXCL on the rename target itself
        # means two truly-concurrent valid POSTs can't both claim it (no
        # check-then-write TOCTOU); the loser gets a clean 409.
        try:
            os.close(os.open(str(intent_file), os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o644))
        except FileExistsError:
            _unlink_quiet(tmp_name)
            return self._send_json(409, {"error": "a ship intent is already pending"})
        except OSError as exc:
            _unlink_quiet(tmp_name)
            sys.stderr.write("gogo-board: cannot write intent: %s\n" % exc)
            return self._send_json(500, {"error": "cannot write intent"})
        # We hold the claim -> replace the empty claim file with the real content.
        try:
            os.replace(tmp_name, str(intent_file))
        except OSError as exc:
            _unlink_quiet(tmp_name)
            _unlink_quiet(str(intent_file))
            sys.stderr.write("gogo-board: cannot write intent: %s\n" % exc)
            return self._send_json(500, {"error": "cannot write intent"})
        self._send_json(202, {"status": "accepted", "action": intent["action"], "items": intent["items"]})


def make_server(host, start_port, tries):
    """Bind the first free port at or after start_port. Returns (server, port)."""
    last = None
    for port in range(start_port, start_port + tries):
        try:
            return ThreadingHTTPServer((host, port), BoardHandler), port
        except OSError as exc:
            last = exc
    raise last if last is not None else OSError("no free port")


def run_selftest():
    """Exercise the pure guards + path safety with no network. True == pass."""
    import shutil

    ok = True

    def check(name, cond):
        nonlocal ok
        if not cond:
            sys.stderr.write("selftest FAIL: %s\n" % name)
            ok = False

    tmp = Path(tempfile.mkdtemp(prefix="gogo-board-selftest-"))
    try:
        board = {
            "repo": "demo",
            "generated": "2026-07-02",
            "columns": [{"id": "ready", "name": "ready"}, {"id": "plan", "name": "plan"}],
            "items": [
                {"slug": "alpha", "title": "A", "class": "ready-to-ship", "column": "ready"},
                {"slug": "beta", "title": "B", "class": "ready-to-ship", "column": "ready"},
                {"slug": "gamma", "title": "G", "class": "unfinished", "column": "plan"},
            ],
        }
        bf = tmp / "board.json"
        bf.write_text(json.dumps(board), encoding="utf-8")

        def intent(**kw):
            return json.dumps(kw).encode("utf-8")

        # ---- intent validation --------------------------------------
        okv, res = validate_intent(intent(schema=2, action="ship", items=["alpha"]), bf)
        check("valid single ship", okv and res["items"] == ["alpha"] and res["action"] == "ship")
        check("valid merged ship", validate_intent(intent(schema=2, action="ship-merged", items=["alpha", "beta"]), bf)[0])
        check("reject bad schema", not validate_intent(intent(schema=1, action="ship", items=["alpha"]), bf)[0])
        check("reject bad action", not validate_intent(intent(schema=2, action="delete", items=["alpha"]), bf)[0])
        check("reject empty items", not validate_intent(intent(schema=2, action="ship", items=[]), bf)[0])
        check("reject non-ready slug", not validate_intent(intent(schema=2, action="ship", items=["gamma"]), bf)[0])
        check("reject unknown slug", not validate_intent(intent(schema=2, action="ship", items=["nope"]), bf)[0])
        check("reject duplicate slugs", not validate_intent(intent(schema=2, action="ship-merged", items=["alpha", "alpha"]), bf)[0])
        check("reject merged-with-one", not validate_intent(intent(schema=2, action="ship-merged", items=["alpha"]), bf)[0])
        check("reject ship-with-two", not validate_intent(intent(schema=2, action="ship", items=["alpha", "beta"]), bf)[0])
        check("reject bad slug chars", not validate_intent(intent(schema=2, action="ship", items=["../etc"]), bf)[0])
        check("reject non-json", not validate_intent(b"not json", bf)[0])

        # ---- ready-set derivation -----------------------------------
        check("ready set", ready_slugs(load_board_map(bf)) == {"alpha", "beta"})
        check("missing board -> None", load_board_map(tmp / "nope.json") is None)
        check("missing board fails intent", not validate_intent(intent(schema=2, action="ship", items=["alpha"]), tmp / "nope.json")[0])

        # ---- path-traversal safety ----------------------------------
        root = tmp / "root"
        root.mkdir()
        (root / "ok.txt").write_text("hi", encoding="ascii")
        rr = root.resolve()
        check("safe within root", safe_path(str(root), "/ok.txt") == (rr / "ok.txt"))
        check("literal traversal contained", str(safe_path(str(root), "/../../etc/passwd")).startswith(str(rr)))
        check("encoded traversal contained", str(safe_path(str(root), "/%2e%2e/%2e%2e/etc/passwd")).startswith(str(rr)))
        check("view-prefixed traversal contained", str(safe_path(str(root), "/view/../../../etc/passwd")).startswith(str(rr)))
        check("root maps to root", safe_path(str(root), "/") == rr)

        # ---- Host-header allowlist (DNS-rebinding guard) -------------
        check("host allow 127.0.0.1", host_allowed("127.0.0.1"))
        check("host allow 127.0.0.1:port", host_allowed("127.0.0.1:4173"))
        check("host allow localhost:port", host_allowed("localhost:4173"))
        check("host reject empty", not host_allowed(""))
        check("host reject evil", not host_allowed("evil.example"))
        check("host reject evil:port", not host_allowed("evil.example:4173"))

        # ---- Origin check on ship (CSRF guard) ----------------------
        check("origin absent ok", origin_allowed(""))
        check("origin self ok", origin_allowed("http://127.0.0.1:4173"))
        check("origin localhost ok", origin_allowed("http://localhost:4173"))
        check("origin evil rejected", not origin_allowed("http://evil.example"))
        check("origin unparseable rejected", not origin_allowed("garbage"))
    finally:
        shutil.rmtree(tmp, ignore_errors=True)

    sys.stdout.write("selftest %s\n" % ("PASS" if ok else "FAIL"))
    return ok


def main(argv):
    ap = argparse.ArgumentParser(description="gogo xplan board server (stdlib, localhost only).")
    ap.add_argument("--port", type=int, default=4173, help="preferred port; the next free port is used if busy")
    ap.add_argument("--dist", default="", help="the built React dist/ directory (contains index.html)")
    ap.add_argument("--data", default=".gogo/resources/xplan-board", help="runtime data dir (board.json, ship-intent.json, server.pid)")
    ap.add_argument("--view-root", default=".gogo/resources/view", help="dir served under /view/ (its parent holds mermaid.min.js + viewer/)")
    ap.add_argument("--selftest", action="store_true", help="run offline self-tests and exit (0 pass / 2 fail)")
    args = ap.parse_args(argv)

    if args.selftest:
        return 0 if run_selftest() else 2

    dist = Path(args.dist)
    if not args.dist or not (dist / "index.html").is_file():
        sys.stderr.write("gogo-board: --dist must point at a built React dist/ (index.html not found) -- run `npm run build`\n")
        return 2

    view_root = Path(args.view_root)
    resources_root = view_root.parent
    data_dir = Path(args.data)
    try:
        os.makedirs(data_dir, exist_ok=True)
    except OSError as exc:
        sys.stderr.write("gogo-board: cannot create data dir %s: %s\n" % (data_dir, exc))
        return 2

    BoardHandler.dist_dir = str(dist.resolve())
    BoardHandler.resources_root = str(resources_root.resolve())
    BoardHandler.data_dir = str(data_dir.resolve())

    try:
        httpd, port = make_server(HOST, args.port, 25)
    except OSError as exc:
        sys.stderr.write("gogo-board: cannot bind a port near %d: %s\n" % (args.port, exc))
        return 2

    pid_file = data_dir / "server.pid"
    try:
        pid_file.write_text("%d\n" % os.getpid(), encoding="ascii")
    except OSError:
        pass

    # A plain `kill <pid>` (SIGTERM) unwinds like Ctrl-C so the pid file is
    # cleaned up in the finally below.
    def _term(_signum, _frame):
        raise KeyboardInterrupt

    signal.signal(signal.SIGTERM, _term)

    sys.stdout.write("gogo board: http://%s:%d\n" % (HOST, port))
    sys.stdout.flush()
    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        httpd.server_close()
        try:
            pid_file.unlink()
        except OSError:
            pass
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
