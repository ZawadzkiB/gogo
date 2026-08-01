---
description: Re-bind THIS session's tmux name to the work item it is actually driving (fixes the stale board dot after you ship one item and start another in the same pane).
argument-hint: "[feature-slug]"
allowed-tools: Read, Bash, Glob, Grep, AskUserQuestion
model: opus
---

Re-bind **this claude session's** tmux session to the work item it is currently
driving, via the `gogo-session-update` skill. Runnable at any time; writes nothing —
the whole effect is one `tmux rename-session`.

Target: $ARGUMENTS  (optional feature slug; pins the target. Empty → infer from this
conversation, or ask — never guess.)

Load `gogo-session-update` and follow it: resolve the host session from `$TMUX`
(no tmux → say so, do nothing), determine the target item, validate it is
non-terminal, derive the action from its status (`RunnableStatus` → `go`, else
`plan`), mint `gogo-<action>-<slug>` (collision-suffixed), rename with the exact
`=<old>` target, and report `old → new`. The board corrects itself on its next tick.
