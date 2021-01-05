package exec

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/vars"
	gocache "github.com/patrickmn/go-cache"
)

type VarNotFoundError struct {
	Name string
}

func (e VarNotFoundError) Error() string {
	return fmt.Sprintf("var %s not found", e.Name)
}

type GetVarStep struct {
	planID          atc.PlanID // TODO: not being used, maybe drop it
	plan            atc.GetVarPlan
	metadata        StepMetadata
	delegateFactory BuildStepDelegateFactory
	lockFactory     lock.LockFactory
	cache           *gocache.Cache
}

func NewGetVarStep(
	planID atc.PlanID,
	plan atc.GetVarPlan,
	metadata StepMetadata,
	delegateFactory BuildStepDelegateFactory, // XXX: not needed yet b/c no image fetching but WHATEVER
	cache *gocache.Cache,
	lockFactory lock.LockFactory,
) Step {
	return &GetVarStep{
		planID:          planID,
		plan:            plan,
		metadata:        metadata,
		delegateFactory: delegateFactory,
		lockFactory:     lockFactory,
		cache:           cache,
	}
}

func (step *GetVarStep) Run(ctx context.Context, state RunState) (bool, error) {
	delegate := step.delegateFactory.BuildStepDelegate(state)
	ctx, span := delegate.StartSpan(ctx, "get_var", tracing.Attrs{
		"name": step.plan.Name,
	})

	ok, err := step.run(ctx, state, delegate)
	tracing.End(span, err)

	return ok, err
}

func (step *GetVarStep) run(ctx context.Context, state RunState, delegate BuildStepDelegate) (bool, error) {
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("get-var-step", lager.Data{
		"step-name": step.plan.Name,
		"job-id":    step.metadata.JobID,
	})

	delegate.Initializing(logger)

	stderr := delegate.Stderr()

	fmt.Fprintln(stderr, "\x1b[1;33mWARNING: the get_var step is experimental and subject to change!\x1b[0m")
	fmt.Fprintln(stderr, "")

	delegate.Starting(logger)

	hash, err := step.hashVarIdentifier(step.plan.Path, step.plan.Type, step.plan.Source)
	if err != nil {
		return false, fmt.Errorf("hash var identifier: %w", err)
	}

	for {
		var acquired bool
		lock, acquired, err := step.lockFactory.Acquire(logger, lock.NewGetVarStepLockID(step.metadata.BuildID, hash))
		if err != nil {
			return false, fmt.Errorf("acquire lock: %w", err)
		}

		if acquired {
			defer lock.Release()
			break
		}

		time.Sleep(time.Second)
	}

	_, found, err := state.Get(vars.Reference{
		Source: step.plan.Name,
		Path:   step.plan.Path,
	})
	if err != nil {
		return false, fmt.Errorf("get var from build vars: %w", err)
	}

	// If the var already exists in the builds vars, nothing needs to be done
	if found {
		delegate.Finished(logger, true)
		return true, nil
	}

	value, found := step.cache.Get(hash)

	// If the var exists within the cache, use the value in the cache
	if found {
		state.AddVar(step.plan.Name, step.plan.Path, value, !step.plan.Reveal)

		delegate.Finished(logger, true)
		return true, nil
	}

	manager, err := creds.ManagerFactories()[step.plan.Type].NewInstance(step.plan.Source)
	if err != nil {
		return false, fmt.Errorf("create manager: %w", err)
	}

	err = manager.Init(logger)
	if err != nil {
		return false, fmt.Errorf("init manager: %w", err)
	}

	defer manager.Close(logger)

	secretsFactory, err := manager.NewSecretsFactory(logger)
	if err != nil {
		return false, fmt.Errorf("create secrets factory: %w", err)
	}

	value, _, found, err = secretsFactory.NewSecrets().Get(step.plan.Path)
	if err != nil {
		return false, fmt.Errorf("create secrets factory: %w", err)
	}

	if !found {
		return false, VarNotFoundError{step.plan.Path}
	}

	step.cache.Add(hash, value, time.Second)

	state.AddVar(step.plan.Name, step.plan.Path, value, !step.plan.Reveal)

	delegate.Finished(logger, true)

	return true, nil
}

func (step *GetVarStep) hashVarIdentifier(path, type_ string, source atc.Source) (string, error) {
	varIdentifier, err := json.Marshal(struct {
		Path string `json:"path"`
		// TODO: Type might not be safe with prototypes, since the type is arbitrary
		Type   string     `json:"type"`
		Source atc.Source `json:"source"`
	}{path, type_, source})
	if err != nil {
		return "", err
	}

	hasher := md5.New()
	hasher.Write([]byte(varIdentifier))
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
