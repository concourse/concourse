package engine

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
	"time"

	garden "github.com/cloudfoundry-incubator/garden/api"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/concourse/turbine"
	tevent "github.com/concourse/turbine/event"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"
	"github.com/vito/go-sse/sse"
)

//go:generate counterfeiter . EngineDB
type EngineDB interface {
	SaveBuildEvent(buildID int, event atc.Event) error
	CompleteBuild(buildID int) error

	SaveBuildEngineMetadata(buildID int, metadata string) error

	SaveBuildStartTime(buildID int, startTime time.Time) error
	SaveBuildEndTime(buildID int, startTime time.Time) error

	SaveBuildInput(buildID int, input db.BuildInput) error
	SaveBuildOutput(buildID int, vr db.VersionedResource) error

	SaveBuildStatus(buildID int, status db.Status) error
}

var ErrBadResponse = errors.New("bad response from turbine")

type TurbineMetadata struct {
	Guid        string `json:"guid"`
	Endpoint    string `json:"endpoint"`
	LastEventID *uint  `json:"last_event_id"`
}

func (metadata TurbineMetadata) Validate() error {
	if metadata.Guid == "" {
		return fmt.Errorf("missing guid")
	}

	if metadata.Endpoint == "" {
		return fmt.Errorf("missing endpoint")
	}

	return nil
}

type turbineEngine struct {
	turbineEndpoint *rata.RequestGenerator
	httpClient      *http.Client
	db              EngineDB
}

func NewTurbineEngine(turbineEndpoint *rata.RequestGenerator, db EngineDB) Engine {
	return &turbineEngine{
		turbineEndpoint: turbineEndpoint,
		db:              db,

		httpClient: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 5 * time.Minute,

				// allow DNS to resolve differently every time
				DisableKeepAlives: true,
			},
		},
	}
}

func (engine *turbineEngine) Name() string {
	return "turbine"
}

func (engine *turbineEngine) CreateBuild(build db.Build, plan atc.BuildPlan) (Build, error) {
	req := new(bytes.Buffer)

	err := json.NewEncoder(req).Encode(engine.convertBuildPlan(plan))
	if err != nil {
		return nil, err
	}

	execute, err := engine.turbineEndpoint.CreateRequest(
		turbine.ExecuteBuild,
		nil,
		req,
	)
	if err != nil {
		return nil, err
	}

	execute.Header.Set("Content-Type", "application/json")

	resp, err := engine.httpClient.Do(execute)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, ErrBadResponse
	}

	var startedBuild turbine.Build
	err = json.NewDecoder(resp.Body).Decode(&startedBuild)
	if err != nil {
		return nil, err
	}

	resp.Body.Close()

	metadata := TurbineMetadata{
		Guid:     startedBuild.Guid,
		Endpoint: resp.Header.Get("X-Turbine-Endpoint"),
	}

	return &turbineBuild{
		guid: metadata.Guid,
		id:   build.ID,

		metadata: metadata,

		db: engine.db,

		httpClient:      engine.httpClient,
		turbineEndpoint: rata.NewRequestGenerator(metadata.Endpoint, turbine.Routes),
	}, nil
}

func (engine *turbineEngine) LookupBuild(build db.Build) (Build, error) {
	var metadata TurbineMetadata
	err := json.Unmarshal([]byte(build.EngineMetadata), &metadata)
	if err != nil {
		return nil, err
	}

	err = metadata.Validate()
	if err != nil {
		return nil, err
	}

	return &turbineBuild{
		guid: metadata.Guid,
		id:   build.ID,

		metadata: metadata,

		db: engine.db,

		httpClient:      engine.httpClient,
		turbineEndpoint: rata.NewRequestGenerator(metadata.Endpoint, turbine.Routes),
	}, nil
}

func (engine *turbineEngine) convertBuildPlan(plan atc.BuildPlan) turbine.Build {
	return turbine.Build{
		Privileged: plan.Privileged,

		Config: engine.convertBuildConfig(plan.Config),

		Inputs:  engine.convertInputs(plan.Inputs, plan.ConfigPath),
		Outputs: engine.convertOutputs(plan.Outputs),
	}
}

func (engine *turbineEngine) convertBuildConfig(config *atc.BuildConfig) turbine.Config {
	if config == nil {
		return turbine.Config{}
	}

	inputs := make([]turbine.InputConfig, len(config.Inputs))
	for i, input := range config.Inputs {
		inputs[i] = turbine.InputConfig{
			Name: input.Name,
			Path: input.Path,
		}
	}

	return turbine.Config{
		Image:  config.Image,
		Params: config.Params,

		Inputs: inputs,

		Run: turbine.RunConfig{
			Path: config.Run.Path,
			Args: config.Run.Args,
		},
	}
}

