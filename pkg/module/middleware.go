package module

import (
	"context"
	"time"

	"github.com/boring-registry/boring-registry/pkg/core"

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

func (mw loggingMiddleware) ListModuleVersions(ctx context.Context, namespace, name, provider string) (modules []core.Module, err error) {
	defer func(begin time.Time) {
		logger := level.Info(mw.logger)
		if err != nil {
			logger = level.Error(mw.logger)
		}

		_ = logger.Log(
			"op", "ListModuleVersions",
			"namespace", namespace,
			"name", name,
			"provider", provider,
			"took", time.Since(begin),
			"err", err,
		)

	}(time.Now())

	return mw.next.ListModuleVersions(ctx, namespace, name, provider)
}

func (mw loggingMiddleware) GetModule(ctx context.Context, namespace, name, provider, version string) (module core.Module, err error) {
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
