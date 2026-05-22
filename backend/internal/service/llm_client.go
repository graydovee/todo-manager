package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/graydovee/todolist/internal/config"
)

// LLMClient communicates with an OpenAI-compatible chat completions API.
type LLMClient struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	model      string
}

// ChatMessage represents a single message in a chat completion request/response.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest is the request body for the chat completions endpoint.
type ChatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

// ChatCompletionResponse is the response body from the chat completions endpoint.
type ChatCompletionResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
}

// NewLLMClient creates a new LLM client with the given configuration.
func NewLLMClient(cfg *config.LLMConfig) *LLMClient {
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	return &LLMClient{
		httpClient: &http.Client{Timeout: timeout},
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
	}
}

// ChatCompletion sends a chat completion request and returns the assistant message content.
func (c *LLMClient) ChatCompletion(ctx context.Context, messages []ChatMessage) (string, error) {
	reqBody := ChatCompletionRequest{
		Model:    c.model,
		Messages: messages,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", c.sanitizeError(fmt.Errorf("failed to create request: %w", err))
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", c.sanitizeError(fmt.Errorf("LLM request failed: %w", err))
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", c.sanitizeError(fmt.Errorf("failed to read response body: %w", err))
	}

	if resp.StatusCode != http.StatusOK {
		return "", c.sanitizeError(fmt.Errorf("LLM request failed: status %d, body: %s", resp.StatusCode, string(respBody)))
	}

	var completionResp ChatCompletionResponse
	if err := json.Unmarshal(respBody, &completionResp); err != nil {
		return "", c.sanitizeError(fmt.Errorf("failed to parse response: %w", err))
	}

	if len(completionResp.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}

	return completionResp.Choices[0].Message.Content, nil
}

// sanitizeError strips the api_key from any error string before returning.
func (c *LLMClient) sanitizeError(err error) error {
	if err == nil {
		return nil
	}
	if c.apiKey == "" {
		return err
	}
	msg := err.Error()
	sanitized := strings.ReplaceAll(msg, c.apiKey, "[REDACTED]")
	if sanitized != msg {
		return fmt.Errorf("%s", sanitized)
	}
	return err
}
