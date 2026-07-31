#!/usr/bin/env bash
# gogo pipeline — notification hook.
#
# Wired to Claude Code's "Notification" event (see hooks/hooks.json). It pings
# you ONLY when you are genuinely needed — a blocking prompt from Claude Code,
# or a gogo work item parked at a user gate (plan acceptance, a decision gate,
# UAT). Routine pipeline churn — a reviewer/tester subagent finishing a phase
# hand-off (`agent_completed`), lifecycle noise — stays silent.
#
# The decision, per stdin payload `notification_type`:
#   notify  — agent_needs_input · worker_permission_prompt · idle_prompt,
#             plus any type containing "permission" (never swallow a prompt)
#   silent  — elicitation_* · auth_success · computer_use_* · push_notification
#   gate    — agent_completed and any unknown/absent type: notify only if a
#             .gogo/work/*/state.md sits at a user gate (see GOGO_GATE_STATUSES;
#             mirrors the CLI's contract.WaitingForInput(), incl. the authoring
#             carve-out: an awaiting-plan-acceptance item with no written
#             plan.md is not a gate) — and only when that gate NEWLY opened:
#             the last-notified gate set is remembered in
#             .gogo/resources/notify/gates (D4, edge-latch; an already-
#             gitignored root), so a parked gate pings once, not per event.
#
# Knobs (env):
#   GOGO_NTFY_TOPIC    — secret ntfy.sh topic for phone push (unset = no push)
#   GOGO_NOTIFY_LEVEL  — gates (default) | all (legacy fire-on-everything) | off
#   GOGO_NOTIFY_DEBUG  — 1 = print one stderr trace line per invocation
#   GOGO_NOTIFY_DRYRUN — 1 = never SEND (probes/selftest). The latch file is
#                        still updated — a dry run is still an invocation.
#
# `bash notify.sh --selftest` runs the decision table against temp fixtures and
# exits 0 (all pass) / 1 (failures) without sending anything. Guarded in CI by
# cli/notify_hook_test.go.
#
# Phone push via ntfy.sh (free, no account): set a SECRET topic name and
# subscribe to it in the ntfy app (iOS/Android) or at https://ntfy.sh/<topic>.
#
#   export GOGO_NTFY_TOPIC="your-secret-topic-9f3a"   # in your shell profile
#
# Without GOGO_NTFY_TOPIC set, this falls back to a local macOS banner (if
# available) and is otherwise a silent no-op — safe to leave installed.

set -euo pipefail

# The user-gate statuses. MIRRORS cli/internal/contract/contract.go
# WaitingForInput() — awaiting-plan-acceptance (minus the authoring carve-out,
# applied in gogo_notify_gates), waiting-for-user, awaiting-uat. Drift is caught
# by cli/notify_hook_test.go TestNotifyHookGateStatusesMatchContract: change
# both or neither.
GOGO_GATE_STATUSES="awaiting-plan-acceptance waiting-for-user awaiting-uat"

# Effective GOGO_NOTIFY_LEVEL: unrecognised values fall back to "gates".
gogo_notify_level() {
  case "${GOGO_NOTIFY_LEVEL:-gates}" in
    all) echo all ;;
    off) echo off ;;
    *) echo gates ;;
  esac
}

# Extract a top-level string field from the JSON payload: jq when present,
# POSIX grep/sed fallback when not. $1=payload $2=key.
gogo_json_field() {
  if command -v jq >/dev/null 2>&1; then
    printf '%s' "$1" | jq -r --arg k "$2" '.[$k] // empty' 2>/dev/null || true
  else
    gogo_json_field_fallback "$1" "$2"
  fi
}

# The no-jq path, callable directly (the selftest pins it). Best-effort
# "key":"value" extraction — no escape handling; degradation, not parity.
gogo_json_field_fallback() {
  printf '%s' "$1" \
    | grep -o "\"$2\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" 2>/dev/null \
    | head -n 1 \
    | sed 's/^"[^"]*"[[:space:]]*:[[:space:]]*"//; s/"$//' || true
}

