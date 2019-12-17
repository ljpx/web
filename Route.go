package web

// Route defines the methods that any HTTP route must implement.
type Route interface {
	Method() string
	Path() string
	Middleware() []Middleware
	Handle(ctx *Context)
}
