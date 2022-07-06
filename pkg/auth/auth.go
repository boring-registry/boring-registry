package auth

import (
	"context"

	"github.com/go-kit/kit/auth/jwt"
	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

func Middleware(logger log.Logger, providers ...Provider) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			tokenRaw := ctx.Value(jwt.JWTTokenContextKey)
			if token, ok := tokenRaw.(string); ok {
				for _, provider := range providers {
					if err := provider.Verify(ctx, token); err != nil {
						level.Debug(logger).Log(
							"msg", "failed to verify token",
							"err", err,
						)
					} else {
						return next(ctx, request)
					}
				}
			}

			return nil, ErrUnauthorized
		}
	}
}
