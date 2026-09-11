package present

import (
	"github.com/concourse/concourse/v8/atc"
	"github.com/concourse/concourse/v8/atc/db"
)

func Component(component db.Component) atc.Component {
	return atc.Component{
		Name:     component.Name(),
		Interval: component.Interval(),
		LastRan:  component.LastRan(),
		Paused:   component.Paused(),
	}
}
