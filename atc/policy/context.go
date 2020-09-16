package policy

import (
	"context"
)

func RecordTeamAndPipeline(ctx context.Context, teamName, pipelineName string) context.Context {
	newCtx := context.WithValue(ctx, teamContextKey{}, teamName)
	return context.WithValue(newCtx, pipelineContextKey{}, pipelineName)
}

func TeamAndPipelineFromContext(ctx context.Context) (string, string) {
	t, ok := ctx.Value(teamContextKey{}).(string)
	if !ok {
		t = ""
	}
	p, ok := ctx.Value(pipelineContextKey{}).(string)
	if !ok {
		p = ""
	}
	return t, p
}

type teamContextKey struct{}
type pipelineContextKey struct{}
