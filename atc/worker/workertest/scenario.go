package workertest

import (
	"context"

	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/concourse/concourse"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbtest"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	"github.com/cppforlife/go-semi-semantic/version"
	. "github.com/onsi/gomega"
)

var dummyLogger = lagertest.NewTestLogger("dummy")

type Scenario struct {
	DB        *dbtest.Scenario
	DBBuilder dbtest.Builder

	Pool    worker.Pool
	Factory *Factory

	TeamID   int
	JobID    int
	JobName  string
	StepName string
}

type SetupFunc func(*Scenario)

func Setup(dbConn db.DbConn, lockFactory lock.LockFactory, setup ...SetupFunc) *Scenario {
	db := worker.NewDB(
		db.NewWorkerFactory(dbConn, db.NewStaticWorkerCache(dummyLogger, dbConn, 0)),
		db.NewTeamFactory(dbConn, lockFactory),
		db.NewVolumeRepository(dbConn),
		db.NewTaskCacheFactory(dbConn),
		db.NewWorkerTaskCacheFactory(dbConn),
		db.NewResourceCacheFactory(dbConn, lockFactory),
		db.NewWorkerBaseResourceTypeFactory(dbConn),
		lockFactory,
	)
	factory := &Factory{DB: db}
	pool := worker.NewPool(
		factory,
		db,
		version.MustNewVersionFromString(concourse.WorkerVersion),
	)
	builder := dbtest.NewBuilder(dbConn, lockFactory)
	return setupWithPool(pool, factory, builder, setup...)
}

func setupWithPool(pool worker.Pool, factory *Factory, builder dbtest.Builder, setup ...SetupFunc) *Scenario {
	scenario := &Scenario{
		DB:        dbtest.Setup(),
		DBBuilder: builder,
		Factory:   factory,
	}
	scenario.Pool = pool
	scenario.Run(setup...)
	return scenario
}

func (s *Scenario) Run(setup ...SetupFunc) {
	for _, f := range setup {
		f(s)
	}
}

func WithBasicJob() SetupFunc {
	return func(s *Scenario) {
		s.DB.Run(s.DBBuilder.WithTeam("team"))
		s.DB.Run(s.DBBuilder.WithPipeline(atc.Config{
			Jobs: []atc.JobConfig{{Name: "job"}},
		}))
		job := s.DB.Job("job")
		s.TeamID = s.DB.Team.ID()
		s.JobID = job.ID()
		s.JobName = job.Name()
		s.StepName = "some-step"
	}
}

func WithTeam(team string) SetupFunc {
	return func(s *Scenario) {
		s.DB.Run(s.DBBuilder.WithTeam(team))
	}
}

func WithWorkers(workers ...Worker) SetupFunc {
	return func(s *Scenario) {
		for _, worker := range workers {
			_, _, found := s.Factory.FindWorker(worker.Name())
			Expect(found).To(BeFalse(), "cannot add worker twice: %s", worker.Name())
			worker.Setup(s)

			s.Factory.Workers = append(s.Factory.Workers, worker)
		}
	}
}

func (s *Scenario) Team(name string) db.Team {
	team, found, err := s.DBBuilder.TeamFactory.FindTeam(name)
	Expect(err).ToNot(HaveOccurred())
	Expect(found).To(BeTrue())
	return team
}

func (s *Scenario) Worker(name string) runtime.Worker {
	worker, found, err := s.Pool.FindWorker(dummyLogger, name)
	Expect(err).ToNot(HaveOccurred())
	Expect(found).To(BeTrue(), "missing worker %q", name)
	return worker
}

func (s *Scenario) WorkerVolume(workerName string, handle string) runtime.Volume {
	worker := s.Worker(workerName)
	volume, found, err := worker.LookupVolume(context.Background(), handle)
	Expect(err).ToNot(HaveOccurred())
	Expect(found).To(BeTrue(), "missing volume %q on worker %q", handle, workerName)
	return volume
}

func (s *Scenario) FindOrCreateResourceCache(workerName string) db.ResourceCache {
	if s.JobName == "" {
		WithBasicJob()(s)
	}
	var build db.Build
	s.DB.Run(s.DBBuilder.WithJobBuild(&build, s.JobName, nil, nil))

	cache, err := s.DBBuilder.ResourceCacheFactory.FindOrCreateResourceCache(
		db.ForBuild(build.ID()),
		dbtest.BaseResourceType,
		atc.Version{},
		atc.Source{},
		atc.Params{},
		nil,
	)
	Expect(err).ToNot(HaveOccurred())
	return cache
}

func (s *Scenario) ContainerVolume(workerName string, containerHandle string, mountPath string) (db.CreatingVolume, db.CreatedVolume) {
	container, ok := s.DB.Container(workerName, db.NewFixedHandleContainerOwner(containerHandle)).(db.CreatingContainer)
	Expect(ok).To(BeTrue(), "container is not in creating state")

	creating, created, err := s.DBBuilder.VolumeRepo.FindContainerVolume(s.TeamID, workerName, container, mountPath)
	Expect(err).ToNot(HaveOccurred())

	return creating, created
}

func (s *Scenario) Streamer(p2p worker.P2PConfig) worker.Streamer {
	return worker.NewStreamer(s.Factory.DB.ResourceCacheFactory, compression.NewGzipCompression(), 0, p2p)
}
