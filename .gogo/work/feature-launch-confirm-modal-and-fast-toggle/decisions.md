# Decisions — feature `launch-confirm-modal-and-fast-toggle`

Forks that needed a human call. gogo appends each as `D<n>` with options and a
recommendation, then records your answer as a `RESOLVED` block. This is the
audit trail that lets the pipeline pause and resume safely.

<!-- Template for each decision — copy and fill:

## D1 — <short title>
- **Phase:** <plan | implement | review | test>
- **Question:** <the fork, stated plainly>
- **Options:**
  - A. <option> — <trade-off>
  - B. <option> — <trade-off>
- **gogo recommends:** <A / B> — <one-line why>
- **Status:** RESOLVED

### RESOLVED (user, 2026-08-02)
**B** — the three-option Select (`Launch / Launch --fast / Cancel`) — the user chose their literal "select option we want" over the f-toggle, accepting the loss of y/n shortcuts. The bare-Enter-launches-once behaviour MUST survive (Select defaults to Launch; confirm_default_test adapts its driving keys, never weakens the fired-exactly-once assertion).
        # OPEN | RESOLVED

### RESOLVED (user, <YYYY-MM-DD>)
<the decision, in the user's terms>
-->

## D1 — How the fast option is chosen in the confirm
- **Phase:** plan
- **Question:** Your words were *"select option we want to move it"*. Should the launch
  confirm stay a **Launch/Cancel confirm with a fast toggle**, or become a **list of launch
  options** you pick from?
- **Options:**
  - A. **Keep the Confirm, add an `f` toggle.** The Launch/Cancel widget is unchanged; `f`
    flips fast, the title shows the effective command and the description spells out
    `speed: fast — token-lean gogo-fast pipeline · f toggles`. **One Enter still launches**
    (the CONFIRM-DEFAULT CONVENTION and its guard `TestConfirmDefaultForwardMovesSubmitOnEnter`
    are untouched), `y`/`n` still work, smallest diff. The cost: `f` is a key you must read
    about, even though the description always shows it.
  - B. **Replace it with a three-option Select** — `Launch` / `Launch --fast` / `Cancel`,
    pre-highlighted on whichever the source config implies. Closest to your literal wording and
    maximally discoverable (both launch modes are visible, no key to learn); still one Enter.
    The cost: for that one form `binding.confirm` stops being the discriminator (a new
    `binding.launchMode` + a completion branch), `y`/`n` are lost, and the forward/destructive
    confirm convention has to state two widget shapes instead of one.
- **gogo recommends:** **A** — it adds the option you asked for without disturbing the one
  keystroke you use most (`m` → Enter), and the speed line makes the state and its key visible
  anyway. Pick **B** if you would rather see both launch modes listed than read a toggle line.
- **Status:** OPEN

## D2 — How many forms get the modal treatment
- **Phase:** plan
- **Question:** All **16** cockpit forms are drawn by the same three lines
  (`view.go:26-30`). Should the modal apply to all of them, or only to the launch confirm you
  complained about?
- **Options:**
  - A. **All 16.** They already share one render site, and the supporting plumbing
    (`enterForm`, the recorded origin, the size producer) is needed for the launch confirm
    anyway — so "all" is a *background selector*, not sixteen changes. Delete (`x`), kill
    (`K`), plan-session (`P`), source edit, plan mint and the pickers stop blanking the screen
    too. The cost: 16 mechanical call-site edits and a wider test surface.
  - B. **The launch confirm only.** Smallest diff and the smallest risk to the other forms —
    but pressing `x` or `K` still "opens a new window", so the cockpit reads inconsistently.
- **gogo recommends:** **A** — the extra work is 16 one-line edits over plumbing that has to
  exist regardless, and the inconsistency in B is the same complaint you just raised, left half
  fixed.
- **Status:** RESOLVED

### RESOLVED (user, 2026-08-02)
**B** — the launch confirm ONLY for now ("launch confirm only for now"); the other 15 form sites keep the full-screen takeover. The overlay plumbing (modal.go) is still built reusable so extending later is a per-site one-liner.

## D3 — What the board behind the modal looks like
- **Phase:** plan
- **Question:** A terminal modal is a composite; the background can either be **dimmed**
  (colours stripped, redrawn faint) or **kept in full colour** underneath.
- **Options:**
  - A. **Strip + dim.** Each background line has its ANSI stripped and is redrawn through the
    existing `dimStyle`, then the box's cells are spliced over it. Deterministic: a plain
    background has no open colour state, so nothing can bleed into the box. It is also the
    classic dimmed-backdrop look, and it keeps the board's text assertable in tests.
  - B. **Keep the background's colours.** Prettier — the board stays fully coloured around the
    dialog. But every spliced row must be cut mid-escape-sequence and the colour state re-opened
    on the right-hand fragment; getting that wrong shows up as a card's background colour
    smearing across the dialog.
- **gogo recommends:** **A** — a modal is supposed to pull focus, and it removes an entire
  class of rendering bug rather than managing it.
- **Status:** RESOLVED

### RESOLVED (user, 2026-08-02)
**A** — dimmed, ANSI-stripped board behind the modal (deterministic, no bleed).

## D4 — `pickerOrigin` — replace it or sit beside it
- **Phase:** plan
- **Question:** The modal needs to know **which view it was opened from**. `pickerOrigin`
  (`model.go:368`) already records that, but only for the three pickers.
- **Options:**
  - A. **Replace it with `formOrigin`,** set at the single `enterForm` choke point and used for
    both the background *and* the return mode. One field for one question — this repo's own
    TEST-006 rule ("a user-visible rule stated in more than one place is ONE constant"), and it
    collapses the five-place `pending*` returnMode contract 0.33.0 recorded as a trap. Verified
    equivalent at every existing site.
  - B. **Add `formOrigin` alongside `pickerOrigin`.** Smaller, more surgical diff — but two
    fields answering the same question, which is exactly the drift shape the repo has been bitten
    by four times.
- **gogo recommends:** **A** — the equivalence was checked site by site, and leaving two origin
  fields in place is how they end up disagreeing.
- **Status:** RESOLVED

### RESOLVED (user, 2026-08-02)
**A** — replace pickerOrigin with one formOrigin field (technical call, resolved by the orchestrator per the recommendation; TEST-006 one-field rule).
