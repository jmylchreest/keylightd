EXT_UUID=keylightd-control@jmylchreest.github.io
EXT_DIR=$(EXT_UUID)
DIST_DIR=$(CURDIR)/../../dist/gnome-extension

.PHONY: all build clean pack version-info test test-gnome test-gnome51 test-gnome-all

all: clean build pack

version-info:
	@echo "Generating version info..."
	@./update-version-info.sh

test:
	@echo "Running tests..."
	@cd tests && ./run-tests.sh

test-gnome:
	@./tests/test-gnome-shell.sh

test-gnome51:
	@GNOME_VERSION=51 ./tests/test-gnome-shell.sh

test-gnome-all:
	@set -e; for version in 48 49 50 51; do GNOME_VERSION=$$version ./tests/test-gnome-shell.sh; done

build: clean version-info test
	@echo "Running build tasks (version info + tests) - no schema compilation needed for GNOME 45+"
	@echo "Build complete."

pack: mkdist build
	@echo "Packing extension with gnome-extensions..."
	gnome-extensions pack $(EXT_DIR) --force \
		--extra-source=preferences/ \
		--extra-source=icons/ \
		--extra-source=controllers/ \
		--extra-source=managers/ \
		--extra-source=ui/ \
		--extra-source=utils.js \
		--extra-source=icons.js \
		--extra-source=icon-names.js \
		--extra-source=version-info.json \
		--out-dir $(DIST_DIR)

mkdist: clean
	@echo "Creating dist directory..."
	mkdir -p $(DIST_DIR)

zip: mkdist build
	@echo "Zipping extension..."
	@echo $(CURDIR) -- $(DIST_DIR)
	@(cd $(EXT_DIR) && zip -r $(DIST_DIR)/$(EXT_UUID).shell-extension.zip . -x "*.git*")

clean:
	rm -rf $(EXT_DIR)/schemas/gschemas.compiled
	rm -rf $(EXT_DIR)/version-info.json
	rm -rf $(DIST_DIR)/$(EXT_UUID).shell-extension.zip
