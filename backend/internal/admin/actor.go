package admin

import "context"

type actorKey struct{}

// WithActor stores the authenticated White Team member's display name on ctx.
func WithActor(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, actorKey{}, name)
}

// ActorFrom reads the actor name set by RequireAdmin, or "" if missing.
func ActorFrom(ctx context.Context) string {
	v, _ := ctx.Value(actorKey{}).(string)
	return v
}
