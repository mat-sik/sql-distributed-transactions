package tracing

import (
	"context"
	"encoding/json"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

func MarshalContext(ctx context.Context) (string, error) {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	carrierJSON, err := json.Marshal(carrier)
	if err != nil {
		return "", err
	}

	return string(carrierJSON), nil
}

func UnmarshalContext(ctx context.Context, carrierJSON string) (context.Context, error) {
	var carrier propagation.MapCarrier
	err := json.Unmarshal([]byte(carrierJSON), &carrier)
	if err != nil {
		return nil, err
	}

	ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
	return ctx, nil
}
