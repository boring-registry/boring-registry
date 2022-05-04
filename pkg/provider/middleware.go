package provider

import (
	"context"
	"fmt"
	"github.com/TierMobility/boring-registry/pkg/core"
	"time"

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

func (mw loggingMiddleware) ListProviderVersions(ctx context.Context, namespace, name string) (providers []core.ProviderVersion, err error) {
	defer func(begin time.Time) {
		logger := level.Info(mw.logger)
		if err != nil {
			logger = level.Error(mw.logger)
		}

		_ = logger.Log(
			"op", "ListProviderVersions",
			"namespace", namespace,
			"name", name,
			"took", time.Since(begin),
			"err", err,
		)

	}(time.Now())

	return mw.next.ListProviderVersions(ctx, namespace, name)
}

func (mw loggingMiddleware) GetProvider(ctx context.Context, namespace, name, version, os, arch string) (provider core.Provider, err error) {
	defer func(begin time.Time) {
		logger := level.Info(mw.logger)
		if err != nil {
			logger = level.Error(mw.logger)
		}

		_ = logger.Log(
			"op", "GetProvider",
			"provider", fmt.Sprintf("%s/%s/%s/%s/%s", namespace, name, version, os, arch),
			"took", time.Since(begin),
			"err", err,
		)

	}(time.Now())

	return mw.next.GetProvider(ctx, namespace, name, version, os, arch)
}
