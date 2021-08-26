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
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/vars"
	"go.opentelemetry.io/otel/trace"
	"sigs.k8s.io/yaml"
)

type buildStepDelegate struct {
	build         db.Build
	planID        atc.PlanID
	clock         clock.Clock
	state         exec.RunState
	stderr        io.Writer
	stdout        io.Writer
	policyChecker policy.Checker
}

func NewBuildStepDelegate(
	build db.Build,
	planID atc.PlanID,
	state exec.RunState,
	clock clock.Clock,
	policyChecker policy.Checker,
) *buildStepDelegate {
	return &buildStepDelegate{
		build:         build,
		planID:        planID,
		clock:         clock,
		state:         state,
		stdout:        nil,
		stderr:        nil,
		policyChecker: policyChecker,
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

func (delegate *buildStepDelegate) SelectedWorker(logger lager.Logger, worker string) {
	err := delegate.build.SaveEvent(event.SelectedWorker{
		Time: time.Now().Unix(),
		Origin: event.Origin{
			ID: event.OriginID(delegate.planID),
		},
		WorkerName: worker,
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

func (delegate *buildStepDelegate) FetchImage(
	ctx context.Context,
	getPlan atc.Plan,
	checkPlan *atc.Plan,
	privileged bool,
) (runtime.ImageSpec, db.ResourceCache, error) {
	err := delegate.checkImagePolicy(getPlan.Get.Source, getPlan.Get.Type, privileged)
	if err != nil {
		return runtime.ImageSpec{}, nil, err
	}

	fetchState := delegate.state.NewLocalScope()

	if checkPlan != nil {
		ok, err := fetchState.Run(ctx, *checkPlan)
		if err != nil {
			return runtime.ImageSpec{}, nil, err
		}

		if !ok {
			return runtime.ImageSpec{}, nil, fmt.Errorf("image check failed")
		}
	}

	ok, err := fetchState.Run(ctx, getPlan)
	if err != nil {
		return runtime.ImageSpec{}, nil, err
	}

	if !ok {
		return runtime.ImageSpec{}, nil, fmt.Errorf("image fetching failed")
	}

	var result exec.GetResult
	if !fetchState.Result(getPlan.ID, &result) {
		return runtime.ImageSpec{}, nil, fmt.Errorf("get did not return a result")
	}

	err = delegate.build.SaveImageResourceVersion(result.ResourceCache)
	if err != nil {
		return runtime.ImageSpec{}, nil, fmt.Errorf("save image version: %w", err)
	}

	artifact, found := fetchState.ArtifactRepository().ArtifactFor(build.ArtifactName(result.Name))
	if !found {
		return runtime.ImageSpec{}, nil, fmt.Errorf("fetched artifact not found")
	}

	return runtime.ImageSpec{
		ImageArtifact: artifact,
		Privileged:    privileged,
	}, result.ResourceCache, nil
}

func (delegate *buildStepDelegate) ConstructAcrossSubsteps(templateBytes []byte, acrossVars []atc.AcrossVar, valueCombinations [][]interface{}) ([]atc.VarScopedPlan, error) {
	template := vars.NewTemplate(templateBytes)
	substeps := make([]atc.VarScopedPlan, len(valueCombinations))
	substepsPublic := make([]*json.RawMessage, len(substeps))
	for i, values := range valueCombinations {
		localVars := vars.StaticVariables{}
		for j, v := range acrossVars {
			localVars[v.Var] = values[j]
		}
		interpolatedBytes, err := template.Evaluate(vars.NamedVariables{".": localVars}, vars.EvaluateOpts{})
		if err != nil {
			return nil, fmt.Errorf("failed to interpolate template: %w", err)
		}
		var subPlan atc.Plan
		// This must use sigs.k8s.io/yaml, since gopkg.in/yaml.v2 doesn't
		// convert from YAML -> JSON first.
		if err := yaml.Unmarshal(interpolatedBytes, &subPlan); err != nil {
			return nil, fmt.Errorf("invalid template bytes: %w", err)
		}

		// Maps from the original subplan ID generated by the planner to the
		// translated ID unique to the substep iteration.
		mappedSubplanIDs := map[atc.PlanID]atc.PlanID{}
		planIDCounter := 0
		subPlan.Each(func(p *atc.Plan) {
			mappedID := atc.PlanID(fmt.Sprintf("%s/%d/%d", delegate.planID, i, planIDCounter))
			mappedSubplanIDs[p.ID] = mappedID
			p.ID = mappedID
			planIDCounter++
		})

		subPlan.Each(func(p *atc.Plan) {
			// Ensure VersionFrom is mapped to the correct subplan within the
			// substep. Note that the VersionFrom plan ID can theoretically
			// reside outside of the substep, in which case no mapping is
			// necessary.
			if p.Get != nil && p.Get.VersionFrom != nil {
				if mappedID, ok := mappedSubplanIDs[*p.Get.VersionFrom]; ok {
					p.Get.VersionFrom = &mappedID
				}
			}
		})
		substeps[i] = atc.VarScopedPlan{
			Step:   subPlan,
			Values: values,
		}
		substepsPublic[i] = substeps[i].Public()
	}

	err := delegate.build.SaveEvent(event.AcrossSubsteps{
		Time: delegate.clock.Now().Unix(),
		Origin: event.Origin{
			ID: event.OriginID(delegate.planID),
		},
		Substeps: substepsPublic,
	})
	if err != nil {
		return nil, fmt.Errorf("save across substeps event: %w", err)
	}

	return substeps, nil
}

func (delegate *buildStepDelegate) checkImagePolicy(imageSource atc.Source, imageType string, privileged bool) error {
	if !delegate.policyChecker.ShouldCheckAction(policy.ActionUseImage) {
		return nil
	}

	redactedSource, err := delegate.redactImageSource(imageSource)
	if err != nil {
		return fmt.Errorf("redact source: %w", err)
	}

	return delegate.checkPolicy(policy.PolicyCheckInput{
		Action:   policy.ActionUseImage,
		Team:     delegate.build.TeamName(),
		Pipeline: delegate.build.PipelineName(),
		Data: map[string]interface{}{
			"image_type":   imageType,
			"image_source": redactedSource,
			"privileged":   privileged,
		},
	})
}

func (delegate *buildStepDelegate) checkPolicy(input policy.PolicyCheckInput) error {
	result, err := delegate.policyChecker.Check(input)
	if err != nil {
		return fmt.Errorf("policy check: %w", err)
	}

	if !result.Allowed() {
		policyCheckErr := policy.PolicyCheckNotPass{
			Messages: result.Messages(),
		}
		if result.ShouldBlock() {
			return policyCheckErr
		} else {
			fmt.Fprintf(delegate.Stderr(), "\x1b[1;33m%s\x1b[0m\n\n", policyCheckErr.Error())
			fmt.Fprintln(delegate.Stderr(), "\x1b[33mWARNING: unblocking from the policy check failure for soft enforcement\x1b[0m")
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
