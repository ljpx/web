package web

import (
	"net/http"
	"time"
)

// MeasuredResponseWriter wraps a standard http.ResponseWriter with additional
// functionality.  Specifically, it records the amount of data written, the
// response code, and the duration of the request/response.
type MeasuredResponseWriter struct {
	w                 http.ResponseWriter
	startTime         time.Time
	statusCode        int
	volume            int64
	hasWrittenHeaders bool
}

// NewMeasuredResponseWriter creates a new MeasuredResponseWriter with the provided
// underlying http.ResponseWriter.
func NewMeasuredResponseWriter(w http.ResponseWriter) *MeasuredResponseWriter {
	return &MeasuredResponseWriter{
		w:         w,
		startTime: time.Now(),
	}
}

var _ http.ResponseWriter = &MeasuredResponseWriter{}

// Header simply returns the headers of the underlying response writer.
func (mrw *MeasuredResponseWriter) Header() http.Header {
	return mrw.w.Header()
}

// Write writes to the underlying response writer, recording the number of bytes
// successfully written.
func (mrw *MeasuredResponseWriter) Write(b []byte) (int, error) {
	n, err := mrw.w.Write(b)
	mrw.volume += int64(n)

	return n, err
}

// WriteHeader records and writes the header if it has not already been written.
func (mrw *MeasuredResponseWriter) WriteHeader(statusCode int) {
	if mrw.hasWrittenHeaders {
		return
	}

	mrw.statusCode = statusCode
	mrw.w.WriteHeader(statusCode)
	mrw.hasWrittenHeaders = true
}

// StatusCode returns the status code that was written for the response.  If the
// status code is yet to be written, or WriteHeader was never explicitly called,
// StatusCode will return http.StatusOK.
func (mrw *MeasuredResponseWriter) StatusCode() int {
	if mrw.statusCode == 0 {
		return http.StatusOK
	}

	return mrw.statusCode
}

// HasWrittenHeaders returns true if WriteHeader has been called.
func (mrw *MeasuredResponseWriter) HasWrittenHeaders() bool {
	return mrw.hasWrittenHeaders
}

// Duration returns the duration between the start of the request and now.
func (mrw *MeasuredResponseWriter) Duration() time.Duration {
	dur := time.Now().Sub(mrw.startTime)

	if dur < time.Millisecond*5 {
		dur = time.Duration(0)
	}

	return dur
}

// Volume returns the number of bytes written to the response writer body.
func (mrw *MeasuredResponseWriter) Volume() int64 {
	return mrw.volume
}
