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
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			tokenValue := ctx.Value(jwt.JWTContextKey)

			// Skip any authorization checks, as there are no providers defined
			if len(providers) == 0 {
				return next(ctx, request)
			}

			if token, ok := tokenValue.(string); ok {
			    var lastError error
				for _, provider := range providers {
					err := provider.Verify(ctx, token)
					if err != nil {
						slog.Debug("failed to verify token", slog.String("err", err.Error()))
						lastError = err
						continue
					} else {
						slog.Debug("successfully verified token")
						return next(ctx, request)
					}
				}
				return nil, fmt.Errorf("failed to verify token: %w", lastError)
			} else {
				return nil, fmt.Errorf("%w: request does not contain a token", core.ErrUnauthorized)
			}

			return nil, core.ErrUnauthorized
		}
	}
}
