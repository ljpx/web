package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ljpx/di"
	"github.com/ljpx/problem"
	"github.com/ljpx/test"
)

type ContextTestFixture struct {
	w *httptest.ResponseRecorder
	r *http.Request
	c di.Container
	x *Context
}

func SetupContextTestFixture() *ContextTestFixture {
	fixture := &ContextTestFixture{}
	fixture.w = httptest.NewRecorder()
	fixture.r = httptest.NewRequest(http.MethodGet, "/", nil)
	fixture.c = di.NewContainer()

	fixture.c.Register(di.Singleton, func(c di.Container) (testInterface, error) {
		return &testStruct{}, nil
	})

	fixture.x = NewContext(fixture.w, fixture.r, fixture.c, &Config{
		DebuggingEnabled:         true,
		ProblemDetailsTypePrefix: "https://testi.ng",
		JSONContentLengthLimit:   1 << 20,
	})

	return fixture
}

func TestContextRequestAndResponseWriter(t *testing.T) {
	// Arrange.
	fixture := SetupContextTestFixture()

	// Act.
	test.That(t, fixture.x.Request()).IsEqualTo(fixture.r)
	test.That(t, fixture.x.ResponseWriter()).IsEqualTo(fixture.w)
}

func TestContextGeneratesCorrelationID(t *testing.T) {
	// Arrange.
	fixture := SetupContextTestFixture()

	// Act.
	test.That(t, fixture.x.GetCorrelationID().IsValid()).IsTrue()
}

func TestContextResolveSuccess(t *testing.T) {
	// Arrange.
	fixture := SetupContextTestFixture()

	// Act.
	var val testInterface
	success := fixture.x.Resolve(&val)

	// Assert.
	test.That(t, success).IsTrue()
	test.That(t, val.Greeting()).IsEqualTo("Hello, World!")
}

func TestContextResolveFailure(t *testing.T) {
	// Arrange.
	fixture := SetupContextTestFixture()

	// Act.
	var val io.Writer
	success := fixture.x.Resolve(&val)

	// Assert.
	test.That(t, success).IsFalse()

	res := fixture.w.Result()
	test.That(t, res.StatusCode).IsEqualTo(http.StatusInternalServerError)

	rawJSON, err := ioutil.ReadAll(res.Body)
	test.That(t, err).IsNil()

	json := string(rawJSON)
	expectedJSON := "{\"type\":\"https://testi.ng/http/internal-server-error\",\"title\":\"Internal Server Error\",\"detail\":\"An internal server error prevented the request from completing.\",\"error\":\"the type `io.Writer` does not have a resolver in this container\"}"
	test.That(t, json).IsEqualTo(expectedJSON)
}

func TestContextMiddlewareArtifactsSymmetric(t *testing.T) {
	// Arrange.
	fixture := SetupContextTestFixture()

	// Act.
	fixture.x.SetMiddlewareArtifact("number", 5)
	number := fixture.x.GetMiddlewareArtifact("number").(int)

	// Assert.
	test.That(t, number).IsEqualTo(5)
}

func TestContextSendsCorrelationID(t *testing.T) {
	// Arrange.
	fixture := SetupContextTestFixture()

	// Act.
	fixture.x.Respond(http.StatusOK)

	// Assert.
	res := fixture.w.Result()
	test.That(t, res.StatusCode).IsEqualTo(http.StatusOK)
	test.That(t, res.Header.Get("Correlation-ID")).IsEqualTo(fixture.x.correlationID.String())
}

func TestContextRespondWithJSONUnmarshallable(t *testing.T) {
	// Arrange.
	fixture := SetupContextTestFixture()

	// Act.
	fixture.x.RespondWithJSON(http.StatusOK, &testUnmarshallableStruct{})

	// Assert.
	res := fixture.w.Result()
	test.That(t, res.StatusCode).IsEqualTo(http.StatusInternalServerError)

	problemDetails := &problem.Details{}
	err := UnmarshalFromResponse(res, problemDetails)
	test.That(t, err).IsNil()

	test.That(t, problemDetails.Type).IsEqualTo("https://testi.ng/http/internal-server-error")
	test.That(t, problemDetails.Title).IsEqualTo("Internal Server Error")
	test.That(t, problemDetails.Error).IsNotEqualTo("")
}

func TestContextRespondWithJSONSuccess(t *testing.T) {
	// Arrange.
	fixture := SetupContextTestFixture()

	// Act.
	fixture.x.RespondWithJSON(http.StatusOK, &testResponseModel{Message: "Hello, World!"})

	// Assert.
	res := fixture.w.Result()
	test.That(t, res.StatusCode).IsEqualTo(http.StatusOK)

	responseModel := &testResponseModel{}
	err := UnmarshalFromResponse(res, responseModel)
	test.That(t, err).IsNil()
	test.That(t, responseModel.Message).IsEqualTo("Hello, World!")
	test.That(t, res.Header.Get("Content-Type")).IsEqualTo("application/json")
	test.That(t, res.Header.Get("Content-Length")).IsEqualTo("27")
}

