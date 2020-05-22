package elasticsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/event"
	"github.com/olivere/elastic/v7"
)

type eventDoc struct {
	BuildID      int              `json:"build_id"`
	BuildName    string           `json:"build_name"`
	JobID        int              `json:"job_id"`
	JobName      string           `json:"job_name"`
	PipelineID   int              `json:"pipeline_id"`
	PipelineName string           `json:"pipeline_name"`
	TeamID       int              `json:"team_id"`
	TeamName     string           `json:"team_name"`
	EventType    atc.EventType    `json:"event"`
	Version      atc.EventVersion `json:"version"`
	Data         *json.RawMessage `json:"data"`
	Tiebreak     int64            `json:"tiebreak"`
}

type Key struct {
	TimeMillis int64
	Tiebreak   int64
}

type EventStore struct {
	logger lager.Logger
	client *elastic.Client

	url string

	counter int64
}

func NewEventStore(logger lager.Logger, url string) *EventStore {
	rand.Seed(time.Now().UnixNano())

	return &EventStore{
		url:    url,
		logger: logger,

		counter: rand.Int63(),
	}
}

func (e *EventStore) Setup(ctx context.Context) error {
	e.logger.Debug("setup-event-store", lager.Data{"url": e.url})
	var err error
	e.client, err = elastic.NewClient(
		elastic.SetURL(e.url),
	)
	if err != nil {
		e.logger.Error("connect-to-cluster-failed", err, lager.Data{"url": e.url})
		return fmt.Errorf("connect to cluster: %w", err)
	}

	_, err = e.client.XPackIlmPutLifecycle().
		Policy(ilmPolicyName).
		BodyString(ilmPolicyJSON).
		Do(ctx)
	if err != nil {
		e.logger.Error("put-ilm-policy-failed", err, lager.Data{"name": ilmPolicyName, "json": ilmPolicyJSON})
		return fmt.Errorf("put ilm policy: %w", err)
	}

	_, err = e.client.IndexPutTemplate(indexTemplateName).
		BodyString(indexTemplateJSON).
		Do(ctx)
	if err != nil {
		e.logger.Error("put-index-template-failed", err, lager.Data{"name": indexTemplateName, "json": indexTemplateJSON})
		return fmt.Errorf("put index template: %w", err)
	}

	err = e.createIndexIfNotExists(ctx, initialIndexName, initialIndexJSON)
	if err != nil {
		e.logger.Error("create-initial-index-failed", err, lager.Data{"name": initialIndexName, "json": initialIndexJSON})
		return fmt.Errorf("create initial index: %w", err)
	}

	return nil
}

func (e *EventStore) createIndexIfNotExists(ctx context.Context, name string, body string) error {
	exists, err := e.client.IndexExists(name).Do(ctx)
	if err != nil {
		e.logger.Error("check-index-exists-failed", err, lager.Data{"name": name})
		return fmt.Errorf("check index exists: %w", err)
	}
	if exists {
		return nil
	}
	_, err = e.client.CreateIndex(name).Body(body).Do(ctx)
	if err != nil && !isAlreadyExists(err) {
		e.logger.Error("create-index-failed", err, lager.Data{"name": name})
		if isAlreadyExists(err) {
			return nil
		}
		return err
	}
	return nil
}

func isAlreadyExists(err error) bool {
	elasticErr, ok := err.(*elastic.Error)
	if !ok {
		return false
	}
	return elasticErr.Status == http.StatusBadRequest && elasticErr.Details.Type == "index_already_exists_exception"
}

func (e *EventStore) Initialize(ctx context.Context, build db.Build) error {
	return nil
}

func (e *EventStore) Finalize(ctx context.Context, build db.Build) error {
	return nil
}

func (e *EventStore) Put(ctx context.Context, build db.Build, events []atc.Event) error {
	bulkRequest := e.client.Bulk()
	for _, evt := range events {
		payload, err := json.Marshal(evt)
		if err != nil {
			e.logger.Error("marshal-event-failed", err)
			return fmt.Errorf("marshal event: %w", err)
		}
		data := json.RawMessage(payload)
		doc := eventDoc{
			BuildID:      build.ID(),
			PipelineID:   build.PipelineID(),
			PipelineName: build.PipelineName(),
			TeamID:       build.TeamID(),
			TeamName:     build.TeamName(),
			EventType:    evt.EventType(),
			Version:      evt.Version(),
			Data:         &data,
			Tiebreak:     atomic.AddInt64(&e.counter, 1),
		}
		bulkRequest = bulkRequest.Add(
			elastic.NewBulkIndexRequest().
				Index(indexPatternPrefix).
				Doc(doc),
		)
	}
	// TODO: should not need to wait_for the index refresh interval, but also shouldn't force refresh
	_, err := bulkRequest.Refresh("wait_for").Do(ctx)
	if err != nil {
		e.logger.Error("bulk-put-failed", err)
		return fmt.Errorf("bulk put: %w", err)
	}
	return nil
}

