package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

type Config struct {
	ServiceName  string
	Exporter     ExporterType
	Endpoint     string
	Enabled      bool
	SamplerType  SamplerType
	SamplerRatio float64
}

type ExporterType string

const (
	ExporterStdout ExporterType = "stdout"
	ExporterOTLP   ExporterType = "otlp"
)

type SamplerType string

const (
	SamplerAlways SamplerType = "always"
	SamplerNever  SamplerType = "never"
	SamplerRatio  SamplerType = "ratio"
	SamplerParent SamplerType = "parent"
)

var defaultConfig = Config{
	ServiceName:  "teradata-secret-plugin",
	Exporter:     ExporterStdout,
	Enabled:      true,
	SamplerType:  SamplerAlways,
	SamplerRatio: 1.0,
}

func DefaultConfig() Config {
	cfg := defaultConfig
	return cfg
}

func Configure(cfg Config) (func(), error) {
	if !cfg.Enabled {
		tracer = otel.Tracer(cfg.ServiceName)
		return func() {}, nil
	}

	if cfg.ServiceName == "" {
		cfg.ServiceName = defaultConfig.ServiceName
	}

	ctx := context.Background()

	exporter, err := createExporter(cfg.Exporter, cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	sampler := createSampler(cfg.SamplerType, cfg.SamplerRatio)

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tracer = tp.Tracer(cfg.ServiceName)

	return func() {
		ctx, cancel := context.WithTimeout(ctx, 5)
		defer cancel()
		_ = tp.Shutdown(ctx)
	}, nil
}

func createExporter(exporterType ExporterType, endpoint string) (sdktrace.SpanExporter, error) {
	switch exporterType {
	case ExporterStdout:
		return stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
		)
	case ExporterOTLP:
		return nil, fmt.Errorf("OTLP exporter not configured - requires OTLP endpoint")
	default:
		return stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
		)
	}
}

func createSampler(samplerType SamplerType, ratio float64) sdktrace.Sampler {
	switch samplerType {
	case SamplerAlways:
		return sdktrace.AlwaysSample()
	case SamplerNever:
		return sdktrace.NeverSample()
	case SamplerRatio:
		return sdktrace.TraceIDRatioBased(ratio)
	case SamplerParent:
		return sdktrace.ParentBased(
			sdktrace.AlwaysSample(),
			sdktrace.WithRemoteParentSampled(sdktrace.AlwaysSample()),
			sdktrace.WithRemoteParentNotSampled(sdktrace.NeverSample()),
			sdktrace.WithLocalParentSampled(sdktrace.AlwaysSample()),
			sdktrace.WithLocalParentNotSampled(sdktrace.NeverSample()),
		)
	default:
		return sdktrace.AlwaysSample()
	}
}

func Tracer() trace.Tracer {
	if tracer == nil {
		tracer = otel.Tracer("teradata-secret-plugin")
	}
	return tracer
}

func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name, opts...)
}

func WithAttributes(attrs ...attribute.KeyValue) trace.SpanStartOption {
	return trace.WithAttributes(attrs...)
}

func EndSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}

func AddSpanAttributes(span trace.Span, attrs ...attribute.KeyValue) {
	span.SetAttributes(attrs...)
}

func RecordSpanError(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

func ContextWithSpan(ctx context.Context, span trace.Span) context.Context {
	return trace.ContextWithSpan(ctx, span)
}

func TraceIDFromSpan(span trace.Span) string {
	if span.SpanContext().HasTraceID() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

func SpanIDFromSpan(span trace.Span) string {
	if span.SpanContext().HasSpanID() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}
