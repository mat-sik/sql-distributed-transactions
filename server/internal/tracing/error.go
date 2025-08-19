package tracing

import (
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func RecordErr(span trace.Span, err error, description string, options ...trace.EventOption) {
	span.SetStatus(codes.Error, description)
	span.RecordError(err, options...)
}
