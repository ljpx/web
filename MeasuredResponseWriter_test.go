package web

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ljpx/test"
)

type MeasuredResponseWriterFixture struct {
	w *httptest.ResponseRecorder
	x *MeasuredResponseWriter
}

func SetupMeasuredResponseWriterFixture() *MeasuredResponseWriterFixture {
	fixture := &MeasuredResponseWriterFixture{}
	fixture.w = httptest.NewRecorder()
	fixture.x = NewMeasuredResponseWriter(fixture.w)

	return fixture
}

func TestMeasuredResponseWriterShouldGetCorrectHeaders(t *testing.T) {
	// Arrange.
	fixture := SetupMeasuredResponseWriterFixture()
	fixture.w.Header().Set("X-Test-Header", "test-value")

	// Act.
	headers := fixture.x.Header()
	testHeaderValue := headers.Get("X-Test-Header")

	// Assert.
	test.That(t, fixture.w.Header().Get("X-Test-Header")).IsEqualTo(testHeaderValue)
}

func TestMeasuredResponseWriterShouldWriteAndRecordVolumeCorrectly(t *testing.T) {
	// Arrange.
	fixture := SetupMeasuredResponseWriterFixture()
	fixture.x.Write([]byte("Hello, World!"))

	// Act.
	volume := fixture.x.Volume()
	response := fixture.w.Result()
	raw, err := ioutil.ReadAll(response.Body)

	// Assert.
	test.That(t, err).IsNil()
	test.That(t, string(raw)).IsEqualTo("Hello, World!")
	test.That(t, volume).IsEqualTo(int64(13))
}

func TestMeasuredResponseWriterShouldOnlySetResponseCodeOnce(t *testing.T) {
	// Arrange.
	fixture := SetupMeasuredResponseWriterFixture()
	fixture.x.WriteHeader(http.StatusBadRequest)
	fixture.x.WriteHeader(http.StatusForbidden)

	// Act.
	response := fixture.w.Result()

	// Assert.
	test.That(t, response.StatusCode).IsEqualTo(http.StatusBadRequest)
	test.That(t, fixture.x.StatusCode()).IsEqualTo(http.StatusBadRequest)
}

func TestMeasuredResponseWriterShouldReturn200ByDefault(t *testing.T) {
	// Arrange.
	fixture := SetupMeasuredResponseWriterFixture()

	// Act and Assert.
	test.That(t, fixture.x.StatusCode()).IsEqualTo(http.StatusOK)
}

func TestMeasuredResponseWriterShouldReturnFalseForHasWrittenHeaders(t *testing.T) {
	// Arrange.
	fixture := SetupMeasuredResponseWriterFixture()

	// Act.
	hasWrittenHeaders := fixture.x.HasWrittenHeaders()

	// Assert.
	test.That(t, hasWrittenHeaders).IsFalse()
}

func TestMeasuredResponseWriterShouldReturnTrueForHasWrittenHeaders(t *testing.T) {
	// Arrange.
	fixture := SetupMeasuredResponseWriterFixture()
	fixture.x.WriteHeader(http.StatusCreated)

	// Act.
	hasWrittenHeaders := fixture.x.HasWrittenHeaders()

	// Assert.
	test.That(t, hasWrittenHeaders).IsTrue()
}

func TestMeasuredResponseWriterShouldReturnCorrectDuration(t *testing.T) {
	// Arrange.
	fixture := SetupMeasuredResponseWriterFixture()
	time.Sleep(time.Millisecond * 50)

	// Act.
	dur := fixture.x.Duration()

	// Assert.
	expected := float64(time.Millisecond) * 50
	actual := float64(dur)
	delta := float64(time.Millisecond) * 5

	test.That(t, actual).IsGreaterThanOrEqualTo(expected - delta)
	test.That(t, actual).IsLessThanOrEqualTo(expected + delta)
}
