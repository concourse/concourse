package engine

import (
	"sync"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
	"github.com/pivotal-golang/lager"
)

type implicitOutput struct {
	plan atc.InputPlan
	info exec.VersionInfo
}

//go:generate counterfeiter . BuildDelegate

type BuildDelegate interface {
	InputCompleted(lager.Logger, atc.InputPlan) exec.CompleteCallback
	ExecutionCompleted(lager.Logger) exec.CompleteCallback
	OutputCompleted(lager.Logger, atc.OutputPlan) exec.CompleteCallback

	Start(lager.Logger)
	Finish(lager.Logger) exec.CompleteCallback
	Aborted(lager.Logger)
}

//go:generate counterfeiter . BuildDelegateFactory

type BuildDelegateFactory interface {
	Delegate(buildID int) BuildDelegate
}

type buildDelegateFactory struct {
	db EngineDB
}

func NewBuildDelegateFactory(db EngineDB) BuildDelegateFactory {
	return buildDelegateFactory{db}
}

func (factory buildDelegateFactory) Delegate(buildID int) BuildDelegate {
	return newBuildDelegate(factory.db, buildID)
}

type delegate struct {
	db EngineDB

	buildID int

	successful bool
	aborted    bool

	implicitOutputs map[string]implicitOutput

	lock sync.Mutex
}

func newBuildDelegate(db EngineDB, buildID int) BuildDelegate {
	return &delegate{
		db: db,

		buildID: buildID,

		successful: true,
		aborted:    false,

		implicitOutputs: make(map[string]implicitOutput),
	}
}

func (delegate *delegate) InputCompleted(logger lager.Logger, plan atc.InputPlan) exec.CompleteCallback {
	return exec.CallbackFunc(func(err error, source exec.ArtifactSource) {
		if err != nil {
			delegate.saveErr(logger, err, event.Origin{
				Type: event.OriginTypeInput,
				Name: plan.Name,
			})

			logger.Error("errored", err)
		} else {
			var info exec.VersionInfo
			if source.Result(&info) {
				delegate.saveInput(logger, plan, info)
				delegate.registerImplicitOutput(plan.Resource, implicitOutput{plan, info})
			}

			logger.Info("finished", lager.Data{"version-info": info})
		}
	})
}

func (delegate *delegate) ExecutionCompleted(logger lager.Logger) exec.CompleteCallback {
	return exec.CallbackFunc(func(err error, source exec.ArtifactSource) {
		if err != nil {
			delegate.saveErr(logger, err, event.Origin{})

			logger.Error("errored", err)
		} else {
			var status exec.ExitStatus
			if source.Result(&status) {
				delegate.saveFinish(status)
			}

			var success exec.Success
			if source.Result(&success) {
				if success == false {
					delegate.successful = false
				}
			}

			logger.Info("finished", lager.Data{
				"status":    status,
				"succeeded": success,
			})
		}
	})
}

func (delegate *delegate) OutputCompleted(logger lager.Logger, plan atc.OutputPlan) exec.CompleteCallback {
	return exec.CallbackFunc(func(err error, source exec.ArtifactSource) {
		if err != nil {
			delegate.saveErr(logger, err, event.Origin{
				Type: event.OriginTypeOutput,
				Name: plan.Name,
			})

			logger.Error("errored", err)
		} else {
			delegate.unregisterImplicitOutput(plan.Name)

			var info exec.VersionInfo
			if source.Result(&info) {
				delegate.saveOutput(logger, plan, info)
			}

			logger.Info("finished", lager.Data{"version-info": info})
		}
	})
}

func (delegate *delegate) Start(logger lager.Logger) {
	// TODO?: make this a callback hooked in to the steps when a certain one starts

	startedAt := time.Now()

	logger.Info("start", lager.Data{"started-at": startedAt})

	err := delegate.db.SaveBuildStartTime(delegate.buildID, startedAt)
	if err != nil {
		logger.Error("failed-to-save-start-time", err)
	}

	err = delegate.db.SaveBuildStatus(delegate.buildID, db.StatusStarted)
	if err != nil {
		logger.Error("failed-to-save-status", err)
	}

	err = delegate.db.SaveBuildEvent(delegate.buildID, event.Start{
		Time: startedAt.Unix(),
	})
	if err != nil {
		logger.Error("failed-to-save-start-event", err)
	}

	err = delegate.db.SaveBuildEvent(delegate.buildID, event.Status{
		Status: atc.StatusStarted,
		Time:   startedAt.Unix(),
	})
	if err != nil {
		logger.Error("failed-to-save-status-event", err)
	}
}

