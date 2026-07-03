package pages

import "embed"

// assetsFS holds the vendored viewer runtime + mermaid snapshot, embedded so
// the `w` page builder works in ANY project with no plugin on disk.
//
// SOURCE OF TRUTH is the repo's assets/ dir (assets/viewer/* and
// assets/mermaid/mermaid.min.js). Re-sync these copies with the Makefile
// target `sync-assets` (run from cli/) whenever the vendored assets change:
//
//	make sync-assets
//
//go:generate sh -c "cp ../../../assets/viewer/*.js ../../../assets/viewer/viewer.css ../../../assets/viewer/viewer.template.html assets/ && cp ../../../assets/mermaid/mermaid.min.js assets/mermaid.min.js"
//go:embed assets
var assetsFS embed.FS
