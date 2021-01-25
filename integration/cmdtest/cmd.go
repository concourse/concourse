package cmdtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"
)

type Cmd struct {
	Path   string
	Env    []string
	Args   []string
	Stdout io.Writer
	Stderr io.Writer
	Silent bool
}

func (cmd Cmd) WithEnv(env ...string) Cmd {
	newEnv := make([]string, len(cmd.Env))
	copy(newEnv, cmd.Env)
	newEnv = append(newEnv, env...)

	cmd.Env = newEnv
	return cmd
}

func (cmd Cmd) WithArgs(args ...string) Cmd {
	newArgs := make([]string, len(cmd.Args))
	copy(newArgs, cmd.Args)
	newArgs = append(newArgs, args...)

	cmd.Args = newArgs
	return cmd
}

func (cmd Cmd) WithTempHome(t *testing.T) (Cmd, error) {
	home, err := ioutil.TempDir("", "cmdtest-home")
	if err != nil {
		return Cmd{}, fmt.Errorf("create home tmpdir: %w", err)
	}

	t.Cleanup(func() {
		os.RemoveAll(home)
	})

	return cmd.WithEnv("HOME=" + home), nil
}

func (cmd Cmd) Silence() Cmd {
	cmd.Silent = true
	return cmd
}

func (cmd Cmd) OutputTo(out io.Writer) Cmd {
	cmd.Stdout = out
	return cmd
}

func (cmd Cmd) Run(args ...string) error {
	if cmd.Env == nil {
		// don't inherit anything
		cmd.Env = []string{}
	}

	execCmd := exec.Command(cmd.Path, append(cmd.Args, args...)...)
	execCmd.Env = cmd.Env
	execCmd.Stdout = cmd.Stdout
	execCmd.Stderr = cmd.Stderr

	var verbose io.Writer = ioutil.Discard
	if testing.Verbose() {
		verbose = os.Stderr
	}

	if !cmd.Silent {
		if execCmd.Stdout != nil && verbose != ioutil.Discard {
			execCmd.Stdout = io.MultiWriter(execCmd.Stdout, verbose)
		} else {
			execCmd.Stdout = verbose
		}

		if execCmd.Stderr != nil && verbose != ioutil.Discard {
			execCmd.Stderr = io.MultiWriter(execCmd.Stderr, verbose)
		} else {
			execCmd.Stderr = verbose
		}
	}

	cmdStr := strings.Join(execCmd.Args, " ")

	fmt.Fprintf(verbose, "\x1b[33m==== EXEC %s\x1b[0m\n", cmdStr)

	err := execCmd.Run()
	if err != nil {
		fmt.Fprintf(verbose, "\x1b[33m==== %s\x1b[0m\n", err)
		return fmt.Errorf("run %s: %w", cmdStr, err)
	}

	return nil
}

func (cmd Cmd) Output(args ...string) (string, error) {
	buf := new(bytes.Buffer)

	cmd.Stdout = buf

	err := cmd.Run(args...)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (cmd Cmd) OutputJSON(dest interface{}, args ...string) error {
	buf := new(bytes.Buffer)

	cmd.Stdout = buf

	err := cmd.Run(args...)
	if err != nil {
		return err
	}

	err = json.Unmarshal(buf.Bytes(), dest)
	if err != nil {
		return err
	}

	return nil
}
