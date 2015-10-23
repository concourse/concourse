package integration_test

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/concourse/fly/pty"
	"github.com/concourse/fly/ui"
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
	expectedPTY, err := pty.Open()
	if err != nil {
		return false, err
	}

	defer expectedPTY.Close()

	buf := new(bytes.Buffer)

	reading := new(sync.WaitGroup)
	reading.Add(1)
	go func() {
		defer reading.Done()
		io.Copy(buf, expectedPTY.PTYR)
	}()

	err = matcher.table.Render(expectedPTY.TTYW)
	if err != nil {
		return false, err
	}

	expectedPTY.TTYW.Close()

	reading.Wait()

	matcher.expected = buf.String()

	switch v := actual.(type) {
	case *exec.Cmd:
		actualPTY, err := pty.Open()
		if err != nil {
			return false, err
		}

		defer actualPTY.Close()

		v.Stdin = actualPTY.TTYR
		v.Stdout = actualPTY.TTYW
		v.Stderr = actualPTY.TTYW

		actualBuf := new(bytes.Buffer)

		reading := new(sync.WaitGroup)
		reading.Add(1)
		go func() {
			defer reading.Done()
			io.Copy(actualBuf, actualPTY.PTYR)
		}()

		err = v.Run()
		if err != nil {
			return false, err
		}

		actualPTY.TTYW.Close()

		reading.Wait()

		matcher.actual = actualBuf.String()
	default:
		return false, fmt.Errorf("unknown type: %T", actual)
	}

	return strings.Contains(matcher.actual, matcher.expected), nil
}

func (matcher *PrintTableMatcher) FailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected\n%s\n(%q)\nTo contain the table\n%s\n(%q)\n", format.IndentString(matcher.actual, 1), matcher.actual, format.IndentString(matcher.expected, 1), matcher.expected)
}

func (matcher *PrintTableMatcher) NegatedFailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected\n%s\n(%q)\nTo not contain the table\n%s\n(%q)\n", format.IndentString(matcher.actual, 1), matcher.actual, format.IndentString(matcher.expected, 1), matcher.expected)
}
