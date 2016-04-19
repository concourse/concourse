package db

import (
	"time"

	"github.com/concourse/atc"
)

type Volume struct {
	Handle     string
	WorkerName string
	TTL        time.Duration
	Identifier VolumeIdentifier
}

// pls gib algebraic data types
type VolumeIdentifier struct {
	ResourceCache *ResourceCacheIdentifier
	COW           *COWIdentifier
	Output        *OutputIdentifier
	Import        *ImportIdentifier
}

type ResourceCacheIdentifier struct {
	ResourceVersion atc.Version
	ResourceHash    string
}

type COWIdentifier struct {
	ParentVolumeHandle string
}

type OutputIdentifier struct {
	Name string
}

type ImportIdentifier struct {
	WorkerName string
	Path       string
	Version    string
}

type SavedVolume struct {
	Volume

	ID        int
	ExpiresIn time.Duration
}
