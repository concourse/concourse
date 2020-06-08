package policy

import (
	"context"
)

func RecordTeam(ctx context.Context, teamName string) context.Context {
	return context.WithValue(ctx, teamContextKey{}, teamName)
}

func TeamFromContext(ctx context.Context) string {
	t, ok := ctx.Value(teamContextKey{}).(string)
	if !ok {
		return ""
	}
	return t
}

type teamContextKey struct{}
