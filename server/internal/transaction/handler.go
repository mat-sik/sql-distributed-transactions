package transaction

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	commons "github.com/mat-sik/sql-distributed-transactions/common/transaction"
)

type EnqueueTransactionHandler struct {
	pool *sql.DB
}

func NewHandler(pool *sql.DB) EnqueueTransactionHandler {
	return EnqueueTransactionHandler{
		pool: pool,
	}
}

func (h EnqueueTransactionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req commons.EnqueueTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := commons.ValidRequest(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
	}

	ctx := context.Background()
	if err := enqueueTransaction(ctx, h.pool, createT); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
