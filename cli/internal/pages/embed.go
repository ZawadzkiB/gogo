package pages

import "embed"

// assetsFS holds the vendored very-nice-mermaid renderer + the gogo viewer,
// embedded so the `w` page builder works in ANY project with no plugin on disk.
//
// SOURCE OF TRUTH is the repo's assets/vnm/ dir. Re-sync these copies with the
// Makefile target `sync-assets` (run from cli/) whenever the vendored assets
// change:
//
//	make sync-assets
//
//go:generate sh -c "cp ../../../assets/vnm/vnm-browser.js ../../../assets/vnm/viewer.js ../../../assets/vnm/viewer.css ../../../assets/vnm/viewer.template.html assets/"
//go:embed assets
var assetsFS embed.FS
