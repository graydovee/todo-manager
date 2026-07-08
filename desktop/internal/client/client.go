package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// Client talks to the todo-manager backend using a Bearer access key. The base
// URL is normalised to "<host>/api/v1".
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// APIError is returned for any non-2xx response. Conflict is a typed sentinel
// that callers can use errors.As on.
type APIError struct {
	Status   int
	Message  string
	Conflict *ConflictResponse
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("request failed with status %d", e.Status)
}

// IsConflict reports whether err is a 409 dependency conflict.
func IsConflict(err error) (*ConflictResponse, bool) {
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Conflict != nil {
		return apiErr.Conflict, true
	}
	return nil, false
}

// New constructs a Client with a 30s timeout.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL:    NormalizeBaseURL(baseURL),
		apiKey:     strings.TrimSpace(apiKey),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// NormalizeBaseURL strips trailing slashes and an optional "/api/v1" suffix,
// then re-appends "/api/v1". Empty input yields "".
func NormalizeBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return strings.TrimSuffix(raw, "/") + "/api/v1"
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	parsed.Path = strings.TrimSuffix(parsed.Path, "/api/v1")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimSuffix(parsed.String(), "/") + "/api/v1"
}

// Ping hits GET /health (no auth) to verify the server is reachable. The health
// route lives outside /api/v1, so it builds the URL from the host root.
func (c *Client) Ping(ctx context.Context) error {
	hostURL, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("parse base url: %w", err)
	}
	hostURL.Path = "/health"
	hostURL.RawQuery = ""
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hostURL.String(), nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("perform request: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("health check failed: %s", resp.Status)
	}
	return nil
}

// CheckAuth hits GET /todos?page_size=1 to verify the API key works.
func (c *Client) CheckAuth(ctx context.Context) error {
	var result PaginatedTodosResponse
	return c.do(ctx, http.MethodGet, "/todos", map[string][]string{"page_size": {"1"}}, nil, &result)
}

// --- Todos -----------------------------------------------------------------

func (c *Client) ListTodos(ctx context.Context, query map[string][]string) (*PaginatedTodosResponse, error) {
	var result PaginatedTodosResponse
	if err := c.do(ctx, http.MethodGet, "/todos", query, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetTodo(ctx context.Context, id string) (*TodoDetail, error) {
	var result TodoDetail
	if err := c.do(ctx, http.MethodGet, "/todos/"+id, nil, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) CreateTodo(ctx context.Context, body map[string]any) (*Todo, error) {
	var result Todo
	if err := c.do(ctx, http.MethodPost, "/todos", nil, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) UpdateTodo(ctx context.Context, id string, body map[string]any) (*Todo, error) {
	var result Todo
	if err := c.do(ctx, http.MethodPatch, "/todos/"+id, nil, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteTodo(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/todos/"+id, nil, nil, nil)
}

func (c *Client) StartTodo(ctx context.Context, id string) (*Todo, error) {
	var result Todo
	if err := c.do(ctx, http.MethodPost, "/todos/"+id+"/start", nil, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) CompleteTodo(ctx context.Context, id string, cascade bool) (*Todo, error) {
	var result Todo
	body := map[string]any{"cascade_dependencies": cascade}
	if err := c.do(ctx, http.MethodPost, "/todos/"+id+"/complete", nil, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ReopenTodo(ctx context.Context, id string, cascade bool) (*Todo, error) {
	var result Todo
	body := map[string]any{"cascade_dependents": cascade}
	if err := c.do(ctx, http.MethodPost, "/todos/"+id+"/reopen", nil, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) SetTodoStatus(ctx context.Context, id, status string) (*Todo, error) {
	var result Todo
	body := map[string]any{"status": status}
	if err := c.do(ctx, http.MethodPatch, "/todos/"+id+"/status", nil, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) PinTodo(ctx context.Context, id string, pinned bool) (*Todo, error) {
	var result Todo
	body := map[string]any{"pinned": pinned}
	if err := c.do(ctx, http.MethodPatch, "/todos/"+id+"/pin", nil, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) HighlightTodo(ctx context.Context, id string, highlighted bool) (*Todo, error) {
	var result Todo
	body := map[string]any{"highlighted": highlighted}
	if err := c.do(ctx, http.MethodPatch, "/todos/"+id+"/highlight", nil, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ListTags(ctx context.Context) ([]string, error) {
	var result []string
	if err := c.do(ctx, http.MethodGet, "/todos/tags", nil, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// --- Comments --------------------------------------------------------------

func (c *Client) ListComments(ctx context.Context, todoID string) ([]Comment, error) {
	var result []Comment
	if err := c.do(ctx, http.MethodGet, "/todos/"+todoID+"/comments", nil, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) CreateComment(ctx context.Context, todoID, content string) (*Comment, error) {
	var result Comment
	body := map[string]any{"content": content}
	if err := c.do(ctx, http.MethodPost, "/todos/"+todoID+"/comments", nil, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteComment(ctx context.Context, todoID, commentID string) error {
	return c.do(ctx, http.MethodDelete, "/todos/"+todoID+"/comments/"+commentID, nil, nil, nil)
}

// --- core request ----------------------------------------------------------

func (c *Client) do(ctx context.Context, method, endpoint string, query map[string][]string, body any, out any) error {
	requestURL, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("parse base url: %w", err)
	}
	requestURL.Path = path.Clean(requestURL.Path + endpoint)
	if len(query) > 0 {
		values := requestURL.Query()
		for key, items := range query {
			for _, item := range items {
				values.Add(key, item)
			}
		}
		requestURL.RawQuery = values.Encode()
	}

	var payload io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		payload = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL.String(), payload)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeAPIError(resp.StatusCode, data)
	}
	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func decodeAPIError(status int, data []byte) error {
	apiErr := &APIError{Status: status}

	// 409 carries the conflict detail.
	if status == http.StatusConflict {
		var conf ConflictResponse
		if err := json.Unmarshal(data, &conf); err == nil && conf.Error != "" {
			apiErr.Message = conf.Error
			apiErr.Conflict = &conf
			return apiErr
		}
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(data, &errResp); err == nil && errResp.Error != "" {
		apiErr.Message = errResp.Error
		return apiErr
	}
	apiErr.Message = http.StatusText(status)
	return apiErr
}
