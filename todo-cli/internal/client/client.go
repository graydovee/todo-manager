package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type APIError struct {
	Status  int         `json:"status"`
	Message string      `json:"error"`
	Details interface{} `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("request failed with status %d", e.Status)
}

func New(baseURL, apiKey string) *Client {
	return NewWithHTTPClient(baseURL, apiKey, &http.Client{
		Timeout: 30 * time.Second,
	})
}

func NewWithHTTPClient(baseURL, apiKey string, httpClient *http.Client) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/") + "/api/v1",
		apiKey:  apiKey,
		httpClient: httpClient,
	}
}

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

func (c *Client) DeleteTodo(ctx context.Context, id string) (map[string]string, error) {
	var result map[string]string
	if err := c.do(ctx, http.MethodDelete, "/todos/"+id, nil, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) StartTodo(ctx context.Context, id string) (map[string]string, error) {
	var result map[string]string
	if err := c.do(ctx, http.MethodPost, "/todos/"+id+"/start", nil, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) SetTodoStatus(ctx context.Context, id string, body map[string]any) (map[string]string, error) {
	var result map[string]string
	if err := c.do(ctx, http.MethodPatch, "/todos/"+id+"/status", nil, body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) CompleteTodo(ctx context.Context, id string, body map[string]any) (interface{}, error) {
	var result interface{}
	if err := c.do(ctx, http.MethodPost, "/todos/"+id+"/complete", nil, body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) ReopenTodo(ctx context.Context, id string, body map[string]any) (interface{}, error) {
	var result interface{}
	if err := c.do(ctx, http.MethodPost, "/todos/"+id+"/reopen", nil, body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) PinTodo(ctx context.Context, id string, body map[string]any) (*Todo, error) {
	var result Todo
	if err := c.do(ctx, http.MethodPatch, "/todos/"+id+"/pin", nil, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) HighlightTodo(ctx context.Context, id string, body map[string]any) (*Todo, error) {
	var result Todo
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

func (c *Client) GetGraph(ctx context.Context) (*TodoGraphResponse, error) {
	var result TodoGraphResponse
	if err := c.do(ctx, http.MethodGet, "/todos/graph", nil, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ListByDateRange(ctx context.Context, query map[string][]string) ([]TodoByDateRangeItem, error) {
	var result []TodoByDateRangeItem
	if err := c.do(ctx, http.MethodGet, "/todos/by-date-range", query, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) ListComments(ctx context.Context, todoID string) ([]Comment, error) {
	var result []Comment
	if err := c.do(ctx, http.MethodGet, "/todos/"+todoID+"/comments", nil, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) CreateComment(ctx context.Context, todoID string, body map[string]any) (*Comment, error) {
	var result Comment
	if err := c.do(ctx, http.MethodPost, "/todos/"+todoID+"/comments", nil, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteComment(ctx context.Context, todoID, commentID string) (map[string]string, error) {
	var result map[string]string
	if err := c.do(ctx, http.MethodDelete, "/todos/"+todoID+"/comments/"+commentID, nil, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

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
	var details interface{}
	if len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, &details); err != nil {
			details = string(data)
		}
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(data, &errResp); err == nil && errResp.Error != "" {
		return &APIError{Status: status, Message: errResp.Error, Details: details}
	}
	return &APIError{Status: status, Message: http.StatusText(status), Details: details}
}