# Classify a notification_type -> notify | silent | gate.
gogo_notify_classify() {
  case "${1:-}" in
    agent_needs_input|worker_permission_prompt|idle_prompt) echo notify ;;
    *permission*) echo notify ;; # hedge: a renamed/future permission prompt is never swallowed
    elicitation_response|elicitation_complete|auth_success|computer_use_enter|computer_use_exit|push_notification) echo silent ;;
    *) echo gate ;; # agent_completed + unknown/absent -> gate-conditional (decisions.md D1)
  esac
}

# Print the status of a state.md: the FIRST TOKEN of the first `- **status:**`
# line OUTSIDE any HTML comment. Handles the two parse traps: the trailing
# same-line legend comment (which enumerates every status), and the template's
# leading multi-line comment block (which contains example field lines).
gogo_state_status() {
  awk '
    { line = $0 }
    incomment {
      if (index(line, "-->")) { incomment = 0 }
      next
    }
    {
      while ((o = index(line, "<!--")) > 0) {
        c = index(substr(line, o), "-->")
        if (c > 0) { line = substr(line, 1, o - 1) substr(line, o + c + 2) }
        else { incomment = 1; line = substr(line, 1, o - 1); break }
      }
      if (line ~ /^[[:space:]]*-[[:space:]]*\*\*status:\*\*/) {
        sub(/^[[:space:]]*-[[:space:]]*\*\*status:\*\*[[:space:]]*/, "", line)
        split(line, a, /[[:space:]]+/)
        print a[1]
        exit
      }
    }
  ' "$1" 2>/dev/null || true
}

# Authoring carve-out — mirrors contract.planWritten: a plan.md counts as
# WRITTEN only with >= 2 "## " sections; absent/stub -> unwritten; present but
# unreadable -> written (a permissions hiccup must never invent a verdict).
gogo_plan_written() {
  local plan="${1%/}/plan.md" n
  [ -e "$plan" ] || return 1
  [ -r "$plan" ] || return 0
  n="$(grep -c '^## ' "$plan" 2>/dev/null || true)"
  [ "${n:-0}" -ge 2 ]
}

# Scan $1/.gogo/work/*/state.md; print one line per open user gate:
# "<feature-folder> · <status>". No .gogo, unreadable dirs, garbage state.md —
# all degrade to "no gates", never a crash.
gogo_notify_gates() {
  local root="${1:-}" dir name st
  [ -n "$root" ] || return 0
  for dir in "$root"/.gogo/work/feature-*/; do
    [ -d "$dir" ] || continue
    [ -f "${dir}state.md" ] || continue
    st="$(gogo_state_status "${dir}state.md")"
    [ -n "$st" ] || continue
    case " $GOGO_GATE_STATUSES " in
      *" $st "*) ;;
      *) continue ;;
    esac
    if [ "$st" = "awaiting-plan-acceptance" ]; then
      gogo_plan_written "$dir" || continue
    fi
    name="${dir%/}"
    name="${name##*/}"
    printf '%s · %s\n' "$name" "$st"
  done
  return 0
}

# D4 (edge-latch): print the lines of the current gate set ($2) that were NOT
# in the last-notified set at $root/.gogo/resources/notify/gates (an already-
# gitignored, regenerable root — never a new untracked file). A missing, empty
# or unreadable seen-file means every open gate is new — the latch degrades to
# fire-on-open-gate, never to silence.
gogo_notify_new_gates() {
  local root="${1:-}" current="${2:-}" seen
  seen="$root/.gogo/resources/notify/gates"
  if [ ! -s "$seen" ] || [ ! -r "$seen" ]; then
    printf '%s\n' "$current"
    return 0
  fi
  printf '%s\n' "$current" | grep -Fxv -f "$seen" || true
}

