package engine

import (
	"context"
	"io"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/runtime"
)

func NewTaskDelegate(
	build db.Build,
	planID atc.PlanID,
	state exec.RunState,
	clock clock.Clock,
	policyChecker policy.Checker,
	dbWorkerFactory db.WorkerFactory,
	lockFactory lock.LockFactory,
) exec.TaskDelegate {
	return &taskDelegate{
		BuildStepDelegate: NewBuildStepDelegate(build, planID, state, clock, policyChecker),

		eventOrigin: event.Origin{ID: event.OriginID(planID)},
		planID:      planID,
		build:       build,
		clock:       clock,

		dbWorkerFactory: dbWorkerFactory,
		lockFactory:     lockFactory,
	}
}

type taskDelegate struct {
	exec.BuildStepDelegate

	planID         atc.PlanID
	config         atc.TaskConfig
	serviceConfigs []atc.TaskConfig
	build          db.Build
	eventOrigin    event.Origin
	clock          clock.Clock

	dbWorkerFactory db.WorkerFactory
	lockFactory     lock.LockFactory
}

func (d *taskDelegate) SetTaskConfig(config atc.TaskConfig) {
	d.config = config
}

func (d *taskDelegate) SetServiceConfigs(configs []atc.TaskConfig) {
	d.serviceConfigs = configs
}

func (d *taskDelegate) Initializing(logger lager.Logger) {
	var shadowedServiceConfigs []event.TaskConfig
	for _, c := range d.serviceConfigs {
		shadowedServiceConfigs = append(shadowedServiceConfigs, event.ShadowTaskConfig(c))
	}
	err := d.build.SaveEvent(event.InitializeTask{
		Origin:         d.eventOrigin,
		Time:           d.clock.Now().Unix(),
		TaskConfig:     event.ShadowTaskConfig(d.config),
		ServiceConfigs: shadowedServiceConfigs,
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-task-event", err)
		return
	}

	logger.Info("initializing")
}

func (d *taskDelegate) InitializingServices(logger lager.Logger) {
	var shadowedServiceConfigs []event.TaskConfig
	for _, c := range d.serviceConfigs {
		shadowedServiceConfigs = append(shadowedServiceConfigs, event.ShadowTaskConfig(c))
	}
	err := d.build.SaveEvent(event.InitializeTask{
		Origin:         d.eventOrigin,
		Time:           d.clock.Now().Unix(),
		TaskConfig:     event.ShadowTaskConfig(d.config),
		ServiceConfigs: shadowedServiceConfigs,
	})
	if err != nil {
		logger.Error("failed-to-save-services-initialize-task-event", err)
		return
	}

	logger.Info("services-initializing")
}

func (d *taskDelegate) Starting(logger lager.Logger) {
	var shadowedServiceConfigs []event.TaskConfig
	for _, c := range d.serviceConfigs {
		shadowedServiceConfigs = append(shadowedServiceConfigs, event.ShadowTaskConfig(c))
	}
	err := d.build.SaveEvent(event.StartTask{
		Origin:         d.eventOrigin,
		Time:           d.clock.Now().Unix(),
		TaskConfig:     event.ShadowTaskConfig(d.config),
		ServiceConfigs: shadowedServiceConfigs,
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-task-event", err)
		return
	}

	logger.Debug("starting")
}

func (d *taskDelegate) Finished(
	logger lager.Logger,
	exitStatus exec.ExitStatus,
) {
	// PR#4398: close to flush stdout and stderr
	d.Stdout().(io.Closer).Close()
	d.Stderr().(io.Closer).Close()

	err := d.build.SaveEvent(event.FinishTask{
		ExitStatus: int(exitStatus),
		Time:       d.clock.Now().Unix(),
		Origin:     d.eventOrigin,
	})
	if err != nil {
		logger.Error("failed-to-save-finish-event", err)
		return
	}

	logger.Info("finished", lager.Data{"exit-status": exitStatus})
}

func (d *taskDelegate) FetchImage(
	ctx context.Context,
	image atc.ImageResource,
	types atc.ResourceTypes,
	privileged bool,
	stepTags atc.Tags,
	skipInterval bool,
) (runtime.ImageSpec, error) {
	image.Name = "image"

	getPlan, checkPlan := atc.FetchImagePlan(d.planID, image, types, stepTags, skipInterval, nil)

	if checkPlan != nil {
		err := d.build.SaveEvent(event.ImageCheck{
			Time: d.clock.Now().Unix(),
			Origin: event.Origin{
				ID: event.OriginID(d.planID),
			},
			PublicPlan: checkPlan.Public(),
		})
		if err != nil {
			return runtime.ImageSpec{}, err
		}
	}

	err := d.build.SaveEvent(event.ImageGet{
		Time: d.clock.Now().Unix(),
		Origin: event.Origin{
			ID: event.OriginID(d.planID),
		},
		PublicPlan: getPlan.Public(),
	})
	if err != nil {
		return runtime.ImageSpec{}, err
	}

	imageSpec, _, err := d.BuildStepDelegate.FetchImage(ctx, getPlan, checkPlan, privileged)
	if err != nil {
		return runtime.ImageSpec{}, err
	}

	return imageSpec, nil
}
