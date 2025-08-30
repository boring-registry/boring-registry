package auth

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/boring-registry/boring-registry/pkg/core"

	"github.com/go-kit/kit/auth/jwt"
	"github.com/go-kit/kit/endpoint"
)

func Middleware(providers ...Provider) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request any) (any, error) {
			tokenValue := ctx.Value(jwt.JWTContextKey)

			// Skip any authorization checks, as there are no providers defined
			if len(providers) == 0 {
				return next(ctx, request)
			}

			token, exists := tokenValue.(string)
			if !exists {
				return nil, fmt.Errorf("%w: request does not contain a token", core.ErrUnauthorized)
			}

			// Iterate through all providers and attempt to verify the token
			for _, provider := range providers {
				err := provider.Verify(ctx, token)
				if err != nil {
					slog.Debug("failed to verify token", slog.String("provider", provider.String()), slog.String("err", err.Error()))
					continue
				} else {
					slog.Debug("successfully verified token", slog.String("provider", provider.String()))
					return next(ctx, request)
				}
			}

			// No provider could verify the token
			return nil, core.ErrUnauthorized
		}
	}
}