# D4: remember the current gate set, pruning gates that closed. Best-effort —
# an unwritable .gogo/ must never fail the hook, and must stay QUIET (the
# braces keep a failed-redirection diagnostic off stderr, like the tty write).
gogo_notify_remember() {
  local root="${1:-}" current="${2:-}" dir
  [ -n "$root" ] && [ -d "$root/.gogo" ] || return 0
  dir="$root/.gogo/resources/notify"
  mkdir -p "$dir" 2>/dev/null || return 0
  if [ -n "$current" ]; then
    { printf '%s\n' "$current" > "$dir/gates"; } 2>/dev/null || true
  else
    { : > "$dir/gates"; } 2>/dev/null || true
  fi
  return 0
}

# The whole decision, side-effect-free — level knob, type classifier, gate scan
# AND the D4 latch READ (only newly-opened gates count). The latch WRITE — the
# one side effect — lives in gogo_notify_main, which re-remembers the pruned
# set from the gatelist field. The selftest calls this directly; main sends.
# $1 = payload JSON. Echoes one line:
#   "<class>\t<verdict>\t<gates>\t<gatelist joined with | ('-' when empty)>\t<message>"
# The gatelist field is never empty ('-' stands for none): tab is IFS
# whitespace, so a genuinely empty middle field would be collapsed by read and
# shift the message left.
gogo_notify_decide() {
  local payload="${1:-}" level ntype msg class verdict gates=0 gatelist="" newlist joined
  level="$(gogo_notify_level)"

  if [ "$level" = "off" ]; then
    printf 'off\tsilent\t0\t-\t\n'
    return 0
  fi

  ntype="$(gogo_json_field "$payload" notification_type)"
  # tr flattens internal newlines/tabs; the sed strips the trailing space the
  # flattened final newline would otherwise leave (TEST-001 — a "verbatim"
  # message must round-trip byte-identical).
  msg="$(gogo_json_field "$payload" message | tr '\n\t' '  ' | sed 's/[[:space:]]*$//')"

  if [ "$level" = "all" ]; then
    [ -n "$msg" ] || msg="gogo needs your input"
    printf 'all\tnotify\t0\t-\t%s\n' "$msg"
    return 0
  fi

  class="$(gogo_notify_classify "$ntype")"
  case "$class" in
    notify)
      verdict=notify
      [ -n "$msg" ] || msg="gogo needs your input"
      ;;
    silent)
      verdict=silent
      msg=""
      ;;
    gate)
      gatelist="$(gogo_notify_gates "${CLAUDE_PROJECT_DIR:-$PWD}")"
      newlist="$(gogo_notify_new_gates "${CLAUDE_PROJECT_DIR:-$PWD}" "$gatelist")"
      if [ -n "$newlist" ]; then
        verdict=notify
        gates="$(printf '%s\n' "$newlist" | grep -c . || true)"
        msg="$(printf '%s\n' "$newlist" | head -n 1)"
        [ "$gates" -le 1 ] || msg="$msg (+$((gates - 1)) more)"
      else
        verdict=silent
        msg=""
      fi
      ;;
  esac
  # "|" is safe as the join char: a feature slug is ^[a-z0-9]+(-[a-z0-9]+)*$
  # per the .gogo contract, and a status token is from the same alphabet — no
  # legitimate gate line can contain "|" (REV-018).
  joined="$(printf '%s' "$gatelist" | tr '\n' '|')"
  printf '%s\t%s\t%s\t%s\t%s\n' "$class" "$verdict" "$gates" "${joined:--}" "$msg"
}

