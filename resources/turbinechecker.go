package resources

import (
	"encoding/json"
	"time"

	TurbineBuilds "github.com/concourse/turbine/api/builds"
	"github.com/concourse/turbine/routes"
	"github.com/gorilla/websocket"
	"github.com/tedsuo/router"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
)

type TurbineChecker struct {
	turbine      *router.RequestGenerator
	pingInterval time.Duration

	dialer *websocket.Dialer

	connections chan *websocket.Conn
}

func NewTurbineChecker(turbine *router.RequestGenerator, pingInterval time.Duration) Checker {
	return &TurbineChecker{
		turbine:      turbine,
		pingInterval: pingInterval,

		connections: make(chan *websocket.Conn, 1),

		dialer: &websocket.Dialer{
			HandshakeTimeout: time.Second,
		},
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

	writer, err := conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return nil, err
	}

	err = json.NewEncoder(writer).Encode(buildInput)
	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	_, reader, err := conn.NextReader()
	if err != nil {
		return nil, err
	}

	var newVersions []builds.Version
	err = json.NewDecoder(reader).Decode(&newVersions)
	if err != nil {
		return nil, err
	}

	checker.release(conn)

	return newVersions, nil
}

func (checker *TurbineChecker) connect() (*websocket.Conn, error) {
	select {
	case conn := <-checker.connections:
		err := conn.SetReadDeadline(time.Now().Add(2 * checker.pingInterval))
		if err != nil {
			return nil, err
		}

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

		conn, _, err := checker.dialer.Dial(req.URL.String(), nil)
		if err != nil {
			return nil, err
		}

		conn.SetPongHandler(func(string) error {
			return conn.SetReadDeadline(time.Now().Add(2 * checker.pingInterval))
		})

		go func() {
			ticker := time.NewTicker(checker.pingInterval)

			for {
				<-ticker.C

				err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second))
				if err != nil {
					break
				}
			}
		}()

		err = conn.SetReadDeadline(time.Now().Add(2 * checker.pingInterval))
		if err != nil {
			conn.Close()
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
