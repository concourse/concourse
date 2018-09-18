package integration_test

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/concourse/fly/ui"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/gbytes"
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
	buf := new(bytes.Buffer)

	switch v := actual.(type) {
	case *exec.Cmd:
		actualBuf := new(bytes.Buffer)

		v.Stdout = actualBuf
		v.Stderr = actualBuf

		err := v.Run()
		if err != nil {
			return false, err
		}

		matcher.actual = actualBuf.String()
	case *gbytes.Buffer:
		matcher.actual = string(v.Contents())
	default:
		return false, fmt.Errorf("unknown type: %T", actual)
	}

	err := matcher.compareDurations()
	if err != nil {
		return false, err
	}

	err = matcher.table.Render(buf, false)
	if err != nil {
		return false, err
	}

	matcher.expected = buf.String()

	return strings.Contains(matcher.actual, matcher.expected), nil
}

func (matcher *PrintTableMatcher) compareDurations() error {
	actualRows := strings.Split(matcher.actual, "\n")
	if len(actualRows)-1 != len(matcher.table.Data) {
		return nil
	}

	for lineIdx, row := range matcher.table.Data {
		actualRow := actualRows[lineIdx]
		actualCells := strings.Fields(actualRow)
		if len(actualCells) != len(row) {
			continue
		}

		for cellIdx, cell := range row {
			tableDuration, parsed := ParseTableDuration(cell.Contents)
			if !parsed {
				continue
			}

			actualCell := actualCells[cellIdx]
			err := tableDuration.MatchString(actualCell)
			if err != nil {
				return err
			}

			matcher.table.Data[lineIdx][cellIdx].Contents = actualCell
		}
	}

	return nil
}

func (matcher *PrintTableMatcher) FailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected\n%s\n(%q)\nTo contain the table\n%s\n(%q)\n", format.IndentString(matcher.actual, 1), matcher.actual, format.IndentString(matcher.expected, 1), matcher.expected)
}

func (matcher *PrintTableMatcher) NegatedFailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected\n%s\n(%q)\nTo not contain the table\n%s\n(%q)\n", format.IndentString(matcher.actual, 1), matcher.actual, format.IndentString(matcher.expected, 1), matcher.expected)
}
