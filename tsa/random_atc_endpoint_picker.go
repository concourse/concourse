package tsa

import (
	"math/rand"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/flag"
)

type randomATCEndpointPicker struct {
	ATCEndpoints []atc.Endpoint
}

func NewRandomATCEndpointPicker(atcURLFlags []flag.URL) EndpointPicker {
	atcEndpoints := []atc.Endpoint{}
	for _, f := range atcURLFlags {
		atcEndpoints = append(atcEndpoints, atc.NewEndpoint(f.String()))
	}

	rand.Seed(time.Now().Unix())

	return &randomATCEndpointPicker{
		ATCEndpoints: atcEndpoints,
	}
}

func (p *randomATCEndpointPicker) Pick() atc.Endpoint {
	return p.ATCEndpoints[rand.Intn(len(p.ATCEndpoints))]
}
