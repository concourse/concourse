package ops_test

import (
	"fmt"
	"io"
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
	"github.com/stretchr/testify/suite"
)

var verbose io.Writer = ioutil.Discard

func TestOps(t *testing.T) {
	if testing.Verbose() {
		verbose = os.Stderr
	}

	suite.Run(t, &OpsSuite{
		Assertions: require.New(t),

		onces: map[string]*sync.Once{},
	})
}

type OpsSuite struct {
	suite.Suite
	*require.Assertions

	onces map[string]*sync.Once
}

func (s *OpsSuite) SetupSuite() {
	if os.Getenv("TEST_CONCOURSE_DEV_IMAGE") == "" {
		s.NoError(cmdtest.Cmd{
			Path: "docker-compose",
		}.Run("build"))
	}
}

func (s *OpsSuite) dockerCompose(overrides ...string) (cmdtest.Cmd, error) {
	name := filepath.Base(s.T().Name())

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
	s.cleanupOnce(func() {
		if s.T().Failed() {
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

func (s *OpsSuite) dynamicDockerCompose(doc *ypath.Document) (cmdtest.Cmd, error) {
	name := filepath.Base(s.T().Name())
	fileName := fmt.Sprintf(".docker-compose.%s.yml", name)

	err := ioutil.WriteFile(fileName, doc.Bytes(), os.ModePerm)
	if err != nil {
		return cmdtest.Cmd{}, err
	}

	s.cleanupOnce(func() {
		os.Remove(fileName)
	})

	return s.dockerCompose(fileName)
}

func (s *OpsSuite) cleanupOnce(cleanup func()) {
	name := s.T().Name()

	once, found := s.onces[name]
	if !found {
		once = new(sync.Once)
		s.onces[name] = once
	}

	s.T().Cleanup(func() {
		once.Do(cleanup)
	})
}

func (s *OpsSuite) addr(dc cmdtest.Cmd, container string, port int) (string, error) {
	out, err := dc.Output("port", container, strconv.Itoa(port))
	if err != nil {
		return "", err
	}

	return strings.TrimRight(strings.Replace(out, "0.0.0.0", "127.0.0.1", 1), "\n"), nil
}

var colSplit = regexp.MustCompile(`\s{2,}`)

func (s *OpsSuite) flyTable(fly cmdtest.Cmd, args ...string) ([]map[string]string, error) {
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

		s.Len(columns, len(headers))

		for j, header := range headers {
			if header == "" || columns[j] == "" {
				continue
			}

			result[i-1][header] = columns[j]
		}
	}

	return result, nil
}

func (s *OpsSuite) initFly(dc cmdtest.Cmd) cmdtest.Cmd {
	webAddr, err := s.addr(dc, "web", 8080)
	s.NoError(err)

	fly, err := cmdtest.Cmd{
		Path: "fly",
		Args: []string{"-t", "opstest"},
	}.WithTempHome(s.T())
	s.NoError(err)

	s.Eventually(func() bool {
		err := fly.Run("login", "-c", "http://"+webAddr, "-u", "test", "-p", "test")
		if err != nil {
			return false
		}

		workers, err := s.flyTable(fly, "workers")
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
