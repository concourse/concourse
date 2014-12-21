package v2event

import "github.com/concourse/atc"

const (
	EventTypeInvalid atc.EventType = ""

	// input fetched
	EventTypeInput atc.EventType = "input"

	// output completed
	EventTypeOutput atc.EventType = "output"
)
