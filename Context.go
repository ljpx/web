package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/ljpx/di"
	"github.com/ljpx/id"
	"github.com/ljpx/problem"
)

// Context represents the context of a single HTTP web request.  It is not
// thread-safe.
type Context struct {
	w      http.ResponseWriter
	r      *http.Request
	c      di.Container
	config *Config

	correlationID       id.ID
	middlewareArtifacts map[string]interface{}
}

// NewContext creates a new context for the provided request.
func NewContext(w http.ResponseWriter, r *http.Request, c di.Container, config *Config) *Context {
	return &Context{
		w:      w,
		r:      r,
		c:      c.Fork(),
		config: config,

		correlationID:       id.New(),
		middlewareArtifacts: make(map[string]interface{}),
	}
}

// GetCorrelationID returns the correlationID for the request.
func (ctx *Context) GetCorrelationID() id.ID {
	return ctx.correlationID
}

// GetMiddlewareArtifact retrieves the middleware artifact with the specified
// name.  It will return nil if the artifact does not exist.
func (ctx *Context) GetMiddlewareArtifact(name string) interface{} {
	v, _ := ctx.middlewareArtifacts[name]
	return v
}

// SetMiddlewareArtifact sets the middleware artifact for the specified name.
func (ctx *Context) SetMiddlewareArtifact(name string, value interface{}) {
	ctx.middlewareArtifacts[name] = value
}

// ResponseWriter returns the http.ResponseWriter.
func (ctx *Context) ResponseWriter() http.ResponseWriter {
	return ctx.w
}

// Container returns the underlying container.
func (ctx *Context) Container() di.Container {
	return ctx.c
}

// Request returns the *http.Request.
func (ctx *Context) Request() *http.Request {
	return ctx.r
}

// Header returns the set of response headers.
func (ctx *Context) Header() http.Header {
	return ctx.w.Header()
}

// GetPathParameter retrieves a path segment parameter from the request.
func (ctx *Context) GetPathParameter(name string) string {
	val, _ := mux.Vars(ctx.r)[name]
	return val
}

// GetQueryParameter retrieves a query parameter from the request.
func (ctx *Context) GetQueryParameter(name string) string {
	return ctx.r.URL.Query().Get(name)
}

// FromJSON retrieves JSON from the request body to place into the provided
// Purifiable.
func (ctx *Context) FromJSON(model Purifiable) bool {
	if !ctx.AssertContentType("application/json") {
		return false
	}

	if !ctx.AssertContentLength(ctx.config.JSONContentLengthLimit) {
		return false
	}

	decoder := json.NewDecoder(ctx.r.Body)
	err := decoder.Decode(model)
	if err != nil {
		problem := ctx.getProblemDetailsForDeserialization(err)
		ctx.RespondWithJSON(http.StatusBadRequest, problem)
		return false
	}

	field, err := model.Purify()
	if err != nil {
		problem := ctx.getProblemDetailsForUnprocessableEntity(field, err)
		ctx.RespondWithJSON(http.StatusUnprocessableEntity, problem)
		return false
	}

	return true
}

// Respond reponds to the request with the provided HTTP code.
func (ctx *Context) Respond(code int) {
	ctx.w.Header().Set("Correlation-ID", ctx.correlationID.String())
	ctx.w.WriteHeader(code)
}

// RespondWithJSON responds to the request with the provided HTTP code and
// model.
func (ctx *Context) RespondWithJSON(code int, model interface{}) {
	rawJSON, err := json.Marshal(model)
	if err != nil {
		rawJSON = ctx.getRawProblemDetailsForSerializationError(err)
		code = http.StatusInternalServerError
	}

	ctx.w.Header().Set("Content-Type", "application/json")
	ctx.w.Header().Set("Content-Length", fmt.Sprintf("%v", len(rawJSON)))
	ctx.Respond(code)
	ctx.w.Write([]byte(rawJSON))
}

// NotFound responds to the request with a NotFound status code.
func (ctx *Context) NotFound(subjectType string, subject string) {
	problem := ctx.getProblemDetailsForNotFound(subjectType, subject)
	ctx.RespondWithJSON(http.StatusNotFound, problem)
}

// InternalServerError responds to the request with an InternalServerError
// status code.
func (ctx *Context) InternalServerError(err error) {
	problem := ctx.getProblemDetailsForInternalServerError(err)
	ctx.RespondWithJSON(http.StatusInternalServerError, problem)
}

// Resolve resolves from the underlying container.  It will return false if
// an error prevented the operation from completing.
func (ctx *Context) Resolve(dependencies ...interface{}) bool {
	err := ctx.c.Resolve(dependencies...)
	if err != nil {
		ctx.InternalServerError(err)
		return false
	}

	return true
}

// AssertContentType ensures that the content type of the request matches one of
// the content types provided.
func (ctx *Context) AssertContentType(allowedContentTypes ...string) bool {
	contentType := ctx.r.Header.Get("Content-Type")
	contentTypeUppercase := strings.TrimSpace(strings.ToUpper(contentType))

	for _, allowedContentType := range allowedContentTypes {
		if contentTypeUppercase == strings.ToUpper(allowedContentType) {
			return true
		}
	}

	problem := ctx.getProblemDetailsForUnsupportedMediaType(contentType, allowedContentTypes)
	ctx.RespondWithJSON(http.StatusUnsupportedMediaType, problem)

	return false
}

