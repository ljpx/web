package web

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

// ByteSizeToFriendlyString returns the provided byte length as a human-friendly
// string e.g. 1024 => 1.00 kB.
func ByteSizeToFriendlyString(length int64) string {
	floatLength := float64(length)

	prefixes := []string{"B", "kB", "MB", "GB", "TB"}
	prefixIndex := 0

	for floatLength >= 1024 && prefixIndex < len(prefixes)-1 {
		floatLength /= 1024
		prefixIndex++
	}

	return fmt.Sprintf("%.2f %v", floatLength, prefixes[prefixIndex])
}

// UnmarshalFromResponse unmarshals the body of an http.Response to a model.
func UnmarshalFromResponse(res *http.Response, model interface{}) error {
	raw, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(raw, model)
}
