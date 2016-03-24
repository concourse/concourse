package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

func Volume(volume db.SavedVolume) atc.Volume {
	var resourceVersion atc.Version
	if volume.Volume.Identifier.ResourceCache != nil {
		resourceVersion = volume.Volume.Identifier.ResourceCache.ResourceVersion
	}

	return atc.Volume{
		ID:                volume.Handle,
		TTLInSeconds:      int64(volume.ExpiresIn.Seconds()),
		ValidityInSeconds: int64(volume.TTL.Seconds()),
		ResourceVersion:   resourceVersion,
		WorkerName:        volume.WorkerName,
	}
}
