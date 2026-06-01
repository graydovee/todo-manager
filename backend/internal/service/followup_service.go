package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/graydovee/todolist/internal/config"
	"github.com/graydovee/todolist/internal/model"
	"github.com/graydovee/todolist/internal/repository"
	"gorm.io/gorm"
)

// ContextMessage represents a single message in the conversation context
// sent by the frontend for follow-up requests.
type ContextMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// FollowupService handles follow-up question interactions on completed summaries.
type FollowupService struct {
	db           *gorm.DB
	followupRepo *repository.FollowupRepo
	summaryRepo  *repository.SummaryRepo
	llmClient    *LLMClient
	llmCfg       *config.LLMConfig
}

// NewFollowupService creates a new FollowupService with the given dependencies.
func NewFollowupService(
	db *gorm.DB,
	followupRepo *repository.FollowupRepo,
	summaryRepo *repository.SummaryRepo,
	llmClient *LLMClient,
	llmCfg *config.LLMConfig,
) *FollowupService {
	return &FollowupService{
		db:           db,
		followupRepo: followupRepo,
		summaryRepo:  summaryRepo,
		llmClient:    llmClient,
		llmCfg:       llmCfg,
	}
}

// AskFollowup validates input, constructs context, streams LLM response,
// and persists the question and answer version on completion.
// Returns a channel of StreamChunk for SSE streaming, the created FollowupMessage, and any error.
func (s *FollowupService) AskFollowup(ctx context.Context, summaryID, userID uint, question string, contextMessages []ContextMessage) (<-chan StreamChunk, *model.FollowupMessage, error) {
	// Validate summary exists and belongs to user
	summary, err := s.summaryRepo.FindByID(nil, summaryID, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("summary not found")
	}

	// Validate summary is in "completed" status
	if summary.Status != model.SummaryStatusCompleted {
		return nil, nil, fmt.Errorf("follow-up is only available for completed summaries")
	}

	// Validate question is non-empty
	trimmedQuestion := strings.TrimSpace(question)
	if trimmedQuestion == "" {
		return nil, nil, fmt.Errorf("a non-empty question is required")
	}

	// Validate question length ≤ 1000 chars
	if len(trimmedQuestion) > 1000 {
		return nil, nil, fmt.Errorf("question exceeds maximum length of 1000 characters")
	}

	// Validate context_messages
	if err := s.validateContextMessages(contextMessages); err != nil {
		return nil, nil, err
	}

	// Truncate context to most recent 20 pairs if exceeded
	contextMessages = s.truncateContext(contextMessages)

	// Construct LLM message array
	messages := s.buildFollowupMessages(summary.ResultContent, contextMessages, trimmedQuestion)

	// Create a cancellable context for the LLM call
	timeout := time.Duration(s.llmCfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	llmCtx, llmCancel := context.WithCancel(ctx)

	// Call ChatCompletionStream
	inputCh, err := s.llmClient.ChatCompletionStream(llmCtx, messages)
	if err != nil {
		llmCancel()
		return nil, nil, fmt.Errorf("LLM request failed: %w", s.sanitizeError(err))
	}

	// Create the followup message record (question only, answer persisted on completion)
	followupMsg := &model.FollowupMessage{
		SummaryID: summaryID,
		Question:  trimmedQuestion,
		CreatedAt: time.Now(),
	}
	if err := s.followupRepo.CreateMessage(nil, followupMsg); err != nil {
		llmCancel()
		return nil, nil, fmt.Errorf("failed to persist followup message: %w", err)
	}

	// Wrap with chunk timeout
	timeoutCh := s.wrapWithChunkTimeout(llmCtx, inputCh, timeout, llmCancel)

	// Create output channel that forwards chunks and handles persistence
	out := make(chan StreamChunk, 16)

	go func() {
		defer close(out)
		defer llmCancel()

		var contentBuilder strings.Builder

		for chunk := range timeoutCh {
			if chunk.Done {
				// Stream completed: persist the answer as a version
				fullContent := contentBuilder.String()
				s.persistVersion(followupMsg.ID, fullContent)

				// Forward done signal
				select {
				case out <- chunk:
				case <-ctx.Done():
				}
				return
			}

			if chunk.Err != nil {
				// Sanitize and forward error
				sanitizedErr := s.sanitizeError(chunk.Err)
				select {
				case out <- StreamChunk{Err: sanitizedErr}:
				case <-ctx.Done():
				}
				return
			}

			// Accumulate content
			contentBuilder.WriteString(chunk.Content)

			// Forward content chunk
			select {
			case out <- chunk:
			case <-ctx.Done():
				return
			}
		}

		// Channel closed unexpectedly: persist whatever we have
		partialContent := contentBuilder.String()
		if partialContent != "" {
			s.persistVersion(followupMsg.ID, partialContent)
			select {
			case out <- StreamChunk{Done: true}:
			case <-ctx.Done():
			}
		}
	}()

	return out, followupMsg, nil
}

// ListFollowups returns all followup messages with versions for a summary,
// after validating the summary belongs to the user.
func (s *FollowupService) ListFollowups(summaryID, userID uint) ([]*model.FollowupMessage, error) {
	// Validate summary exists and belongs to user
	_, err := s.summaryRepo.FindByID(nil, summaryID, userID)
	if err != nil {
		return nil, fmt.Errorf("summary not found")
	}

	messages, err := s.followupRepo.FindBySummaryID(nil, summaryID)
	if err != nil {
		return nil, fmt.Errorf("failed to list followup messages: %w", err)
	}

	return messages, nil
}

// validateContextMessages checks that context_messages meet all constraints.
func (s *FollowupService) validateContextMessages(messages []ContextMessage) error {
	if len(messages) > 20 {
		return fmt.Errorf("context_messages exceeds maximum of 20 items")
	}

	for _, msg := range messages {
		if msg.Role != "user" && msg.Role != "assistant" {
			return fmt.Errorf("context_messages role must be 'user' or 'assistant'")
		}
		if len(msg.Content) > 2000 {
			return fmt.Errorf("context_messages content exceeds maximum length of 2000 characters")
		}
	}

	return nil
}

// truncateContext keeps only the most recent 20 pairs (40 messages) if exceeded.
func (s *FollowupService) truncateContext(messages []ContextMessage) []ContextMessage {
	maxPairs := 20
	maxMessages := maxPairs * 2

	if len(messages) <= maxMessages {
		return messages
	}

	// Keep the most recent messages
	return messages[len(messages)-maxMessages:]
}

// buildFollowupMessages constructs the LLM message array for a follow-up question.
func (s *FollowupService) buildFollowupMessages(summaryContent string, contextMessages []ContextMessage, question string) []ChatMessage {
	var messages []ChatMessage

	// System prompt with summary content
	systemPrompt := fmt.Sprintf(
		"You are a helpful assistant. The user has received the following summary of their work:\n\n%s\n\n"+
			"Answer the user's follow-up questions based on this summary context and the conversation history. "+
			"If the question cannot be answered from the available context, indicate that clearly.",
		summaryContent,
	)
	messages = append(messages, ChatMessage{Role: "system", Content: systemPrompt})

	// Context messages (previous Q&A pairs)
	for _, msg := range contextMessages {
		messages = append(messages, ChatMessage{Role: msg.Role, Content: msg.Content})
	}

	// New question
	messages = append(messages, ChatMessage{Role: "user", Content: question})

	return messages
}

// persistVersion creates a new FollowupMessageVersion for the given message.
func (s *FollowupService) persistVersion(messageID uint, content string) {
	nextVersion, err := s.followupRepo.GetNextVersionNumber(nil, messageID)
	if err != nil {
		slog.Error("failed to get next version number", "message_id", messageID, "error", err)
		return
	}

	version := &model.FollowupMessageVersion{
		FollowupMessageID: messageID,
		Content:           content,
		VersionNumber:     nextVersion,
		CreatedAt:         time.Now(),
	}

	if err := s.followupRepo.CreateVersion(nil, version); err != nil {
		slog.Error("failed to persist followup version", "message_id", messageID, "error", err)
	}
}

// sanitizeError removes API keys and base URL from error messages.
func (s *FollowupService) sanitizeError(err error) error {
	if err == nil {
		return nil
	}

	msg := err.Error()
	modified := false

	if s.llmCfg.APIKey != "" && strings.Contains(msg, s.llmCfg.APIKey) {
		msg = strings.ReplaceAll(msg, s.llmCfg.APIKey, "[REDACTED]")
		modified = true
	}

	if s.llmCfg.BaseURL != "" && strings.Contains(msg, s.llmCfg.BaseURL) {
		msg = strings.ReplaceAll(msg, s.llmCfg.BaseURL, "[REDACTED]")
		modified = true
	}

	if modified {
		return fmt.Errorf("%s", msg)
	}

	return err
}

// wrapWithChunkTimeout wraps an LLM streaming channel with a per-chunk timeout.
// Reuses the same pattern as SummaryService.wrapWithChunkTimeout.
func (s *FollowupService) wrapWithChunkTimeout(ctx context.Context, input <-chan StreamChunk, timeout time.Duration, cancel context.CancelFunc) <-chan StreamChunk {
	out := make(chan StreamChunk, 16)

	go func() {
		defer close(out)
		timer := time.NewTimer(timeout)
		defer timer.Stop()

		for {
			select {
			case chunk, ok := <-input:
				if !ok {
					return
				}

				select {
				case out <- chunk:
				case <-ctx.Done():
					return
				}

				if chunk.Done || chunk.Err != nil {
					return
				}

				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(timeout)

			case <-timer.C:
				cancel()
				select {
				case out <- StreamChunk{Err: fmt.Errorf("timeout: no data received for %d seconds", int(timeout.Seconds()))}:
				case <-ctx.Done():
				}
				return

			case <-ctx.Done():
				return
			}
		}
	}()

	return out
}
