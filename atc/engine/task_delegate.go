package engine

import (
	"context"
	"fmt"
	"io"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/builds"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/vars"
)

func NewTaskDelegate(
	build db.Build,
	planID atc.PlanID,
	state exec.RunState,
	clock clock.Clock,
	policyChecker policy.Checker,
	artifactSourcer worker.ArtifactSourcer,
	dbWorkerFactory db.WorkerFactory,
	lockFactory lock.LockFactory,
	globalSecrets creds.Secrets,
) exec.TaskDelegate {
	return &taskDelegate{
		BuildStepDelegate: NewBuildStepDelegate(build, planID, state, clock, policyChecker, artifactSourcer, globalSecrets),

		eventOrigin: event.Origin{ID: event.OriginID(planID)},
		planID:      planID,
		build:       build,
		clock:       clock,
		state:       state,

		dbWorkerFactory: dbWorkerFactory,
		lockFactory:     lockFactory,
	}
}

type taskDelegate struct {
	exec.BuildStepDelegate

	planID      atc.PlanID
	config      atc.TaskConfig
	build       db.Build
	eventOrigin event.Origin
	clock       clock.Clock
	state       exec.RunState

	dbWorkerFactory db.WorkerFactory
	lockFactory     lock.LockFactory
}

func (d *taskDelegate) SetTaskConfig(config atc.TaskConfig) {
	d.config = config
}

func (d *taskDelegate) Initializing(logger lager.Logger) {
	err := d.build.SaveEvent(event.InitializeTask{
		Origin:     d.eventOrigin,
		Time:       d.clock.Now().Unix(),
		TaskConfig: event.ShadowTaskConfig(d.config),
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-task-event", err)
		return
	}

	logger.Info("initializing")
}

func (d *taskDelegate) Starting(logger lager.Logger) {
	err := d.build.SaveEvent(event.StartTask{
		Origin:     d.eventOrigin,
		Time:       d.clock.Now().Unix(),
		TaskConfig: event.ShadowTaskConfig(d.config),
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
	strategy worker.ContainerPlacementStrategy,
	chosenWorker worker.Client,
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
	varSources atc.VarSourceConfigs,
	privileged bool,
	stepTags atc.Tags,
) (worker.ImageSpec, error) {
	image.Name = "image"

	getPlan, checkPlan, err := builds.FetchImagePlan(d.planID, image, types, varSources, stepTags)
	if err != nil {
		return worker.ImageSpec{}, err
	}

	if checkPlan != nil {
		err := d.build.SaveEvent(event.ImageCheck{
			Time: d.clock.Now().Unix(),
			Origin: event.Origin{
				ID: event.OriginID(d.planID),
			},
			PublicPlan: checkPlan.Public(),
		})
		if err != nil {
			return worker.ImageSpec{}, err
		}
	}

	err = d.build.SaveEvent(event.ImageGet{
		Time: d.clock.Now().Unix(),
		Origin: event.Origin{
			ID: event.OriginID(d.planID),
		},
		PublicPlan: getPlan.Public(),
	})
	if err != nil {
		return worker.ImageSpec{}, err
	}

	imageSpec, _, err := d.BuildStepDelegate.FetchImage(ctx, getPlan, checkPlan, privileged)
	if err != nil {
		return worker.ImageSpec{}, err
	}

	return imageSpec, nil
}

func (d *taskDelegate) FetchVariables(ctx context.Context, varSourceConfigs atc.VarSourceConfigs) vars.Variables {
	return &TaskVariables{
		delegate:         d,
		varSourceConfigs: varSourceConfigs,
		ctx:              ctx,
	}
}

type TaskVariables struct {
	delegate         *taskDelegate
	varSourceConfigs atc.VarSourceConfigs
	getVarPlanNum    int
	ctx              context.Context
}

func (v *TaskVariables) Get(ref vars.Reference) (interface{}, bool, error) {
	childState := v.delegate.state.NewScope()

	v.getVarPlanNum++

	planID := atc.PlanID(fmt.Sprintf("%s/task-var", v.delegate.planID))

	plan := atc.Plan{
		ID: planID,
		GetVar: &atc.GetVarPlan{
			Name:   ref.Source,
			Path:   ref.Path,
			Fields: ref.Fields,
		},
	}

	if ref.Source != "" {
		varSourceConfig, found := v.varSourceConfigs.Lookup(ref.Source)
		if !found {
			return nil, false, atc.UnknownVarSourceError{ref.Source}
		}
		subGetVarPlans, err := v.varSourceConfigs.Without(ref.Source).GetVarPlans(planID, varSourceConfig.Config)
		if err != nil {
			return nil, false, err
		}

		plan.GetVar.Type = varSourceConfig.Type
		plan.GetVar.Source = varSourceConfig.Config
		plan.GetVar.VarPlans = subGetVarPlans
	}

	err := v.delegate.build.SaveEvent(event.SubGetVar{
		Time: v.delegate.clock.Now().Unix(),
		Origin: event.Origin{
			ID: event.OriginID(planID),
		},
		PublicPlan: plan.Public(),
	})
	if err != nil {
		return nil, false, fmt.Errorf("save sub get var event: %w", err)
	}

	ok, err := childState.Run(v.ctx, plan)
	if err != nil {
		return nil, false, fmt.Errorf("run sub get var: %w", err)
	}

	if !ok {
		return nil, false, nil
	}

	var value interface{}
	if !childState.Result(planID, &value) {
		return nil, false, fmt.Errorf("get var did not return a value")
	}

	return value, true, nil
}
