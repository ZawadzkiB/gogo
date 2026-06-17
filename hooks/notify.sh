#!/usr/bin/env bash
# gogo pipeline — notification hook.
#
# Wired to Claude Code's "Notification" event (see hooks/hooks.json). It fires
# when the agent pauses for you — e.g. to accept a plan or answer a decision gate
# — so you get pinged even if away from the terminal. Runs locally; no remote
# session needed.
#
# Phone push via ntfy.sh (free, no account): set a SECRET topic name and
# subscribe to it in the ntfy app (iOS/Android) or at https://ntfy.sh/<topic>.
#
#   export GOGO_NTFY_TOPIC="your-secret-topic-9f3a"   # in your shell profile
#
# Without GOGO_NTFY_TOPIC set, this falls back to a local macOS banner (if
# available) and is otherwise a silent no-op — safe to leave installed.

set -euo pipefail

# Claude Code passes the hook event as JSON on stdin; pull the message if jq is
# present, else use a generic title.
payload="$(cat 2>/dev/null || true)"
msg=""
if command -v jq >/dev/null 2>&1 && [ -n "$payload" ]; then
  msg="$(printf '%s' "$payload" | jq -r '.message // empty' 2>/dev/null || true)"
fi
[ -n "$msg" ] || msg="gogo needs your input"
title="gogo • ${PWD##*/}"

# 1) Phone push via ntfy (if a topic is configured)
if [ -n "${GOGO_NTFY_TOPIC:-}" ]; then
  curl -fsS \
    -H "Title: $title" \
    -H "Tags: bell" \
    -d "$msg" \
    "https://ntfy.sh/${GOGO_NTFY_TOPIC}" >/dev/null 2>&1 || true
fi

# 2) Local macOS banner (best-effort; harmless elsewhere)
if command -v osascript >/dev/null 2>&1; then
  osascript -e "display notification \"${msg//\"/\\\"}\" with title \"${title//\"/\\\"}\"" >/dev/null 2>&1 || true
fi

# 3) Terminal bell
printf '\a' >/dev/tty 2>/dev/null || true

exit 0