func (e *EventStore) Get(ctx context.Context, build db.Build, requested int, cursor *db.Key) ([]event.Envelope, error) {
	offset, err := e.offset(cursor)
	if err != nil {
		e.logger.Error("offset-failed", err)
		return nil, err
	}

	req := e.client.Search(indexPatternPrefix).
		Query(elastic.NewTermQuery("build_id", build.ID())).
		Sort("data.time", true).
		Sort("tiebreak", true).
		Size(requested)
	if offset.TimeMillis > 0 {
		req = req.SearchAfter(offset.TimeMillis, offset.Tiebreak)
	}

	searchResult, err := req.Do(ctx)
	if err != nil {
		e.logger.Error("search-failed", err)
		return nil, fmt.Errorf("perform search: %w", err)
	}

	numHits := len(searchResult.Hits.Hits)
	if numHits == 0 {
		return []event.Envelope{}, nil
	}
	events := make([]event.Envelope, numHits)
	for i, hit := range searchResult.Hits.Hits {
		var envelope event.Envelope
		if err = json.Unmarshal(hit.Source, &envelope); err != nil {
			e.logger.Error("unmarshal-hit-failed", err)
			return nil, fmt.Errorf("unmarshal source to event.Envelope: %w", err)
		}
		events[i] = envelope
	}

	lastHit := searchResult.Hits.Hits[numHits-1]
	var target struct {
		Tiebreak int64 `json:"tiebreak"`
		Data     struct {
			Time int64 `json:"time"`
		} `json:"data"`
	}
	if err = json.Unmarshal(lastHit.Source, &target); err != nil {
		e.logger.Error("unmarshal-last-hit-failed", err)
		return nil, fmt.Errorf("unmarshal last hit: %w", err)
	}
	*cursor = Key{
		TimeMillis: target.Data.Time * 1000,
		Tiebreak:   target.Tiebreak,
	}

	return events, nil
}

func (e *EventStore) offset(cursor *db.Key) (Key, error) {
	if cursor == nil || *cursor == nil {
		return Key{}, nil
	}
	offset, ok := (*cursor).(Key)
	if !ok {
		return Key{}, fmt.Errorf("invalid Key type (expected elasticsearch.Key, got %T)", *cursor)
	}
	return offset, nil
}

func (e *EventStore) Delete(ctx context.Context, builds []db.Build) error {
	buildIDs := make([]int, len(builds))
	for i, build := range builds {
		buildIDs[i] = build.ID()
	}
	err := e.asyncDelete(ctx, elastic.NewTermsQuery("build_id", buildIDs))
	if err != nil {
		e.logger.Error("delete-builds-failed", err, lager.Data{"build_ids": buildIDs})
		return fmt.Errorf("delete builds: %w", err)
	}
	return nil
}

func (e *EventStore) DeletePipeline(ctx context.Context, pipeline db.Pipeline) error {
	err := e.asyncDelete(ctx, elastic.NewTermQuery("pipeline_id", pipeline.ID()))
	if err != nil {
		e.logger.Error("delete-pipeline-failed", err, lager.Data{"pipeline_id": pipeline.ID()})
		return fmt.Errorf("delete pipeline: %w", err)
	}
	return nil
}

func (e *EventStore) DeleteTeam(ctx context.Context, team db.Team) error {
	err := e.asyncDelete(ctx, elastic.NewTermQuery("team_id", team.ID()))
	if err != nil {
		e.logger.Error("delete-team-failed", err, lager.Data{"team_id": team.ID()})
		return fmt.Errorf("delete team: %w", err)
	}
	return nil
}

func (e *EventStore) asyncDelete(ctx context.Context, query elastic.Query) error {
	_, err := e.client.DeleteByQuery(indexPatternPrefix).
		Query(query).
		DoAsync(ctx)
	return err
}
