package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Middleware interface {
	// WrapHandler wraps the given HTTP handler for instrumentation.
	WrapHandler(handler http.Handler) http.HandlerFunc
}

type middleware struct {
	metrics *HttpMetrics
}

// WrapHandler wraps the given HTTP handler for instrumentation:
// It reports HTTP metrics to the registered collectors.
// Each has a constant label named "handler" with the provided handlerName as value.
func (m *middleware) WrapHandler(handler http.Handler) http.HandlerFunc {
	wrappedHandler := promhttp.InstrumentHandlerCounter(
		m.metrics.RequestsTotal,
		promhttp.InstrumentHandlerDuration(
			m.metrics.RequestDuration,
			promhttp.InstrumentHandlerRequestSize(
				m.metrics.RequestSize,
				promhttp.InstrumentHandlerResponseSize(
					m.metrics.ResponseSize,
					handler,
				),
			),
		),
	)

	return wrappedHandler.ServeHTTP
}

// NewMiddleware returns a Middleware interface.
func NewMiddleware(metrics *HttpMetrics) Middleware {
	return &middleware{
		metrics: metrics,
	}
}
