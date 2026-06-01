package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/graydovee/todolist/internal/config"
	"github.com/graydovee/todolist/internal/model"
	"github.com/graydovee/todolist/internal/repository"
	"gorm.io/gorm"
)

// EnrichedTodoData holds enriched context data collected for prompt building.
type EnrichedTodoData struct {
	Comments      map[uint][]*model.Comment           // todoID -> comments
	Dependencies  map[uint]*DependencyInfo            // todoID -> dependency info
	StatusHistory map[uint][]*model.TodoStatusHistory // todoID -> history entries in range
}

// DependencyInfo holds prerequisite and dependent references for a todo.
type DependencyInfo struct {
	Prerequisites []TodoDependencyRef // todos this one depends on
	Dependents    []TodoDependencyRef // todos that depend on this one
}

// TodoDependencyRef is a lightweight reference to a related todo.
type TodoDependencyRef struct {
	Code   string
	Title  string
	Status string
}

// SummaryService handles AI summary creation and management.
type SummaryService struct {
	db                *gorm.DB
	summaryRepo       *repository.SummaryRepo
	todoRepo          *repository.TodoRepo
	commentRepo       *repository.CommentRepo
	relationRepo      *repository.RelationRepo
	statusHistoryRepo *repository.StatusHistoryRepo
	llmClient         *LLMClient
	llmCfg            *config.LLMConfig
}

