package web

// ContextHandlerFunc is an alias for a function that accepts a Context as its
// one and only parameter.
type ContextHandlerFunc func(ctx *Context)
