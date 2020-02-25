package accessor

import (
	"context"
	"errors"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/MasterOfBinary/gobatch/batch"
	"github.com/MasterOfBinary/gobatch/source"
	"github.com/concourse/concourse/atc/db"
)

type batcher struct {
	batch       *batch.Batch
	source      chan interface{}
	userFactory db.UserFactory
}

type user struct {
	sub       string
	name      string
	connector string
}

func (u user) ID() int              { return 0 }
func (u user) Sub() string          { return u.sub }
func (u user) Name() string         { return u.name }
func (u user) Connector() string    { return u.connector }
func (u user) LastLogin() time.Time { return time.Now() }

func NewBatcher(logger lager.Logger, userFactory db.UserFactory, batchConfig *batch.ConfigValues) *batcher {
	ch := make(chan interface{})
	s := source.Channel{
		Input: ch,
	}

	userBatch := batch.New(batch.NewConstantConfig(batchConfig))

	b := &batcher{
		batch:       userBatch,
		source:      ch,
		userFactory: userFactory,
	}

	errs := userBatch.Go(context.Background(), &s, b)
	go func() {
		for {
			err := <-errs
			logger.Error("failed-to-upsert-user", err)
		}
	}()

	return b
}

func (b *batcher) CreateOrUpdateUser(name string, connector string, sub string) error {

	b.source <- user{
		name:      name,
		connector: connector,
		sub:       sub,
	}

	return nil
}

func (b *batcher) Process(ctx context.Context, ps *batch.PipelineStage) {
	defer ps.Close()

	users := map[string]db.User{}

	for item := range ps.Input {
		if user, okParse := item.Get().(user); okParse {
			users[user.sub] = user
		} else {
			ps.Errors <- errors.New("failed-to-parse-user")
		}
	}

	err := b.userFactory.BatchUpsertUsers(users)
	if err != nil {
		ps.Errors <- err
	}
}
