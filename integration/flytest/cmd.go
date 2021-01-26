package flytest

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/concourse/concourse/integration/cmdtest"
	"github.com/concourse/concourse/integration/dctest"
	"github.com/stretchr/testify/require"
)

type Cmd struct {
	cmdtest.Cmd
}

func Init(t *testing.T, dc dctest.Cmd) Cmd {
	webAddr, err := dc.Addr("web", 8080)
	require.NoError(t, err)

	fly := Cmd{
		Cmd: cmdtest.Cmd{
			Path: "fly",
			Args: []string{"-t", "opstest"},
		}.WithTempHome(t),
	}

	require.Eventually(t, func() bool {
		err := fly.Run("login", "-c", "http://"+webAddr, "-u", "test", "-p", "test")
		if err != nil {
			return false
		}

		workers, err := fly.Table(t, "workers")
		if err != nil {
			return false
		}

		for _, w := range workers {
			if w["state"] == "running" {
				return true
			}
		}

		return false
	}, time.Minute, time.Second)

	return fly
}

type Table []map[string]string

var colSplit = regexp.MustCompile(`\s{2,}`)

func (cmd Cmd) Table(t *testing.T, args ...string) (Table, error) {
	table, err := cmd.WithArgs("--print-table-headers").Output(args...)
	if err != nil {
		return nil, err
	}

	result := []map[string]string{}
	var headers []string

	rows := strings.Split(table, "\n")
	for i, row := range rows {
		columns := colSplit.Split(strings.TrimSpace(row), -1)

		if i == 0 {
			headers = columns
			continue
		}

		if row == "" {
			continue
		}

		result = append(result, map[string]string{})

		require.Len(t, columns, len(headers))

		for j, header := range headers {
			if header == "" || columns[j] == "" {
				continue
			}

			result[i-1][header] = columns[j]
		}
	}

	return result, nil
}
