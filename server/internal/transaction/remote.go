package transaction

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

type remoteClient struct {
	client *http.Client
}

func (c remoteClient) tryExecRemoteTransaction(ctx context.Context, t transaction) (*http.Response, error) {
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
