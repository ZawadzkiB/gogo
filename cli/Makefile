# gogo CLI — the deterministic cockpit binary.
#
#   make build         build ./gogo
#   make test          run the unit/golden tests
#   make vet fmt       static checks / format
#   make sync-assets   re-copy the vendored viewer + mermaid into the embed dir
#                      (source of truth is the repo's assets/ dir)
#
BINARY := gogo
ASSET_SRC := ../assets
EMBED_DIR := internal/pages/assets

.PHONY: build test vet fmt sync-assets clean

build: sync-assets
	go build -o $(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

# Keep the embedded viewer runtime in lock-step with assets/ (the source of
# truth). Run this whenever assets/viewer/* or assets/mermaid/mermaid.min.js
# change so `w` page builds ship the current renderer.
sync-assets:
	cp $(ASSET_SRC)/viewer/geometry.js \
	   $(ASSET_SRC)/viewer/viewport.js \
	   $(ASSET_SRC)/viewer/mermaid-parse.js \
	   $(ASSET_SRC)/viewer/render.js \
	   $(ASSET_SRC)/viewer/interactive.js \
	   $(ASSET_SRC)/viewer/viewer.css \
	   $(ASSET_SRC)/viewer/viewer.template.html \
	   $(EMBED_DIR)/
	cp $(ASSET_SRC)/mermaid/mermaid.min.js $(EMBED_DIR)/mermaid.min.js

clean:
	rm -f $(BINARY)
