package transaction

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/mat-sik/sql-distributed-transactions/server/internal/tracing"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"strings"
	"time"

	commons "github.com/mat-sik/sql-distributed-transactions/common/transaction"
)

type EnqueueTransactionHandler struct {
	tracer trace.Tracer
	pool   *sql.DB
}

func NewHandler(tracer trace.Tracer, pool *sql.DB) EnqueueTransactionHandler {
	return EnqueueTransactionHandler{
		tracer: tracer,
		pool:   pool,
	}
}

func (h EnqueueTransactionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	ctx, span := h.tracer.Start(ctx, "enqueueTransactionHandler")
	defer span.End()

	var req commons.EnqueueTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.SetStatus(codes.Error, "Failed to unmarshal the request body")
		span.RecordError(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := commons.ValidRequest(req); err != nil {
		span.SetStatus(codes.Error, "Failed to validate the request")
		span.RecordError(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	carrierJSON, err := tracing.MarshalContext(ctx)
	if err != nil {
		span.SetStatus(codes.Error, "Failed to marshal the trace context")
		span.RecordError(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	createT := createTransaction{
		Host:   req.Host,
		Path:   req.Path,
		Method: strings.ToUpper(req.Method),
		Payload: sql.NullString{
			String: req.Payload,
			Valid:  req.Payload != "",
		},
		carrierJSON: carrierJSON,
	}

	span.AddEvent("Trying to enqueue the transaction")
	if err = enqueueTransaction(ctx, h.pool, createT); err != nil {
		span.SetStatus(codes.Error, "Failed to enqueue the transaction")
		span.RecordError(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	span.AddEvent("Enqueued the transaction")
}
