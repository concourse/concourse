package atc

import (
	"io"
	"path/filepath"
)

type Resource struct {
	Name         string `json:"name"`
	PipelineName string `json:"pipeline_name"`
	TeamName     string `json:"team_name"`
	Type         string `json:"type"`
	LastChecked  int64  `json:"last_checked,omitempty"`

	Paused bool `json:"paused,omitempty"`

	FailingToCheck  bool   `json:"failing_to_check,omitempty"`
	CheckSetupError string `json:"check_setup_error,omitempty"`
	CheckError      string `json:"check_error,omitempty"`

	PinnedVersion  Version `json:"pinned_version,omitempty"`
	PinnedInConfig bool    `json:"pinned_in_config,omitempty"`
}

type IOConfig struct {
	Stdout io.Writer
	Stderr io.Writer
}

func ResourcesDir(suffix string) string {
	return filepath.Join("/tmp", "build", suffix)
}
