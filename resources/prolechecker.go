package resources

import (
	"encoding/json"
	"log"

	"code.google.com/p/go.net/websocket"
	"github.com/tedsuo/router"
	ProleBuilds "github.com/winston-ci/prole/api/builds"
	"github.com/winston-ci/prole/routes"

	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
)

type ProleChecker struct {
	prole *router.RequestGenerator

	connections chan *websocket.Conn
}

func NewProleChecker(prole *router.RequestGenerator) Checker {
	return &ProleChecker{
		prole: prole,

		connections: make(chan *websocket.Conn, 1),
	}
}

func (checker *ProleChecker) CheckResource(resource config.Resource, from builds.Version) []builds.Version {
	conn, err := checker.connect()
	if err != nil {
		return nil
	}

	buildInput := ProleBuilds.Input{
		Type:    resource.Type,
		Source:  ProleBuilds.Source(resource.Source),
		Version: ProleBuilds.Version(from),
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

func (checker *ProleChecker) connect() (*websocket.Conn, error) {
	select {
	case conn := <-checker.connections:
		return conn, nil
	default:
		req, err := checker.prole.RequestForHandler(
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

func (checker *ProleChecker) release(conn *websocket.Conn) {
	select {
	case checker.connections <- conn:
	default:
		conn.Close()
	}
}
