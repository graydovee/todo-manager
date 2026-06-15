package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/graydovee/todo-manager/internal/config"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// LLMClient wraps the official openai-go SDK client.
type LLMClient struct {
	client *openai.Client
	model  string
	apiKey string // kept for error sanitization
}

// ChatMessage represents a single message in a chat completion request/response.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// StreamChunk represents a single content delta from the LLM streaming response.
type StreamChunk struct {
	Content string
	Done    bool
	Err     error
}

// NewLLMClient creates a client using the official openai-go SDK.
// Supports custom base URL for non-OpenAI compatible APIs.
func NewLLMClient(cfg *config.LLMConfig) *LLMClient {
	opts := []option.RequestOption{
		option.WithAPIKey(cfg.APIKey),
		option.WithBaseURL(strings.TrimRight(cfg.BaseURL, "/") + "/v1/"),
	}

	client := openai.NewClient(opts...)

	return &LLMClient{
		client: &client,
		model:  cfg.Model,
		apiKey: cfg.APIKey,
	}
}

// ChatCompletion sends a non-streaming request (for language detection).
func (c *LLMClient) ChatCompletion(ctx context.Context, messages []ChatMessage) (string, error) {
	params := openai.ChatCompletionNewParams{
		Model:    c.model,
		Messages: c.toSDKMessages(messages),
	}

	resp, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", c.sanitizeError(fmt.Errorf("LLM request failed: %w", err))
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}

	return resp.Choices[0].Message.Content, nil
}

// ChatCompletionStream sends a streaming request and returns a channel of chunks.
// Uses the SDK's built-in SSE parsing and stream iteration.
// No fixed total-request timeout is set; the caller manages cancellation via context.
func (c *LLMClient) ChatCompletionStream(ctx context.Context, messages []ChatMessage) (<-chan StreamChunk, error) {
	params := openai.ChatCompletionNewParams{
		Model:    c.model,
		Messages: c.toSDKMessages(messages),
	}

	stream := c.client.Chat.Completions.NewStreaming(ctx, params)

	ch := make(chan StreamChunk, 16)

	go func() {
		defer close(ch)
		defer stream.Close()

		for stream.Next() {
			chunk := stream.Current()
			if len(chunk.Choices) == 0 {
				continue
			}
			content := chunk.Choices[0].Delta.Content
			if content == "" {
				continue
			}
			select {
			case ch <- StreamChunk{Content: content}:
			case <-ctx.Done():
				ch <- StreamChunk{Err: ctx.Err()}
				return
			}
		}

		if err := stream.Err(); err != nil {
			select {
			case ch <- StreamChunk{Err: c.sanitizeError(fmt.Errorf("LLM streaming failed: %w", err))}:
			case <-ctx.Done():
			}
			return
		}

		// Signal completion
		select {
		case ch <- StreamChunk{Done: true}:
		case <-ctx.Done():
		}
	}()

	return ch, nil
}

// toSDKMessages converts internal ChatMessage slice to SDK message params.
func (c *LLMClient) toSDKMessages(messages []ChatMessage) []openai.ChatCompletionMessageParamUnion {
	sdkMessages := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			sdkMessages = append(sdkMessages, openai.SystemMessage(msg.Content))
		case "user":
			sdkMessages = append(sdkMessages, openai.UserMessage(msg.Content))
		case "assistant":
			sdkMessages = append(sdkMessages, openai.AssistantMessage(msg.Content))
		default:
			// Default to user message for unknown roles
			sdkMessages = append(sdkMessages, openai.UserMessage(msg.Content))
		}
	}
	return sdkMessages
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
