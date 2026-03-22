package tracing

import (
	"context"
)

type Config struct {
	ServiceName  string
	Exporter     string
	Endpoint     string
	Enabled      bool
	SamplerType  string
	SamplerRatio float64
}

func DefaultConfig() Config {
	return Config{
		Enabled: false,
	}
}

func Configure(cfg Config) (func(), error) {
	return func() {}, nil
}

type KeyValue struct {
	Key   string
	Value interface{}
}

func String(key string, value string) KeyValue {
	return KeyValue{Key: key, Value: value}
}

func Int(key string, value int) KeyValue {
	return KeyValue{Key: key, Value: value}
}

func Bool(key string, value bool) KeyValue {
	return KeyValue{Key: key, Value: value}
}

func Int64(key string, value int64) KeyValue {
	return KeyValue{Key: key, Value: value}
}

type noopSpan struct{}

func (noopSpan) End()                            {}
func (noopSpan) RecordError(err error)           {}
func (noopSpan) SetStatus(code codes, msg string) {}
func (noopSpan) AddEvent(msg string)             {}
func (noopSpan) SetName(name string)             {}
func (noopSpan) SpanContext() SpanContext        { return SpanContext{} }
func (noopSpan) Parent() SpanContext             { return SpanContext{} }
func (noopSpan) SetAttributes(attrs ...KeyValue) {}
func (noopSpan) TracerProvider() TracerProvider  { return TracerProvider{} }

type SpanContext struct{}

func (SpanContext) HasTraceID() bool    { return false }
func (SpanContext) HasSpanID() bool     { return false }
func (SpanContext) TraceID() TraceID    { return TraceID{} }
func (SpanContext) SpanID() SpanID      { return SpanID{} }

type TraceID struct{}

func (TraceID) String() string { return "" }

type SpanID struct{}

func (SpanID) String() string { return "" }

type TracerProvider struct{}

func (TracerProvider) Tracer(name string) Tracer { return Tracer{} }

type Tracer struct{}

func (Tracer) Start(ctx context.Context, name string, opts ...SpanStartOption) (context.Context, Span) {
	return ctx, noopSpan{}
}

type Span interface {
	End()
	RecordError(err error)
	SetStatus(code codes, msg string)
	AddEvent(msg string)
	SetName(name string)
	SpanContext() SpanContext
	Parent() SpanContext
	SetAttributes(attrs ...KeyValue)
	TracerProvider() TracerProvider
}

type codes int

const (
	Unset codes = iota
	Error
	Ok
)

type SpanStartOption func(*spanConfig)

type spanConfig struct{}

type SpanEndOption func(*spanConfig)

func WithAttributes(attrs ...KeyValue) SpanStartOption {
	return func(c *spanConfig) {}
}

func GetTracer() interface{} {
	return noopSpan{}
}

func StartSpan(ctx context.Context, name string, opts ...SpanStartOption) (context.Context, Span) {
	return ctx, noopSpan{}
}

func EndSpan(span Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(Error, err.Error())
	}
	span.End()
}

func AddSpanAttributes(span Span, attrs ...KeyValue) {
	span.SetAttributes(attrs...)
}

func RecordSpanError(span Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(Error, err.Error())
	}
}

func SpanFromContext(ctx context.Context) Span {
	return noopSpan{}
}

func ContextWithSpan(ctx context.Context, span Span) context.Context {
	return ctx
}

func TraceIDFromSpan(span Span) string {
	return ""
}

func SpanIDFromSpan(span Span) string {
	return ""
}
