package auth

import "context"

const SessionCookieName = "pc_camp_admin_session"

type actorKey struct{}

func WithActor(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, actorKey{}, name)
}

func ActorFrom(ctx context.Context) string {
	v, _ := ctx.Value(actorKey{}).(string)
	return v
}
