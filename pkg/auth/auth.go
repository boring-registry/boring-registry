package auth

import (
	"context"
	"fmt"

	"github.com/go-kit/kit/endpoint"
	httptransport "github.com/go-kit/kit/transport/http"
)

// Middleware provides basic endpoint auth.
func Middleware(keys ...string) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			// If we didn't provide any API keys we stop early here.
			if len(keys) < 1 {
				return next(ctx, request)
			}

			found := false
			for _, key := range keys {
				key := fmt.Sprintf("Bearer %s", key)
				if key == ctx.Value(httptransport.ContextKeyRequestAuthorization) {
					found = true
				}
			}

			if !found {
				return nil, ErrInvalidKey
			}

			return next(ctx, request)
		}
	}
}