# Deliver over the three channels (all best-effort); echo the channels attempted.
# GOGO_NOTIFY_DRYRUN=1 reports the channels that WOULD be used and sends nothing
# (the selftest's end-to-end seam).
gogo_notify_send() {
  local title="$1" msg="$2" channels="" dry="${GOGO_NOTIFY_DRYRUN:-}"

  # 1) Phone push via ntfy (if a topic is configured). --data-raw so a message
  #    starting with "@" is never read as a filename (curl's -d would).
  if [ -n "${GOGO_NTFY_TOPIC:-}" ]; then
    if [ "$dry" != "1" ]; then
      curl -fsS \
        -H "Title: $title" \
        -H "Tags: bell" \
        --data-raw "$msg" \
        "https://ntfy.sh/${GOGO_NTFY_TOPIC}" >/dev/null 2>&1 || true
    fi
    channels="ntfy"
  fi

  # 2) Local macOS banner (best-effort; harmless elsewhere). The strings travel
  #    as argv, never interpolated into the AppleScript source — no escaping
  #    surface, so no payload text can become code.
  if command -v osascript >/dev/null 2>&1; then
    if [ "$dry" != "1" ]; then
      osascript - "$msg" "$title" >/dev/null 2>&1 <<'AS' || true
on run argv
  display notification (item 1 of argv) with title (item 2 of argv)
end run
AS
    fi
    channels="${channels:+$channels,}banner"
  fi

  # 3) Terminal bell (the braces keep the no-tty redirection error off stderr)
  if [ "$dry" = "1" ]; then
    if { : >/dev/tty; } 2>/dev/null; then
      channels="${channels:+$channels,}bell"
    fi
  elif { printf '\a' >/dev/tty; } 2>/dev/null; then
    channels="${channels:+$channels,}bell"
  fi

  printf '%s' "$channels"
}

