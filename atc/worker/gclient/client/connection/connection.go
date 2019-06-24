package connection

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/routes"
	"code.cloudfoundry.org/garden/transport"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/rata"
)

var ErrDisconnected = errors.New("disconnected")
var ErrInvalidMessage = errors.New("invalid message payload")

//go:generate counterfeiter . Connection
type Connection interface {
	Ping() error

	Capacity() (garden.Capacity, error)

	Create(ctx context.Context, spec garden.ContainerSpec) (string, error)
	List(properties garden.Properties) ([]string, error)

	// Destroys the container with the given handle. If the container cannot be
	// found, garden.ContainerNotFoundError is returned. If deletion fails for another
	// reason, another error type is returned.
	Destroy(ctx context.Context,handle string) error

	Stop(ctx context.Context, handle string, kill bool) error

	Info(handle string) (garden.ContainerInfo, error)
	BulkInfo(handles []string) (map[string]garden.ContainerInfoEntry, error)
	BulkMetrics(handles []string) (map[string]garden.ContainerMetricsEntry, error)

	StreamIn(ctx context.Context, handle string, spec garden.StreamInSpec) error
	StreamOut(ctx context.Context, handle string, spec garden.StreamOutSpec) (io.ReadCloser, error)

	CurrentBandwidthLimits(handle string) (garden.BandwidthLimits, error)
	CurrentCPULimits(handle string) (garden.CPULimits, error)
	CurrentDiskLimits(handle string) (garden.DiskLimits, error)
	CurrentMemoryLimits(handle string) (garden.MemoryLimits, error)

	Run(ctx context.Context, handle string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error)
	Attach(ctx context.Context, handle string, processID string, io garden.ProcessIO) (garden.Process, error)

	NetIn(handle string, hostPort, containerPort uint32) (uint32, uint32, error)
	NetOut(handle string, rule garden.NetOutRule) error
	BulkNetOut(handle string, rules []garden.NetOutRule) error

	SetGraceTime(handle string, graceTime time.Duration) error

	Properties(handle string) (garden.Properties, error)
	Property(handle string, name string) (string, error)
	SetProperty(handle string, name string, value string) error

	Metrics(handle string) (garden.Metrics, error)
	RemoveProperty(handle string, name string) error
}

//go:generate counterfeiter . HijackStreamer
type HijackStreamer interface {
	Stream(ctx context.Context, handler string, body io.Reader, params rata.Params, query url.Values, contentType string) (io.ReadCloser, error)
	Hijack(ctx context.Context, handler string, body io.Reader, params rata.Params, query url.Values, contentType string) (net.Conn, *bufio.Reader, error)
}

type connection struct {
	hijacker HijackStreamer
	log      lager.Logger
}

type Error struct {
	StatusCode int
	Message    string
}

func (err Error) Error() string {
	return err.Message
}

func NewWithHijacker(hijacker HijackStreamer, log lager.Logger) Connection {
	return &connection{
		hijacker: hijacker,
		log:      log,
	}
}

func (c *connection) Ping() error {
	return c.do(context.Background(), routes.Ping, nil, &struct{}{}, nil, nil)
}

func (c *connection) Capacity() (garden.Capacity, error) {
	capacity := garden.Capacity{}
	err := c.do(context.Background(), routes.Capacity, nil, &capacity, nil, nil)
	if err != nil {
		return garden.Capacity{}, err
	}

	return capacity, nil
}

func (c *connection) Create(ctx context.Context, spec garden.ContainerSpec) (string, error) {
	res := struct {
		Handle string `json:"handle"`
	}{}

	err := c.do(ctx, routes.Create, spec, &res, nil, nil)
	if err != nil {
		return "", err
	}

	return res.Handle, nil
}

func (c *connection) Stop(ctx context.Context, handle string, kill bool) error {
	return c.do(
		ctx,
		routes.Stop,
		map[string]bool{
			"kill": kill,
		},
		&struct{}{},
		rata.Params{
			"handle": handle,
		},
		nil,
	)
}

func (c *connection) Destroy(ctx context.Context, handle string) error {
	return c.do(
		ctx,
		routes.Destroy,
		nil,
		&struct{}{},
		rata.Params{
			"handle": handle,
		},
		nil,
	)
}

func (c *connection) Run(ctx context.Context, handle string, spec garden.ProcessSpec, processIO garden.ProcessIO) (garden.Process, error) {
	reqBody := new(bytes.Buffer)

	err := transport.WriteMessage(reqBody, spec)
	if err != nil {
		return nil, err
	}

	hijackedConn, hijackedResponseReader, err := c.hijacker.Hijack(
		ctx,
		routes.Run,
		reqBody,
		rata.Params{
			"handle": handle,
		},
		nil,
		"application/json",
	)
	if err != nil {
		return nil, err
	}

	return c.streamProcess(ctx, handle, processIO, hijackedConn, hijackedResponseReader)
}

