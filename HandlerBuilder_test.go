package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ljpx/di"
	"github.com/ljpx/problem"
	"github.com/ljpx/test"
)

type HandlerBuilderFixture struct {
	x *HandlerBuilder
}

func SetupHandlerBuilderFixture() *HandlerBuilderFixture {
	fixture := &HandlerBuilderFixture{}

	fixture.x = NewHandlerBuilder(di.NewContainer(), &Config{
		DebuggingEnabled:         true,
		ProblemDetailsTypePrefix: "https://testi.ng",
		JSONContentLengthLimit:   1 << 20,
	})

	fixture.x.Use(&testRoute{})

	return fixture
}

func TestHandlerBuilderNotFound(t *testing.T) {
	// Arrange.
	fixture := SetupHandlerBuilderFixture()
	handler := fixture.x.Build()

	// Act.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/hello?withQuery=1", nil)
	handler.ServeHTTP(w, r)

	// Assert.
	res := w.Result()
	test.That(t, res.StatusCode).IsEqualTo(http.StatusNotFound)

	problem := &problem.Details{}
	err := UnmarshalFromResponse(res, problem)
	test.That(t, err).IsNil()

	test.That(t, problem.Type).IsEqualTo("https://testi.ng/http/not-found")
	test.That(t, problem.Detail).IsEqualTo("The path '/hello' was not found.")
}

func TestHandlerBuilderSuccess(t *testing.T) {
	// Arrange.
	fixture := SetupHandlerBuilderFixture()
	handler := fixture.x.Build()

	// Act.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test/hello?val2=world", nil)
	r.Header.Set("X-Extra", "!")
	handler.ServeHTTP(w, r)

	// Assert.
	res := w.Result()
	test.That(t, res.StatusCode).IsEqualTo(http.StatusOK)

	resModel := &testResponseModel{}
	err := UnmarshalFromResponse(res, resModel)
	test.That(t, err).IsNil()

	test.That(t, resModel.Message).IsEqualTo("hello world!")
}

func TestHandlerBuilderPanic(t *testing.T) {
	// Arrange.
	fixture := SetupHandlerBuilderFixture()
	handler := fixture.x.Build()

	// Act.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test/hello?val2=world", nil)
	r.Header.Set("X-Extra", "panic")
	handler.ServeHTTP(w, r)

	// Assert.
	res := w.Result()
	test.That(t, res.StatusCode).IsEqualTo(http.StatusInternalServerError)

	problem := &problem.Details{}
	err := UnmarshalFromResponse(res, problem)
	test.That(t, err).IsNil()

	test.That(t, problem.Type).IsEqualTo("https://testi.ng/http/internal-server-error")
	test.That(t, problem.Error).IsEqualTo("something to panic about")
}

// -----------------------------------------------------------------------------

type testRoute struct{}

var _ Route = &testRoute{}

func (*testRoute) Method() string {
	return http.MethodGet
}

func (*testRoute) Path() string {
	return "/test/{val1}"
}

func (*testRoute) Middleware() []Middleware {
	return []Middleware{
		&testMiddleware{},
	}
}

func (*testRoute) Handle(ctx *Context) {
	val1 := ctx.GetPathParameter("val1")
	val2 := ctx.GetQueryParameter("val2")
	val3 := ctx.GetMiddlewareArtifact("extra")

	if val3 == "panic" {
		panic("something to panic about")
	}

	ctx.RespondWithJSON(http.StatusOK, &testResponseModel{
		Message: fmt.Sprintf("%v %v%v", val1, val2, val3),
	})
}

type testMiddleware struct{}

var _ Middleware = &testMiddleware{}

func (*testMiddleware) Handle(ctx *Context) bool {
	ctx.SetMiddlewareArtifact("extra", ctx.Request().Header.Get("X-Extra"))
	return true
}
