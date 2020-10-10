package ui

import "github.com/concourse/concourse/atc"

func BuildStatusCell(status atc.BuildStatus) TableCell {
	var statusCell TableCell
	statusCell.Contents = status.String()

	switch status {
	case atc.StatusPending:
		statusCell.Color = PendingColor
	case atc.StatusStarted:
		statusCell.Color = StartedColor
	case atc.StatusSucceeded:
		statusCell.Color = SucceededColor
	case atc.StatusFailed:
		statusCell.Color = FailedColor
	case atc.StatusErrored:
		statusCell.Color = ErroredColor
	case atc.StatusAborted:
		statusCell.Color = AbortedColor
	default:
		// ?
		statusCell.Color = BlinkingErrorColor
	}

	return statusCell
}
