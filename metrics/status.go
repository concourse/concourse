package metrics

import (
	"context"
)

const (
	StatusSucceeded = "succeeded"
	StatusErrored   = "errored"
	StatusTimeout   = "timeout"

	LabelStatus = "status"
)

func StatusFromError(err error) string {
	if err != nil {
		if err == context.DeadlineExceeded {
			return StatusTimeout
		}

		return StatusErrored
	}

	return StatusSucceeded
}
