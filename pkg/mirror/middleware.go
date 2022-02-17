package mirror

import (
	"context"
	"github.com/TierMobility/boring-registry/pkg/core"
	"io"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

const (
	// upstreamTimeout has to be shorter than terraform context timeout
	upstreamTimeout = 2 * time.Second
)

// Middleware is a Service middleware.
type Middleware func(Service) Service

type loggingMiddleware struct {
	next   Service
	logger log.Logger
}

func (mw loggingMiddleware) ListProviderVersions(ctx context.Context, provider core.Provider) (providerVersions *ProviderVersions, err error) {
	defer func(begin time.Time) {
		logger := level.Info(mw.logger)
		if err != nil {
			logger = level.Error(mw.logger)
		}

		_ = logger.Log(
			"op", "ListProviderVersions",
			"hostname", provider.Hostname,
			"namespace", provider.Namespace,
			"name", provider.Name,
			"took", time.Since(begin),
			"err", err,
		)

	}(time.Now())

	return mw.next.ListProviderVersions(ctx, provider)
}

func (mw loggingMiddleware) ListProviderInstallation(ctx context.Context, provider core.Provider) (archives *Archives, err error) {
	defer func(begin time.Time) {
		logger := level.Info(mw.logger)
		if err != nil {
			logger = level.Error(mw.logger)
		}

		_ = logger.Log(
			"op", "ListProviderInstallation",
			"hostname", provider.Hostname,
			"namespace", provider.Namespace,
			"name", provider.Name,
			"took", time.Since(begin),
			"err", err,
		)

	}(time.Now())

	return mw.next.ListProviderInstallation(ctx, provider)
}

func (mw loggingMiddleware) RetrieveProviderArchive(ctx context.Context, provider core.Provider) (_ io.Reader, err error) {
	defer func(begin time.Time) {
		logger := level.Info(mw.logger)
		if err != nil {
			logger = level.Error(mw.logger)
		}

		_ = logger.Log(
			"op", "RetrieveProviderArchive",
			"hostname", provider.Hostname,
			"namespace", provider.Namespace,
			"name", provider.Name,
			"version", provider.Version,
			"os", provider.OS,
			"arch", provider.Arch,
			"took", time.Since(begin),
			"err", err,
		)

	}(time.Now())

	return mw.next.RetrieveProviderArchive(ctx, provider)
}

func (mw loggingMiddleware) MirrorProvider(ctx context.Context, provider core.Provider, a, b, c io.Reader) (err error) {
	defer func(begin time.Time) {
		logger := level.Info(mw.logger)
		if err != nil {
			logger = level.Error(mw.logger)
		}

		_ = logger.Log(
			"op", "MirrorProvider",
			"hostname", provider.Hostname,
			"namespace", provider.Namespace,
			"name", provider.Name,
			"version", provider.Version,
			"os", provider.OS,
			"arch", provider.Arch,
			"took", time.Since(begin),
			"err", err,
		)

	}(time.Now())

	return mw.next.MirrorProvider(ctx, provider, a, b, c)
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
