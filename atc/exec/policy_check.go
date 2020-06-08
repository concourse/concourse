package exec

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/policy"
)

//go:generate counterfeiter . ImagePolicyChecker

type ImagePolicyChecker interface {
	Check(teamName, pipelineName, step string, imageSourceType string, imageSource atc.Source) (bool, error)
}

type checker struct {
	policyChecker *policy.Checker
}

func NewImagePolicyChecker(policyChecker *policy.Checker) ImagePolicyChecker {
	if policyChecker == nil {
		return nil
	}
	return &checker{policyChecker: policyChecker}
}

func (c *checker) Check(teamName, pipelineName, step string, imageSourceType string, imageSource atc.Source) (bool, error) {

	// Actions in skip list will not go through policy check.
	if c.policyChecker.ShouldSkipAction(policy.ActionUsingImage) {
		return true, nil
	}

	if _, ok := imageSource["password"]; ok {
		delete(imageSource, "password")
	}

	imageInfo := map[string]interface{}{
		"step":              step,
		"image_source_type": imageSourceType,
		"image_source":      imageSource,
	}

	input := policy.PolicyCheckInput{
		Action:   policy.ActionUsingImage,
		Team:     teamName,
		Pipeline: pipelineName,
		Data:     imageInfo,
	}

	return c.policyChecker.Check(input)
}
