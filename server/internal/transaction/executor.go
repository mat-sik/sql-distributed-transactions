package transaction

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/mat-sik/sql-distributed-transactions/server/internal/config"
	"log/slog"
	"math"
	"net/http"
	"sync"
	"time"
)

type Executor struct {
	pool         *sql.DB
	remoteClient remoteClient
	config       config.Executor
}

func NewExecutor(pool *sql.DB, client *http.Client, config config.Executor) Executor {
	return Executor{
		pool:         pool,
		remoteClient: remoteClient{client: client},
		config:       config,
	}
}

func (e Executor) Start(ctx context.Context) {
	slog.Info("starting the executor", "worker amount", e.config.WorkerAmount, "sender amount", e.config.SenderAmount, "batch size", e.config.BatchSize)

	wg := &sync.WaitGroup{}
	for i := 0; i < e.config.WorkerAmount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker := workerExecutor{
				pool:         e.pool,
				remoteClient: e.remoteClient,
				config:       e.config,
			}
			worker.start(ctx)
		}()
	}
	wg.Wait()
}

type workerExecutor struct {
	pool         *sql.DB
	remoteClient remoteClient
	config       config.Executor
}

func (e workerExecutor) start(ctx context.Context) {
	ticker := time.NewTicker(e.config.ExecuteTransactionInterval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := e.execTransactionBatch(ctx)
			if err != nil {
				slog.Error("encountered error while trying to execute a transaction", "error", err)
			}
		}
	}
}

func (e workerExecutor) execTransactionBatch(ctx context.Context) (err error) {
	tx, err := e.pool.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		err = handleTransactionFinalization(tx, err)
	}()

	transactions, err := fetchLockedTransactions(ctx, tx, e.config.BatchSize)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	return e.tryExecRemoteTransactions(ctx, tx, transactions)
}

func (e workerExecutor) tryExecRemoteTransactions(ctx context.Context, tx *sql.Tx, transactions []transaction) error {
	toSendCh := make(chan transaction, len(transactions))
	for _, t := range transactions {
		toSendCh <- t
	}
	close(toSendCh)

	wg := &sync.WaitGroup{}
	responsesCh := make(chan transactionResponse, len(transactions))
	for i := 0; i < e.config.SenderAmount; i++ {
		wg.Add(1)
		go e.execSender(ctx, wg, toSendCh, responsesCh)
	}
	wg.Wait()

	executedCount := 0
	for i := 0; i < len(transactions); i++ {
		select {
		case <-ctx.Done():
			slog.Warn("failed to execute all transactions because of the timeout", "executed", executedCount)
			return nil
		case tResp := <-responsesCh:
			if tResp.ID == 0 && tResp.StatusCode == 0 {
				slog.Warn("failed to execute a transaction because of the timeout", "id", tResp.ID)
				continue
			}
			if err := e.updateTransactionState(ctx, tx, tResp); err != nil {
				return err
			}
			executedCount++
		}
	}
	return nil
}

func (e workerExecutor) execSender(
	ctx context.Context,
	wg *sync.WaitGroup,
	toSendCh <-chan transaction,
	responsesCh chan<- transactionResponse,
) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case t, ok := <-toSendCh:
			if !ok {
				return
			}
			resp, err := e.tryExecRemoteTransaction(ctx, t)
			if err != nil {
				responsesCh <- transactionResponse{}
				return
			}
			responsesCh <- transactionResponse{
				ID:         t.ID,
				StatusCode: resp.StatusCode,
			}
		}
	}
}

func (e workerExecutor) tryExecRemoteTransaction(ctx context.Context, t transaction) (*http.Response, error) {
	timer := time.NewTicker(time.Second)
	for i := 0; ; i++ {
		resp, err := e.remoteClient.tryExecRemoteTransactionInstrumented(ctx, t)
		if err == nil {
			return resp, nil
		}

		reSendAfter := 2 * time.Duration(math.Pow(2, float64(i))) * 100 * time.Millisecond
		slog.Warn("failed to send transaction", "err", err, "transaction id", t.ID, "re-sending after", reSendAfter.String())

		timer.Reset(reSendAfter)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func (e workerExecutor) updateTransactionState(ctx context.Context, tx *sql.Tx, tResp transactionResponse) error {
	if tResp.StatusCode == http.StatusInternalServerError {
		return updateLockedTransactionState(ctx, tx, tResp.ID, RETRY)
	}
	return updateLockedTransactionState(ctx, tx, tResp.ID, DONE)
}

type transactionResponse struct {
	ID         int
	StatusCode int
}

func handleTransactionFinalization(tx *sql.Tx, err error) error {
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
