package present

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func Component(component db.Component) atc.Component {
	return atc.Component{
		Name:     component.Name(),
		Interval: component.Interval(),
		LastRan:  component.LastRan(),
		Paused:   component.Paused(),
	}
}
