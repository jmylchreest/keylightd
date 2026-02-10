//go:build bindings

package main

// acquireLock is a no-op during Wails binding generation.
func acquireLock() func() {
	return func() {}
}
