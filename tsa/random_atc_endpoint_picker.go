package tsa

import (
	"math/rand"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/flag"
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

	rand.Seed(time.Now().Unix())

	return &randomATCEndpointPicker{
		ATCEndpoints: atcEndpoints,
	}
}

func (p *randomATCEndpointPicker) Pick() *rata.RequestGenerator {
	return p.ATCEndpoints[rand.Intn(len(p.ATCEndpoints))]
}