func TestContextAssertContentTypeSuccess(t *testing.T) {
	// Arrange.
	fixture := SetupContextTestFixture()
	fixture.r.Header.Set("Content-Type", "image/png")

	// Act.
	passed := fixture.x.AssertContentType("image/PNG")

	// Assert.
	test.That(t, passed).IsTrue()
}

func TestContextAssertContentTypeFailure(t *testing.T) {
	// Arrange.
	fixture := SetupContextTestFixture()
	fixture.r.Header.Set("Content-Type", "image/jpeg")

	// Act.
	passed := fixture.x.AssertContentType("image/PNG", "image/gif")

	// Assert.
	test.That(t, passed).IsFalse()

	res := fixture.w.Result()
	test.That(t, res.StatusCode).IsEqualTo(http.StatusUnsupportedMediaType)

	rawJSON, err := ioutil.ReadAll(res.Body)
	test.That(t, err).IsNil()

	json := string(rawJSON)
	expectedJSON := `{"type":"https://testi.ng/http/unsupported-media-type","title":"Unsupported Media Type","detail":"The Content-Type 'image/jpeg' is not supported by this endpoint.","specifics":{"allowedContentTypes":["image/PNG","image/gif"],"providedContentType":"image/jpeg"}}`
	test.That(t, json).IsEqualTo(expectedJSON)
}

func TestContextAssertContentLengthSuccess(t *testing.T) {
	// Arrange.
	fixture := SetupContextTestFixture()
	fixture.r = httptest.NewRequest(http.MethodGet, "/", bytes.NewBufferString("Hello, World!"))
	fixture.r.Header.Set("Content-Length", "13")
	fixture.x.r = fixture.r

	// Act.
	passed := fixture.x.AssertContentLength(13)

	// Assert.
	test.That(t, passed).IsTrue()
}

func TestContextAssertContentLengthFailureTooLarge(t *testing.T) {
	// Arrange.
	fixture := SetupContextTestFixture()
	fixture.r = httptest.NewRequest(http.MethodGet, "/", bytes.NewBufferString("Hello, World!"))
	fixture.r.Header.Set("Content-Length", "13")
	fixture.x.r = fixture.r

	// Act.
	passed := fixture.x.AssertContentLength(12)

	// Assert.
	test.That(t, passed).IsFalse()

	res := fixture.w.Result()
	test.That(t, res.StatusCode).IsEqualTo(http.StatusRequestEntityTooLarge)

	rawJSON, err := ioutil.ReadAll(res.Body)
	test.That(t, err).IsNil()

	json := string(rawJSON)
	expectedJSON := `{"type":"https://testi.ng/http/request-entity-too-large","title":"Request Entity Too Large","detail":"The provided request entity of length 13.00 B (13 bytes) exceeds the maximum of 12.00 B (12 bytes) on this endpoint.","specifics":{"contentLength":13,"maximumContentLength":12}}`
	test.That(t, json).IsEqualTo(expectedJSON)
}

func TestContextAssertContentLengthFailureNotProvided(t *testing.T) {
	// Arrange.
	fixture := SetupContextTestFixture()

	// Act.
	passed := fixture.x.AssertContentLength(12)

	// Assert.
	test.That(t, passed).IsFalse()

	res := fixture.w.Result()
	test.That(t, res.StatusCode).IsEqualTo(http.StatusLengthRequired)

	rawJSON, err := ioutil.ReadAll(res.Body)
	test.That(t, err).IsNil()

	json := string(rawJSON)
	expectedJSON := `{"type":"https://testi.ng/http/length-required","title":"Length Required","detail":"This endpoint requires that the Content-Length header be set to a positive, non-zero value."}`
	test.That(t, json).IsEqualTo(expectedJSON)
}

func TestContextAssertMethodSuccess(t *testing.T) {
	// Arrange.
	fixture := SetupContextTestFixture()

	// Act.
	passed := fixture.x.AssertMethod(http.MethodPost, http.MethodGet)

	// Assert.
	test.That(t, passed).IsTrue()
}

func TestContextAssertMethodFailure(t *testing.T) {
	// Arrange.
	fixture := SetupContextTestFixture()

	// Act.
	passed := fixture.x.AssertMethod(http.MethodPost, http.MethodPut)

	// Assert.
	test.That(t, passed).IsFalse()

	res := fixture.w.Result()
	test.That(t, res.StatusCode).IsEqualTo(http.StatusMethodNotAllowed)

	rawJSON, err := ioutil.ReadAll(res.Body)
	test.That(t, err).IsNil()

	json := string(rawJSON)
	expectedJSON := `{"type":"https://testi.ng/http/method-not-allowed","title":"Method Not Allowed","detail":"This endpoint does not allow use of the 'GET' method.","specifics":{"allowedMethods":["POST","PUT"],"methodUsed":"GET"}}`
	test.That(t, json).IsEqualTo(expectedJSON)
}

