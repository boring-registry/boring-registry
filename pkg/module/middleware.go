package module

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

// Middleware is a Service middleware.
type Middleware func(Service) Service

type loggingMiddleware struct {
	next   Service
	logger log.Logger
}

// LoggingMiddleware is a logging Service middleware.
func LoggingMiddleware(logger log.Logger) Middleware {
	return func(next Service) Service {
		return &loggingMiddleware{
			logger: logger,
			next:   next,
		}
	}
}

func (mw loggingMiddleware) ListModuleVersions(ctx context.Context, namespace, name, provider string) (modules []Module, err error) {
	defer func(begin time.Time) {
		logger := level.Info(mw.logger)
		if err != nil {
			logger = level.Error(mw.logger)
		}

		if len(modules) > 0 {
			_ = logger.Log(
				"op", "ListModuleVersions",
				"module", modules[0].ID(false),
				"took", time.Since(begin),
				"err", err,
			)
		}

	}(time.Now())

	return mw.next.ListModuleVersions(ctx, namespace, name, provider)
}

func (mw loggingMiddleware) GetModule(ctx context.Context, namespace, name, provider, version string) (module Module, err error) {
	defer func(begin time.Time) {
		logger := level.Info(mw.logger)
		if err != nil {
			logger = level.Error(mw.logger)
		}

		_ = logger.Log(
			"op", "GetModule",
			"module", module.ID(true),
			"took", time.Since(begin),
			"err", err,
		)

	}(time.Now())

	return mw.next.GetModule(ctx, namespace, name, provider, version)
}

// AuthMiddleware provides basic endpoint auth.
func AuthMiddleware(keys ...string) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			// If we didn't provide any API keys we stop early here.
			if len(keys) < 1 {
				return next(ctx, request)
			}

			found := false

			for _, key := range keys {
				if fmt.Sprintf("Bearer %s", key) == ctx.Value(headerAuthorization) {
					found = true
				}
			}

			if !found {
				return nil, fmt.Errorf("invalid key")
			}

			return next(ctx, request)
		}
	}
}
