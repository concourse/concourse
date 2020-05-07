package exec

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/policy"
)

//go:generate counterfeiter . PolicyChecker

type PolicyChecker interface {
	Check(teamName, pipelineName, step string, imageSource atc.Source) (bool, error)
}

type checker struct {
	policyChecker *policy.Checker
}

func NewImagePolicyChecker(policyChecker *policy.Checker) PolicyChecker {
	if policyChecker == nil {
		return nil
	}
	return &checker{policyChecker: policyChecker}
}

func (c *checker) Check(teamName, pipelineName, step string, imageSource atc.Source) (bool, error) {

	// Actions in skip list will not go through policy check.
	if c.policyChecker.ShouldSkipAction(policy.ActionUsingImage) {
		return true, nil
	}

	imageInfo := map[string]string{
		"step": step,
	}

	if repository, ok := imageSource["repository"].(string); ok {
		imageInfo["repository"] = repository
	} else {
		// If imageSource doesn't have repository defined, then skip policy check.
		return true, nil
	}

	if tag, ok := imageSource["tag"].(string); ok {
		imageInfo["tag"] = tag
	} else {
		imageInfo["tag"] = "latest"
	}

	input := policy.PolicyCheckInput{
		Action:         policy.ActionUsingImage,
		Team:           teamName,
		Pipeline:       pipelineName,
		Data:           imageInfo,
	}

	return c.policyChecker.Check(input)
}