func TestContextFromJSONContentTypeIncorrect(t *testing.T) {
	// Arrange.
	fixture := SetupContextTestFixture()
	fixture.r = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"message":"Hello, World!"}`))
	fixture.r.Header.Set("Content-Type", "application/not-json")
	fixture.x.r = fixture.r

	// Act.
	reqModel := &testRequestModel{}
	passed := fixture.x.FromJSON(reqModel)

	// Assert.
	test.That(t, passed).IsFalse()

	res := fixture.w.Result()
	test.That(t, res.StatusCode).IsEqualTo(http.StatusUnsupportedMediaType)
}

func TestContextFromJSONContentLengthMissing(t *testing.T) {
	// Arrange.
	fixture := SetupContextTestFixture()
	fixture.r = httptest.NewRequest(http.MethodPost, "/", nil)
	fixture.r.Header.Set("Content-Type", "application/json")
	fixture.x.r = fixture.r

	// Act.
	reqModel := &testRequestModel{}
	passed := fixture.x.FromJSON(reqModel)

	// Assert.
	test.That(t, passed).IsFalse()

	res := fixture.w.Result()
	test.That(t, res.StatusCode).IsEqualTo(http.StatusLengthRequired)
}

func TestContextFromJSONInvalidJSON(t *testing.T) {
	// Arrange.
	fixture := SetupContextTestFixture()
	fixture.r = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"message":"Hello, World!"`))
	fixture.r.Header.Set("Content-Type", "application/json")
	fixture.x.r = fixture.r

	// Act.
	reqModel := &testRequestModel{}
	passed := fixture.x.FromJSON(reqModel)

	// Assert.
	test.That(t, passed).IsFalse()

	res := fixture.w.Result()
	test.That(t, res.StatusCode).IsEqualTo(http.StatusBadRequest)
}

func TestContextFromJSONPurifyFailure(t *testing.T) {
	// Arrange.
	fixture := SetupContextTestFixture()
	fixture.r = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"message":"invalid"}`))
	fixture.r.Header.Set("Content-Type", "application/json")
	fixture.x.r = fixture.r

	// Act.
	reqModel := &testRequestModel{}
	passed := fixture.x.FromJSON(reqModel)

	// Assert.
	test.That(t, passed).IsFalse()

	res := fixture.w.Result()
	test.That(t, res.StatusCode).IsEqualTo(http.StatusUnprocessableEntity)
}

func TestContextFromJSONSuccess(t *testing.T) {
	// Arrange.
	fixture := SetupContextTestFixture()
	fixture.r = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"message":"Hello, World!"}`))
	fixture.r.Header.Set("Content-Type", "application/json")
	fixture.x.r = fixture.r

	// Act.
	reqModel := &testRequestModel{}
	passed := fixture.x.FromJSON(reqModel)

	// Assert.
	test.That(t, passed).IsTrue()
	test.That(t, reqModel.Message).IsEqualTo("Hello, World!")
}

func TestContextNotFound(t *testing.T) {
	// Arrange.
	fixture := SetupContextTestFixture()

	// Act.
	fixture.x.NotFound("User", "1234")

	// Assert.
	res := fixture.w.Result()
	test.That(t, res.StatusCode).IsEqualTo(http.StatusNotFound)

	rawJSON, err := ioutil.ReadAll(res.Body)
	test.That(t, err).IsNil()

	json := string(rawJSON)
	expectedJSON := `{"type":"https://testi.ng/http/not-found","title":"Not Found","detail":"The User '1234' was not found.","specifics":{"subject":"1234","subjectType":"User"}}`
	test.That(t, json).IsEqualTo(expectedJSON)
}

func TestContextInternalServerError(t *testing.T) {
	// Arrange.
	fixture := SetupContextTestFixture()

	// Act.
	fixture.x.InternalServerError(fmt.Errorf("ahhh"))

	// Assert.
	res := fixture.w.Result()
	test.That(t, res.StatusCode).IsEqualTo(http.StatusInternalServerError)

	rawJSON, err := ioutil.ReadAll(res.Body)
	test.That(t, err).IsNil()

	json := string(rawJSON)
	expectedJSON := `{"type":"https://testi.ng/http/internal-server-error","title":"Internal Server Error","detail":"An internal server error prevented the request from completing.","error":"ahhh"}`
	test.That(t, json).IsEqualTo(expectedJSON)
}

// -----------------------------------------------------------------------------

type testRequestModel struct {
	Message string `json:"message"`
}

var _ Purifiable = &testRequestModel{}

func (m *testRequestModel) Purify() (string, error) {
	if m.Message == "invalid" {
		return "message", fmt.Errorf("cannot be the string 'invalid'")
	}

	return "", nil
}

type testResponseModel struct {
	Message string `json:"message"`
}

type testUnmarshallableStruct struct{}

var _ json.Marshaler = &testUnmarshallableStruct{}

func (s *testUnmarshallableStruct) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("cannot be marshalled")
}

type testInterface interface {
	Greeting() string
}

type testStruct struct{}

var _ testInterface = &testStruct{}

func (*testStruct) Greeting() string {
	return "Hello, World!"
}