// NewSummaryService creates a new SummaryService with the given dependencies.
func NewSummaryService(
	db *gorm.DB,
	summaryRepo *repository.SummaryRepo,
	todoRepo *repository.TodoRepo,
	commentRepo *repository.CommentRepo,
	relationRepo *repository.RelationRepo,
	statusHistoryRepo *repository.StatusHistoryRepo,
	llmClient *LLMClient,
	llmCfg *config.LLMConfig,
) *SummaryService {
	return &SummaryService{
		db:                db,
		summaryRepo:       summaryRepo,
		todoRepo:          todoRepo,
		commentRepo:       commentRepo,
		relationRepo:      relationRepo,
		statusHistoryRepo: statusHistoryRepo,
		llmClient:         llmClient,
		llmCfg:            llmCfg,
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

// CreateSummaryWithTodos validates inputs, creates a summary record with status "analyzing".
// If todoIDs is non-empty, it validates that all IDs belong to the user and stores them as JSON.
// If todoIDs is empty/nil, it falls back to existing date-range query behavior.
// The language parameter is persisted so that StreamAnalysis can skip auto-detection when set.
// The customPrompt parameter, if non-empty, is persisted and appended to the LLM prompt.
func (s *SummaryService) CreateSummaryWithTodos(userID uint, startDate, endDate time.Time, todoIDs []uint, language string, customPrompt string) (*model.Summary, error) {
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

	// If todoIDs provided, validate they all belong to the user
	var todoIDsJSON string
	if len(todoIDs) > 0 {
		// Query todos that match the given IDs and belong to the user
		foundTodos, err := s.todoRepo.FindByIDsAndUser(nil, todoIDs, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate todo IDs: %w", err)
		}

		// Build a set of found IDs
		foundIDSet := make(map[uint]bool, len(foundTodos))
		for _, t := range foundTodos {
			foundIDSet[t.ID] = true
		}

		// Find invalid IDs (not found or not belonging to user)
		var invalidIDs []uint
		for _, id := range todoIDs {
			if !foundIDSet[id] {
				invalidIDs = append(invalidIDs, id)
			}
		}

		if len(invalidIDs) > 0 {
			return nil, fmt.Errorf("invalid todo IDs: %v", invalidIDs)
		}

		// Serialize todoIDs as JSON array string
		todoIDsJSON = s.serializeTodoIDs(todoIDs)
	}

	// Create summary record with "analyzing" status
	summary := &model.Summary{
		UserID:    userID,
		StartDate: startDate,
		EndDate:   endDate,
		Status:    model.SummaryStatusAnalyzing,
		TodoIDs:   todoIDsJSON,
		Language:  language,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Persist custom prompt if non-empty
	if customPrompt != "" {
		summary.CustomPrompt = customPrompt
	}

	if err := s.summaryRepo.Create(nil, summary); err != nil {
		return nil, fmt.Errorf("failed to create summary: %w", err)
	}

	return summary, nil
}

// serializeTodoIDs converts a slice of uint IDs to a JSON array string.
func (s *SummaryService) serializeTodoIDs(ids []uint) string {
	if len(ids) == 0 {
		return ""
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("%d", id)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// runAnalysis fetches todos with activity in the date range, loads enriched data,
// performs two-step LLM interaction (language detection then analysis), and updates
// the summary record with the result.
func (s *SummaryService) runAnalysis(summaryID, userID uint, startDate, endDate time.Time) {
	slog.Info("starting summary analysis", "summary_id", summaryID, "user_id", userID,
		"start_date", startDate.Format("2006-01-02"), "end_date", endDate.Format("2006-01-02"))

	// Fetch todos with activity in date range using updated_at filter
	todos, err := s.todoRepo.FindByUserAndUpdatedAtRange(nil, userID, startDate, endDate)
	if err != nil {
		errMsg := fmt.Sprintf("failed to fetch todos: %v", err)
		slog.Error("summary analysis failed", "summary_id", summaryID, "error", errMsg)
		s.updateSummaryError(summaryID, userID, errMsg)
		return
	}

	slog.Info("summary analysis found todos", "summary_id", summaryID, "count", len(todos))

	// Handle empty result: set summary to "no activity found" message
	if len(todos) == 0 {
		summary, err := s.summaryRepo.FindByID(nil, summaryID, userID)
		if err != nil {
			slog.Error("summary analysis failed to find summary record", "summary_id", summaryID, "error", err)
			return
		}
		summary.Status = model.SummaryStatusCompleted
		summary.ResultContent = fmt.Sprintf("No activity found in the specified period (%s to %s).",
			startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
		summary.UpdatedAt = time.Now()
		s.summaryRepo.Update(nil, summary)
		slog.Info("summary analysis completed with no activity", "summary_id", summaryID)
		return
	}

	// Load enriched data: comments, dependencies, status history
	enrichedData, err := s.loadEnrichedData(todos, startDate, endDate)
	if err != nil {
		errMsg := fmt.Sprintf("failed to load enriched data: %v", err)
		slog.Error("summary analysis failed", "summary_id", summaryID, "error", errMsg)
		s.updateSummaryError(summaryID, userID, errMsg)
		return
	}

	// Step 1: Detect language from todo content (with its own timeout)
	timeout := time.Duration(s.llmCfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	langCtx, langCancel := context.WithTimeout(context.Background(), timeout)
	language := s.detectLanguage(langCtx, todos)
	langCancel()
	slog.Info("summary analysis detected language", "summary_id", summaryID, "language", language)

	// Step 2: Build enriched prompt with all context and detected language
	prompt := s.buildEnrichedPrompt(todos, enrichedData, language, startDate, endDate, "")

	// Call LLM with enriched prompt for analysis (with its own timeout)
	analysisCtx, analysisCancel := context.WithTimeout(context.Background(), timeout)
	defer analysisCancel()

	messages := []ChatMessage{
		{Role: "system", Content: fmt.Sprintf("You are a productivity analyst. Generate the entire summary in %s. Analyze the user's todo items and provide a helpful summary in markdown format. Include insights about completion rates, categories of work, and suggestions for improvement.", language)},
		{Role: "user", Content: prompt},
	}

	response, err := s.llmClient.ChatCompletion(analysisCtx, messages)
	if err != nil {
		errMsg := fmt.Sprintf("LLM analysis request failed: %v", err)
		slog.Error("summary analysis failed", "summary_id", summaryID, "error", errMsg)
		s.updateSummaryError(summaryID, userID, errMsg)
		return
	}

	// Update summary with successful result
	summary, err := s.summaryRepo.FindByID(nil, summaryID, userID)
	if err != nil {
		slog.Error("summary analysis failed to find summary record for update", "summary_id", summaryID, "error", err)
		return
	}
	summary.Status = model.SummaryStatusCompleted
	summary.ResultContent = response
	summary.UpdatedAt = time.Now()
	s.summaryRepo.Update(nil, summary)
	slog.Info("summary analysis completed successfully", "summary_id", summaryID)
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

// buildLanguageDetectionPrompt concatenates todo titles and descriptions into a
// lightweight prompt asking the LLM to identify the primary language.
func (s *SummaryService) buildLanguageDetectionPrompt(todos []*model.Todo) string {
	var sb strings.Builder
	sb.WriteString("Based on the following todo items, determine the primary language used. Reply with ONLY one word: \"Chinese\" or \"English\".\n\n")
	for _, todo := range todos {
		sb.WriteString("Title: ")
		sb.WriteString(todo.Title)
		sb.WriteString("\n")
		if todo.Description != "" {
			sb.WriteString("Description: ")
			sb.WriteString(todo.Description)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// detectLanguage calls the LLM with a language detection prompt, parses the
// response, and defaults to "Chinese" on failure or unrecognized value.
func (s *SummaryService) detectLanguage(ctx context.Context, todos []*model.Todo) string {
	prompt := s.buildLanguageDetectionPrompt(todos)

	messages := []ChatMessage{
		{Role: "system", Content: "You are a language detection assistant. Respond with only one word: Chinese or English."},
		{Role: "user", Content: prompt},
	}

	response, err := s.llmClient.ChatCompletion(ctx, messages)
	if err != nil {
		return "Chinese"
	}

	// Normalize and check for recognized language values
	normalized := strings.TrimSpace(response)
	lower := strings.ToLower(normalized)
	if strings.Contains(lower, "english") {
		return "English"
	}
	if strings.Contains(lower, "chinese") {
		return "Chinese"
	}

	// Default to Chinese for unrecognized values
	return "Chinese"
}

// updateSummaryError sets the summary status to "error" with the given message.
func (s *SummaryService) updateSummaryError(summaryID, userID uint, errMsg string) {
	summary, err := s.summaryRepo.FindByID(nil, summaryID, userID)
	if err != nil {
		slog.Error("updateSummaryError: failed to find summary", "summary_id", summaryID, "error", err)
		return
	}
	summary.Status = model.SummaryStatusError
	summary.ResultContent = errMsg
	summary.UpdatedAt = time.Now()
	if err := s.summaryRepo.Update(nil, summary); err != nil {
		slog.Error("updateSummaryError: failed to update summary", "summary_id", summaryID, "error", err)
	}
}

// loadEnrichedData collects comments, dependency info, and status history
// for the given set of todos within the specified time range.
func (s *SummaryService) loadEnrichedData(todos []*model.Todo, start, end time.Time) (*EnrichedTodoData, error) {
	result := &EnrichedTodoData{
		Comments:      make(map[uint][]*model.Comment),
		Dependencies:  make(map[uint]*DependencyInfo),
		StatusHistory: make(map[uint][]*model.TodoStatusHistory),
	}

	if len(todos) == 0 {
		return result, nil
	}

	// Collect all todo IDs
	todoIDs := make([]uint, 0, len(todos))
	for _, t := range todos {
		todoIDs = append(todoIDs, t.ID)
	}

	// Batch-load comments for all todos
	for _, id := range todoIDs {
		comments, err := s.commentRepo.FindByTodoID(nil, id)
		if err != nil {
			return nil, fmt.Errorf("failed to load comments for todo %d: %w", id, err)
		}
		if len(comments) > 0 {
			result.Comments[id] = comments
		}
	}

	// Batch-load relations and resolve dependency info
	// Collect all related todo IDs that need resolution
	relatedTodoIDs := make(map[uint]bool)
	type relationEntry struct {
		todoID   uint
		relation *model.TodoRelation
		isSource bool // true if this todo is the source (depends on target)
	}
	var allRelations []relationEntry

	for _, id := range todoIDs {
		// Find relations where this todo is the source (this todo depends on targets)
		sourceRels, err := s.relationRepo.FindBySource(nil, id)
		if err != nil {
			return nil, fmt.Errorf("failed to load source relations for todo %d: %w", id, err)
		}
		for _, rel := range sourceRels {
			if rel.RelationType == model.RelationDependsOn {
				allRelations = append(allRelations, relationEntry{todoID: id, relation: rel, isSource: true})
				relatedTodoIDs[rel.TargetID] = true
			}
		}

		// Find relations where this todo is the target (other todos depend on this one)
		targetRels, err := s.relationRepo.FindByTarget(nil, id)
		if err != nil {
			return nil, fmt.Errorf("failed to load target relations for todo %d: %w", id, err)
		}
		for _, rel := range targetRels {
			if rel.RelationType == model.RelationDependsOn {
				allRelations = append(allRelations, relationEntry{todoID: id, relation: rel, isSource: false})
				relatedTodoIDs[rel.SourceID] = true
			}
		}
	}

	// Resolve related todos to get their code, title, and status
	// Build a cache from the todos we already have
	todoCache := make(map[uint]*model.Todo)
	for _, t := range todos {
		todoCache[t.ID] = t
	}

	// Load any related todos not already in our set
	for relID := range relatedTodoIDs {
		if _, exists := todoCache[relID]; !exists {
			var relTodo model.Todo
			if err := s.db.Where("id = ?", relID).First(&relTodo).Error; err == nil {
				todoCache[relID] = &relTodo
			}
		}
	}

	// Build dependency info from collected relations
	for _, entry := range allRelations {
		depInfo, exists := result.Dependencies[entry.todoID]
		if !exists {
			depInfo = &DependencyInfo{}
			result.Dependencies[entry.todoID] = depInfo
		}

		if entry.isSource {
			// This todo depends on the target (target is a prerequisite)
			if target, ok := todoCache[entry.relation.TargetID]; ok {
				depInfo.Prerequisites = append(depInfo.Prerequisites, TodoDependencyRef{
					Code:   target.Code,
					Title:  target.Title,
					Status: target.Status,
				})
			}
		} else {
			// Another todo (source) depends on this todo (this todo is a prerequisite for source)
			if source, ok := todoCache[entry.relation.SourceID]; ok {
				depInfo.Dependents = append(depInfo.Dependents, TodoDependencyRef{
					Code:   source.Code,
					Title:  source.Title,
					Status: source.Status,
				})
			}
		}
	}

	// Batch-load status history entries within the time range
	historyMap, err := s.statusHistoryRepo.FindByTodoIDsAndTimeRange(nil, todoIDs, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to load status history: %w", err)
	}
	result.StatusHistory = historyMap

	return result, nil
}

// buildEnrichedPrompt builds a comprehensive prompt string with enriched todo data
// and tag-grouped output instructions for the LLM analysis step.
// If customPrompt is non-empty, it is appended after the output instructions.
func (s *SummaryService) buildEnrichedPrompt(todos []*model.Todo, enrichedData *EnrichedTodoData, language string, startDate, endDate time.Time, customPrompt string) string {
	var sb strings.Builder

	// System-level instruction with language directive
	fmt.Fprintf(&sb, "Generate the entire summary in %s.\n", language)
	fmt.Fprintf(&sb, "Analyze the following todo activity from %s to %s.\n\n",
		startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	// Build per-todo sections
	for i, todo := range todos {
		// Basic info
		fmt.Fprintf(&sb, "--- Todo %d ---\n", i+1)
		fmt.Fprintf(&sb, "Title: %s\n", todo.Title)
		fmt.Fprintf(&sb, "Code: %s\n", todo.Code)
		fmt.Fprintf(&sb, "Category: %s\n", todo.Category)
		fmt.Fprintf(&sb, "Priority: %s\n", todo.Priority)
		fmt.Fprintf(&sb, "Status: %s\n", todo.Status)
		if todo.Description != "" {
			fmt.Fprintf(&sb, "Description: %s\n", todo.Description)
		}
		// Tags
		if len(todo.Tags) > 0 {
			tags := make([]string, 0, len(todo.Tags))
			for _, t := range todo.Tags {
				tags = append(tags, t.Tag)
			}
			fmt.Fprintf(&sb, "Tags: %s\n", strings.Join(tags, ", "))
		}

		// Comments section (omit if empty)
		if comments, ok := enrichedData.Comments[todo.ID]; ok && len(comments) > 0 {
			sb.WriteString("Comments:\n")
			for _, c := range comments {
				fmt.Fprintf(&sb, "  - [%s] %s\n", c.CreatedAt.Format("2006-01-02 15:04:05"), c.Content)
			}
		}

		// Dependencies section (omit if empty)
		if depInfo, ok := enrichedData.Dependencies[todo.ID]; ok {
			hasDeps := len(depInfo.Prerequisites) > 0 || len(depInfo.Dependents) > 0
			if hasDeps {
				sb.WriteString("Dependencies:\n")
				if len(depInfo.Prerequisites) > 0 {
					sb.WriteString("  Prerequisites:\n")
					for _, p := range depInfo.Prerequisites {
						fmt.Fprintf(&sb, "    - [%s] %s (Status: %s)\n", p.Code, p.Title, p.Status)
					}
				}
				if len(depInfo.Dependents) > 0 {
					sb.WriteString("  Dependents:\n")
					for _, d := range depInfo.Dependents {
						fmt.Fprintf(&sb, "    - [%s] %s (Status: %s)\n", d.Code, d.Title, d.Status)
					}
				}
			}
		}

		// Status history section (omit if empty)
		if history, ok := enrichedData.StatusHistory[todo.ID]; ok && len(history) > 0 {
			sb.WriteString("Status History:\n")
			for _, h := range history {
				fmt.Fprintf(&sb, "  - %s -> %s at %s\n", h.OldStatus, h.NewStatus, h.ChangedAt.Format("2006-01-02 15:04:05"))
			}
		}

		sb.WriteString("\n")
	}

	// Tag-grouped output instructions
	sb.WriteString("--- Output Instructions ---\n")
	sb.WriteString("Organize the summary output grouped by tags/labels:\n")
	sb.WriteString("- Group todos by their tags. Each tag forms a separate section.\n")
	sb.WriteString("- If a todo has multiple tags, include it in each relevant tag group.\n")
	sb.WriteString("- If a todo has no tags, place it under a dedicated \"Untagged\" group.\n")
	sb.WriteString("- For each tag group, include:\n")
	sb.WriteString("  - Activity summary: what was worked on\n")
	sb.WriteString("  - Completion progress: how many todos are completed vs open\n")
	sb.WriteString("  - Notable status transitions: significant changes during the period\n")

	// Append custom prompt if non-empty
	if customPrompt != "" {
		fmt.Fprintf(&sb, "\nAdditional user requirements: %s\n", customPrompt)
	}

	return sb.String()
}

// wrapWithChunkTimeout wraps an LLM streaming channel with a per-chunk timeout.
// It resets a timer on each received chunk. If no chunk arrives within the timeout
// duration, it calls cancel() to abort the LLM context and sends a timeout error
// through the output channel. The timeout starts immediately (covers first-chunk wait).
// As long as chunks keep arriving within the interval, no timeout occurs regardless
// of total elapsed time.
func (s *SummaryService) wrapWithChunkTimeout(ctx context.Context, input <-chan StreamChunk, timeout time.Duration, cancel context.CancelFunc) <-chan StreamChunk {
	out := make(chan StreamChunk, 16)

	go func() {
		defer close(out)
		timer := time.NewTimer(timeout)
		defer timer.Stop()

		for {
			select {
			case chunk, ok := <-input:
				if !ok {
					// Input channel closed without Done/Err signal; treat as done.
					return
				}

				// Forward the chunk to output.
				select {
				case out <- chunk:
				case <-ctx.Done():
					return
				}

				// If this is a terminal chunk (Done or Err), stop processing.
				if chunk.Done || chunk.Err != nil {
					return
				}

				// Reset the timer for the next chunk.
				if !timer.Stop() {
					// Drain the timer channel if it already fired.
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(timeout)

			case <-timer.C:
				// Timeout: no chunk received within the configured duration.
				cancel()
				select {
				case out <- StreamChunk{Err: fmt.Errorf("timeout: no data received for %d seconds", int(timeout.Seconds()))}:
				case <-ctx.Done():
				}
				return

			case <-ctx.Done():
				// Context cancelled externally (e.g., client disconnect).
				return
			}
		}
	}()

	return out
}

// StreamAnalysis performs LLM analysis and streams results via channel.
// It handles chunk timeout internally and closes the channel on completion/error.
func (s *SummaryService) StreamAnalysis(ctx context.Context, summaryID, userID uint) (<-chan StreamChunk, error) {
	// 1. Fetch the summary record, verify ownership and status
	summary, err := s.summaryRepo.FindByID(nil, summaryID, userID)
	if err != nil {
		return nil, fmt.Errorf("summary not found: %w", err)
	}

	if summary.Status != model.SummaryStatusAnalyzing {
		return nil, fmt.Errorf("summary is not in analyzing status")
	}

	// 2. Determine which todos to use
	var todos []*model.Todo
	if summary.TodoIDs != "" {
		// Parse the JSON array of todo IDs
		todoIDs, parseErr := parseTodoIDs(summary.TodoIDs)
		if parseErr != nil {
			s.updateSummaryError(summaryID, userID, fmt.Sprintf("failed to parse todo IDs: %v", parseErr))
			return nil, fmt.Errorf("failed to parse todo IDs: %w", parseErr)
		}
		todos, err = s.todoRepo.FindByIDsAndUser(nil, todoIDs, userID)
		if err != nil {
			s.updateSummaryError(summaryID, userID, fmt.Sprintf("failed to fetch todos by IDs: %v", err))
			return nil, fmt.Errorf("failed to fetch todos: %w", err)
		}
	} else {
		// Fall back to date range query
		todos, err = s.todoRepo.FindByUserAndUpdatedAtRange(nil, userID, summary.StartDate, summary.EndDate)
		if err != nil {
			s.updateSummaryError(summaryID, userID, fmt.Sprintf("failed to fetch todos: %v", err))
			return nil, fmt.Errorf("failed to fetch todos: %w", err)
		}
	}

	// Handle empty result
	if len(todos) == 0 {
		noActivityMsg := fmt.Sprintf("No activity found in the specified period (%s to %s).",
			summary.StartDate.Format("2006-01-02"), summary.EndDate.Format("2006-01-02"))
		summary.Status = model.SummaryStatusCompleted
		summary.ResultContent = noActivityMsg
		summary.UpdatedAt = time.Now()
		s.summaryRepo.Update(nil, summary)

		// Return a channel that immediately sends the message and signals done
		out := make(chan StreamChunk, 2)
		out <- StreamChunk{Content: noActivityMsg}
		out <- StreamChunk{Done: true}
		close(out)
		return out, nil
	}

	// 3. Load enriched data
	enrichedData, err := s.loadEnrichedData(todos, summary.StartDate, summary.EndDate)
	if err != nil {
		errMsg := fmt.Sprintf("failed to load enriched data: %v", err)
		s.updateSummaryError(summaryID, userID, errMsg)
		return nil, fmt.Errorf("failed to load enriched data: %w", err)
	}

	// 4. Determine language: use persisted value if specified, otherwise auto-detect
	timeout := time.Duration(s.llmCfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	var language string
	if summary.Language != "" {
		// Language was specified by user at creation time; skip auto-detection
		language = summary.Language
		slog.Info("stream analysis using specified language", "summary_id", summaryID, "language", language)
	} else {
		// Auto-detect language from todo content via LLM
		langCtx, langCancel := context.WithTimeout(ctx, timeout)
		language = s.detectLanguage(langCtx, todos)
		langCancel()
		slog.Info("stream analysis detected language", "summary_id", summaryID, "language", language)
	}

	// 5. Build enriched prompt
	prompt := s.buildEnrichedPrompt(todos, enrichedData, language, summary.StartDate, summary.EndDate, summary.CustomPrompt)

	// 6. Create a cancellable context for the LLM call
	llmCtx, llmCancel := context.WithCancel(ctx)

	messages := []ChatMessage{
		{Role: "system", Content: fmt.Sprintf("You are a productivity analyst. Generate the entire summary in %s. Analyze the user's todo items and provide a helpful summary in markdown format. Include insights about completion rates, categories of work, and suggestions for improvement.", language)},
		{Role: "user", Content: prompt},
	}

	// 7. Call ChatCompletionStream
	inputCh, err := s.llmClient.ChatCompletionStream(llmCtx, messages)
	if err != nil {
		llmCancel()
		errMsg := fmt.Sprintf("LLM streaming request failed: %v", err)
		s.updateSummaryError(summaryID, userID, errMsg)
		return nil, fmt.Errorf("LLM streaming request failed: %w", err)
	}

	// 8. Wrap with chunk timeout
	timeoutCh := s.wrapWithChunkTimeout(llmCtx, inputCh, timeout, llmCancel)

	// 9. Create output channel that forwards chunks and handles persistence
	out := make(chan StreamChunk, 16)

	go func() {
		defer close(out)
		defer llmCancel()

		var contentBuilder strings.Builder

		for chunk := range timeoutCh {
			if chunk.Done {
				// Stream completed successfully: persist full content
				fullContent := contentBuilder.String()
				summary, findErr := s.summaryRepo.FindByID(nil, summaryID, userID)
				if findErr == nil {
					summary.Status = model.SummaryStatusCompleted
					summary.ResultContent = fullContent
					summary.UpdatedAt = time.Now()
					s.summaryRepo.Update(nil, summary)
				}
				// Forward done signal
				select {
				case out <- chunk:
				case <-ctx.Done():
				}
				return
			}

			if chunk.Err != nil {
				// Check if this is a context cancellation (client disconnect)
				if ctx.Err() != nil {
					// Client disconnected: save partial content
					partialContent := contentBuilder.String()
					if partialContent != "" {
						summary, findErr := s.summaryRepo.FindByID(nil, summaryID, userID)
						if findErr == nil {
							summary.Status = model.SummaryStatusError
							summary.ResultContent = fmt.Sprintf("Client disconnected. Partial content:\n%s", partialContent)
							summary.UpdatedAt = time.Now()
							s.summaryRepo.Update(nil, summary)
						}
					} else {
						s.updateSummaryError(summaryID, userID, "client disconnected")
					}
				} else {
					// LLM error or timeout: update summary with sanitized error
					errMsg := chunk.Err.Error()
					s.updateSummaryError(summaryID, userID, errMsg)
				}
				// Forward error to caller
				select {
				case out <- chunk:
				case <-ctx.Done():
				}
				return
			}

			// Accumulate content
			contentBuilder.WriteString(chunk.Content)

			// Forward content chunk to caller
			select {
			case out <- chunk:
			case <-ctx.Done():
				// Client disconnected while forwarding: save partial content
				partialContent := contentBuilder.String()
				if partialContent != "" {
					summary, findErr := s.summaryRepo.FindByID(nil, summaryID, userID)
					if findErr == nil {
						summary.Status = model.SummaryStatusError
						summary.ResultContent = fmt.Sprintf("Client disconnected. Partial content:\n%s", partialContent)
						summary.UpdatedAt = time.Now()
						s.summaryRepo.Update(nil, summary)
					}
				} else {
					s.updateSummaryError(summaryID, userID, "client disconnected")
				}
				return
			}
		}

		// If we exit the loop without Done/Err (channel closed unexpectedly),
		// save whatever we have as partial content
		partialContent := contentBuilder.String()
		if partialContent != "" {
			summary, findErr := s.summaryRepo.FindByID(nil, summaryID, userID)
			if findErr == nil {
				summary.Status = model.SummaryStatusCompleted
				summary.ResultContent = partialContent
				summary.UpdatedAt = time.Now()
				s.summaryRepo.Update(nil, summary)
			}
			select {
			case out <- StreamChunk{Done: true}:
			case <-ctx.Done():
			}
		}
	}()

	return out, nil
}

// parseTodoIDs parses a JSON array string (e.g. "[1,2,3]") into a slice of uint.
func parseTodoIDs(jsonStr string) ([]uint, error) {
	if jsonStr == "" {
		return nil, nil
	}
	var ids []uint
	if err := json.Unmarshal([]byte(jsonStr), &ids); err != nil {
		return nil, fmt.Errorf("invalid todo_ids JSON: %w", err)
	}
	return ids, nil
}

// GetSummary retrieves a single summary by ID for the given user.
func (s *SummaryService) GetSummary(userID, id uint) (*model.Summary, error) {
	return s.summaryRepo.FindByID(nil, id, userID)
}

// GetSummaryByID retrieves a summary by ID without user ownership check.
// Used when the caller needs to distinguish between "not found" and "wrong user".
func (s *SummaryService) GetSummaryByID(id uint) (*model.Summary, error) {
	return s.summaryRepo.FindByIDOnly(nil, id)
}

// ListSummaries returns the user's summaries (up to 50, ordered by created_at desc).
func (s *SummaryService) ListSummaries(userID uint) ([]*model.Summary, error) {
	return s.summaryRepo.ListByUser(nil, userID, 50)
}

// DeleteSummary removes a summary record for the given user.
func (s *SummaryService) DeleteSummary(userID, id uint) error {
	return s.summaryRepo.Delete(nil, id, userID)
}
