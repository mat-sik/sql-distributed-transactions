package transaction

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/mat-sik/sql-distributed-transactions/server/internal/tracing"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"strings"
	"time"

	commons "github.com/mat-sik/sql-distributed-transactions/common/transaction"
)

type EnqueueTransactionHandler struct {
	tracer     trace.Tracer
	repository Repository
}

func NewHandler(tracer trace.Tracer, repository Repository) EnqueueTransactionHandler {
	return EnqueueTransactionHandler{
		tracer:     tracer,
		repository: repository,
	}
}

func (h EnqueueTransactionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	ctx, span := h.tracer.Start(ctx, "enqueueTransactionHandler")
	defer span.End()

	var req commons.EnqueueTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handleErr(span, w, err, http.StatusInternalServerError, "Failed to unmarshal the request body")
		return
	}

	if err := commons.ValidRequest(req); err != nil {
		handleErr(span, w, err, http.StatusInternalServerError, "Failed to validate the request")
		return
	}

	carrierJSON, err := tracing.MarshalContext(ctx)
	if err != nil {
		handleErr(span, w, err, http.StatusInternalServerError, "Failed to marshal the trace context")
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
	if err = h.repository.enqueueTransaction(ctx, createT); err != nil {
		handleErr(span, w, err, http.StatusInternalServerError, "Failed to enqueue the transaction")
		return
	}

	span.AddEvent("Enqueued the transaction")
}

func handleErr(span trace.Span, w http.ResponseWriter, err error, code int, description string, options ...trace.EventOption) {
	tracing.RecordErr(span, err, description, options...)
	http.Error(w, err.Error(), code)
}
