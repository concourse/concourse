package ops_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/concourse/concourse/integration/cmdtest"
	"github.com/concourse/concourse/integration/ypath"
	"github.com/stretchr/testify/require"
)

var onces = new(sync.Map)

func dockerCompose(t *testing.T, overrides ...string) (cmdtest.Cmd, error) {
	name := filepath.Base(t.Name())

	files := []string{"docker-compose.yml"}
	for _, file := range overrides {
		files = append(files, file)
	}

	dc := cmdtest.Cmd{
		Path: "docker-compose",
		Env: []string{
			"COMPOSE_FILE=" + strings.Join(files, ":"),
			"COMPOSE_PROJECT_NAME=" + name,
		},
	}

	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "TEST_") {
			dc = dc.WithEnv(env)
		}
	}

	// clean up docker-compose when the test finishes
	cleanupOnce(t, func() {
		if t.Failed() {
			logFile, err := os.Create("logs/" + name + ".log")
			if err == nil {
				dc.Silence().OutputTo(logFile).Run("logs", "--no-color")
				logFile.Close()
			}
		}

		dc.Run("kill")
		dc.Run("down")
	})

	return dc, nil
}

func dynamicDockerCompose(t *testing.T, doc *ypath.Document) (cmdtest.Cmd, error) {
	name := filepath.Base(t.Name())
	fileName := fmt.Sprintf(".docker-compose.%s.yml", name)

	err := ioutil.WriteFile(fileName, doc.Bytes(), os.ModePerm)
	if err != nil {
		return cmdtest.Cmd{}, err
	}

	cleanupOnce(t, func() {
		os.Remove(fileName)
	})

	return dockerCompose(t, fileName)
}

func cleanupOnce(t *testing.T, cleanup func()) {
	name := t.Name()

	once, _ := onces.LoadOrStore(name, new(sync.Once))

	t.Cleanup(func() {
		once.(*sync.Once).Do(cleanup)
	})
}

func addr(dc cmdtest.Cmd, container string, port int) (string, error) {
	out, err := dc.Output("port", container, strconv.Itoa(port))
	if err != nil {
		return "", err
	}

	return strings.TrimRight(strings.Replace(out, "0.0.0.0", "127.0.0.1", 1), "\n"), nil
}

var colSplit = regexp.MustCompile(`\s{2,}`)

func flyTable(t *testing.T, fly cmdtest.Cmd, args ...string) ([]map[string]string, error) {
	table, err := fly.WithArgs("--print-table-headers").Output(args...)
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

func initFly(t *testing.T, dc cmdtest.Cmd) cmdtest.Cmd {
	webAddr, err := addr(dc, "web", 8080)
	require.NoError(t, err)

	fly, err := cmdtest.Cmd{
		Path: "fly",
		Args: []string{"-t", "opstest"},
	}.WithTempHome(t)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		err := fly.Run("login", "-c", "http://"+webAddr, "-u", "test", "-p", "test")
		if err != nil {
			return false
		}

		workers, err := flyTable(t, fly, "workers")
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
