package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/graydovee/todolist/internal/config"
	"github.com/graydovee/todolist/internal/model"
	"github.com/graydovee/todolist/internal/repository"
	"gorm.io/gorm"
)

// SummaryService handles AI summary creation and management.
type SummaryService struct {
	db          *gorm.DB
	summaryRepo *repository.SummaryRepo
	todoRepo    *repository.TodoRepo
	llmClient   *LLMClient
	llmCfg      *config.LLMConfig
}

// NewSummaryService creates a new SummaryService with the given dependencies.
func NewSummaryService(
	db *gorm.DB,
	summaryRepo *repository.SummaryRepo,
	todoRepo *repository.TodoRepo,
	llmClient *LLMClient,
	llmCfg *config.LLMConfig,
) *SummaryService {
	return &SummaryService{
		db:          db,
		summaryRepo: summaryRepo,
		todoRepo:    todoRepo,
		llmClient:   llmClient,
		llmCfg:      llmCfg,
	}
}

// CreateSummary validates inputs, creates a summary record with status "analyzing",
// and spawns a background goroutine to perform the LLM analysis.
func (s *SummaryService) CreateSummary(userID uint, startDate, endDate time.Time) (*model.Summary, error) {
	// Validate: end date must not be earlier than start date
	if endDate.Before(startDate) {
		return nil, fmt.Errorf("end_date must not be earlier than start_date")
	}

	// Validate: neither date can be in the future
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	if startDate.After(today) || endDate.After(today) {
		return nil, fmt.Errorf("dates must not be in the future")
	}

	// Validate LLM configuration
	if err := config.ValidateLLMConfig(s.llmCfg); err != nil {
		return nil, err
	}

	// Create summary record with "analyzing" status
	summary := &model.Summary{
		UserID:    userID,
		StartDate: startDate,
		EndDate:   endDate,
		Status:    model.SummaryStatusAnalyzing,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.summaryRepo.Create(nil, summary); err != nil {
		return nil, fmt.Errorf("failed to create summary: %w", err)
	}

	// Spawn background analysis
	go s.runAnalysis(summary.ID, userID, startDate, endDate)

	return summary, nil
}

// runAnalysis fetches todos in the date range, builds a prompt, calls the LLM,
// and updates the summary record with the result.
func (s *SummaryService) runAnalysis(summaryID, userID uint, startDate, endDate time.Time) {
	// Fetch todos in date range for the user
	var todos []*model.Todo
	err := s.db.Where("user_id = ? AND created_at >= ? AND created_at <= ?",
		userID, startDate, endDate.Add(24*time.Hour-time.Second)).
		Find(&todos).Error
	if err != nil {
		s.updateSummaryError(summaryID, userID, fmt.Sprintf("failed to fetch todos: %v", err))
		return
	}

	// Build prompt from todos
	prompt := s.buildPrompt(todos, startDate, endDate)

	// Call LLM
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.llmCfg.Timeout)*time.Second)
	defer cancel()

	messages := []ChatMessage{
		{Role: "system", Content: "You are a productivity analyst. Analyze the user's todo items and provide a helpful summary in markdown format. Include insights about completion rates, categories of work, and suggestions for improvement."},
		{Role: "user", Content: prompt},
	}

	response, err := s.llmClient.ChatCompletion(ctx, messages)
	if err != nil {
		s.updateSummaryError(summaryID, userID, fmt.Sprintf("LLM request failed: %v", err))
		return
	}

	// Update summary with successful result
	summary, err := s.summaryRepo.FindByID(nil, summaryID, userID)
	if err != nil {
		return
	}
	summary.Status = model.SummaryStatusCompleted
	summary.ResultContent = response
	summary.UpdatedAt = time.Now()
	s.summaryRepo.Update(nil, summary)
}

// buildPrompt constructs the LLM prompt from the list of todos.
func (s *SummaryService) buildPrompt(todos []*model.Todo, startDate, endDate time.Time) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Please analyze my todo activity from %s to %s.\n\n",
		startDate.Format("2006-01-02"), endDate.Format("2006-01-02")))

	if len(todos) == 0 {
		sb.WriteString("No todos were created during this period.\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("I have %d todos during this period:\n\n", len(todos)))
	for i, todo := range todos {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s (Category: %s, Priority: %s, Status: %s)",
			i+1, todo.Code, todo.Title, todo.Category, todo.Priority, todo.Status))
		if todo.Description != "" {
			sb.WriteString(fmt.Sprintf("\n   Description: %s", todo.Description))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// updateSummaryError sets the summary status to "error" with the given message.
func (s *SummaryService) updateSummaryError(summaryID, userID uint, errMsg string) {
	summary, err := s.summaryRepo.FindByID(nil, summaryID, userID)
	if err != nil {
		return
	}
	summary.Status = model.SummaryStatusError
	summary.ResultContent = errMsg
	summary.UpdatedAt = time.Now()
	s.summaryRepo.Update(nil, summary)
}

// GetSummary retrieves a single summary by ID for the given user.
func (s *SummaryService) GetSummary(userID, id uint) (*model.Summary, error) {
	return s.summaryRepo.FindByID(nil, id, userID)
}

// ListSummaries returns the user's summaries (up to 50, ordered by created_at desc).
func (s *SummaryService) ListSummaries(userID uint) ([]*model.Summary, error) {
	return s.summaryRepo.ListByUser(nil, userID, 50)
}

// DeleteSummary removes a summary record for the given user.
func (s *SummaryService) DeleteSummary(userID, id uint) error {
	return s.summaryRepo.Delete(nil, id, userID)
}
