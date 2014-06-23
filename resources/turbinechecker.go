package resources

import (
	"encoding/json"

	"code.google.com/p/go.net/websocket"
	TurbineBuilds "github.com/concourse/turbine/api/builds"
	"github.com/concourse/turbine/routes"
	"github.com/tedsuo/router"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
)

type TurbineChecker struct {
	turbine *router.RequestGenerator

	connections chan *websocket.Conn
}

func NewTurbineChecker(turbine *router.RequestGenerator) Checker {
	return &TurbineChecker{
		turbine: turbine,

		connections: make(chan *websocket.Conn, 1),
	}
}

func (checker *TurbineChecker) CheckResource(resource config.Resource, from builds.Version) ([]builds.Version, error) {
	conn, err := checker.connect()
	if err != nil {
		return nil, err
	}

	buildInput := TurbineBuilds.Input{
		Type:    resource.Type,
		Source:  TurbineBuilds.Source(resource.Source),
		Version: TurbineBuilds.Version(from),
	}

	err = json.NewEncoder(conn).Encode(buildInput)
	if err != nil {
		return nil, err
	}

	var newVersions []builds.Version
	err = json.NewDecoder(conn).Decode(&newVersions)
	if err != nil {
		return nil, err
	}

	checker.release(conn)

	return newVersions, nil
}

func (checker *TurbineChecker) connect() (*websocket.Conn, error) {
	select {
	case conn := <-checker.connections:
		return conn, nil
	default:
		req, err := checker.turbine.RequestForHandler(
			routes.CheckInputStream,
			nil,
			nil,
		)
		if err != nil {
			return nil, err
		}

		req.URL.Scheme = "ws"

		return websocket.Dial(req.URL.String(), "", "http://0.0.0.0")
	}
}

func (checker *TurbineChecker) release(conn *websocket.Conn) {
	select {
	case checker.connections <- conn:
	default:
		conn.Close()
	}
}
