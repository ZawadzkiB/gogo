# gogo CLI — the deterministic cockpit binary.
#
#   make build         build ./gogo
#   make test          run the unit/golden tests
#   make vet fmt       static checks / format
#   make sync-assets   re-copy the vendored very-nice-mermaid + viewer into the
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
# truth). Run this whenever assets/vnm/* changes.
sync-assets:
	cp $(ASSET_SRC)/vnm/vnm-browser.js \
	   $(ASSET_SRC)/vnm/viewer.js \
	   $(ASSET_SRC)/vnm/viewer.css \
	   $(ASSET_SRC)/vnm/viewer.template.html \
	   $(EMBED_DIR)/

clean:
	rm -f $(BINARY)
