package engine

import (
	"sync"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
)

type implicitOutput struct {
	plan atc.InputPlan
	info exec.VersionInfo
}

//go:generate counterfeiter . BuildDelegate

type BuildDelegate interface {
	InputCompleted(atc.InputPlan) exec.CompleteCallback
	ExecutionCompleted() exec.CompleteCallback
	OutputCompleted(atc.OutputPlan) exec.CompleteCallback

	Start()
	Finish() exec.CompleteCallback
	Aborted()
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

func (delegate *delegate) InputCompleted(plan atc.InputPlan) exec.CompleteCallback {
	return exec.CallbackFunc(func(err error, source exec.ArtifactSource) {
		if err != nil {
			delegate.saveErr(err, event.Origin{
				Type: event.OriginTypeInput,
				Name: plan.Name,
			})
		} else {
			var info exec.VersionInfo
			if source.Result(&info) {
				delegate.saveInput(plan, info)
				delegate.registerImplicitOutput(plan.Resource, implicitOutput{plan, info})
			}
		}
	})
}

func (delegate *delegate) ExecutionCompleted() exec.CompleteCallback {
	return exec.CallbackFunc(func(err error, source exec.ArtifactSource) {
		if err != nil {
			delegate.saveErr(err, event.Origin{})
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
		}
	})
}

func (delegate *delegate) OutputCompleted(plan atc.OutputPlan) exec.CompleteCallback {
	return exec.CallbackFunc(func(err error, source exec.ArtifactSource) {
		if err != nil {
			delegate.saveErr(err, event.Origin{
				Type: event.OriginTypeOutput,
				Name: plan.Name,
			})
		} else {
			delegate.unregisterImplicitOutput(plan.Name)

			var info exec.VersionInfo
			if source.Result(&info) {
				delegate.saveOutput(plan, info)
			}
		}
	})
}

func (delegate *delegate) Start() {
	// TODO?: make this a callback hooked in to the steps when a certain one starts

	time := time.Now()

	delegate.db.SaveBuildStartTime(delegate.buildID, time)

	delegate.db.SaveBuildStatus(delegate.buildID, db.StatusStarted)

	delegate.db.SaveBuildEvent(delegate.buildID, event.Start{
		Time: time.Unix(),
	})

	delegate.db.SaveBuildEvent(delegate.buildID, event.Status{
		Status: atc.StatusStarted,
		Time:   time.Unix(),
	})
}

func (delegate *delegate) Finish() exec.CompleteCallback {
	return exec.CallbackFunc(func(err error, source exec.ArtifactSource) {
		if delegate.aborted {
			delegate.saveStatus(atc.StatusAborted)
		} else if err != nil {
			delegate.saveStatus(atc.StatusErrored)
		} else if delegate.successful {
			delegate.saveStatus(atc.StatusSucceeded)

			for _, o := range delegate.implicitOutputs {
				delegate.saveImplicitOutput(o.plan, o.info)
			}
		} else {
			delegate.saveStatus(atc.StatusFailed)
		}
	})
}

func (delegate *delegate) Aborted() {
	delegate.aborted = true
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

func (delegate *delegate) saveStatus(status atc.BuildStatus) {
	// TODO handle errs

	time := time.Now()

	delegate.db.SaveBuildEndTime(delegate.buildID, time)

	delegate.db.SaveBuildStatus(delegate.buildID, db.Status(status))

	delegate.db.SaveBuildEvent(delegate.buildID, event.Status{
		Status: status,
		Time:   time.Unix(),
	})

	delegate.db.CompleteBuild(delegate.buildID)
}

func (delegate *delegate) saveErr(err error, origin event.Origin) {
	// TODO handle errs

	delegate.db.SaveBuildEvent(delegate.buildID, event.Error{
		Message: err.Error(),
		Origin:  origin,
	})
}

func (delegate *delegate) saveInput(plan atc.InputPlan, info exec.VersionInfo) {
	ev := event.Input{
		Plan:            plan,
		FetchedVersion:  info.Version,
		FetchedMetadata: info.Metadata,
	}

	delegate.db.SaveBuildEvent(delegate.buildID, ev)

	delegate.db.SaveBuildInput(delegate.buildID, db.BuildInput{
		Name:              plan.Name,
		VersionedResource: vrFromInput(ev),
	})
}

func (delegate *delegate) saveOutput(plan atc.OutputPlan, info exec.VersionInfo) {
	ev := event.Output{
		Plan:            plan,
		CreatedVersion:  info.Version,
		CreatedMetadata: info.Metadata,
	}

	delegate.db.SaveBuildEvent(delegate.buildID, ev)

	delegate.db.SaveBuildOutput(delegate.buildID, vrFromOutput(ev))
}

func (delegate *delegate) saveImplicitOutput(plan atc.InputPlan, info exec.VersionInfo) {
	metadata := make([]db.MetadataField, len(info.Metadata))
	for i, md := range info.Metadata {
		metadata[i] = db.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	delegate.db.SaveBuildOutput(delegate.buildID, db.VersionedResource{
		Resource: plan.Resource,
		Type:     plan.Type,
		Source:   db.Source(plan.Source),
		Version:  db.Version(info.Version),
		Metadata: metadata,
	})
}
