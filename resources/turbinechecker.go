package resources

import (
	"net"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	"github.com/concourse/turbine"
)

type TurbineChecker struct {
	turbine *rata.RequestGenerator

	dialer      *websocket.Dialer
	connections chan *websocket.Conn

	responses chan []builds.Version
}

func NewTurbineChecker(turbine *rata.RequestGenerator) Checker {
	return &TurbineChecker{
		turbine: turbine,

		connections: make(chan *websocket.Conn, 1),
		dialer: &websocket.Dialer{
			NetDial: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 5 * time.Second,
			}).Dial,
			HandshakeTimeout: time.Second,
		},
	}
}

func (checker *TurbineChecker) CheckResource(resource config.Resource, from builds.Version) ([]builds.Version, error) {
	conn, err := checker.connect()
	if err != nil {
		return nil, err
	}

	buildInput := turbine.Input{
		Type:    resource.Type,
		Source:  turbine.Source(resource.Source),
		Version: turbine.Version(from),
	}

	err = conn.WriteJSON(buildInput)
	if err != nil {
		return nil, err
	}

	var versions []builds.Version
	err = conn.ReadJSON(&versions)
	if err != nil {
		return nil, err
	}

	checker.release(conn)

	return versions, nil
}

func (checker *TurbineChecker) connect() (*websocket.Conn, error) {
	select {
	case conn := <-checker.connections:
		return conn, nil
	default:
		req, err := checker.turbine.CreateRequest(
			turbine.CheckInputStream,
			nil,
			nil,
		)
		if err != nil {
			return nil, err
		}

		if req.URL.Scheme == "https" {
			req.URL.Scheme = "wss"
		} else {
			req.URL.Scheme = "ws"
		}

		conn, _, err := checker.dialer.Dial(req.URL.String(), nil)
		if err != nil {
			return nil, err
		}

		return conn, nil
	}
}

func (checker *TurbineChecker) release(conn *websocket.Conn) {
	select {
	case checker.connections <- conn:
	default:
		conn.Close()
	}
}
