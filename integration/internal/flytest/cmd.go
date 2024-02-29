package flytest

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/concourse/concourse/integration/internal/cmdtest"
	"github.com/concourse/concourse/integration/internal/dctest"
	"github.com/stretchr/testify/require"
)

type Cmd struct {
	cmdtest.Cmd
}

func InitUnauthenticated(t *testing.T, dc dctest.Cmd) (Cmd, string) {
	webAddr := dc.Addr(t, "web", 8080)

	webURL := "http://" + webAddr

	cmd, home := cmdtest.Cmd{
		Path: "fly",
		Args: []string{"-t", "opstest"},
	}.WithTempHome(t)

	cliURL := fmt.Sprintf(
		"%s/api/v1/cli?arch=%s&platform=%s",
		webURL,
		runtime.GOARCH,
		runtime.GOOS,
	)

	var flyResp *http.Response
	require.Eventually(t, func() bool {
		var err error
		flyResp, err = http.Get(cliURL)
		return err == nil
	}, time.Minute, time.Second)

	flyPath := filepath.Join(home, "fly")

	flyFile, err := os.OpenFile(flyPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	require.NoError(t, err)

	_, err = io.Copy(flyFile, flyResp.Body)
	require.NoError(t, err)

	require.NoError(t, flyFile.Close())

	cmd.Path = flyPath

	return Cmd{
		Cmd: cmd,
	}, webURL
}

func Init(t *testing.T, dc dctest.Cmd) Cmd {
	fly, webURL := InitUnauthenticated(t, dc)

	fly.Run(t, "login", "-c", webURL, "-u", "test", "-p", "test")

	fly.WaitForRunningWorker(t)

	return fly
}

func InitOverrideCredentials(t *testing.T, dc dctest.Cmd) Cmd {
	fly, webURL := InitUnauthenticated(t, dc)

	fly.WithEnv("CONCOURSE_VAULT_CLIENT_CERT=/vault-certs/concourse.crt")
	fly.WithEnv("CONCOURSE_VAULT_CLIENT_KEY=/vault-certs/concourse.key")
	fly.Run(t, "login", "-c", webURL, "-u", "test", "-p", "test")

	fly.WaitForRunningWorker(t)

	return fly
}

func (cmd Cmd) WaitForRunningWorker(t *testing.T) {
	require.Eventually(t, func() bool {
		for _, w := range cmd.Table(t, "workers") {
			if w["state"] == "running" {
				return true
			}
		}

		return false
	}, time.Minute, time.Second, "should have a running worker")
}

type Table []map[string]string

var colSplit = regexp.MustCompile(`\s{2,}`)

func (cmd Cmd) Table(t *testing.T, args ...string) Table {
	table := cmd.WithArgs("--print-table-headers").Output(t, args...)

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

	return result
}

func (cmd Cmd) PipelineIsPaused(t *testing.T, pipeline string) bool {
	pausedPipelines := cmd.Output(t, "paused-pipelines")
	return strings.Contains(pausedPipelines, pipeline)
}
