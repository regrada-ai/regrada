package record

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/regrada-ai/regrada/internal/backend"
	"github.com/regrada-ai/regrada/internal/config"
	"github.com/regrada-ai/regrada/internal/git"
	"github.com/regrada-ai/regrada/internal/trace"
)

// Uploader handles uploading traces to the backend in real-time
type Uploader struct {
	client    *backend.Client
	gitClient *git.ExecClient
	gitSHA    string
	gitBranch string
	enabled   bool
}

// NewUploader creates a new uploader if backend is configured
func NewUploader(cfg *config.ProjectConfig) *Uploader {
	// Check if backend upload is enabled
	if cfg.Backend.Enabled == nil || !*cfg.Backend.Enabled {
		return &Uploader{enabled: false}
	}
	if cfg.Backend.Upload.Traces == nil || !*cfg.Backend.Upload.Traces {
		return &Uploader{enabled: false}
	}

	apiKey := os.Getenv(cfg.Backend.APIKeyEnv)
	if apiKey == "" || cfg.Backend.ProjectID == "" {
		return &Uploader{enabled: false}
	}

	// Collect git context once
	gitClient := git.NewExecClient()
	gitSHA, _ := gitClient.GetCurrentCommit()
	gitBranch, _ := gitClient.GetCurrentBranch()

	return &Uploader{
		client:    backend.NewClient(apiKey, cfg.Backend.ProjectID),
		gitClient: gitClient,
		gitSHA:    gitSHA,
		gitBranch: gitBranch,
		enabled:   true,
	}
}

// Upload uploads a single trace to the backend
func (u *Uploader) Upload(ctx context.Context, t trace.Trace) error {
	if !u.enabled {
		return nil
	}

	// Convert trace to upload format
	traceData := map[string]interface{}{
		"trace_id":   t.TraceID,
		"timestamp":  t.Timestamp,
		"provider":   t.Provider,
		"model":      t.Model,
		"git_sha":    u.gitSHA,
		"git_branch": u.gitBranch,
	}

	// Marshal request and response to JSON
	requestJSON, _ := json.Marshal(t.Request)
	responseJSON, _ := json.Marshal(t.Response)

	traceData["request"] = json.RawMessage(requestJSON)
	traceData["response"] = json.RawMessage(responseJSON)

	// Add metrics if available
	if t.Metrics.LatencyMS > 0 {
		traceData["latency_ms"] = t.Metrics.LatencyMS
	}
	if t.Metrics.TokensIn > 0 {
		traceData["tokens_in"] = t.Metrics.TokensIn
	}
	if t.Metrics.TokensOut > 0 {
		traceData["tokens_out"] = t.Metrics.TokensOut
	}

	// Upload as a batch of one
	traces := []map[string]interface{}{traceData}

	// Use background context with timeout to avoid blocking proxy
	uploadCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := u.client.UploadTracesBatch(uploadCtx, traces); err != nil {
		// Log error but don't fail the recording
		fmt.Printf("Warning: failed to upload trace to backend: %v\n", err)
		return err
	}

	return nil
}

// IsEnabled returns whether the uploader is enabled
func (u *Uploader) IsEnabled() bool {
	return u.enabled
}