func (c *connection) Attach(ctx context.Context, handle string, processID string, processIO garden.ProcessIO) (garden.Process, error) {
	reqBody := new(bytes.Buffer)

	hijackedConn, hijackedResponseReader, err := c.hijacker.Hijack(
		ctx,
		routes.Attach,
		reqBody,
		rata.Params{
			"handle": handle,
			"pid":    processID,
		},
		nil,
		"",
	)
	if err != nil {
		return nil, err
	}

	return c.streamProcess(ctx, handle, processIO, hijackedConn, hijackedResponseReader)
}

func (c *connection) streamProcess(ctx context.Context, handle string, processIO garden.ProcessIO, hijackedConn net.Conn, hijackedResponseReader *bufio.Reader) (garden.Process, error) {
	decoder := json.NewDecoder(hijackedResponseReader)

	payload := &transport.ProcessPayload{}
	if err := decoder.Decode(payload); err != nil {
		return nil, err
	}

	processPipeline := &processStream{
		processID: payload.ProcessID,
		conn:      hijackedConn,
	}

	hijack := func(streamType string) (net.Conn, io.Reader, error) {
		params := rata.Params{
			"handle":   handle,
			"pid":      processPipeline.ProcessID(),
			"streamid": payload.StreamID,
		}

		return c.hijacker.Hijack(
			ctx,
			streamType,
			nil,
			params,
			nil,
			"application/json",
		)
	}

	process := newProcess(payload.ProcessID, processPipeline)
	streamHandler := newStreamHandler(c.log)
	streamHandler.streamIn(processPipeline, processIO.Stdin)

	var stdoutConn net.Conn
	if processIO.Stdout != nil {
		var (
			stdout io.Reader
			err    error
		)
		stdoutConn, stdout, err = hijack(routes.Stdout)
		if err != nil {
			werr := fmt.Errorf("connection: failed to hijack stream %s: %s", routes.Stdout, err)
			process.exited(0, werr)
			hijackedConn.Close()
			return process, nil
		}
		streamHandler.streamOut(processIO.Stdout, stdout)
	}

	var stderrConn net.Conn
	if processIO.Stderr != nil {
		var (
			stderr io.Reader
			err    error
		)
		stderrConn, stderr, err = hijack(routes.Stderr)
		if err != nil {
			werr := fmt.Errorf("connection: failed to hijack stream %s: %s", routes.Stderr, err)
			process.exited(0, werr)
			hijackedConn.Close()
			return process, nil
		}
		streamHandler.streamOut(processIO.Stderr, stderr)
	}

	go func() {
		defer hijackedConn.Close()
		if stdoutConn != nil {
			defer stdoutConn.Close()
		}
		if stderrConn != nil {
			defer stderrConn.Close()
		}

		exitCode, err := streamHandler.wait(decoder)
		process.exited(exitCode, err)
	}()

	return process, nil
}

func (c *connection) NetIn(handle string, hostPort, containerPort uint32) (uint32, uint32, error) {
	res := &transport.NetInResponse{}

	err := c.do(
		context.Background(),
		routes.NetIn,
		&transport.NetInRequest{
			Handle:        handle,
			HostPort:      hostPort,
			ContainerPort: containerPort,
		},
		res,
		rata.Params{
			"handle": handle,
		},
		nil,
	)

	if err != nil {
		return 0, 0, err
	}

	return res.HostPort, res.ContainerPort, nil
}

func (c *connection) BulkNetOut(handle string, rules []garden.NetOutRule) error {
	return c.do(
		context.Background(),
		routes.BulkNetOut,
		rules,
		&struct{}{},
		rata.Params{
			"handle": handle,
		},
		nil,
	)
}

func (c *connection) NetOut(handle string, rule garden.NetOutRule) error {
	return c.do(
		context.Background(),
		routes.NetOut,
		rule,
		&struct{}{},
		rata.Params{
			"handle": handle,
		},
		nil,
	)
}

func (c *connection) Property(handle string, name string) (string, error) {
	var res struct {
		Value string `json:"value"`
	}

	err := c.do(
		context.Background(),
		routes.Property,
		nil,
		&res,
		rata.Params{
			"handle": handle,
			"key":    name,
		},
		nil,
	)

	return res.Value, err
}

func (c *connection) SetProperty(handle string, name string, value string) error {
	err := c.do(
		context.Background(),
		routes.SetProperty,
		map[string]string{
			"value": value,
		},
		&struct{}{},
		rata.Params{
			"handle": handle,
			"key":    name,
		},
		nil,
	)

	if err != nil {
		return err
	}

	return nil
}

func (c *connection) RemoveProperty(handle string, name string) error {
	err := c.do(
		context.Background(),
		routes.RemoveProperty,
		nil,
		&struct{}{},
		rata.Params{
			"handle": handle,
			"key":    name,
		},
		nil,
	)

	if err != nil {
		return err
	}

	return nil
}