func (delegate *delegate) Finish(logger lager.Logger) exec.CompleteCallback {
	return exec.CallbackFunc(func(err error, source exec.ArtifactSource) {
		if delegate.aborted {
			delegate.saveStatus(logger, atc.StatusAborted)

			logger.Info("aborted")
		} else if err != nil {
			delegate.saveStatus(logger, atc.StatusErrored)

			logger.Error("errored", err)
		} else if delegate.successful {
			delegate.saveStatus(logger, atc.StatusSucceeded)

			implicits := logger.Session("implicit-outputs")

			for _, o := range delegate.implicitOutputs {
				delegate.saveImplicitOutput(implicits.Session(o.plan.Name), o.plan, o.info)
			}

			logger.Info("succeeded")
		} else {
			delegate.saveStatus(logger, atc.StatusFailed)

			logger.Info("failed")
		}
	})
}

func (delegate *delegate) Aborted(lager.Logger) {
	delegate.aborted = true

	logger.Info("aborted")
}

func (delegate *delegate) registerImplicitOutput(resource string, output implicitOutput) {
	delegate.lock.Lock()
	delegate.implicitOutputs[resource] = output
	delegate.lock.Unlock()
}

func (delegate *delegate) unregisterImplicitOutput(resource string) {
	delegate.lock.Lock()
	delete(delegate.implicitOutputs, resource)
	delegate.lock.Unlock()
}

func (delegate *delegate) saveFinish(status exec.ExitStatus) {
	delegate.db.SaveBuildEvent(delegate.buildID, event.Finish{
		ExitStatus: int(status),
		Time:       time.Now().Unix(),
	})
}

func (delegate *delegate) saveStatus(logger lager.Logger, status atc.BuildStatus) {
	time := time.Now()

	err := delegate.db.SaveBuildEndTime(delegate.buildID, time)
	if err != nil {
		logger.Error("failed-to-save-end-time", err)
	}

	err = delegate.db.SaveBuildStatus(delegate.buildID, db.Status(status))
	if err != nil {
		logger.Error("failed-to-save-status", err)
	}

	err = delegate.db.SaveBuildEvent(delegate.buildID, event.Status{
		Status: status,
		Time:   time.Unix(),
	})
	if err != nil {
		logger.Error("failed-to-save-status-event", err)
	}

	err = delegate.db.CompleteBuild(delegate.buildID)
	if err != nil {
		logger.Error("failed-to-complete-build", err)
	}
}

func (delegate *delegate) saveErr(logger lager.Logger, errVal error, origin event.Origin) {
	err := delegate.db.SaveBuildEvent(delegate.buildID, event.Error{
		Message: errVal.Error(),
		Origin:  origin,
	})
	if err != nil {
		logger.Error("failed-to-save-error-event", err)
	}
}

func (delegate *delegate) saveInput(logger lager.Logger, plan atc.InputPlan, info exec.VersionInfo) {
	ev := event.Input{
		Plan:            plan,
		FetchedVersion:  info.Version,
		FetchedMetadata: info.Metadata,
	}

	err := delegate.db.SaveBuildEvent(delegate.buildID, ev)
	if err != nil {
		logger.Error("failed-to-save-input-event", err)
	}

	err = delegate.db.SaveBuildInput(delegate.buildID, db.BuildInput{
		Name:              plan.Name,
		VersionedResource: vrFromInput(ev),
	})
	if err != nil {
		logger.Error("failed-to-save-input", err)
	}
}

func (delegate *delegate) saveOutput(logger lager.Logger, plan atc.OutputPlan, info exec.VersionInfo) {
	ev := event.Output{
		Plan:            plan,
		CreatedVersion:  info.Version,
		CreatedMetadata: info.Metadata,
	}

	err := delegate.db.SaveBuildEvent(delegate.buildID, ev)
	if err != nil {
		logger.Error("failed-to-save-output-event", err)
	}

	err = delegate.db.SaveBuildOutput(delegate.buildID, vrFromOutput(ev))
	if err != nil {
		logger.Error("failed-to-save-output", err)
	}
}

func (delegate *delegate) saveImplicitOutput(logger lager.Logger, plan atc.InputPlan, info exec.VersionInfo) {
	metadata := make([]db.MetadataField, len(info.Metadata))
	for i, md := range info.Metadata {
		metadata[i] = db.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	err := delegate.db.SaveBuildOutput(delegate.buildID, db.VersionedResource{
		Resource: plan.Resource,
		Type:     plan.Type,
		Source:   db.Source(plan.Source),
		Version:  db.Version(info.Version),
		Metadata: metadata,
	})
	if err != nil {
		logger.Error("failed-to-save", err)
		return
	}

	logger.Info("saved", lager.Data{"resource": plan.Resource})
}