func (engine *turbineEngine) convertInputs(inputs []atc.InputPlan, configPath string) []turbine.Input {
	tinputs := make([]turbine.Input, len(inputs))
	for i, input := range inputs {
		var config string
		if strings.HasPrefix(configPath, input.Name+"/") {
			config = configPath[len(input.Name)+1:]
		}

		tinputs[i] = turbine.Input{
			Name:       input.Name,
			Resource:   input.Resource,
			Type:       input.Type,
			Source:     turbine.Source(input.Source),
			Params:     turbine.Params(input.Params),
			Version:    turbine.Version(input.Version),
			ConfigPath: config,
		}
	}

	return tinputs
}

func (engine *turbineEngine) convertOutputs(outputs []atc.OutputPlan) []turbine.Output {
	toutputs := make([]turbine.Output, len(outputs))
	for i, output := range outputs {
		toutputs[i] = turbine.Output{
			Name:   output.Name,
			Type:   output.Type,
			On:     engine.convertOutputConditions(output.On),
			Source: turbine.Source(output.Source),
			Params: turbine.Params(output.Params),
		}
	}

	return toutputs
}

func (engine *turbineEngine) convertOutputConditions(conds atc.OutputConditions) turbine.OutputConditions {
	tconds := make(turbine.OutputConditions, len(conds))
	for i, cond := range conds {
		tconds[i] = turbine.OutputCondition(cond)
	}

	return tconds
}

type turbineBuild struct {
	guid string
	id   int

	metadata  TurbineMetadata
	metadataL sync.Mutex

	db EngineDB

	turbineEndpoint *rata.RequestGenerator
	httpClient      *http.Client
}

func (build *turbineBuild) Metadata() string {
	build.metadataL.Lock()
	defer build.metadataL.Unlock()

	payload, err := json.Marshal(build.metadata)
	if err != nil {
		panic("failed to marshal turbine metadata: " + err.Error())
	}

	return string(payload)
}

func (build *turbineBuild) Abort() error {
	abort, err := build.turbineEndpoint.CreateRequest(
		turbine.AbortBuild,
		rata.Params{"guid": build.guid},
		nil,
	)
	if err != nil {
		return err
	}

	resp, err := build.httpClient.Do(abort)
	if err != nil {
		return err
	}

	resp.Body.Close()

	if resp.StatusCode > 300 {
		return fmt.Errorf("bad response: %s", resp.Status)
	}

	return nil
}

func (build *turbineBuild) Hijack(spec garden.ProcessSpec, processIO garden.ProcessIO) (garden.Process, error) {
	specPayload, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}

	hijackReq, err := build.turbineEndpoint.CreateRequest(
		turbine.HijackBuild,
		rata.Params{"guid": build.guid},
		bytes.NewBuffer(specPayload),
	)
	if err != nil {
		return nil, err
	}

	hijackReq.Header.Set("Content-Type", "application/json")

	hijackURL := hijackReq.URL

	conn, err := net.Dial("tcp", hijackURL.Host)
	if err != nil {
		return nil, err
	}

	client := httputil.NewClientConn(conn, nil)

	resp, err := client.Do(hijackReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, ErrBadResponse
	}

	conn, br := client.Hijack()

	return newTurbineProcess(conn, br, processIO), nil
}

