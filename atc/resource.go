package atc

import (
	"fmt"
	"io"
	"path/filepath"
)

var EnableGlobalResources bool

type Resource struct {
	Name         string `json:"name"`
	PipelineName string `json:"pipeline_name"`
	TeamName     string `json:"team_name"`
	Type         string `json:"type"`
	LastChecked  int64  `json:"last_checked,omitempty"`

	FailingToCheck  bool   `json:"failing_to_check,omitempty"`
	CheckSetupError string `json:"check_setup_error,omitempty"`
	CheckError      string `json:"check_error,omitempty"`

	PinnedVersion  Version `json:"pinned_version,omitempty"`
	PinnedInConfig bool    `json:"pinned_in_config,omitempty"`
	PinComment     string  `json:"pin_comment,omitempty"`
}

type IOConfig struct {
	Stdout io.Writer
	Stderr io.Writer
}

func ResourcesDir(suffix string) string {
	return filepath.Join("/tmp", "build", suffix)
}

type ErrResourceScriptFailed struct {
	Path       string
	Dir        string
	Args       []string
	ExitStatus int

	Stderr string
}

func (err ErrResourceScriptFailed) Error() string {
	var msg string

	if err.Dir != "" {
		msg = fmt.Sprintf(
			"resource script '%s %s' failed: exit status %d",
			err.Path,
			err.Dir,
			err.ExitStatus,
		)
	} else if err.Args != nil {
		msg = fmt.Sprintf(
			"resource script '%s %v' failed: exit status %d",
			err.Path,
			err.Args,
			err.ExitStatus,
		)
	} else {
		msg = fmt.Sprintf(
			"resource script '%s' failed: exit status %d",
			err.Path,
			err.ExitStatus,
		)
	}

	if len(err.Stderr) > 0 {
		msg += "\n\nstderr:\n" + err.Stderr
	}

	return msg
}
