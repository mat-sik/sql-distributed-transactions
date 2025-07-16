package transaction

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/cenkalti/backoff/v5"
	"github.com/mat-sik/sql-distributed-transactions/server/internal/config"
	"github.com/mat-sik/sql-distributed-transactions/server/internal/tracing"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type Executor struct {
	tracer       trace.Tracer
	meter        metric.Meter
	pool         *sql.DB
	remoteClient remoteClient
	config       config.Executor
}

func NewExecutor(tracer trace.Tracer, meter metric.Meter, pool *sql.DB, client *http.Client, config config.Executor) Executor {
	return Executor{
		tracer:       tracer,
		meter:        meter,
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
				tracer:       e.tracer,
				meter:        e.meter,
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
	tracer       trace.Tracer
	meter        metric.Meter
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
	ctx, span := e.tracer.Start(ctx, "executeTransactionBatch")
	defer span.End()

	span.AddEvent("Trying to begin a sql transaction")
	tx, err := e.pool.BeginTx(ctx, nil)
	if err != nil {
		span.SetStatus(codes.Error, "Failed to begin a sql transaction")
		span.RecordError(err)
		return err
	}
	defer func() {
		span.AddEvent("Trying to finalize the sql transaction")
		err = handleTransactionFinalization(tx, err)
		if err != nil {
			span.SetStatus(codes.Error, "Failed to finalize the sql transaction")
			span.RecordError(err)
		}
	}()

	span.AddEvent("Trying to fetch locked transactions")
	transactions, err := fetchLockedTransactions(ctx, tx, e.config.BatchSize)
	if errors.Is(err, sql.ErrNoRows) {
		span.RecordError(err)
		return nil
	}
	if err != nil {
		span.SetStatus(codes.Error, "Encountered error while fetching the locked transactions")
		span.RecordError(err)
		return err
	}
	span.AddEvent("Fetched locked transactions", trace.WithAttributes(
		attribute.Int("transaction count", len(transactions)),
	))

	return e.tryExecRemoteTransactions(ctx, tx, transactions)
}

func (e workerExecutor) tryExecRemoteTransactions(ctx context.Context, tx *sql.Tx, transactions []transaction) error {
	ctx, span := e.tracer.Start(ctx, "tryExecRemoteTransactions")
	defer span.End()

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
	var err error
	for i := 0; i < len(transactions); i++ {
		select {
		case <-ctx.Done():
			err = errors.New("failed to execute each one of transactions because of the timeout or cancellation")
			span.SetStatus(codes.Error, "Failed to execute all transactions")
			span.RecordError(err, trace.WithAttributes(
				attribute.Int("executed count", executedCount),
			))
			return nil
		case tResp := <-responsesCh:
			executedCount, err = e.handleTransactionResponse(ctx, tx, tResp, executedCount)
			if err != nil {
				span.SetStatus(codes.Error, "Failed to execute all transactions")
				span.RecordError(err, trace.WithAttributes(
					attribute.Int("executed count", executedCount),
				))
				return err
			}
		}
	}
	return nil
}

func (e workerExecutor) handleTransactionResponse(ctx context.Context, tx *sql.Tx, tResp transactionResponse, executedCount int) (int, error) {
	ctx = otel.GetTextMapPropagator().Extract(ctx, tResp.carrier)

	ctx, span := e.tracer.Start(ctx, "handleTransactionResponse")
	defer span.End()

	if tResp.ID == 0 && tResp.StatusCode == 0 {
		span.AddEvent("Failed to execute a transaction because of the timeout", trace.WithAttributes(
			attribute.Int("executed count", executedCount),
			attribute.Int("transaction id", tResp.ID),
		))
		return executedCount, nil
	}

	span.AddEvent("Trying to update transaction state")
	if err := e.updateTransactionState(ctx, tx, tResp); err != nil {
		span.SetStatus(codes.Error, "Failed to update a transaction state")
		span.RecordError(err, trace.WithAttributes(
			attribute.Int("transaction id", tResp.ID),
			attribute.Int("transaction result status code", tResp.StatusCode),
		))
		return executedCount, err
	}

	executedCount++
	span.AddEvent("Updated the transaction state", trace.WithAttributes(
		attribute.Int("executed count", executedCount),
	))
	return executedCount, nil
}

func (e workerExecutor) execSender(
	ctx context.Context,
	wg *sync.WaitGroup,
	toSendCh <-chan transaction,
	responsesCh chan<- transactionResponse,
) {
	ctx, span := e.tracer.Start(ctx, "execSender")
	defer span.End()

	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			err := errors.New("failed to execute every transaction because of the timeout or cancellation")
			span.SetStatus(codes.Error, "Failed to execute all transactions")
			span.RecordError(err)
			return
		case t, ok := <-toSendCh:
			if !ok {
				span.AddEvent("All transactions in the batch has been sent")
				return
			}

			tResp, err := e.handleTransaction(ctx, t)
			if err != nil {
				span.SetStatus(codes.Error, "Failed to handle the transaction")
				span.RecordError(err, trace.WithAttributes(
					attribute.Int("transaction id", t.ID),
				))
				return
			}
			responsesCh <- tResp
		}
	}
}

