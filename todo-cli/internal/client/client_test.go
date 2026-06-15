package client

import (
	"context"
	"io"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestClientAddsAuthorizationHeaderAndNormalizesURL(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("unexpected authorization header: %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/api/v1/todos" {
			t.Fatalf("unexpected path: %q", r.URL.Path)
		}
		body, _ := json.Marshal(PaginatedTodosResponse{})
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(body))),
		}, nil
	})}

	c := NewWithHTTPClient("http://example.com", "secret", httpClient)
	if _, err := c.ListTodos(context.Background(), nil); err != nil {
		t.Fatalf("list todos: %v", err)
	}
}

func TestClientMapsAPIError(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := json.Marshal(ErrorResponse{Error: "unauthorized"})
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(body))),
		}, nil
	})}

	c := NewWithHTTPClient("http://example.com", "secret", httpClient)
	_, err := c.ListTodos(context.Background(), nil)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Status != http.StatusUnauthorized || apiErr.Message != "unauthorized" {
		t.Fatalf("unexpected api error: %+v", apiErr)
	}
}
