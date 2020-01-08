package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/ljpx/di"
	"github.com/ljpx/logging"
)

// HandlerBuilder is used to build a handler that can be passed to any HTTP
// server.  Once Build has been called, the HandlerBuilder is invalid and can
// no longer be used.  HandlerBuilder is not thread-safe.
type HandlerBuilder struct {
	c      di.Container
	config *Config
	logger logging.Logger

	routesByPath map[string][]Route
	hasBeenBuilt bool
}

// NewHandlerBuilder creates a new handler builder with the provided config and
// container.
func NewHandlerBuilder(c di.Container, logger logging.Logger, config *Config) *HandlerBuilder {
	return &HandlerBuilder{
		c:      c,
		config: config,
		logger: logger,

		routesByPath: make(map[string][]Route),
	}
}

// Use adds a route to the list of routes this handler should expose.
func (b *HandlerBuilder) Use(route Route) {
	b.assertNotAlreadyBuilt()

	path := purifyPath(route.Path())
	b.routesByPath[path] = append(b.routesByPath[path], route)
}

// Build builds a http.Handler that can be passed to any server.
func (b *HandlerBuilder) Build() http.Handler {
	b.assertNotAlreadyBuilt()
	b.hasBeenBuilt = true

	mx := mux.NewRouter()

	for path, routes := range b.routesByPath {
		ctxHandler := buildHandlerForPath(path, routes)
		requestHandler := buildHandlerFromRequest(b.c, b.logger, b.config, ctxHandler)
		mx.HandleFunc(path, requestHandler)
	}

	notFoundRequestHandler := buildHandlerFromRequest(b.c, b.logger, b.config, func(ctx *Context) {
		ctx.NotFound("path", ctx.r.URL.Path)
	})

	mx.PathPrefix("/").HandlerFunc(notFoundRequestHandler)

	return mx
}

func (b *HandlerBuilder) assertNotAlreadyBuilt() {
	if b.hasBeenBuilt {
		panic("a HandlerBuilder can not be used after Build has been called")
	}
}

func buildHandlerFromRequest(c di.Container, logger logging.Logger, config *Config, ctxHandler ContextHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mrw := NewMeasuredResponseWriter(w)
		ctx := NewContext(mrw, r, c, config)

		defer func() {
			if p := recover(); p != nil && !mrw.HasWrittenHeaders() {
				err := fmt.Errorf("%v", p)
				ctx.InternalServerError(err)
			}

			logmsg := fmt.Sprintf("• %v %v %v %v\n", mrw.statusCode, mrw.Duration(), ByteSizeToFriendlyString(mrw.volume), r.URL.Path)
			logger.Printf(logmsg)
		}()

		ctxHandler(ctx)
	}
}

func buildHandlerForPath(path string, routes []Route) ContextHandlerFunc {
	handlerByMethod := make(map[string]ContextHandlerFunc)
	allowedMethods := []string{}

	for _, route := range routes {
		method := route.Method()

		handlerByMethod[method] = buildHandlerForRoute(route)
		allowedMethods = append(allowedMethods, method)
	}

	return func(ctx *Context) {
		if !ctx.AssertMethod(allowedMethods...) {
			return
		}

		handlerByMethod[ctx.r.Method](ctx)
	}
}

func buildHandlerForRoute(route Route) ContextHandlerFunc {
	return func(ctx *Context) {
		for _, mw := range route.Middleware() {
			shouldContinue := mw.Handle(ctx)
			if !shouldContinue {
				return
			}
		}

		route.Handle(ctx)
	}
}

func purifyPath(path string) string {
	return strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
}
