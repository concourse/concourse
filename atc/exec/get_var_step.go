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

type CacheEntry struct {
	value      interface{}
	expiration *time.Time
	found      bool
}

type VarNotFoundError struct {
	Source string
	Path   string
}

func (e VarNotFoundError) Error() string {
	return fmt.Sprintf("var %s:%s not found", e.Source, e.Path)
}

type LocalVarNotFound struct {
	Path string
}

func (e LocalVarNotFound) Error() string {
	return fmt.Sprintf("var %s not found in local variables", e.Path)
}

type GetVarStep struct {
	planID            atc.PlanID // TODO: not being used, maybe drop it
	plan              atc.GetVarPlan
	metadata          StepMetadata
	delegateFactory   BuildStepDelegateFactory
	lockFactory       lock.LockFactory
	cache             *gocache.Cache
	varSourcePool     creds.VarSourcePool
	secretCacheConfig creds.SecretCacheConfig
}

func NewGetVarStep(
	planID atc.PlanID,
	plan atc.GetVarPlan,
	metadata StepMetadata,
	delegateFactory BuildStepDelegateFactory, // XXX: not needed yet b/c no image fetching but WHATEVER
	cache *gocache.Cache,
	lockFactory lock.LockFactory,
	varSourcePool creds.VarSourcePool,
	secretCacheConfig creds.SecretCacheConfig,
) Step {
	return &GetVarStep{
		planID:            planID,
		plan:              plan,
		metadata:          metadata,
		delegateFactory:   delegateFactory,
		lockFactory:       lockFactory,
		cache:             cache,
		varSourcePool:     varSourcePool,
		secretCacheConfig: secretCacheConfig,
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

	hash, err := HashVarIdentifier(step.plan.Path, step.plan.Type, step.plan.Source, step.metadata.TeamID)
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

	varsRef := vars.Reference{
		Source: step.plan.Name,
		Path:   step.plan.Path,
		Fields: step.plan.Fields,
	}

	var value interface{}
	var found bool

	// Fetch from local variables if the source is "."
	if varsRef.Source == "." {
		value, found, err = state.LocalVariables().Get(varsRef)
		if err != nil {
			return false, fmt.Errorf("get var from build vars: %w", err)
		}

		if !found {
			return false, LocalVarNotFound{varsRef.Path}
		}
	} else {
		// Fetch from cache if source is populated and caching is enabled
		var entry interface{}
		if step.secretCacheConfig.Enabled {
			entry, found = step.cache.Get(hash)
			value = entry.(CacheEntry).value
		}

		// If the secret is not found in the cache or caching is not enabled, fetch
		// the var from the var source
		var expiration *time.Time
		if !found {
			value, expiration, found, err = step.runGetVar(state, delegate, varsRef, ctx, logger)
			if err != nil {
				return false, err
			}

			// Cache the resulting value if caching is enabled
			if step.secretCacheConfig.Enabled {
				// here we want to cache secret value, expiration, and found flag too
				// meaning that "secret not found" responses will be cached too!
				entry = CacheEntry{value: value, expiration: expiration, found: found}

				if found {
					// take default cache ttl
					duration := step.secretCacheConfig.Duration
					if expiration != nil {
						// if secret lease time expires sooner, make duration smaller than default duration
						itemDuration := expiration.Sub(time.Now())
						if itemDuration < duration {
							duration = itemDuration
						}
					}

					step.cache.Set(hash, entry, duration)
				} else {
					// cache secret not found
					step.cache.Set(hash, entry, step.secretCacheConfig.DurationNotFound)
				}
			}
		}
	}

	// Traverse the var to find the value of the field (if given)
	result, err := vars.Traverse(value, varsRef.String(), step.plan.Fields)
	if err != nil {
		return false, err
	}

	state.Track(varsRef, result)

	state.StoreResult(step.planID, result)
	delegate.Finished(logger, true)
	return true, nil
}

func (step *GetVarStep) runGetVar(state RunState, delegate BuildStepDelegate, ref vars.Reference, ctx context.Context, logger lager.Logger) (interface{}, *time.Time, bool, error) {
	// Loop over each var source and try to match a var source to the source
	// provided in the var
	varSourceConfig, found := state.VarSourceConfigs().Lookup(ref.Source)
	if !found {
		return nil, nil, false, vars.MissingSourceError{Name: ref.String(), Source: ref.Source}
	}

	// Grab out the manager factory for th
	factory := creds.ManagerFactories()[ref.Source]
	if factory == nil {
		return nil, nil, false, fmt.Errorf("unknown credential manager type: %s", ref.Source)
	}

	// Evaluate the var source's config. If the config of the var source has
	// templated vars then it will end up recursing to evaluate the var
	// source config's vars until it is able to evaluate a source that does
	// not have any templated vars or is evaluated using the global
	// credential manager.
	source, ok := varSourceConfig.Config.(map[string]interface{})
	if !ok {
		return nil, nil, false, fmt.Errorf("invalid source for %s", varSourceConfig.Name)
	}

	// Pass in a list of var source configs that don't include the var source
	// that we are currently trying to evaluate
	evaluatedConfig, err := creds.NewSource(delegate.Variables(ctx, state.VarSourceConfigs().Without(ref.Source)), source).Evaluate()
	if err != nil {
		return nil, nil, false, fmt.Errorf("evaluate: %w", err)
	}

	secrets, err := step.varSourcePool.FindOrCreate(logger, evaluatedConfig, factory)
	if err != nil {
		return nil, nil, false, fmt.Errorf("find or create var source: %w", err)
	}

	return step.lookupVarOnSecretPaths(secrets, ref, true)
}

func (step *GetVarStep) lookupVarOnSecretPaths(secrets creds.Secrets, ref vars.Reference, allowRootPath bool) (interface{}, *time.Time, bool, error) {
	lookupPaths := secrets.NewSecretLookupPaths(step.metadata.TeamName, step.metadata.PipelineName, allowRootPath)
	if len(lookupPaths) == 0 {
		// if no paths are specified (i.e. for fake & noop secret managers), then try 1-to-1 var->secret mapping
		return secrets.Get(ref.Path)
	}

	// try to find a secret according to our var->secret lookup paths
	for _, rule := range lookupPaths {
		// prepends any additional prefix paths to front of the path
		secretPath, err := rule.VariableToSecretPath(ref.Path)
		if err != nil {
			return nil, nil, false, err
		}

		result, expiration, found, err := secrets.Get(secretPath)
		if err != nil {
			return nil, nil, false, err
		}

		if !found {
			continue
		}

		return result, expiration, true, nil
	}

	return nil, nil, false, nil
}

func HashVarIdentifier(path, type_ string, source atc.Source, teamID int) (string, error) {
	varIdentifier, err := json.Marshal(struct {
		Path string `json:"path"`
		// TODO: Type might not be safe with prototypes, since the type is arbitrary
		Type   string     `json:"type"`
		Source atc.Source `json:"source"`
		TeamID int        `json:"team_id"`
	}{path, type_, source, teamID})
	if err != nil {
		return "", err
	}

	hasher := md5.New()
	hasher.Write([]byte(varIdentifier))
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
