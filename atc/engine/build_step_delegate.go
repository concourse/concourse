package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/tracing"
	"go.opentelemetry.io/otel/api/trace"
)

type buildStepDelegate struct {
	build           db.Build
	planID          atc.PlanID
	clock           clock.Clock
	state           exec.RunState
	stderr          io.Writer
	stdout          io.Writer
	policyChecker   policy.Checker
	artifactSourcer worker.ArtifactSourcer
}

func NewBuildStepDelegate(
	build db.Build,
	planID atc.PlanID,
	state exec.RunState,
	clock clock.Clock,
	policyChecker policy.Checker,
	artifactSourcer worker.ArtifactSourcer,
) *buildStepDelegate {
	return &buildStepDelegate{
		build:           build,
		planID:          planID,
		clock:           clock,
		state:           state,
		stdout:          nil,
		stderr:          nil,
		policyChecker:   policyChecker,
		artifactSourcer: artifactSourcer,
	}
}

func (delegate *buildStepDelegate) StartSpan(
	ctx context.Context,
	component string,
	extraAttrs tracing.Attrs,
) (context.Context, trace.Span) {
	attrs := delegate.build.TracingAttrs()
	for k, v := range extraAttrs {
		attrs[k] = v
	}

	return tracing.StartSpan(ctx, component, attrs)
}

type credVarsIterator struct {
	line string
}

func (it *credVarsIterator) YieldCred(name, value string) {
	for _, lineValue := range strings.Split(value, "\n") {
		lineValue = strings.TrimSpace(lineValue)
		// Don't consider a single char as a secret.
		if len(lineValue) > 1 {
			it.line = strings.Replace(it.line, lineValue, "((redacted))", -1)
		}
	}
}

func (delegate *buildStepDelegate) Stdout() io.Writer {
	if delegate.stdout != nil {
		return delegate.stdout
	}
	if delegate.state.RedactionEnabled() {
		delegate.stdout = newDBEventWriterWithSecretRedaction(
			delegate.build,
			event.Origin{
				Source: event.OriginSourceStdout,
				ID:     event.OriginID(delegate.planID),
			},
			delegate.clock,
			delegate.buildOutputFilter,
		)
	} else {
		delegate.stdout = newDBEventWriter(
			delegate.build,
			event.Origin{
				Source: event.OriginSourceStdout,
				ID:     event.OriginID(delegate.planID),
			},
			delegate.clock,
		)
	}
	return delegate.stdout
}

func (delegate *buildStepDelegate) Stderr() io.Writer {
	if delegate.stderr != nil {
		return delegate.stderr
	}
	if delegate.state.RedactionEnabled() {
		delegate.stderr = newDBEventWriterWithSecretRedaction(
			delegate.build,
			event.Origin{
				Source: event.OriginSourceStderr,
				ID:     event.OriginID(delegate.planID),
			},
			delegate.clock,
			delegate.buildOutputFilter,
		)
	} else {
		delegate.stderr = newDBEventWriter(
			delegate.build,
			event.Origin{
				Source: event.OriginSourceStderr,
				ID:     event.OriginID(delegate.planID),
			},
			delegate.clock,
		)
	}
	return delegate.stderr
}

