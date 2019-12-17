package web

// Middleware defines the methods that any HTTP middleware must implement.  If
// the Handle method returns true, the request will continue to be propagated to
// subsequent middleware handlers and eventually the route handler.
type Middleware interface {
	Handle(ctx *Context) bool
}
