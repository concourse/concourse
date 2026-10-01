package flaghelpers

import (
	"errors"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/vars"
)

func ApplyInstanceVars(pipelineRef *atc.PipelineRef, pairs []YAMLVariablePairFlag) error {
	if len(pairs) == 0 {
		return nil
	}

	if len(pipelineRef.InstanceVars) > 0 {
		return errors.New("instance vars specified both in pipeline name and via --instance-var")
	}

	pipelineRef.InstanceVars = InstanceVarsFromPairs(pairs)
	return nil
}

func InstanceVarsFromPairs(pairs []YAMLVariablePairFlag) atc.InstanceVars {
	if len(pairs) == 0 {
		return nil
	}

	var kvPairs vars.KVPairs
	for _, pair := range pairs {
		kvPairs = append(kvPairs, vars.KVPair(pair))
	}
	return atc.InstanceVars(kvPairs.Expand())
}
