package main

import (
	"context"
	"fmt"
	api "github.com/mat-sik/sql-distributed-transactions-api/transaction"
	commons "github.com/mat-sik/sql-distributed-transactions-common/transaction"
	"github.com/mat-sik/sql-distributed-transactions/client/internal/config"
	"log/slog"
	"math"
	"net/http"
	"os"
	"sync"
	"time"
)

func main() {
	ctx := context.Background()

	loggerConfig := config.NewLoggerConfig(ctx)
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: loggerConfig.Level,
	})

	logger := slog.New(jsonHandler)
	slog.SetDefault(logger)

	httpClient := &http.Client{}
	tClient := api.NewClient(ctx, httpClient)

	clientConfig := config.NewClientConfig(ctx)
	slog.Info("creating client", "config", clientConfig)

	toSendCh := make(chan commons.EnqueueTransactionRequest, clientConfig.ToSend)

	for i := 0; i < clientConfig.ToSend; i++ {
		req := commons.EnqueueTransactionRequest{
			Host:    clientConfig.DummyHost,
			Path:    fmt.Sprintf("/call/%d", i),
			Method:  "POST",
			Payload: fmt.Sprintf(`{"iteration": %d}`, i),
		}
		toSendCh <- req
	}
	close(toSendCh)

	wg := sync.WaitGroup{}

	for i := 0; i < clientConfig.WorkerCount; i++ {
		wg.Add(1)
		go sendAll(ctx, &wg, tClient, toSendCh)
	}

	wg.Wait()
}

func sendAll(ctx context.Context, wg *sync.WaitGroup, tClient api.Client, toSendCh chan commons.EnqueueTransactionRequest) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case req, ok := <-toSendCh:
			if !ok {
				return
			}
			i := 0.0
			for err := tClient.EnqueueTransaction(ctx, req); err != nil; {
				i := min(i, 5.0)
				sendAfter := 2 * time.Duration(math.Pow(2, i)) * 100 * time.Millisecond
				slog.Warn("failed to send transaction", "err", err, "payload", req.Payload, "re-sending after", sendAfter.String())
				time.Sleep(sendAfter)
				i++
			}
		}
	}
}
