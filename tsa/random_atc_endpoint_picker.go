package tsa

import (
	"math/rand"
	"time"

	"github.com/concourse/concourse/atc/routes"
	"github.com/concourse/flag"
)

type randomATCEndpointPicker struct {
	ATCEndpoints []routes.Endpoint
}

func NewRandomATCEndpointPicker(atcURLFlags []flag.URL) EndpointPicker {
	atcEndpoints := []routes.Endpoint{}
	for _, f := range atcURLFlags {
		atcEndpoints = append(atcEndpoints, routes.NewEndpoint(f.String()))
	}

	rand.Seed(time.Now().Unix())

	return &randomATCEndpointPicker{
		ATCEndpoints: atcEndpoints,
	}
}

func (p *randomATCEndpointPicker) Pick() routes.Endpoint {
	return p.ATCEndpoints[rand.Intn(len(p.ATCEndpoints))]
}