func (e workerExecutor) handleTransaction(ctx context.Context, t transaction) (transactionResponse, error) {
	ctx, span := e.tracer.Start(ctx, "handleTransaction")
	defer span.End()

	storedCtx, err := tracing.UnmarshalContext(ctx, t.CarrierJSON)
	if err != nil {
		span.SetStatus(codes.Error, "Failed to unmarshal the trace context")
		span.RecordError(err)
		return transactionResponse{}, err
	}

	spanLinkOption := trace.WithLinks(
		trace.Link{
			SpanContext: trace.SpanContextFromContext(ctx),
		},
	)

	return e.handleTransactionWithPropagatedContext(storedCtx, spanLinkOption, t)
}

func (e workerExecutor) handleTransactionWithPropagatedContext(
	ctx context.Context,
	option trace.SpanStartOption,
	t transaction,
) (transactionResponse, error) {
	ctx, span := e.tracer.Start(ctx, "handleTransactionWithPropagatedContext", option)
	defer span.End()

	span.AddEvent("Trying to execute a remote transaction", trace.WithAttributes(
		attribute.Int("transaction id", t.ID),
	))
	resp, err := e.tryExecRemoteTransaction(ctx, t)
	if err != nil {
		return transactionResponse{}, err
	}
	span.AddEvent("Executed the remote transaction", trace.WithAttributes(
		attribute.Int("response status", resp.StatusCode),
	))

	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	tResp := transactionResponse{
		ID:         t.ID,
		StatusCode: resp.StatusCode,
		carrier:    carrier,
	}
	return tResp, nil
}

func (e workerExecutor) tryExecRemoteTransaction(ctx context.Context, t transaction) (*http.Response, error) {
	operation := func() (*http.Response, error) {
		return e.remoteClient.tryExecRemoteTransaction(ctx, t)
	}
	return backoff.Retry(ctx, operation, backoff.WithBackOff(backoff.NewExponentialBackOff()))
}

func (e workerExecutor) updateTransactionState(ctx context.Context, tx *sql.Tx, tResp transactionResponse) error {
	ctx, span := e.tracer.Start(ctx, "updateTransactionState")
	defer span.End()

	newState := DONE
	if tResp.StatusCode == http.StatusInternalServerError {
		newState = RETRY
	}

	span.AddEvent("Trying to update the transaction state", trace.WithAttributes(
		attribute.String("new state", string(newState)),
	))

	err := updateLockedTransactionState(ctx, tx, tResp.ID, newState)
	if err != nil {
		span.SetStatus(codes.Error, "Failed to update the transaction state")
		span.RecordError(err)
		return err
	}
	return nil
}

type transactionResponse struct {
	ID         int
	StatusCode int
	carrier    propagation.MapCarrier
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
