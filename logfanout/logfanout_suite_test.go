package logfanout_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLogfanout(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Logfanout Suite")
}

func rawMSG(msg string) *json.RawMessage {
	payload := []byte(msg)
	return (*json.RawMessage)(&payload)
}
