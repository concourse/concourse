package resources

import (
	"encoding/json"
	"log"

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

func (checker *TurbineChecker) CheckResource(resource config.Resource, from builds.Version) []builds.Version {
	conn, err := checker.connect()
	if err != nil {
		return nil
	}

	buildInput := TurbineBuilds.Input{
		Type:    resource.Type,
		Source:  TurbineBuilds.Source(resource.Source),
		Version: TurbineBuilds.Version(from),
	}

	err = json.NewEncoder(conn).Encode(buildInput)
	if err != nil {
		log.Println("encoding input failed:", err)
		return nil
	}

	var newVersions []builds.Version
	err = json.NewDecoder(conn).Decode(&newVersions)
	if err != nil {
		log.Println("invalid check response:", err)
		return nil
	}

	checker.release(conn)

	return newVersions
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
