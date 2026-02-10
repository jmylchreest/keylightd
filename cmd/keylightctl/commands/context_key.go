package commands

// ClientContextKey is used for storing the client in context for commands.
// All command handlers and the main entry point must use this same key
// to ensure the client can be retrieved from the context.
var ClientContextKey = &struct{}{}
