package db

import (
	"time"

	"github.com/concourse/atc"
)

type Volume struct {
	Handle               string
	WorkerName           string
	TTL                  time.Duration
	OriginalVolumeHandle string
	VolumeIdentifier
}

type VolumeIdentifier struct {
	ResourceVersion atc.Version
	ResourceHash    string
}

type SavedVolume struct {
	Volume

	ID        int
	ExpiresIn time.Duration
}
