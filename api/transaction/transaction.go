package transaction

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/mat-sik/sql-distributed-transactions/api/internal/config"
	commons "github.com/mat-sik/sql-distributed-transactions/common/transaction"
	"net/http"
)

type Client struct {
	client       *http.Client
	serverConfig config.Server
}

func NewClient(ctx context.Context, client *http.Client) Client {
	return Client{
		client:       client,
		serverConfig: config.NewServer(ctx),
	}
}

func (c Client) EnqueueTransaction(ctx context.Context, enqueueReq commons.EnqueueTransactionRequest) error {
	body, err := json.Marshal(enqueueReq)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/transactions/enqueue", c.serverConfig.URL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to enqueue transaction, received status code: %s", resp.Status)
	}
	return nil
}