func (build *turbineBuild) Resume(logger lager.Logger) error {
	events, err := build.subscribe(build.metadata.LastEventID)
	if err != nil {
		if err == ErrBadResponse {
			logger.Info("saving-orphaned-build-as-errored")

			err := build.db.SaveBuildStatus(build.id, db.StatusErrored)
			if err != nil {
				logger.Error("failed-to-save-orphaned-build-as-errored", err)
				return err
			}

			return nil
		}

		return err
	}

	defer events.Close()

	outputs := map[string]db.VersionedResource{}

	for {
		e, id, err := events.Next()
		if err != nil {
			if err == errEndOfStream {
				logger.Info("event-stream-completed")

				del, err := build.turbineEndpoint.CreateRequest(
					turbine.DeleteBuild,
					rata.Params{"guid": build.guid},
					nil,
				)
				if err != nil {
					logger.Error("failed-to-create-delete-request", err)
					return err
				}

				resp, err := http.DefaultClient.Do(del)
				if err != nil {
					logger.Error("failed-to-delete-build", err)
					return err
				}

				resp.Body.Close()

				return build.db.CompleteBuild(build.id)
			}

			return err
		}

		evLog := logger.Session("event", lager.Data{"event": e})

		err = build.db.SaveBuildEvent(build.id, e)
		if err != nil {
			evLog.Error("failed-to-save-build-event", err)
			return err
		}

		err = build.updateLastEventID(id)
		if err != nil {
			evLog.Error("failed-to-update-metadata", err)
			return err
		}

		switch ev := e.(type) {
		case event.Status:
			evLog.Info("processing-build-status")

			if ev.Status == atc.StatusStarted {
				err = build.db.SaveBuildStartTime(build.id, time.Unix(ev.Time, 0))
				if err != nil {
					evLog.Error("failed-to-save-build-start-time", err)
					return err
				}
			} else {
				err = build.db.SaveBuildEndTime(build.id, time.Unix(ev.Time, 0))
				if err != nil {
					evLog.Error("failed-to-save-build-end-time", err)
					return err
				}
			}

			err = build.db.SaveBuildStatus(build.id, db.Status(ev.Status))
			if err != nil {
				evLog.Error("failed-to-save-build-status", err)
				return err
			}

			if ev.Status == atc.StatusSucceeded {
				for _, output := range outputs {
					err := build.db.SaveBuildOutput(build.id, output)
					if err != nil {
						evLog.Error("failed-to-save-build-output", err)
						return err
					}
				}
			}

		case event.Input:
			evLog.Info("processing-build-input")

			if ev.Plan.Resource == "" {
				break
			}

			vr := vrFromInput(ev)

			err = build.db.SaveBuildInput(build.id, db.BuildInput{
				Name:              ev.Plan.Name,
				VersionedResource: vr,
			})
			if err != nil {
				evLog.Error("failed-to-save-build-input", err)
				return err
			}

			// record implicit output
			outputs[ev.Plan.Resource] = vr

		case event.Output:
			evLog.Info("processing-build-output")
			outputs[ev.Plan.Name] = vrFromOutput(ev)
		}
	}

	return nil
}

func (build *turbineBuild) subscribe(lastID *uint) (*turbineEventSource, error) {
	getEvents, err := build.turbineEndpoint.CreateRequest(
		turbine.GetBuildEvents,
		rata.Params{"guid": build.guid},
		nil,
	)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(getEvents)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrBadResponse
	}

	return &turbineEventSource{
		reader: sse.NewReader(resp.Body),
		lastID: lastID,
		closer: resp.Body,
	}, nil
}

func (build *turbineBuild) updateLastEventID(newID uint) error {
	build.metadataL.Lock()
	build.metadata.LastEventID = &newID
	build.metadataL.Unlock()

	return build.db.SaveBuildEngineMetadata(build.id, build.Metadata())
}

