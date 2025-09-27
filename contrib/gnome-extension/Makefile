EXT_UUID=keylightd-control@jmylchreest.github.io
EXT_DIR=$(EXT_UUID)
DIST_DIR=$(CURDIR)/../../dist/gnome-extension

.PHONY: all build clean pack version-info test

all: clean build pack

version-info:
	@echo "Generating version info..."
	@./update-version-info.sh

test:
	@echo "Running tests..."
	@cd tests && ./run-tests.sh

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
