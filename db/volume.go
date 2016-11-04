package db

import (
	"time"

	"github.com/concourse/atc"
)

type Volume struct {
	Handle       string
	WorkerName   string
	TeamID       int
	ContainerTTL *time.Duration
	TTL          time.Duration
	SizeInBytes  int64
}

type ResourceCacheIdentifier struct {
	ResourceVersion atc.Version
	ResourceHash    string
}

type SavedVolume struct {
	Volume

	ID        int
	ExpiresIn time.Duration
}