func (c *connection) CurrentBandwidthLimits(handle string) (garden.BandwidthLimits, error) {
	res := garden.BandwidthLimits{}

	err := c.do(
		context.Background(),
		routes.CurrentBandwidthLimits,
		nil,
		&res,
		rata.Params{
			"handle": handle,
		},
		nil,
	)

	return res, err
}

func (c *connection) CurrentCPULimits(handle string) (garden.CPULimits, error) {
	res := garden.CPULimits{}

	err := c.do(
		context.Background(),
		routes.CurrentCPULimits,
		nil,
		&res,
		rata.Params{
			"handle": handle,
		},
		nil,
	)

	return res, err
}

func (c *connection) CurrentDiskLimits(handle string) (garden.DiskLimits, error) {
	res := garden.DiskLimits{}

	err := c.do(
		context.Background(),
		routes.CurrentDiskLimits,
		nil,
		&res,
		rata.Params{
			"handle": handle,
		},
		nil,
	)

	return res, err
}

func (c *connection) CurrentMemoryLimits(handle string) (garden.MemoryLimits, error) {
	res := garden.MemoryLimits{}

	err := c.do(
		context.Background(),
		routes.CurrentMemoryLimits,
		nil,
		&res,
		rata.Params{
			"handle": handle,
		},
		nil,
	)

	return res, err
}

func (c *connection) StreamIn(ctx context.Context, handle string, spec garden.StreamInSpec) error {
	body, err := c.hijacker.Stream(
		ctx,
		routes.StreamIn,
		spec.TarStream,
		rata.Params{
			"handle": handle,
		},
		url.Values{
			"user":        []string{spec.User},
			"destination": []string{spec.Path},
		},
		"application/x-tar",
	)
	if err != nil {
		return err
	}

	return body.Close()
}

func (c *connection) StreamOut(ctx context.Context, handle string, spec garden.StreamOutSpec) (io.ReadCloser, error) {
	return c.hijacker.Stream(
		ctx,
		routes.StreamOut,
		nil,
		rata.Params{
			"handle": handle,
		},
		url.Values{
			"user":   []string{spec.User},
			"source": []string{spec.Path},
		},
		"",
	)
}

func (c *connection) List(filterProperties garden.Properties) ([]string, error) {
	values := url.Values{}
	for name, val := range filterProperties {
		values[name] = []string{val}
	}

	res := &struct {
		Handles []string
	}{}

	if err := c.do(
		context.Background(),
		routes.List,
		nil,
		&res,
		nil,
		values,
	); err != nil {
		return nil, err
	}

	return res.Handles, nil
}

func (c *connection) SetGraceTime(handle string, graceTime time.Duration) error {
	return c.do(context.Background(),routes.SetGraceTime, graceTime, &struct{}{}, rata.Params{"handle": handle}, nil)
}

func (c *connection) Properties(handle string) (garden.Properties, error) {
	res := make(garden.Properties)
	err := c.do(context.Background(),routes.Properties, nil, &res, rata.Params{"handle": handle}, nil)
	return res, err
}

func (c *connection) Metrics(handle string) (garden.Metrics, error) {
	res := garden.Metrics{}
	err := c.do(context.Background(),routes.Metrics, nil, &res, rata.Params{"handle": handle}, nil)
	return res, err
}

func (c *connection) Info(handle string) (garden.ContainerInfo, error) {
	res := garden.ContainerInfo{}

	err := c.do(context.Background(),routes.Info, nil, &res, rata.Params{"handle": handle}, nil)
	if err != nil {
		return garden.ContainerInfo{}, err
	}

	return res, nil
}

func (c *connection) BulkInfo(handles []string) (map[string]garden.ContainerInfoEntry, error) {
	res := make(map[string]garden.ContainerInfoEntry)
	queryParams := url.Values{
		"handles": []string{strings.Join(handles, ",")},
	}
	err := c.do(context.Background(),routes.BulkInfo, nil, &res, nil, queryParams)
	return res, err
}

func (c *connection) BulkMetrics(handles []string) (map[string]garden.ContainerMetricsEntry, error) {
	res := make(map[string]garden.ContainerMetricsEntry)
	queryParams := url.Values{
		"handles": []string{strings.Join(handles, ",")},
	}
	err := c.do(context.Background(),routes.BulkMetrics, nil, &res, nil, queryParams)
	return res, err
}

func (c *connection) do(
	ctx context.Context,
	handler string,
	req, res interface{},
	params rata.Params,
	query url.Values,
) error {
	var body io.Reader

	if req != nil {
		buf := new(bytes.Buffer)

		err := transport.WriteMessage(buf, req)
		if err != nil {
			return err
		}

		body = buf
	}

	contentType := ""
	if req != nil {
		contentType = "application/json"
	}

	response, err := c.hijacker.Stream(
		ctx,
		handler,
		body,
		params,
		query,
		contentType,
	)
	if err != nil {
		return err
	}

	defer response.Close()

	return json.NewDecoder(response).Decode(res)
}
