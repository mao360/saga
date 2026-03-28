package observability

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

type Telemetry struct {
	Tracer     trace.Tracer
	Propagator propagation.TextMapPropagator
	Metrics    *Metrics
	shutdown   func(context.Context) error
}

func New(serviceName string, log *slog.Logger) (*Telemetry, error) {
	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(propagator)

	metrics := NewMetrics(prometheus.DefaultRegisterer)
	t := &Telemetry{
		Tracer:     otel.Tracer(serviceName),
		Propagator: propagator,
		Metrics:    metrics,
		shutdown:   func(context.Context) error { return nil },
	}

	endpoint := normalizeEndpoint(strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")))
	if endpoint == "" {
		if log != nil {
			log.Info("otel tracing exporter disabled (OTEL_EXPORTER_OTLP_ENDPOINT is empty)")
		}
		return t, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			attribute.String("service.namespace", "saga"),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	t.Tracer = otel.Tracer(serviceName)
	t.shutdown = tp.Shutdown

	if log != nil {
		log.Info("otel tracing exporter enabled", "endpoint", endpoint)
	}
	return t, nil
}

func (t *Telemetry) Shutdown(ctx context.Context) error {
	if t == nil || t.shutdown == nil {
		return nil
	}
	return t.shutdown(ctx)
}

func normalizeEndpoint(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "http://")
	raw = strings.TrimPrefix(raw, "https://")
	return raw
}
