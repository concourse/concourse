package testhelpers

import (
	"encoding/json"
	"fmt"
	"github.com/onsi/gomega/types"
	"net/http"
)

func IncludeHeaderEntries(expected map[string]string) types.GomegaMatcher {
	return &includeHeaderEntries{
		expected: expected,
	}
}

type includeHeaderEntries struct {
	expected map[string]string
}

func (h includeHeaderEntries) Match(actual interface{}) (bool, error) {
	actualResponse, ok := actual.(*http.Response)
	if !ok {
		return false, fmt.Errorf("IncludeHeaderEntries matcher expects a *http.Response but got %T", actual)
	}
	return h.matchHeaderEntries(actualResponse.Header)
}

func (h includeHeaderEntries) matchHeaderEntries(header http.Header) (success bool, err error) {
	for expectedKey, expectedVal := range h.expected {

		_, keyFound := header[expectedKey]
		if !keyFound {
			return false, err
		}

		actualVal := header.Get(expectedKey)
		if expectedVal != actualVal {
			return false, err
		}
	}

	return true, nil
}

func (h includeHeaderEntries) FailureMessage(actual interface{}) (message string) {
	actualResponse, _ := actual.(*http.Response)
	bytes, _ := json.Marshal(actualResponse.Header)
	actualHeaders := string(bytes)

	bytes, _ = json.Marshal(h.expected)
	expectedHeaders := string(bytes)

	return fmt.Sprintf("Expected http response header\n\t%s\nto include headers\n\t%s", actualHeaders, expectedHeaders)
}

func (h includeHeaderEntries) NegatedFailureMessage(actual interface{}) (message string) {
	actualResponse, _ := actual.(*http.Response)
	bytes, _ := json.Marshal(actualResponse.Header)
	actualHeaders := string(bytes)

	bytes, _ = json.Marshal(h.expected)
	expectedHeaders := string(bytes)

	return fmt.Sprintf("Expected http response header\n\t%s\nto not include headers\n\t%s", actualHeaders, expectedHeaders)
}
