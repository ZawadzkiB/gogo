# Adjustments — feature `launch-confirm-modal-and-fast-toggle`

Log of changes / clarifications requested during planning (phase ①) and of any
plan deltas agreed later (UAT rounds). Newest last.

_No adjustments yet — the plan is at its first presentation._

## 2026-08-02 — acceptance-gate fork outcomes (delta to the drafted approach)

- **D1 = B (three-option select), not the drafted-default f-toggle.** The confirm becomes
  `huh.NewSelect` with `Launch / Launch --fast / Cancel` (default Launch). Checklist deltas:
  `binding.confirm` is replaced for THIS form by a select value; y/n shortcuts go away for the
  launch confirm; the bare-Enter-launches-once guard (confirm_default_test.go) keeps its
  fired-exactly-once assertion with adapted driving keys. The live TitleFunc command preview
  becomes unnecessary for fast (the choice IS the option label) but the title still names the
  session/root/permission line.
- **D2 = B (launch confirm only, "for now").** The modal composite applies at the launch-confirm
  site only; the other 15 form sites keep the full-screen takeover. modal.go stays reusable so a
  later feature can extend per-site.
- D3 = A (dimmed strip backdrop), D4 = A (formOrigin replaces pickerOrigin) — as drafted.
