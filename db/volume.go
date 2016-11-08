package db

import "github.com/concourse/atc"

type ResourceCacheIdentifier struct {
	ResourceVersion atc.Version
	ResourceHash    string
}
