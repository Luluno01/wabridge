package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"wabridge/internal/action"
)

// APIClient implements action.Backend by making HTTP requests to the bridge's
// REST API. It is used in MCP mode where the MCP server reads SQLite directly
// for queries but delegates actions (send, download, sync) to the bridge.
type APIClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewAPIClient creates an APIClient targeting the given base URL
// (e.g. "http://localhost:8080").
func NewAPIClient(baseURL string) *APIClient {
	return &APIClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Compile-time check that APIClient satisfies action.Backend.
var _ action.Backend = (*APIClient)(nil)

func (c *APIClient) SendMessage(ctx context.Context, recipient, text string) error {
	body := map[string]string{
		"recipient": recipient,
		"message":   text,
	}
	_, err := c.doPost(ctx, "/api/send", body)
	return err
}

func (c *APIClient) SendFile(ctx context.Context, recipient, filePath string) error {
	body := map[string]string{
		"recipient": recipient,
		"file_path": filePath,
	}
	_, err := c.doPost(ctx, "/api/send-file", body)
	return err
}

func (c *APIClient) SendAudioMessage(ctx context.Context, recipient, filePath string) error {
	body := map[string]string{
		"recipient": recipient,
		"file_path": filePath,
	}
	_, err := c.doPost(ctx, "/api/send-audio", body)
	return err
}

func (c *APIClient) DownloadMedia(ctx context.Context, messageID, chatJID string) (string, error) {
	body := map[string]string{
		"message_id": messageID,
		"chat_jid":   chatJID,
	}
	resp, err := c.doPost(ctx, "/api/download", body)
	if err != nil {
		return "", err
	}

	// The server returns Data as map[string]string{"path": "..."}.
	// After JSON round-tripping, Data arrives as map[string]any.
	dataMap, ok := resp.Data.(map[string]any)
	if !ok {
		return "", fmt.Errorf("unexpected response data type: %T", resp.Data)
	}
	path, ok := dataMap["path"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'path' in response data")
	}
	return path, nil
}

func (c *APIClient) RequestHistorySync(ctx context.Context) error {
	_, err := c.doPost(ctx, "/api/sync-history", nil)
	return err
}

// doPost sends a POST request with an optional JSON body and returns the
// decoded apiResponse. It returns an error if the request fails, the response
// cannot be decoded, or the server reports success=false.
func (c *APIClient) doPost(ctx context.Context, path string, body any) (*apiResponse, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request to %s: %w", path, err)
	}
	defer resp.Body.Close()

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response from %s: %w", path, err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("api error from %s: %s", path, apiResp.Message)
	}

	return &apiResp, nil
}
