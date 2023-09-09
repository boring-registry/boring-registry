package mirror

import (
	"context"
	"time"

	"github.com/TierMobility/boring-registry/pkg/core"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

type mirrorSource struct {
	isMirror bool
}

func (m *mirrorSource) fromMirror() bool {
	return m.isMirror
}

// Middleware is a Service middleware.
type Middleware func(Service) Service

type loggingMiddleware struct {
	next   Service
	logger log.Logger
}

func (mw loggingMiddleware) ListProviderVersions(ctx context.Context, provider *core.Provider) (providerVersions *ProviderVersions, err error) {
	defer func(begin time.Time) {
		l := []interface{}{
			"op", "ListProviderVersions",
			"hostname", provider.Hostname,
			"namespace", provider.Namespace,
			"name", provider.Name,
			"took", time.Since(begin),
		}
		logger := level.Info(mw.logger)
		if err != nil {
			logger = level.Error(mw.logger)
			l = append(l, "err", err)
		} else {
			l = append(l, "mirror", providerVersions.fromMirror)
		}

		_ = logger.Log(l...)
	}(time.Now())

	return mw.next.ListProviderVersions(ctx, provider)
}

func (mw loggingMiddleware) ListProviderInstallation(ctx context.Context, provider *core.Provider) (archives *Archives, err error) {
	defer func(begin time.Time) {
		l := []interface{}{
			"op", "ListProviderInstallation",
			"hostname", provider.Hostname,
			"namespace", provider.Namespace,
			"name", provider.Name,
			"took", time.Since(begin),
		}
		logger := level.Info(mw.logger)
		if err != nil {
			logger = level.Error(mw.logger)
			l = append(l, "err", err)
		} else {
			l = append(l, "mirror", archives.fromMirror)
		}

		_ = logger.Log(l...)
	}(time.Now())

	return mw.next.ListProviderInstallation(ctx, provider)
}

func (mw loggingMiddleware) RetrieveProviderArchive(ctx context.Context, provider *core.Provider) (response *retrieveProviderArchiveResponse, err error) {
	defer func(begin time.Time) {
		l := []interface{}{
			"op", "RetrieveProviderArchive",
			"hostname", provider.Hostname,
			"namespace", provider.Namespace,
			"name", provider.Name,
			"version", provider.Version,
			"os", provider.OS,
			"arch", provider.Arch,
			"took", time.Since(begin),
		}
		logger := level.Info(mw.logger)
		if err != nil {
			logger = level.Error(mw.logger)
			l = append(l, "err", err)
		} else {
			l = append(l, "mirror", response.fromMirror())
		}

		_ = logger.Log(l...)
	}(time.Now())

	return mw.next.RetrieveProviderArchive(ctx, provider)
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
