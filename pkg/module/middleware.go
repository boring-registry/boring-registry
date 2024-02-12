package module

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

func (mw loggingMiddleware) ListModuleVersions(ctx context.Context, namespace, name, provider string) (modules []core.Module, err error) {
	defer func(begin time.Time) {
		logger := slog.Default().With(
			slog.String("op", "ListModuleVersions"),
			slog.Group("module",
				slog.String("namespace", namespace),
				slog.String("name", name),
				slog.String("provider", provider),
			),
		)
		if err != nil {
			logger.Error("failed to list module", slog.String("err", err.Error()))
			return
		}

		logger.Info("list module version", slog.String("took", time.Since(begin).String()))
	}(time.Now())

	return mw.next.ListModuleVersions(ctx, namespace, name, provider)
}

func (mw loggingMiddleware) GetModule(ctx context.Context, namespace, name, provider, version string) (module core.Module, err error) {
	defer func(begin time.Time) {
		logger := slog.Default().With(
			slog.String("op", "GetModule"),
			slog.Group("module",
				slog.String("namespace", namespace),
				slog.String("name", name),
				slog.String("provider", provider),
			),
		)

		if err != nil {
			logger.Error("failed to get module", slog.String("err", err.Error()))
			return
		}

		logger.Info("get module", slog.String("took", time.Since(begin).String()), slog.String("module", module.ID(true)))
	}(time.Now())

	return mw.next.GetModule(ctx, namespace, name, provider, version)
}
