package dctest

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/concourse/concourse/integration/internal/cmdtest"
	"github.com/concourse/concourse/integration/internal/ypath"
	"github.com/stretchr/testify/require"
)

type Cmd struct {
	cmdtest.Cmd
}

func Init(t *testing.T, files ...string) Cmd {
	name := filepath.Base(t.Name())

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
			err := os.MkdirAll("logs", os.ModePerm)
			if err == nil {
				logFile, err := os.Create("logs/" + name + ".log")
				if err == nil {
					dc.Silence().OutputTo(logFile).Run("logs", "--no-color")
					logFile.Close()
				}
			}
		}

		dc.Run("kill")
		dc.Run("down")
	})

	return Cmd{
		Cmd: dc,
	}
}

func InitDynamic(t *testing.T, doc *ypath.Document) Cmd {
	name := filepath.Base(t.Name())
	fileName := fmt.Sprintf(".docker-compose.%s.yml", name)

	err := ioutil.WriteFile(fileName, doc.Bytes(), os.ModePerm)
	require.NoError(t, err)

	cleanupOnce(t, func() {
		os.Remove(fileName)
	})

	return Init(t, fileName)
}

func (cmd Cmd) Addr(container string, port int) (string, error) {
	out, err := cmd.Output("port", container, strconv.Itoa(port))
	if err != nil {
		return "", err
	}

	return strings.TrimRight(strings.Replace(out, "0.0.0.0", "127.0.0.1", 1), "\n"), nil
}

var onces = new(sync.Map)

func cleanupOnce(t *testing.T, cleanup func()) {
	name := t.Name()

	once, _ := onces.LoadOrStore(name, new(sync.Once))

	t.Cleanup(func() {
		once.(*sync.Once).Do(cleanup)
	})
}
