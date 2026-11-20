package auth

import (
	"context"
	"log/slog"
)

// Caller identifies who made an authenticated request. It is attached to the
// request context once a token verifies, and only ever logged.
type Caller struct {
	Provider string
	Subject  string
	Username string
	Email    string
	Client   string
}

// Identifier is implemented by providers that can say who a verified token
// belongs to. Verify still decides access; Identify is called only afterwards.
type Identifier interface {
	Identify(ctx context.Context, token string) (Caller, error)
}

type callerKey struct{}

// WithCaller returns a copy of ctx carrying c.
func WithCaller(ctx context.Context, c Caller) context.Context {
	return context.WithValue(ctx, callerKey{}, c)
}

// CallerFromContext returns the caller attached to ctx, if any.
func CallerFromContext(ctx context.Context) (Caller, bool) {
	c, ok := ctx.Value(callerKey{}).(Caller)
	return c, ok
}

// LogAttr renders the context's caller as a "caller" log group. It returns an
// empty attribute, which slog drops, when the request carries no caller.
func LogAttr(ctx context.Context) slog.Attr {
	c, ok := CallerFromContext(ctx)
	if !ok {
		return slog.Attr{}
	}
	attrs := []any{slog.String("provider", c.Provider)}
	for _, kv := range [][2]string{
		{"sub", c.Subject},
		{"username", c.Username},
		{"email", c.Email},
		{"client", c.Client},
	} {
		if kv[1] != "" {
			attrs = append(attrs, slog.String(kv[0], kv[1]))
		}
	}
	return slog.Group("caller", attrs...)
}
