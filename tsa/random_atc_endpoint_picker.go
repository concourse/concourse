package tsa

import (
	"math/rand"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/flag/v2"
	"github.com/tedsuo/rata"
)

type randomATCEndpointPicker struct {
	ATCEndpoints []*rata.RequestGenerator
}

func NewRandomATCEndpointPicker(atcURLFlags []flag.URL) EndpointPicker {
	atcEndpoints := []*rata.RequestGenerator{}
	for _, f := range atcURLFlags {
		atcEndpoints = append(atcEndpoints, rata.NewRequestGenerator(f.String(), atc.Routes))
	}

	return &randomATCEndpointPicker{
		ATCEndpoints: atcEndpoints,
	}
}

func (p *randomATCEndpointPicker) Pick() *rata.RequestGenerator {
	return p.ATCEndpoints[rand.Intn(len(p.ATCEndpoints))]
}