// AssertContentLength ensures that a content length was provided, and that it
// is in (0, max].
func (ctx *Context) AssertContentLength(max int64) bool {
	contentLength := ctx.r.ContentLength

	if contentLength > max {
		problem := ctx.getProblemDetailsForRequestEntityTooLarge(contentLength, max)
		ctx.RespondWithJSON(http.StatusRequestEntityTooLarge, problem)
		return false
	}

	if contentLength <= 0 {
		problem := ctx.getProblemDetailsForLengthRequired()
		ctx.RespondWithJSON(http.StatusLengthRequired, problem)
		return false
	}

	return true
}

// AssertMethod ensures that the incoming request is using one of the provided
// methods.
func (ctx *Context) AssertMethod(allowedMethods ...string) bool {
	methodUpperCase := strings.ToUpper(ctx.r.Method)

	for _, allowedMethod := range allowedMethods {
		if methodUpperCase == strings.ToUpper(allowedMethod) {
			return true
		}
	}

	problem := ctx.getProblemDetailsForMethodNotAllowed(ctx.r.Method, allowedMethods)
	ctx.RespondWithJSON(http.StatusMethodNotAllowed, problem)

	return false
}

func (ctx *Context) getProblemDetailsForUnsupportedMediaType(providedContentType string, allowedContentTypes []string) *problem.Details {
	return &problem.Details{
		Type:   fmt.Sprintf("%v/http/unsupported-media-type", ctx.config.ProblemDetailsTypePrefix),
		Title:  "Unsupported Media Type",
		Detail: fmt.Sprintf("The Content-Type '%v' is not supported by this endpoint.", providedContentType),
		Specifics: map[string]interface{}{
			"providedContentType": providedContentType,
			"allowedContentTypes": allowedContentTypes,
		},
	}
}

func (ctx *Context) getProblemDetailsForRequestEntityTooLarge(contentLength, max int64) *problem.Details {
	detailFormat := "The provided request entity of length %v (%v bytes) exceeds the maximum of %v (%v bytes) on this endpoint."
	return &problem.Details{
		Type:   fmt.Sprintf("%v/http/request-entity-too-large", ctx.config.ProblemDetailsTypePrefix),
		Title:  "Request Entity Too Large",
		Detail: fmt.Sprintf(detailFormat, ByteSizeToFriendlyString(contentLength), contentLength, ByteSizeToFriendlyString(max), max),
		Specifics: map[string]interface{}{
			"contentLength":        contentLength,
			"maximumContentLength": max,
		},
	}
}

func (ctx *Context) getProblemDetailsForLengthRequired() *problem.Details {
	return &problem.Details{
		Type:   fmt.Sprintf("%v/http/length-required", ctx.config.ProblemDetailsTypePrefix),
		Title:  "Length Required",
		Detail: "This endpoint requires that the Content-Length header be set to a positive, non-zero value.",
	}
}

func (ctx *Context) getProblemDetailsForMethodNotAllowed(method string, allowedMethods []string) *problem.Details {
	return &problem.Details{
		Type:   fmt.Sprintf("%v/http/method-not-allowed", ctx.config.ProblemDetailsTypePrefix),
		Title:  "Method Not Allowed",
		Detail: fmt.Sprintf(`This endpoint does not allow use of the '%v' method.`, method),
		Specifics: map[string]interface{}{
			"methodUsed":     method,
			"allowedMethods": allowedMethods,
		},
	}
}

func (ctx *Context) getProblemDetailsForDeserialization(err error) *problem.Details {
	problem := &problem.Details{
		Type:   fmt.Sprintf("%v/json/deserialization", ctx.config.ProblemDetailsTypePrefix),
		Title:  "Deserialization Error",
		Detail: "The provided request body could not be meaningfully deserialized.  It appears to be invalid.",
	}

	if ctx.config.DebuggingEnabled {
		problem.AttachError(err)
	}

	return problem
}

func (ctx *Context) getProblemDetailsForUnprocessableEntity(field string, err error) *problem.Details {
	return &problem.Details{
		Type:   fmt.Sprintf("%v/http/unprocessable-entity", ctx.config.ProblemDetailsTypePrefix),
		Title:  "Unprocessable Entity",
		Detail: fmt.Sprintf(`The provided request body was understood but contained some invalid values.`),
		Specifics: map[string]interface{}{
			"field": field,
			"error": err.Error(),
		},
	}
}

func (ctx *Context) getProblemDetailsForNotFound(subjectType string, subject string) *problem.Details {
	return &problem.Details{
		Type:   fmt.Sprintf("%v/http/not-found", ctx.config.ProblemDetailsTypePrefix),
		Title:  "Not Found",
		Detail: fmt.Sprintf(`The %v '%v' was not found.`, subjectType, subject),
		Specifics: map[string]interface{}{
			"subjectType": subjectType,
			"subject":     subject,
		},
	}
}

func (ctx *Context) getProblemDetailsForInternalServerError(err error) *problem.Details {
	problem := &problem.Details{
		Type:   fmt.Sprintf("%v/http/internal-server-error", ctx.config.ProblemDetailsTypePrefix),
		Title:  "Internal Server Error",
		Detail: fmt.Sprintf("An internal server error prevented the request from completing."),
	}

	if ctx.config.DebuggingEnabled && err != nil {
		problem.AttachError(err)
	}

	return problem
}

func (ctx *Context) getRawProblemDetailsForSerializationError(err error) []byte {
	formatJSON := `{"type":"%v/http/internal-server-error","title":"Internal Server Error","detail":"Serialization of the response model failed."%v}`

	errStr := ""
	if ctx.config.DebuggingEnabled && err != nil {
		errStr = fmt.Sprintf(`,"error":"%v"`, err.Error())
	}

	return []byte(fmt.Sprintf(formatJSON, ctx.config.ProblemDetailsTypePrefix, errStr))
}
