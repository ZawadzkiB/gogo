# very-nice-mermaid - 0.27.0 (2026-07-29)

gogo's diagrams are now rendered by
**[very-nice-mermaid](https://www.npmjs.com/package/very-nice-mermaid)**. `mmdc` is gone
from the codebase, and so is the **3.3 MB vendored `mermaid.min.js`** plus the ~44 KB
hand-rolled renderer that used to lay a diagram out with mermaid and then **re-parse
mermaid's own rendered SVG** back into a `{nodes,edges}` model. The vendored footprint drops
**3.35 MB -> 462 KB (~7x)**, and **every** gogo kind - flow, use-case, sequence, class,
activity - is now fully interactive offline, where before only the flowchart family was
(sequence / class / state fell back to a plain pan-zoom canvas).

Verified against the repo's own 212-diagram corpus: **190 render interactively**, and the
22 that do not are diagrams **mermaid.js rejects identically** - pre-existing invalid DSL
that was already failing, silently, in the old viewer. **Zero regressions.**

## What changed

- **The renderer is vendored, not the parser.** `assets/vnm/vnm-browser.js` is
  very-nice-mermaid's browser build with its single trailing ESM `export { … }` swapped for
  `window.vnm = { … }`, because `file://` blocks ES-module `src` **and** `fetch`, so a
  classic `<script src>` is the only thing that loads. It is **generated** -
  `assets/vnm/build-bundle.mjs` rebuilds it and fails loudly if upstream's bundle shape
  changes.
- **Layout is computed at authoring time.** `assets/vnm/layout.mjs` turns a folder's `.mmd`
  set into `layouts.json`. This is the crux: routing a sequence / class / state diagram from
  raw DSL needs mermaid's `detectType`, i.e. a dynamic `import("mermaid")` that `file://`
  can never resolve - so an in-browser-only design silently renders those kinds as a
  **garbage flowchart**. Doing the parse in Node keeps the page dependency-free and correct.
  `serializeModel` is mandatory here: a positioned model carries `Map`/`Set` values that
  plain `JSON.stringify` flattens, after which the renderer dies on
  `model.classDefs is not iterable`.
- **Three tiers, never a blank figure.** `assets/vnm/viewer.js` mounts each figure from a
  **prebuilt layout**, else parses **inline DSL** (flowcharts, zero deps), else shows an
  inlined **static SVG**. A diagram whose layout step failed carries its real parse error
  (message + source excerpt + caret) into the page, so the viewer never blames a missing
  install for what is actually bad DSL.
- **Entity fidelity fixed.** 53 diagrams escape `&lt;` / `&gt;` because the same DSL also
  ships as a ` ```mermaid ` fence where GitHub draws labels as HTML. very-nice-mermaid draws
  SVG `<text>`, so those showed literally; both the layout tool and the viewer now decode
  before rendering. That incidentally made **2 more diagrams parse** than the raw CLI managed.
- **Both page builders swapped together.** The `/gogo:view` skill and the Go `internal/pages`
  builder (`gogo view --web`) share the new template, and `pages.go` inlines the bundle's
  `layouts.json`, merging any `before/` set under `before-<stem>` keys so compare mode gets
  prebuilt models on both sides. The `go:embed` set and `make sync-assets` follow.
- **Migration.** `layouts.json` was generated for every existing bundle (123 files), so
  archived plans and changelog entries render interactively instead of erroring.

## Key outcomes

- **Footprint:** `mermaid.min.js` 3.3 MB + 5 renderer modules ~44 KB -> `vnm-browser.js`
  447 KB + `viewer.js` 8 KB + `viewer.css` 7 KB. Two vendored copies as before (the
  `assets/` source of truth and the `go:embed` mirror), never a third.
- **`mermaid-parse.js` is deleted.** Reading a node/edge model back out of mermaid's
  rendered SVG was the most fragile thing in the viewer; very-nice-mermaid hands over a real
  model, so the whole reverse-engineering layer is gone.
- **Interactivity is uniform.** Drag, live edge re-routing, minimap, fit / zoom / reset,
  SVG-PNG export and gogo's ⛶ expand-to-modal now apply to sequence, class and state
  diagrams too. The minimap is suppressed in half-width compare cells, where it covered a
  large fraction of an already-cramped canvas.

## Decisions (one-liners)

- **D1 - layout at authoring, mount in browser** (over static SVG, or DSL-in-HTML): the only
  option that is both fully interactive and correct for all five kinds offline.
- **D2 - `very-nice-mermaid` stays optional**, exactly as `mmdc` was: absent -> exit 3,
  flowcharts still render in-browser, other kinds show a named error. Never installed mid-run.
- **D3 - sources keep their escaped entities**; decoding happens at render time, so the
  ` ```mermaid ` fence stays correct on GitHub.
- **D4 - the 22 invalid diagrams were left as-is** (out of scope): they now surface an
  actionable parse error instead of failing silently.

## Known limitations (carried forward)

- **22 diagrams carry invalid Mermaid** (semicolons inside message/note text; escaped quotes
  inside class notes). mermaid.js rejects them too, so this is pre-existing, not new - they
  render as an inline error naming the line. Fixing them is a separate change.
- **`\n` inside a label renders literally** in 101 diagrams. mermaid.js behaves the same way,
  so this is not a regression; `<br/>` is the correct authoring form and works.
- **Prebuilt layouts are a committed artifact.** A brand-new sequence / class / state diagram
  is not interactive until the authoring step runs `layout.mjs` (or `/gogo:view` builds it on
  demand). Flowcharts never need it.

Full audit trail: this change was made directly (no `.gogo/work/` feature folder); the
verification evidence is summarized above.
