#!/usr/bin/env bash
# gogo — SessionStart advisory.
#
# If gogo is installed but this project isn't initialised yet, print a one-line
# reminder so the user knows to run /gogo:build. Read-only, always succeeds, and
# stays silent once `.gogo/knowledge/` exists. This is advisory only — the
# authoritative config gate lives in the gogo skill, which refuses to plan
# without config even if this hook is skipped.

set -euo pipefail

dir="${CLAUDE_PROJECT_DIR:-$PWD}"

if [ ! -d "$dir/.gogo/knowledge" ]; then
  echo "gogo is installed but not initialised for this project — run /gogo:build to set up the knowledge config."
fi

exit 0
