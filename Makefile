# keylightd Makefile
#
# Usage:
#   make release                     Auto-bump patch version, tag locally
#   make release VERSION=1.2.0       Tag specific version locally
#   make release-push                Auto-bump, tag, and push
#   make release-push VERSION=1.2.0  Tag specific version and push

.PHONY: release release-push check-version

# Auto-detect next version from latest git tag.
# If VERSION is passed, use that; otherwise bump patch from latest tag.
# Uses git tag -l + grep to exclude pre-release tags (e.g. v0.0.48-dev.24).
ifndef VERSION
  LATEST_TAG := $(shell git tag -l 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | grep -v -E '\-' | head -1)
  ifdef LATEST_TAG
    _MAJOR := $(shell echo $(LATEST_TAG) | sed 's/^v//' | cut -d. -f1)
    _MINOR := $(shell echo $(LATEST_TAG) | sed 's/^v//' | cut -d. -f2)
    _PATCH := $(shell echo $(LATEST_TAG) | sed 's/^v//' | cut -d. -f3)
    VERSION := $(_MAJOR).$(_MINOR).$(shell echo $$(($(_PATCH) + 1)))
  else
    VERSION := 0.0.1
  endif
endif

# Validate VERSION looks like semver
check-version:
	@echo "$(VERSION)" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$$' || \
		(echo "ERROR: VERSION must be semver (e.g., 1.2.3), got: $(VERSION)" && exit 1)

# Tag a release locally
release: check-version
	@echo "Tagging v$(VERSION)..."
	@git tag -a "v$(VERSION)" -m "v$(VERSION)"
	@echo "Tagged v$(VERSION). Push with:"
	@echo "  git push origin main v$(VERSION)"

# Release and push in one step
release-push: release
	git push origin main "v$(VERSION)"