func (delegate *buildStepDelegate) Initializing(logger lager.Logger) {
	err := delegate.build.SaveEvent(event.Initialize{
		Origin: event.Origin{
			ID: event.OriginID(delegate.planID),
		},
		Time: time.Now().Unix(),
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-event", err)
		return
	}

	logger.Info("initializing")
}

func (delegate *buildStepDelegate) Starting(logger lager.Logger) {
	err := delegate.build.SaveEvent(event.Start{
		Origin: event.Origin{
			ID: event.OriginID(delegate.planID),
		},
		Time: time.Now().Unix(),
	})
	if err != nil {
		logger.Error("failed-to-save-start-event", err)
		return
	}

	logger.Debug("starting")
}

func (delegate *buildStepDelegate) Finished(logger lager.Logger, succeeded bool) {
	// PR#4398: close to flush stdout and stderr
	delegate.Stdout().(io.Closer).Close()
	delegate.Stderr().(io.Closer).Close()

	err := delegate.build.SaveEvent(event.Finish{
		Origin: event.Origin{
			ID: event.OriginID(delegate.planID),
		},
		Time:      time.Now().Unix(),
		Succeeded: succeeded,
	})
	if err != nil {
		logger.Error("failed-to-save-finish-event", err)
		return
	}

	logger.Info("finished")
}

func (delegate *buildStepDelegate) WaitingForWorker(logger lager.Logger) {
	err := delegate.build.SaveEvent(event.WaitingForWorker{
		Time: time.Now().Unix(),
		Origin: event.Origin{
			ID: event.OriginID(delegate.planID),
		},
	})
	if err != nil {
		logger.Error("failed-to-save-waiting-for-worker-event", err)
		return
	}
}

func (delegate *buildStepDelegate) SelectedWorker(logger lager.Logger, workerName string) {
	err := delegate.build.SaveEvent(event.SelectedWorker{
		Time: time.Now().Unix(),
		Origin: event.Origin{
			ID: event.OriginID(delegate.planID),
		},
		WorkerName: workerName,
	})
	if err != nil {
		logger.Error("failed-to-save-selected-worker-event", err)
		return
	}
}

func (delegate *buildStepDelegate) Errored(logger lager.Logger, message string) {
	err := delegate.build.SaveEvent(event.Error{
		Message: message,
		Origin: event.Origin{
			ID: event.OriginID(delegate.planID),
		},
		Time: delegate.clock.Now().Unix(),
	})
	if err != nil {
		logger.Error("failed-to-save-error-event", err)
	}
}

// Name of the artifact fetched when using image_resource. Note that this only
// exists within a local scope, so it doesn't pollute the build state.
const defaultImageName = "image"

func (delegate *buildStepDelegate) FetchImage(
	ctx context.Context,
	image atc.ImageResource,
	types atc.VersionedResourceTypes,
	privileged bool,
) (worker.ImageSpec, error) {
	err := delegate.checkImagePolicy(image, privileged)
	if err != nil {
		return worker.ImageSpec{}, err
	}

	fetchState := delegate.state.NewLocalScope()

	imageName := defaultImageName
	if image.Name != "" {
		imageName = image.Name
	}

	version := image.Version
	if version == nil {
		checkID := delegate.planID + "/image-check"

		checkPlan := atc.Plan{
			ID: checkID,
			Check: &atc.CheckPlan{
				Name:   imageName,
				Type:   image.Type,
				Source: image.Source,

				VersionedResourceTypes: types,

				Tags: image.Tags,
			},
		}

		err := delegate.build.SaveEvent(event.ImageCheck{
			Time: delegate.clock.Now().Unix(),
			Origin: event.Origin{
				ID: event.OriginID(delegate.planID),
			},
			PublicPlan: checkPlan.Public(),
		})
		if err != nil {
			return worker.ImageSpec{}, fmt.Errorf("save image check event: %w", err)
		}

		ok, err := fetchState.Run(ctx, checkPlan)
		if err != nil {
			return worker.ImageSpec{}, err
		}

		if !ok {
			return worker.ImageSpec{}, fmt.Errorf("image check failed")
		}

		if !fetchState.Result(checkID, &version) {
			return worker.ImageSpec{}, fmt.Errorf("check did not return a version")
		}
	}

	getID := delegate.planID + "/image-get"

	getPlan := atc.Plan{
		ID: getID,
		Get: &atc.GetPlan{
			Name:    imageName,
			Type:    image.Type,
			Source:  image.Source,
			Version: &version,
			Params:  image.Params,

			VersionedResourceTypes: types,

			Tags: image.Tags,
		},
	}

	err = delegate.build.SaveEvent(event.ImageGet{
		Time: delegate.clock.Now().Unix(),
		Origin: event.Origin{
			ID: event.OriginID(delegate.planID),
		},
		PublicPlan: getPlan.Public(),
	})
	if err != nil {
		return worker.ImageSpec{}, fmt.Errorf("save image get event: %w", err)
	}

	ok, err := fetchState.Run(ctx, getPlan)
	if err != nil {
		return worker.ImageSpec{}, err
	}

	if !ok {
		return worker.ImageSpec{}, fmt.Errorf("image fetching failed")
	}

	var cache db.UsedResourceCache
	if !fetchState.Result(getID, &cache) {
		return worker.ImageSpec{}, fmt.Errorf("get did not return a cache")
	}

	err = delegate.build.SaveImageResourceVersion(cache)
	if err != nil {
		return worker.ImageSpec{}, fmt.Errorf("save image version: %w", err)
	}

	art, found := fetchState.ArtifactRepository().ArtifactFor(build.ArtifactName(imageName))
	if !found {
		return worker.ImageSpec{}, fmt.Errorf("fetched artifact not found")
	}

	source, err := delegate.artifactSourcer.SourceImage(lagerctx.FromContext(ctx), art)
	if err != nil {
		return worker.ImageSpec{}, fmt.Errorf("wire image: %w", err)
	}

	return worker.ImageSpec{
		ImageArtifactSource: source,
		Privileged:          privileged,
	}, nil
}

func (delegate *buildStepDelegate) checkImagePolicy(image atc.ImageResource, privileged bool) error {
	if !delegate.policyChecker.ShouldCheckAction(policy.ActionUseImage) {
		return nil
	}

	redactedSource, err := delegate.redactImageSource(image.Source)
	if err != nil {
		return fmt.Errorf("redact source: %w", err)
	}

	result, err := delegate.policyChecker.Check(policy.PolicyCheckInput{
		Action:   policy.ActionUseImage,
		Team:     delegate.build.TeamName(),
		Pipeline: delegate.build.PipelineName(),
		Data: map[string]interface{}{
			"image_type":   image.Type,
			"image_source": redactedSource,
			"privileged":   privileged,
		},
	})
	if err != nil {
		return fmt.Errorf("perform check: %w", err)
	}

	if !result.Allowed {
		return policy.PolicyCheckNotPass{
			Reasons: result.Reasons,
		}
	}

	return nil
}

func (delegate *buildStepDelegate) buildOutputFilter(str string) string {
	it := &credVarsIterator{line: str}
	delegate.state.IterateInterpolatedCreds(it)
	return it.line
}

func (delegate *buildStepDelegate) redactImageSource(source atc.Source) (atc.Source, error) {
	b, err := json.Marshal(&source)
	if err != nil {
		return source, err
	}
	s := delegate.buildOutputFilter(string(b))
	newSource := atc.Source{}
	err = json.Unmarshal([]byte(s), &newSource)
	if err != nil {
		return source, err
	}
	return newSource, nil
}
