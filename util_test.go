package web

import (
	"net/http/httptest"
	"testing"

	"github.com/ljpx/test"
)

func TestByteSizeToFriendlyString(t *testing.T) {
	testCases := []struct {
		given    int64
		expected string
	}{
		{given: 1, expected: "1.00 B"},
		{given: 1023, expected: "1023.00 B"},
		{given: 1024, expected: "1.00 kB"},
		{given: 1536, expected: "1.50 kB"},
		{given: 1048576, expected: "1.00 MB"},
		{given: 3214822145, expected: "2.99 GB"},
	}

	for _, testCase := range testCases {
		actual := ByteSizeToFriendlyString(testCase.given)
		test.That(t, actual).IsEqualTo(testCase.expected)
	}
}

func TestUnmarshalFromResponseRecorder(t *testing.T) {
	// Arrange.
	m := &struct{ Name string }{}
	w := httptest.NewRecorder()
	w.Write([]byte(`{"Name":"John Smith"}`))

	// Act.
	err := UnmarshalFromResponse(w.Result(), m)

	// Assert.
	test.That(t, err).IsNil()
	test.That(t, m.Name).IsEqualTo("John Smith")
}
