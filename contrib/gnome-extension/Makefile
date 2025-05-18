EXT_UUID=keylightd-control@jmylchreest.github.io
EXT_DIR=$(EXT_UUID)
DIST_DIR=../../dist/gnome-extension

.PHONY: all build clean pack

all: clean build pack 

build: clean
	@echo "Building schemas..."
	glib-compile-schemas $(EXT_DIR)/schemas
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
		--out-dir $(DIST_DIR)

mkdist: clean
	@echo "Creating dist directory..."
	mkdir -p $(DIST_DIR)

zip: mkdist build
	@echo "Zipping extension..."
	zip -r $(DIST_DIR)/$(EXT_UUID).shell-extension.zip $(EXT_DIR)

clean:
	rm -rf $(EXT_DIR)/schemas/gschemas.compiled
	rm -rf $(DIST_DIR)/$(EXT_UUID).shell-extension.zip 