package transaction

import (
	"context"
	"database/sql"
	"github.com/mat-sik/sql-distributed-transactions/server/internal/logging"
	"log/slog"
)

func CreateTransactionsTableIfNotExist(ctx context.Context, pool *sql.DB) error {
	query := `
		CREATE TABLE IF NOT EXISTS transactions (
    	id BIGSERIAL NOT NULL,
    	host TEXT NOT NULL,
    	path TEXT NOT NULL,
    	method TEXT NOT NULL,
    	payload TEXT NULL,
    	state TEXT NOT NULL,
    	created_at TIMESTAMP DEFAULT now(),
    	PRIMARY KEY (id)
		)
	`
	_, err := pool.ExecContext(ctx, query)
	return err
}

func fetchLockedTransactions(ctx context.Context, tx *sql.Tx, batchSize int) ([]transaction, error) {
	query := `
		SELECT id, host, path, method, payload
		FROM transactions
		WHERE state != 'DONE'
		ORDER BY id
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer logging.LoggedClose(stmt)

	rows, err := stmt.QueryContext(ctx, batchSize)
	if err != nil {
		return nil, err
	}
	defer logging.LoggedClose(rows)

	var transactions []transaction
	for rows.Next() {
		var t transaction
		if err = rows.Scan(&t.ID, &t.Host, &t.Path, &t.Method, &t.Payload); err != nil {
			return nil, err
		}
		transactions = append(transactions, t)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	slog.Debug("fetched n transactions", "n", len(transactions))

	return transactions, nil
}

func updateLockedTransactionState(ctx context.Context, tx *sql.Tx, id int, state state) error {
	slog.Debug("updating transaction state", "id", id, "state", state)

	query := `
		UPDATE transactions SET state = $2 WHERE id = $1
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	defer logging.LoggedClose(stmt)

	_, err = stmt.ExecContext(ctx, id, state)
	return err
}

type transaction struct {
	ID      int
	Host    string
	Path    string
	Method  string
	Payload sql.NullString
}

func enqueueTransaction(ctx context.Context, pool *sql.DB, createTransaction createTransaction) error {
	slog.Debug("trying to enqueue transaction", "transaction", createTransaction)

	query := `
		INSERT INTO transactions (host, path, method, payload, state) VALUES ($1, $2, $3, $4, $5)
	`

	stmt, err := pool.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	defer logging.LoggedClose(stmt)

	_, err = stmt.ExecContext(
		ctx,
		createTransaction.Host,
		createTransaction.Path,
		createTransaction.Method,
		createTransaction.Payload,
		PENDING,
	)

	return err
}

type createTransaction struct {
	Host    string
	Path    string
	Method  string
	Payload sql.NullString
}

type state string

const (
	DONE    state = "DONE"
	PENDING state = "PENDING"
	RETRY   state = "RETRY"
)
