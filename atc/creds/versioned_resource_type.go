package creds

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/vars"
)

// It's tempting to just vars.InterpolateInto(...) an atc.VersionedResourceTypes directly, but
// the prior behaviour is to only interpolate Source, so let's leave this as is
func InterpolateVersionedResourceTypes(raw atc.VersionedResourceTypes, v vars.Variables) (atc.VersionedResourceTypes, error) {
	out := make(atc.VersionedResourceTypes, len(raw))
	for i, vrt := range raw {
		out[i] = vrt
		var src atc.Source
		if err := vars.InterpolateInto(vrt.Source, vars.NewResolver(v), &src); err != nil {
			return nil, err
		}
		out[i].Source = src
	}
	return out, nil
}
