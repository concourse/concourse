package runtime

import "fmt"

// FileNotFoundError is the error to return from StreamFile when the given path
// does not exist.
type FileNotFoundError struct {
	Path string
}

// Error prints a helpful message including the file path. The user will see
// this message if e.g. their task config path does not exist.
func (err FileNotFoundError) Error() string {
	return fmt.Sprintf("file not found: %s", err.Path)
}

type ErrResourceScriptFailed struct {
	Path       string
	Args       []string
	ExitStatus int

	Stderr string
}

func (err ErrResourceScriptFailed) Error() string {
	msg := fmt.Sprintf(
		"resource script '%s %v' failed: exit status %d",
		err.Path,
		err.Args,
		err.ExitStatus,
	)

	if len(err.Stderr) > 0 {
		msg += "\n\nstderr:\n" + err.Stderr
	}

	return msg
}
