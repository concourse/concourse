package api_test

import (
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
			err = fmt.Errorf("response does not contain header '%s'", expectedKey)
			return false, err
		}

		actualVal := header.Get(expectedKey)
		if expectedVal != actualVal {
			err = fmt.Errorf("expected response header '%s' to have value '%s', but got '%s'", expectedKey, expectedVal, actualVal)
			return false, err
		}
	}

	return true, nil
}

func (h includeHeaderEntries) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nto include headers\n\t%#v", actual, h.expected)
}

func (h includeHeaderEntries) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nto not include headers\n\t%#v", actual, h.expected)
}
