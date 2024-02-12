package mirror

import (
	"context"
	"log/slog"
	"time"

	"github.com/boring-registry/boring-registry/pkg/core"
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
	next Service
}

func (mw loggingMiddleware) ListProviderVersions(ctx context.Context, provider *core.Provider) (providerVersions *ListProviderVersionsResponse, err error) {
	defer func(begin time.Time) {
		logger := slog.Default().With(
			slog.String("op", "ListProviderVersions"),
			slog.Group("provider",
				slog.String("hostname", provider.Hostname),
				slog.String("namespace", provider.Namespace),
				slog.String("name", provider.Name),
			),
		)

		if err != nil {
			logger.Error("failed to list provider versions", slog.String("err", err.Error()))
			return
		}

		logger.Info("list provider version", slog.String("took", time.Since(begin).String()), slog.Bool("mirror", providerVersions.fromMirror()))
	}(time.Now())

	return mw.next.ListProviderVersions(ctx, provider)
}

func (mw loggingMiddleware) ListProviderInstallation(ctx context.Context, provider *core.Provider) (archives *ListProviderInstallationResponse, err error) {
	defer func(begin time.Time) {
		logger := slog.Default().With(
			slog.String("op", "ListProviderInstallation"),
			slog.Group("provider",
				slog.String("hostname", provider.Hostname),
				slog.String("namespace", provider.Namespace),
				slog.String("name", provider.Name),
			),
		)

		if err != nil {
			logger.Error("failed to list provider installation", slog.String("err", err.Error()))
			return
		}

		logger.Info("list provider installation", slog.String("took", time.Since(begin).String()), slog.Bool("mirror", archives.fromMirror()))
	}(time.Now())

	return mw.next.ListProviderInstallation(ctx, provider)
}

func (mw loggingMiddleware) RetrieveProviderArchive(ctx context.Context, provider *core.Provider) (response *retrieveProviderArchiveResponse, err error) {
	defer func(begin time.Time) {
		logger := slog.Default().With(
			slog.String("op", "RetrieveProviderArchive"),
			slog.Group("provider",
				slog.String("hostname", provider.Hostname),
				slog.String("namespace", provider.Namespace),
				slog.String("name", provider.Name),
				slog.String("version", provider.Version),
				slog.String("os", provider.OS),
				slog.String("arch", provider.Arch),
			),
		)

		if err != nil {
			logger.Error("failed to retrieve provider archive", slog.String("err", err.Error()))
			return
		}

		logger.Info("retrieve provider archive", slog.String("took", time.Since(begin).String()), slog.Bool("mirror", response.fromMirror()))
	}(time.Now())

	return mw.next.RetrieveProviderArchive(ctx, provider)
}

// LoggingMiddleware is a logging Service middleware.
func LoggingMiddleware() Middleware {
	return func(next Service) Service {
		return &loggingMiddleware{
			next: next,
		}
	}
}
