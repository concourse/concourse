package present

import (
	"encoding/json"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

func Volume(volume db.VolumeData) atc.Volume {
	version, _ := json.Marshal(volume.ResourceVersion)

	return atc.Volume{
		ID:              volume.Handle,
		TTLInSeconds:    int64(volume.TTL.Seconds()),
		ResourceVersion: (*json.RawMessage)(&version),
		WorkerName:      volume.WorkerName,
	}
}
