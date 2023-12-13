package auth

import (
	"context"
	"fmt"

	"github.com/TierMobility/boring-registry/pkg/core"

	"github.com/go-kit/kit/auth/jwt"
	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

func Middleware(logger log.Logger, providers ...Provider) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			tokenValue := ctx.Value(jwt.JWTContextKey)

			// Skip any authorization checks, as there are no providers defined
			if len(providers) == 0 {
				return next(ctx, request)
			}

			if token, ok := tokenValue.(string); ok {
				for _, provider := range providers {
					err := provider.Verify(ctx, token)
					if err != nil {
						_ = level.Debug(logger).Log(
							"provider", provider,
							"msg", "failed to verify token",
							"err", err,
						)
						return nil, fmt.Errorf("failed to verify token: %w", err)
					} else {
						_ = level.Debug(logger).Log(
							"provider", provider,
							"msg", "successfully verified token",
							"err", err,
						)

						return next(ctx, request)
					}
				}
			} else {
				return nil, fmt.Errorf("%w: request does not contain a token", core.ErrUnauthorized)
			}

			return nil, core.ErrUnauthorized
		}
	}
}
