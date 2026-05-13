// Package obs sets up observability: structured logging, distributed tracing, and metrics.
package obs

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

const serviceName = "job-discovery"

// Obs bundles all observability handles used across the service.
type Obs struct {
	Logger   *slog.Logger
	Tracer   trace.Tracer
	Registry *prometheus.Registry

	HTTPRequestsTotal       *prometheus.CounterVec
	HTTPRequestDuration     *prometheus.HistogramVec
	NATSPublishedTotal      *prometheus.CounterVec
	ListingsDiscoveredTotal *prometheus.CounterVec
}

// New initialises slog, OTel, and Prometheus. The returned shutdown function
// must be called on service exit to flush spans.
func New(ctx context.Context, logLevel string) (*Obs, func(), error) {
	logger := buildLogger(logLevel)

	tracer, shutdown, err := buildTracer(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("building tracer: %w", err)
	}

	reg := prometheus.NewRegistry()
	factory := promauto.With(reg)

	o := &Obs{
		Logger:   logger,
		Tracer:   tracer,
		Registry: reg,

		HTTPRequestsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests by route, method, and status code.",
		}, []string{"route", "method", "status"}),

		HTTPRequestDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency by route and method.",
			Buckets: prometheus.DefBuckets,
		}, []string{"route", "method"}),

		NATSPublishedTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "nats_messages_published_total",
			Help: "Total NATS JetStream messages published by subject.",
		}, []string{"subject"}),

		ListingsDiscoveredTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "listings_discovered_total",
			Help: "Total listings processed during refresh, by result.",
		}, []string{"result"}),
	}

	return o, shutdown, nil
}

func buildLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	return slog.New(h).With(slog.String("service", serviceName))
}

func buildTracer(ctx context.Context) (trace.Tracer, func(), error) {
	endpoint, ok := os.LookupEnv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if !ok || endpoint == "" {
		// No endpoint configured — use a noop tracer.
		return noop.NewTracerProvider().Tracer(serviceName), func() {}, nil
	}

	exp, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("creating OTLP HTTP exporter: %w", err)
	}

	res, err := resource.New(
		ctx,
		resource.WithAttributes(semconv.ServiceNameKey.String(serviceName)),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("creating OTel resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)

	shutdown := func() {
		_ = tp.Shutdown(ctx)
	}

	return tp.Tracer(serviceName), shutdown, nil
}
