package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const DefaultAPIURL = "https://api.regrada.com"


// Client handles HTTP requests to the Regrada backend
type Client struct {
	apiURL    string
	apiKey    string
	projectID string
	client    *http.Client
}

// NewClient creates a new backend client
func NewClient(apiKey, projectID string) *Client {
	// Allow overriding API URL via environment for local development
	apiURL := os.Getenv("REGRADA_API_URL")
	if apiURL == "" {
		apiURL = DefaultAPIURL
	}

	return &Client{
		apiURL:    apiURL,
		apiKey:    apiKey,
		projectID: projectID,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// UploadTracesBatch uploads a batch of traces to the backend
func (c *Client) UploadTracesBatch(ctx context.Context, traces []map[string]interface{}) error {
	url := fmt.Sprintf("%s/v1/projects/%s/traces/batch", c.apiURL, c.projectID)
	
	payload := map[string]interface{}{
		"traces": traces,
	}
	
	return c.post(ctx, url, payload)
}

// UploadTestRun uploads test run results to the backend
func (c *Client) UploadTestRun(ctx context.Context, testRun map[string]interface{}) error {
	url := fmt.Sprintf("%s/v1/projects/%s/test-runs", c.apiURL, c.projectID)
	return c.post(ctx, url, testRun)
}

// post makes a POST request to the backend
func (c *Client) post(ctx context.Context, url string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("backend returned error %d: %s", resp.StatusCode, string(bodyBytes))
	}
	
	return nil
}
