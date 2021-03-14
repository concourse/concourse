package workertest

import (
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse"
	"github.com/concourse/concourse/atc"
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
	StepName string
}

type SetupFunc func(*Scenario)

func Setup(dbConn db.Conn, lockFactory lock.LockFactory, setup ...SetupFunc) *Scenario {
	poolFactory := func(factory worker.Factory) worker.Pool {
		return worker.Pool{
			Factory: factory,
			DB: worker.DB{
				WorkerFactory:                 db.NewWorkerFactory(dbConn),
				TeamFactory:                   db.NewTeamFactory(dbConn, lockFactory),
				VolumeRepo:                    db.NewVolumeRepository(dbConn),
				ResourceCacheFactory:          db.NewResourceCacheFactory(dbConn, lockFactory),
				TaskCacheFactory:              db.NewTaskCacheFactory(dbConn),
				WorkerTaskCacheFactory:        db.NewWorkerTaskCacheFactory(dbConn),
				WorkerBaseResourceTypeFactory: db.NewWorkerBaseResourceTypeFactory(dbConn),
				LockFactory:                   lockFactory,
			},
			WorkerVersion: version.MustNewVersionFromString(concourse.WorkerVersion),
		}
	}
	builder := dbtest.NewBuilder(dbConn, lockFactory)
	return SetupWithPool(poolFactory, builder, setup...)
}

func SetupWithPool(poolFactory PoolFactory, builder dbtest.Builder, setup ...SetupFunc) *Scenario {
	scenario := &Scenario{
		DB:        dbtest.Setup(),
		DBBuilder: builder,

		Factory: &Factory{},
	}
	scenario.Pool = poolFactory(scenario.Factory)
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
	Expect(found).To(BeTrue())
	return worker
}

func (s *Scenario) WorkerVolume(workerName string, handle string) runtime.Volume {
	worker := s.Worker(workerName)
	volume, found, err := worker.LookupVolume(dummyLogger, handle)
	Expect(err).ToNot(HaveOccurred())
	Expect(found).To(BeTrue())
	return volume
}

func (s *Scenario) FindOrCreateResourceCache(workerName string, containerHandle string) db.UsedResourceCache {
	container := s.DB.Container(workerName, db.NewFixedHandleContainerOwner(containerHandle))
	cache, err := s.DBBuilder.ResourceCacheFactory.FindOrCreateResourceCache(
		db.ForContainer(container.ID()),
		"some-resource",
		atc.Version{},
		atc.Source{},
		atc.Params{},
		atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name: "some-resource",
					Type: dbtest.BaseResourceType,
				},
			},
		},
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
