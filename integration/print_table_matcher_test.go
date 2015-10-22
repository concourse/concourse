package integration_test

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/concourse/fly/ui"
	"github.com/kr/pty"
	"github.com/onsi/gomega/format"
)

type PrintTableMatcher struct {
	table ui.Table

	actual   string
	expected string
}

func PrintTable(table ui.Table) *PrintTableMatcher {
	return &PrintTableMatcher{table: table}
}

func (matcher *PrintTableMatcher) Match(actual interface{}) (bool, error) {
	expectedPTY, expectedTTY, err := pty.Open()
	if err != nil {
		return false, err
	}

	defer expectedTTY.Close()
	defer expectedPTY.Close()

	buf := new(bytes.Buffer)

	reading := new(sync.WaitGroup)
	reading.Add(1)
	go func() {
		defer reading.Done()
		io.Copy(buf, expectedPTY)
	}()

	err = matcher.table.Render(expectedTTY)
	if err != nil {
		return false, err
	}

	expectedTTY.Close()

	reading.Wait()

	matcher.expected = buf.String()

	switch v := actual.(type) {
	case *exec.Cmd:
		actualPTY, actualTTY, err := pty.Open()
		if err != nil {
			return false, err
		}

		defer actualTTY.Close()
		defer actualPTY.Close()

		v.Stdout = actualTTY
		v.Stderr = actualTTY

		actualBuf := new(bytes.Buffer)

		reading := new(sync.WaitGroup)
		reading.Add(1)
		go func() {
			defer reading.Done()
			io.Copy(actualBuf, actualPTY)
		}()

		err = v.Run()
		if err != nil {
			return false, err
		}

		actualTTY.Close()

		reading.Wait()

		matcher.actual = actualBuf.String()
	default:
		return false, fmt.Errorf("unknown type: %T", actual)
	}

	return strings.Contains(matcher.actual, matcher.expected), nil
}

func (matcher *PrintTableMatcher) FailureMessage(actual interface{}) string {
	return format.Message(matcher.actual, "To contain the table", matcher.expected)
}

func (matcher *PrintTableMatcher) NegatedFailureMessage(actual interface{}) string {
	return format.Message(matcher.actual, "To not contain the table", matcher.expected)
}
