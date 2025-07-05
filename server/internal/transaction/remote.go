package transaction

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

type remoteClient struct {
	client *http.Client
}

func (c remoteClient) tryExecRemoteTransactionInstrumented(ctx context.Context, t transaction) (*http.Response, error) {
	start := time.Now()

	inFlightGauge.Inc()
	defer inFlightGauge.Dec()

	resp, err := c.tryExecRemoteTransaction(ctx, t)
	defer func() {
		duration.WithLabelValues(getStatusCode(resp, err)).Observe(time.Since(start).Seconds())
	}()

	counter.WithLabelValues(getStatusCode(resp, err)).Inc()

	return resp, err
}

func getStatusCode(resp *http.Response, err error) string {
	if err != nil {
		return "error"
	}
	if resp == nil {
		return "unknown"
	}
	return strconv.Itoa(resp.StatusCode)
}

func (c remoteClient) tryExecRemoteTransaction(ctx context.Context, t transaction) (*http.Response, error) {
	slog.Debug("executing remote transaction", "transaction", t)

	url := getUnsecureURL(t.Host, t.Path)

	var body io.Reader
	if t.Payload.Valid {
		body = bytes.NewBufferString(t.Payload.String)
	}

	req, err := http.NewRequestWithContext(ctx, t.Method, url, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.client.Do(req)
}

func getSecureURL(host string, path string) string {
	return fmt.Sprintf("https://%s%s", host, path)
}

func getUnsecureURL(host string, path string) string {
	return fmt.Sprintf("http://%s%s", host, path)
}
