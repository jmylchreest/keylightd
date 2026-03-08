//go:build bindings

package main

import "embed"

// assets is a stub used during Wails binding generation and linting,
// where the frontend/dist directory has not been built yet.
var assets embed.FS
