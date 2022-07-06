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
			tokenValue := ctx.Value(jwt.JWTTokenContextKey)

			if token, ok := tokenValue.(string); ok {
				for _, provider := range providers {
					err := provider.Verify(ctx, token)
					if err != nil {
						level.Debug(logger).Log(
							"provider", provider,
							"msg", "failed to verify token",
							"err", err,
						)
					} else {
						level.Debug(logger).Log(
							"provider", provider,
							"msg", "successfully verified token",
							"err", err,
						)

						return next(ctx, request)
					}
				}
			}

			return nil, ErrUnauthorized
		}
	}
}
