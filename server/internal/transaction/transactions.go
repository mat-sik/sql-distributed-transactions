package transaction

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/mat-sik/sql-distributed-transactions/server/internal/logging"
)

type Repository interface {
	beginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	finishTx(tx *sql.Tx, err error) error
	enqueueTransaction(ctx context.Context, createTransaction createTransaction) error
	fetchLockedTransactions(ctx context.Context, tx *sql.Tx, batchSize int) ([]transaction, error)
	updateLockedTransactionState(ctx context.Context, tx *sql.Tx, id int, state state) error
}

type SQLRepository struct {
	pool *sql.DB
}

func NewSQLRepository(pool *sql.DB) *SQLRepository {
	return &SQLRepository{
		pool: pool,
	}
}

func (r SQLRepository) beginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return r.pool.BeginTx(ctx, opts)
}

func (r SQLRepository) finishTx(tx *sql.Tx, err error) error {
	var txFinalizeErr error
	if p := recover(); p != nil {
		txFinalizeErr = tx.Rollback()
	} else if err != nil {
		txFinalizeErr = tx.Rollback()
	} else {
		txFinalizeErr = tx.Commit()
	}
	if err != nil && txFinalizeErr != nil {
		return fmt.Errorf("%v: %w", txFinalizeErr, err)
	} else if txFinalizeErr != nil {
		return txFinalizeErr
	}
	return err
}

func (r SQLRepository) fetchLockedTransactions(ctx context.Context, tx *sql.Tx, batchSize int) ([]transaction, error) {
	query := `
		SELECT id, host, path, method, payload, carrier_json
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
		if err = rows.Scan(&t.ID, &t.Host, &t.Path, &t.Method, &t.Payload, &t.CarrierJSON); err != nil {
			return nil, err
		}
		transactions = append(transactions, t)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return transactions, nil
}

func (r SQLRepository) updateLockedTransactionState(ctx context.Context, tx *sql.Tx, id int, state state) error {
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
	ID          int
	Host        string
	Path        string
	Method      string
	Payload     sql.NullString
	CarrierJSON string
}

func (r SQLRepository) enqueueTransaction(ctx context.Context, createTransaction createTransaction) error {
	query := `
		INSERT INTO transactions (host, path, method, payload, state, carrier_json) VALUES ($1, $2, $3, $4, $5, $6)
	`

	stmt, err := r.pool.PrepareContext(ctx, query)
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
		createTransaction.carrierJSON,
	)

	return err
}

type createTransaction struct {
	Host        string
	Path        string
	Method      string
	Payload     sql.NullString
	carrierJSON string
}

type state string

const (
	DONE    state = "DONE"
	PENDING state = "PENDING"
	RETRY   state = "RETRY"
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
		carrier_json TEXT NOT NULL,
    	PRIMARY KEY (id)
		)
	`
	_, err := pool.ExecContext(ctx, query)
	return err
}
