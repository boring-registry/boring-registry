package provider

import (
	"context"
	"log/slog"
	"time"

	"github.com/boring-registry/boring-registry/pkg/core"
)

// Middleware is a Service middleware.
type Middleware func(Service) Service

type loggingMiddleware struct {
	next Service
}

// LoggingMiddleware is a logging Service middleware.
func LoggingMiddleware() Middleware {
	return func(next Service) Service {
		return &loggingMiddleware{
			next: next,
		}
	}
}

func (mw loggingMiddleware) ListProviderVersions(ctx context.Context, namespace, name string) (versions *core.ProviderVersions, err error) {
	defer func(begin time.Time) {
		logger := slog.Default().With(
			slog.String("op", "ListProviderVersions"),
			slog.Group("provider",
				slog.String("namespace", namespace),
				slog.String("name", name),
			),
		)

		if err != nil {
			logger.Error("failed to list provider versions", slog.String("err", err.Error()))
			return
		}

		logger.Info("list provider version", slog.String("took", time.Since(begin).String()))
	}(time.Now())

	return mw.next.ListProviderVersions(ctx, namespace, name)
}

func (mw loggingMiddleware) GetProvider(ctx context.Context, namespace, name, version, os, arch string) (provider *core.Provider, err error) {
	defer func(begin time.Time) {
		logger := slog.Default().With(
			slog.String("op", "GetProvider"),
			slog.Group("provider",
				slog.String("namespace", namespace),
				slog.String("name", name),
				slog.String("version", version),
				slog.String("os", os),
				slog.String("arch", arch),
			),
		)

		if err != nil {
			logger.Error("failed to get provider", slog.String("err", err.Error()))
			return
		}

		logger.Info("get provider", slog.String("took", time.Since(begin).String()))
	}(time.Now())

	return mw.next.GetProvider(ctx, namespace, name, version, os, arch)
}
