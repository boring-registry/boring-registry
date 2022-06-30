package auth

import (
	"context"
	"errors"

	"github.com/go-kit/kit/auth/jwt"
	"github.com/go-kit/kit/endpoint"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
)

func Middleware(providers ...Provider) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			token, ok := ctx.Value(jwt.JWTTokenContextKey).(string)
			if ok {
				for _, provider := range providers {
					if err := provider.Verify(ctx, token); err == nil {
						return next(ctx, request)
					}
				}
			}

			return nil, ErrUnauthorized
		}
	}
}
