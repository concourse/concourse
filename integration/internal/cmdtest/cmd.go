package cmdtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

type Cmd struct {
	Path   string
	Env    []string
	Args   []string
	Stdin  io.Reader
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

func (cmd Cmd) WithTempHome(t *testing.T) (Cmd, string) {
	home := t.TempDir()
	return cmd.WithEnv("HOME=" + home), home
}

func (cmd Cmd) Silence() Cmd {
	cmd.Silent = true
	return cmd
}

func (cmd Cmd) WithInput(in io.Reader) Cmd {
	cmd.Stdin = in
	return cmd
}

func (cmd Cmd) OutputTo(out io.Writer) Cmd {
	cmd.Stdout = out
	return cmd
}

func (cmd Cmd) Run(args ...string) error {
	env := []string{
		// only inherit $PATH; we don't want to pass *everything* along because
		// then it's unclear what's necessary for the tests, but $PATH seems
		// necessary for basic functionality
		"PATH=" + os.Getenv("PATH"),
	}

	execCmd := exec.Command(cmd.Path, append(cmd.Args, args...)...)
	execCmd.Env = append(env, cmd.Env...)
	execCmd.Stdin = cmd.Stdin
	execCmd.Stdout = cmd.Stdout
	execCmd.Stderr = cmd.Stderr

	// always write to os.Stderr
	//
	// assuming these tests are run like go test ./..., we want output to be
	// shown when the test fails, and 'go test' already does that at the package
	// level.
	//
	// we could try to set this only if testing.Verbose(), but that would mean we
	// have to pass -v, which would result in showing all output even if the
	// tests pass, which is probably too noisy.
	verbose := os.Stderr

	if !cmd.Silent {
		if execCmd.Stdout != nil {
			execCmd.Stdout = io.MultiWriter(execCmd.Stdout, verbose)
		} else {
			execCmd.Stdout = verbose
		}

		if execCmd.Stderr != nil {
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
