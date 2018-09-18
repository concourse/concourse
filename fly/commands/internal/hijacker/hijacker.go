package hijacker

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/fly/pty"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/gorilla/websocket"
	"github.com/mgutz/ansi"
	"github.com/tedsuo/rata"
)

type ProcessIO struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

type Hijacker struct {
	tlsConfig        *tls.Config
	requestGenerator *rata.RequestGenerator
	token            *rc.TargetToken
	interval         time.Duration
}

func New(tlsConfig *tls.Config, requestGenerator *rata.RequestGenerator, token *rc.TargetToken) *Hijacker {
	return &Hijacker{
		tlsConfig:        tlsConfig,
		requestGenerator: requestGenerator,
		token:            token,
		interval:         10 * time.Second,
	}
}

func (h *Hijacker) SetHeartbeatInterval(interval time.Duration) {
	h.interval = interval
}

func (h *Hijacker) Hijack(teamName, handle string, spec atc.HijackProcessSpec, pio ProcessIO) (int, error) {
	url, header, err := h.hijackRequestParts(teamName, handle)
	if err != nil {
		return -1, err
	}

	dialer := websocket.Dialer{
		TLSClientConfig: h.tlsConfig,
		Proxy:           http.ProxyFromEnvironment,
	}
	conn, _, err := dialer.Dial(url, header)
	if err != nil {
		return -1, err
	}

	defer conn.Close()

	err = conn.WriteJSON(spec)
	if err != nil {
		return -1, err
	}

	inputs := make(chan atc.HijackInput, 1)
	finished := make(chan struct{}, 1)

	go h.monitorTTYSize(inputs, finished)
	go func() {
		io.Copy(&stdinWriter{inputs}, pio.In)
		inputs <- atc.HijackInput{Closed: true}
	}()
	go h.handleInput(conn, inputs, finished)

	exitStatus := h.handleOutput(conn, pio)

	close(finished)

	return exitStatus, nil
}

func (h *Hijacker) hijackRequestParts(teamName, handle string) (string, http.Header, error) {
	hijackReq, err := h.requestGenerator.CreateRequest(
		atc.HijackContainer,
		rata.Params{"id": handle, "team_name": teamName},
		nil,
	)

	if err != nil {
		panic(err)
	}

	if h.token != nil {
		hijackReq.Header.Add("Authorization", h.token.Type+" "+h.token.Value)
	}

	wsUrl := hijackReq.URL

	var found bool
	wsUrl.Scheme, found = websocketSchemeMap[wsUrl.Scheme]
	if !found {
		return "", nil, fmt.Errorf("unknown target scheme: %s", wsUrl.Scheme)
	}

	return wsUrl.String(), hijackReq.Header, nil
}

func (h *Hijacker) handleOutput(conn *websocket.Conn, pio ProcessIO) int {
	var exitStatus int
	for {
		var output atc.HijackOutput
		err := conn.ReadJSON(&output)
		if err != nil {
			if !websocket.IsCloseError(err) && !websocket.IsUnexpectedCloseError(err) {
				fmt.Println(err)
			}
			break
		}

		if output.ExitStatus != nil {
			exitStatus = *output.ExitStatus
		} else if len(output.Error) > 0 {
			fmt.Fprintf(ui.Stderr, "%s\n", ansi.Color(output.Error, "red+b"))
			exitStatus = 255
		} else if len(output.Stdout) > 0 {
			pio.Out.Write(output.Stdout)
		} else if len(output.Stderr) > 0 {
			pio.Err.Write(output.Stderr)
		}
	}

	return exitStatus
}

func (h *Hijacker) handleInput(conn *websocket.Conn, inputs <-chan atc.HijackInput, finished chan struct{}) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case input := <-inputs:
			err := conn.WriteJSON(input)
			if err != nil {
				fmt.Fprintf(ui.Stderr, "failed to send input: %s", err.Error())
				return
			}
		case t := <-ticker.C:
			err := conn.WriteControl(websocket.PingMessage, []byte(t.String()), time.Now().Add(time.Second))
			if err != nil {
				fmt.Fprintf(ui.Stderr, "failed to send heartbeat: %s", err.Error())
			}
		case <-finished:
			return
		}
	}
}

func (h *Hijacker) monitorTTYSize(inputs chan<- atc.HijackInput, finished chan struct{}) {
	resized := pty.ResizeNotifier()

	for {
		select {
		case <-resized:
			rows, cols, err := pty.Getsize(os.Stdin)
			if err == nil {
				inputs <- atc.HijackInput{
					TTYSpec: &atc.HijackTTYSpec{
						WindowSize: atc.HijackWindowSize{
							Columns: cols,
							Rows:    rows,
						},
					},
				}
			}
		case <-finished:
			return
		}
	}
}

type stdinWriter struct {
	inputs chan<- atc.HijackInput
}

func (w *stdinWriter) Write(d []byte) (int, error) {
	w.inputs <- atc.HijackInput{
		Stdin: d,
	}

	return len(d), nil
}

var websocketSchemeMap = map[string]string{
	"http":  "ws",
	"https": "wss",
}