func vrFromInput(input event.Input) db.VersionedResource {
	metadata := make([]db.MetadataField, len(input.FetchedMetadata))
	for i, md := range input.FetchedMetadata {
		metadata[i] = db.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	return db.VersionedResource{
		Resource: input.Plan.Resource,
		Type:     input.Plan.Type,
		Source:   db.Source(input.Plan.Source),
		Version:  db.Version(input.FetchedVersion),
		Metadata: metadata,
	}
}

func vrFromOutput(output event.Output) db.VersionedResource {
	metadata := make([]db.MetadataField, len(output.CreatedMetadata))
	for i, md := range output.CreatedMetadata {
		metadata[i] = db.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	return db.VersionedResource{
		Resource: output.Plan.Name,
		Type:     output.Plan.Type,
		Source:   db.Source(output.Plan.Source),
		Version:  db.Version(output.CreatedVersion),
		Metadata: metadata,
	}
}

var errEndOfStream = errors.New("end of stream")
var errReadClosedStream = errors.New("read of closed stream")
var errCloseClosedStream = errors.New("close of closed stream")

type turbineEventSource struct {
	reader *sse.Reader
	lastID *uint

	currentVersion string

	closer io.Closer
	closed bool
}

func (source *turbineEventSource) Next() (atc.Event, uint, error) {
	if source.closed {
		return nil, 0, errReadClosedStream
	}

	for {
		se, err := source.reader.Next()
		if err != nil {
			return nil, 0, err
		}

		var id uint
		_, err = fmt.Sscanf(se.ID, "%d", &id)
		if err != nil {
			return nil, 0, err
		}

		if se.Name == "version" {
			var version atc.EventVersion
			err := json.Unmarshal(se.Data, &version)
			if err != nil {
				return nil, 0, err
			}

			source.currentVersion = string(version)

			continue
		}

		if source.lastID != nil && id <= *source.lastID {
			// skip old IDs.
			//
			// can't use Last-Event-ID because we must see the version to know how to
			// process the later events.
			continue
		}

		switch source.currentVersion {
		case "1.0":
			fallthrough
		case "1.1":
			tev, err := tevent.ParseEvent(tevent.EventType(se.Name), se.Data)
			if err != nil {
				return nil, 0, err
			}

			switch tev.(type) {
			case tevent.End:
				return nil, 0, errEndOfStream
			default:
				return source.convertEvent(tev), id, nil
			}
		}
	}

	panic("unreachable")
}

func (source *turbineEventSource) Close() error {
	if source.closed {
		return errCloseClosedStream
	}

	source.closed = true
	return source.closer.Close()
}

func (source *turbineEventSource) convertEvent(tev tevent.Event) atc.Event {
	switch e := tev.(type) {
	case tevent.Error:
		return event.Error{
			Message: e.Message,
			Origin:  source.convertOrigin(e.Origin),
		}
	case tevent.Finish:
		return event.Finish(e)
	case tevent.Initialize:
		return event.Initialize{
			BuildConfig: atc.BuildConfig{
				Image:  e.BuildConfig.Image,
				Inputs: source.convertBuildInputConfigs(e.BuildConfig.Inputs),
				Params: e.BuildConfig.Params,
				Run:    atc.BuildRunConfig(e.BuildConfig.Run),
			},
		}
	case tevent.Input:
		return event.Input{
			Plan: atc.InputPlan{
				Name:     e.Input.Name,
				Resource: e.Input.Resource,
				Type:     e.Input.Type,
				Source:   atc.Source(e.Input.Source),
				Version:  atc.Version(e.Input.Version),
				Params:   atc.Params(e.Input.Params),
			},
			FetchedVersion:  atc.Version(e.Input.Version),
			FetchedMetadata: source.convertMetadata(e.Input.Metadata),
		}
	case tevent.Log:
		return event.Log{
			Payload: e.Payload,
			Origin:  source.convertOrigin(e.Origin),
		}
	case tevent.Output:
		return event.Output{
			Plan: atc.OutputPlan{
				Name:   e.Output.Name,
				Type:   e.Output.Type,
				On:     source.convertOutputConditions(e.Output.On),
				Source: atc.Source(e.Output.Source),
				Params: atc.Params(e.Output.Params),
			},
			CreatedVersion:  atc.Version(e.Output.Version),
			CreatedMetadata: source.convertMetadata(e.Output.Metadata),
		}
	case tevent.Start:
		return event.Start(e)
	case tevent.Status:
		return event.Status{
			Status: atc.BuildStatus(e.Status),
			Time:   e.Time,
		}
	default:
		panic("unknown type: " + tev.EventType())
	}
}

func (source *turbineEventSource) convertMetadata(tm []turbine.MetadataField) []atc.MetadataField {
	meta := make([]atc.MetadataField, len(tm))
	for i, m := range tm {
		meta[i] = atc.MetadataField{
			Name:  m.Name,
			Value: m.Value,
		}
	}

	return meta
}

func (source *turbineEventSource) convertOutputConditions(tcs turbine.OutputConditions) atc.OutputConditions {
	cs := make(atc.OutputConditions, len(tcs))
	for i, c := range tcs {
		cs[i] = atc.OutputCondition(c)
	}

	return cs
}

func (source *turbineEventSource) convertOrigin(to tevent.Origin) event.Origin {
	return event.Origin{
		Type: event.OriginType(to.Type),
		Name: to.Name,
	}
}

func (source *turbineEventSource) convertBuildInputConfigs(tbics []turbine.InputConfig) []atc.BuildInputConfig {
	inputs := make([]atc.BuildInputConfig, len(tbics))
	for i, tbic := range tbics {
		inputs[i] = atc.BuildInputConfig(tbic)
	}

	return inputs
}

func newTurbineProcess(conn net.Conn, br *bufio.Reader, processIO garden.ProcessIO) garden.Process {
	process := &turbineProcess{
		conn:   gob.NewEncoder(conn),
		closer: conn,
		br:     br,
		wg:     new(sync.WaitGroup),
	}

	process.trackIO(processIO)

	return process
}

type turbineProcess struct {
	conn   *gob.Encoder
	closer io.Closer
	br     *bufio.Reader
	wg     *sync.WaitGroup
}

func (process *turbineProcess) ID() uint32 {
	return 0
}

func (process *turbineProcess) SetTTY(spec garden.TTYSpec) error {
	return process.conn.Encode(turbine.HijackPayload{
		TTYSpec: &spec,
	})
}

func (process *turbineProcess) Wait() (int, error) {
	process.wg.Wait()
	process.closer.Close()
	return 0, nil
}

func (process *turbineProcess) Write(b []byte) (int, error) {
	err := process.conn.Encode(turbine.HijackPayload{
		Stdin: b,
	})
	if err != nil {
		return 0, err
	}

	return len(b), nil
}

func (process *turbineProcess) trackIO(processIO garden.ProcessIO) {
	process.wg.Add(1)

	go func() {
		defer process.wg.Done()
		io.Copy(processIO.Stdout, process.br)
	}()

	go io.Copy(process, processIO.Stdin)
}