gogo_notify_main() {
  local payload ntype class verdict gates gatelist msg channels="none"
  payload="$(cat 2>/dev/null || true)"
  ntype="$(gogo_json_field "$payload" notification_type)"

  IFS=$'\t' read -r class verdict gates gatelist msg <<EOF
$(gogo_notify_decide "$payload")
EOF

  # D4 edge-latch WRITE — the hook's one state side effect: re-remember the
  # pruned current gate set decide() already scanned (no second scan). This
  # runs on DRYRUN too — DRYRUN means "send nothing", not "remember nothing" —
  # which is what makes this call site reachable from the selftest (REV-010).
  if [ "$class" = "gate" ] && [ "$(gogo_notify_level)" = "gates" ]; then
    if [ "$gatelist" = "-" ]; then gatelist=""; fi
    gogo_notify_remember "${CLAUDE_PROJECT_DIR:-$PWD}" "$(printf '%s' "$gatelist" | tr '|' '\n')"
  fi

  if [ "$verdict" = "notify" ]; then
    channels="$(gogo_notify_send "gogo • ${PWD##*/}" "$msg")"
    [ -n "$channels" ] || channels="none"
  fi

  if [ "${GOGO_NOTIFY_DEBUG:-}" = "1" ]; then
    printf 'gogo-notify: type=%s level=%s class=%s verdict=%s gates=%s channels=%s\n' \
      "${ntype:-(none)}" "$(gogo_notify_level)" "$class" "$verdict" "$gates" "$channels" >&2
  fi

  exit 0
}

# --- selftest ---------------------------------------------------------------
# Runs gogo_notify_decide (and the fallback parser) over a payload x fixture
# table. Sends nothing. Exit codes: 0 = all passed, 1 = failures or setup error.
gogo_selftest() {
  local tmp fails=0 total=0 script="$0"
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/gogo-notify-selftest.XXXXXX")" || {
    echo "selftest: mktemp failed" >&2
    return 1
  }

  mk_state() { # $1=root $2=feature $3...=state.md lines
    local dir="$1/.gogo/work/$2" line
    mkdir -p "$dir"
    shift 2
    : > "$dir/state.md"
    for line in "$@"; do printf '%s\n' "$line" >> "$dir/state.md"; done
  }

  # gate roots — one open gate each. gate-note pins the FIRST-TOKEN split (a
  # trailing note must not hide the gate); gate-indent pins the stateLine
  # tolerance (leading whitespace is still a status line, like the Go parser).
  mk_state "$tmp/gate-uat" feature-x '- **status:** awaiting-uat'
  mk_state "$tmp/gate-wfu" feature-y '- **status:** waiting-for-user'
  mk_state "$tmp/gate-plan" feature-z '- **status:** awaiting-plan-acceptance'
  printf '# plan\n## Goal\ntext\n## Approach\ntext\n' > "$tmp/gate-plan/.gogo/work/feature-z/plan.md"
  mk_state "$tmp/gate-note" feature-n '- **status:** awaiting-uat (uat round 2)'
  mk_state "$tmp/gate-indent" feature-i '  - **status:** awaiting-uat'

  # quiet root — busy statuses plus both FR5 parse traps. feature-b's commented
  # example line sits at COLUMN 0 so only the comment-block tracking can
  # suppress it (an indented line would be rejected for the weaker reason of
  # not matching the status pattern... which it no longer is, post-REV-005).
  mk_state "$tmp/quiet" feature-a \
    '- **status:** implementing   <!-- awaiting-plan-acceptance | plan-accepted | implementing | waiting-for-user | awaiting-uat -->'
  mk_state "$tmp/quiet" feature-b \
    '# State - feature b' \
    '<!--' \
    'example lines a naive grep would match:' \
    '- **status:** awaiting-uat' \
    '-->' \
    '- **status:** reviewing'

  # authoring carve-out roots
  mk_state "$tmp/authoring" feature-w '- **status:** awaiting-plan-acceptance'
  mk_state "$tmp/stub" feature-v '- **status:** awaiting-plan-acceptance'
  printf '# plan\n## Goal\nonly one section\n' > "$tmp/stub/.gogo/work/feature-v/plan.md"

  # multi-gate root
  mk_state "$tmp/multi" feature-a1 '- **status:** awaiting-uat'
  mk_state "$tmp/multi" feature-a2 '- **status:** waiting-for-user'

  # no .gogo at all
  mkdir -p "$tmp/empty"

  # unreadable .gogo/work (skipped when running as root — chmod 000 is a no-op there)
  mk_state "$tmp/unread" feature-u '- **status:** awaiting-uat'
  chmod 000 "$tmp/unread/.gogo/work" 2>/dev/null || true

  expect() { # $1=name $2=root $3=level $4=payload $5=want-verdict [$6=want-msg-substring]
    local out class verdict gates gatelist msg
    total=$((total + 1))
    out="$(CLAUDE_PROJECT_DIR="$2" GOGO_NOTIFY_LEVEL="$3" gogo_notify_decide "$4")"
    IFS=$'\t' read -r class verdict gates gatelist msg <<EOF
$out
EOF
    if [ "$verdict" != "$5" ]; then
      echo "FAIL $1: verdict=$verdict want=$5 (class=$class gates=$gates msg=$msg)" >&2
      fails=$((fails + 1))
      return 0
    fi
    if [ -n "${6:-}" ]; then
      case "$msg" in
        *"$6"*) ;;
        *)
          echo "FAIL $1: msg=\"$msg\" does not contain \"$6\"" >&2
          fails=$((fails + 1))
          ;;
      esac
    fi
    return 0
  }

  local p_completed p_needs p_perm p_idle p_unknown p_notype p_hedge
  p_completed='{"hook_event_name":"Notification","message":"gogo-reviewer finished","notification_type":"agent_completed"}'
  p_needs='{"notification_type":"agent_needs_input","message":"gogo-tester needs your input: pick an option"}'
  p_perm='{"notification_type":"worker_permission_prompt","message":"gogo-tester needs permission for Bash"}'
  p_idle='{"notification_type":"idle_prompt","message":"Claude is waiting for your input"}'
  p_unknown='{"notification_type":"mystery_type_v9","message":"??"}'
  p_notype='{"message":"hello"}'
  p_hedge='{"notification_type":"main_permission_prompt","message":"Claude needs your permission to use Bash"}'

  # FR2 — blocking prompts notify, gate or no gate, message verbatim
  expect needs-no-gate "$tmp/quiet" gates "$p_needs" notify "needs your input"
  expect needs-gate "$tmp/gate-uat" gates "$p_needs" notify
  expect perm-no-gate "$tmp/quiet" gates "$p_perm" notify "gogo-tester needs permission for Bash"
  expect perm-gate "$tmp/gate-uat" gates "$p_perm" notify
  expect idle-no-gate "$tmp/quiet" gates "$p_idle" notify
  expect idle-gate "$tmp/gate-uat" gates "$p_idle" notify
  expect perm-hedge "$tmp/quiet" gates "$p_hedge" notify "needs your permission"

  # FR3 — lifecycle noise is silent even with a gate open
  local t
  for t in elicitation_response elicitation_complete auth_success computer_use_enter computer_use_exit push_notification; do
    expect "silent-$t" "$tmp/gate-uat" gates "{\"notification_type\":\"$t\"}" silent
  done

  # FR4 — agent_completed is gate-conditional; the gate ping names the gate
  expect completed-no-gate "$tmp/quiet" gates "$p_completed" silent
  expect completed-uat "$tmp/gate-uat" gates "$p_completed" notify 'feature-x · awaiting-uat'
  expect completed-wfu "$tmp/gate-wfu" gates "$p_completed" notify 'feature-y · waiting-for-user'
  expect completed-plan "$tmp/gate-plan" gates "$p_completed" notify 'feature-z · awaiting-plan-acceptance'
  expect completed-multi "$tmp/multi" gates "$p_completed" notify '(+1 more)'
  expect completed-note "$tmp/gate-note" gates "$p_completed" notify 'feature-n · awaiting-uat'
  expect completed-indent "$tmp/gate-indent" gates "$p_completed" notify 'feature-i · awaiting-uat'

  # D1 — unknown/absent type falls through to the gate scan
  expect unknown-no-gate "$tmp/quiet" gates "$p_unknown" silent
  expect unknown-gate "$tmp/gate-uat" gates "$p_unknown" notify
  expect notype-no-gate "$tmp/quiet" gates "$p_notype" silent
  expect notype-gate "$tmp/gate-wfu" gates "$p_notype" notify

  # FR5 — authoring carve-out: an unwritten/stub plan is not a gate
  expect authoring "$tmp/authoring" gates "$p_completed" silent
  expect stub-plan "$tmp/stub" gates "$p_completed" silent

  # FR7 — the level knob
  expect level-off "$tmp/gate-uat" off "$p_needs" silent
  expect level-all "$tmp/quiet" all '{"notification_type":"auth_success"}' notify
  expect level-bogus "$tmp/quiet" bogus '{"notification_type":"auth_success"}' silent

  # degradation — no .gogo, unreadable .gogo, empty/garbage payload: silent, no crash
  expect empty-root "$tmp/empty" gates "$p_completed" silent
  if [ "$(id -u)" != "0" ]; then
    expect unreadable "$tmp/unread" gates "$p_completed" silent
  fi
  expect empty-payload "$tmp/quiet" gates '' silent
  expect garbage-payload "$tmp/quiet" gates 'not json at all' silent

  # the no-jq fallback parser, pinned directly
  total=$((total + 1))
  if [ "$(gogo_json_field_fallback "$p_completed" notification_type)" != "agent_completed" ]; then
    echo 'FAIL fallback-type: expected agent_completed' >&2
    fails=$((fails + 1))
  fi
  total=$((total + 1))
  if [ "$(gogo_json_field_fallback "$p_perm" message)" != "gogo-tester needs permission for Bash" ]; then
    echo 'FAIL fallback-message: verbatim extraction broke' >&2
    fails=$((fails + 1))
  fi

  # TEST-001 — a verbatim message must round-trip EXACTLY: no trailing space
  # left over from flattening the extractor's final newline.
  local exact_out exact_msg
  total=$((total + 1))
  exact_out="$(CLAUDE_PROJECT_DIR="$tmp/quiet" GOGO_NOTIFY_LEVEL=gates gogo_notify_decide "$p_perm")"
  IFS=$'\t' read -r _ _ _ _ exact_msg <<EOF
