package elasticsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/event"
	"github.com/olivere/elastic/v7"
)

type eventDoc struct {
	EventID      db.EventKey      `json:"event_id"`
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
}

type Store struct {
	logger lager.Logger
	client *elastic.Client

	eventIDAllocator db.BuildEventIDAllocator

	URL string `long:"url" description:"URL of Elasticsearch cluster."`
}

func (e *Store) IsConfigured() bool {
	return e.URL != ""
}

func (e *Store) Setup(ctx context.Context, conn db.Conn) error {
	e.logger = lagerctx.FromContext(ctx)

	e.logger.Debug("setup-event-store", lager.Data{"url": e.URL})
	var err error
	e.client, err = elastic.NewClient(
		elastic.SetURL(e.URL),
		elastic.SetHealthcheckTimeoutStartup(1 * time.Minute),
	)
	if err != nil {
		e.logger.Error("connect-to-cluster-failed", err, lager.Data{"url": e.URL})
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

	e.eventIDAllocator = db.NewBuildEventIDAllocator(conn)

	return nil
}

func (e *Store) Close(ctx context.Context) error {
	e.client.Stop()
	return nil
}

func (e *Store) createIndexIfNotExists(ctx context.Context, name string, body string) error {
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

func (e *Store) Initialize(ctx context.Context, build db.Build) error {
	return e.eventIDAllocator.Initialize(ctx, build.ID())
}

func (e *Store) Finalize(ctx context.Context, build db.Build) error {
	return e.eventIDAllocator.Finalize(ctx, build.ID())
}

func (e *Store) Put(ctx context.Context, build db.Build, events []atc.Event) (db.EventKey, error) {
	if len(events) == 0 {
		return 0, nil
	}
	idBlock, err := e.eventIDAllocator.Allocate(ctx, build.ID(), len(events))
	if err != nil {
		e.logger.Error("allocate-ids", err)
		return 0, fmt.Errorf("allocate ids: %w", err)
	}
	bulkRequest := e.client.Bulk()
	var doc eventDoc
	var eventID db.EventKey
	for _, evt := range events {
		var ok bool
		eventID, ok = idBlock.Next()
		if !ok {
			err := fmt.Errorf("not enough event ids allocated")
			e.logger.Error("not-enough-event-ids-allocated", err)
			return 0, err
		}
		payload, err := json.Marshal(evt)
		if err != nil {
			e.logger.Error("marshal-event-failed", err)
			return 0, fmt.Errorf("marshal event: %w", err)
		}
		data := json.RawMessage(payload)
		doc = eventDoc{
			EventID:      eventID,
			BuildID:      build.ID(),
			BuildName:    build.Name(),
			JobID:        build.JobID(),
			JobName:      build.JobName(),
			PipelineID:   build.PipelineID(),
			PipelineName: build.PipelineName(),
			TeamID:       build.TeamID(),
			TeamName:     build.TeamName(),
			EventType:    evt.EventType(),
			Version:      evt.Version(),
			Data:         &data,
		}
		bulkRequest = bulkRequest.Add(
			elastic.NewBulkIndexRequest().
				Index(indexPatternPrefix).
				Doc(doc),
		)
	}
	_, err = bulkRequest.Do(ctx)
	if err != nil {
		e.logger.Error("bulk-put-failed", err)
		return 0, fmt.Errorf("bulk put: %w", err)
	}

	return eventID, nil
}

func (e *Store) Get(ctx context.Context, build db.Build, requested int, cursor *db.EventKey) ([]event.Envelope, error) {
	req := e.client.Search(indexPatternPrefix).
		Query(elastic.NewTermQuery("build_id", build.ID())).
		Sort("event_id", true).
		Size(requested)
	if *cursor != 0 {
		req = req.SearchAfter(*cursor)
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
		EventID db.EventKey `json:"event_id"`
	}
	if err = json.Unmarshal(lastHit.Source, &target); err != nil {
		e.logger.Error("unmarshal-last-hit-failed", err)
		return nil, fmt.Errorf("unmarshal last hit: %w", err)
	}
	*cursor = target.EventID

	return events, nil
}

func (e *Store) Delete(ctx context.Context, builds []db.Build) error {
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

func (e *Store) DeletePipeline(ctx context.Context, pipeline db.Pipeline) error {
	err := e.asyncDelete(ctx, elastic.NewTermQuery("pipeline_id", pipeline.ID()))
	if err != nil {
		e.logger.Error("delete-pipeline-failed", err, lager.Data{"pipeline_id": pipeline.ID()})
		return fmt.Errorf("delete pipeline: %w", err)
	}
	return nil
}

func (e *Store) DeleteTeam(ctx context.Context, team db.Team) error {
	err := e.asyncDelete(ctx, elastic.NewTermQuery("team_id", team.ID()))
	if err != nil {
		e.logger.Error("delete-team-failed", err, lager.Data{"team_id": team.ID()})
		return fmt.Errorf("delete team: %w", err)
	}
	return nil
}

func (e *Store) asyncDelete(ctx context.Context, query elastic.Query) error {
	_, err := e.client.DeleteByQuery(indexPatternPrefix).
		Query(query).
		DoAsync(ctx)
	return err
}