$exact_out
EOF
  if [ "$exact_msg" != 'gogo-tester needs permission for Bash' ]; then
    echo "FAIL msg-exact: [$exact_msg] is not byte-identical to the payload message" >&2
    fails=$((fails + 1))
  fi

  # end to end — the whole script over stdin (main's read, the record parse,
  # the verdict dispatch, exit 0), via the dry-run seam: nothing is sent.
  # DEDICATED roots (REV-019): the script's latch write mutates its root, so
  # e2e cases never share a root with the expect-level fixtures above.
  local e2e_out e2e_rc
  mk_state "$tmp/e2e-gate" feature-x '- **status:** awaiting-uat'
  mk_state "$tmp/e2e-quiet" feature-a '- **status:** implementing'
  total=$((total + 1))
  e2e_out="$(printf '%s' "$p_completed" | GOGO_NOTIFY_DRYRUN=1 GOGO_NOTIFY_DEBUG=1 GOGO_NOTIFY_LEVEL=gates CLAUDE_PROJECT_DIR="$tmp/e2e-gate" bash "$script" 2>&1)"
  e2e_rc=$?
  case "$e2e_out" in
    *"verdict=notify"*) [ "$e2e_rc" -eq 0 ] || { echo "FAIL e2e-notify: exit=$e2e_rc" >&2; fails=$((fails + 1)); } ;;
    *)
      echo "FAIL e2e-notify: trace missing verdict=notify (exit=$e2e_rc): $e2e_out" >&2
      fails=$((fails + 1))
      ;;
  esac
  total=$((total + 1))
  e2e_out="$(printf '%s' "$p_completed" | GOGO_NOTIFY_DRYRUN=1 GOGO_NOTIFY_DEBUG=1 GOGO_NOTIFY_LEVEL=gates CLAUDE_PROJECT_DIR="$tmp/e2e-quiet" bash "$script" 2>&1)"
  e2e_rc=$?
  case "$e2e_out" in
    *"verdict=silent"*) [ "$e2e_rc" -eq 0 ] || { echo "FAIL e2e-silent: exit=$e2e_rc" >&2; fails=$((fails + 1)); } ;;
    *)
      echo "FAIL e2e-silent: trace missing verdict=silent (exit=$e2e_rc): $e2e_out" >&2
      fails=$((fails + 1))
      ;;
  esac

  # D4 — the edge-latch, END TO END with no hand-written state: the SCRIPT's
  # own first run must both ping and remember (the write half bites — REV-010),
  # the second run must be silent, and the diff isolates only a new gate.
  local latch_root="$tmp/gate-latch" latch_new
  mk_state "$latch_root" feature-x '- **status:** awaiting-uat'
  total=$((total + 1))
  e2e_out="$(printf '%s' "$p_completed" | GOGO_NOTIFY_DRYRUN=1 GOGO_NOTIFY_DEBUG=1 GOGO_NOTIFY_LEVEL=gates CLAUDE_PROJECT_DIR="$latch_root" bash "$script" 2>&1)"
  case "$e2e_out" in
    *"verdict=notify"*) ;;
    *)
      echo "FAIL latch-first: expected notify on a first-seen gate: $e2e_out" >&2
      fails=$((fails + 1))
      ;;
  esac
  total=$((total + 1))
  if [ "$(cat "$latch_root/.gogo/resources/notify/gates" 2>/dev/null)" != 'feature-x · awaiting-uat' ]; then
    echo 'FAIL latch-wrote: the first run did not remember the gate set it notified about' >&2
    fails=$((fails + 1))
  fi
  total=$((total + 1))
  e2e_out="$(printf '%s' "$p_completed" | GOGO_NOTIFY_DRYRUN=1 GOGO_NOTIFY_DEBUG=1 GOGO_NOTIFY_LEVEL=gates CLAUDE_PROJECT_DIR="$latch_root" bash "$script" 2>&1)"
  case "$e2e_out" in
    *"verdict=silent"*) ;;
    *)
      echo "FAIL latch-seen: expected silent on an already-notified gate: $e2e_out" >&2
      fails=$((fails + 1))
      ;;
  esac
  mk_state "$latch_root" feature-y '- **status:** waiting-for-user'
  latch_new="$(gogo_notify_new_gates "$latch_root" "$(gogo_notify_gates "$latch_root")")"
  total=$((total + 1))
  if [ "$latch_new" != 'feature-y · waiting-for-user' ]; then
    echo "FAIL latch-diff: expected only the newly-opened gate, got: $latch_new" >&2
    fails=$((fails + 1))
  fi

  # REV-011 — a read-only .gogo/ must stay QUIET (no leaked redirection error)
  # and still exit 0. 555 keeps the scan readable; the remember write fails.
  if [ "$(id -u)" != "0" ]; then
    mk_state "$tmp/rogogo" feature-r '- **status:** awaiting-uat'
    chmod 555 "$tmp/rogogo/.gogo" 2>/dev/null || true
    total=$((total + 1))
    e2e_out="$(printf '%s' "$p_completed" | GOGO_NOTIFY_DRYRUN=1 GOGO_NOTIFY_DEBUG=1 GOGO_NOTIFY_LEVEL=gates CLAUDE_PROJECT_DIR="$tmp/rogogo" bash "$script" 2>&1)"
    e2e_rc=$?
    case "$e2e_out" in
      *"Permission denied"*)
        echo "FAIL readonly-gogo: a read-only .gogo leaked a redirection error: $e2e_out" >&2
        fails=$((fails + 1))
        ;;
      *)
        if [ "$e2e_rc" -ne 0 ]; then
          echo "FAIL readonly-gogo: exit=$e2e_rc" >&2
          fails=$((fails + 1))
        fi
        ;;
    esac
    chmod 755 "$tmp/rogogo/.gogo" 2>/dev/null || true

    # REV-016 — the WRITE braces need their own pin: with .gogo itself at 555
    # the mkdir short-circuits before the write. Here mkdir -p succeeds (the
    # dir exists) and only the write into notify/ fails — un-bracing the
    # write in gogo_notify_remember makes this case leak "Permission denied".
    mk_state "$tmp/rogogo2" feature-r2 '- **status:** awaiting-uat'
    mkdir -p "$tmp/rogogo2/.gogo/resources/notify"
    chmod 555 "$tmp/rogogo2/.gogo/resources/notify" 2>/dev/null || true
    total=$((total + 1))
    e2e_out="$(printf '%s' "$p_completed" | GOGO_NOTIFY_DRYRUN=1 GOGO_NOTIFY_DEBUG=1 GOGO_NOTIFY_LEVEL=gates CLAUDE_PROJECT_DIR="$tmp/rogogo2" bash "$script" 2>&1)"
    e2e_rc=$?
    case "$e2e_out" in
      *"Permission denied"*)
        echo "FAIL readonly-notify-dir: an unwritable latch dir leaked a redirection error: $e2e_out" >&2
        fails=$((fails + 1))
        ;;
      *)
        if [ "$e2e_rc" -ne 0 ]; then
          echo "FAIL readonly-notify-dir: exit=$e2e_rc" >&2
          fails=$((fails + 1))
        fi
        ;;
    esac
    chmod 755 "$tmp/rogogo2/.gogo/resources/notify" 2>/dev/null || true
  fi

  # cleanup (guarded, scoped — no glob-rm, no bare-variable rm)
  chmod 755 "$tmp/unread/.gogo/work" 2>/dev/null || true
  case "$tmp" in
    */gogo-notify-selftest.*) find "$tmp" -depth -delete 2>/dev/null || true ;;
  esac

  echo "selftest: $((total - fails))/$total passed"
  [ "$fails" -eq 0 ]
}

case "${1:-}" in
  --selftest)
    if gogo_selftest; then exit 0; else exit 1; fi
    ;;
esac

gogo_notify_main
