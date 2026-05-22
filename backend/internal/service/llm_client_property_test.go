package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graydovee/todolist/internal/config"
	"pgregory.net/rapid"
)

// Feature: ai-summary, Property 4: Error messages never expose the API key
// **Validates: Requirements 7.5**
//
// Property: For any LLM API failure (network error, timeout, non-2xx response)
// and for any configured api_key value, the error message returned by the LLM
// service SHALL NOT contain the api_key string.
func TestProperty_ErrorMessagesNeverExposeAPIKey(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random non-empty API key
		apiKey := rapid.StringMatching(`[a-zA-Z0-9\-_]{5,50}`).Draw(rt, "apiKey")

		// Choose an error scenario
		scenario := rapid.IntRange(0, 2).Draw(rt, "scenario")

		var errResult error

		switch scenario {
		case 0:
			// Scenario: non-2xx response with body containing the API key
			statusCode := rapid.SampledFrom([]int{
				http.StatusBadRequest,
				http.StatusUnauthorized,
				http.StatusForbidden,
				http.StatusNotFound,
				http.StatusTooManyRequests,
				http.StatusInternalServerError,
				http.StatusBadGateway,
				http.StatusServiceUnavailable,
			}).Draw(rt, "statusCode")

			// Create a test server that returns a non-2xx status with body that may contain the API key
			includeKeyInBody := rapid.Bool().Draw(rt, "includeKeyInBody")
			var responseBody string
			if includeKeyInBody {
				responseBody = fmt.Sprintf(`{"error": "invalid key: %s"}`, apiKey)
			} else {
				responseBody = `{"error": "something went wrong"}`
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(statusCode)
				w.Write([]byte(responseBody))
			}))
			defer server.Close()

			client := NewLLMClient(&config.LLMConfig{
				Model:   "test-model",
				BaseURL: server.URL,
				APIKey:  apiKey,
				Timeout: 5,
			})

			_, errResult = client.ChatCompletion(context.Background(), []ChatMessage{
				{Role: "user", Content: "hello"},
			})

		case 1:
			// Scenario: network error (server immediately closes connection)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Close the connection abruptly by hijacking
				hj, ok := w.(http.Hijacker)
				if ok {
					conn, _, _ := hj.Hijack()
					conn.Close()
				}
			}))
			defer server.Close()

			client := NewLLMClient(&config.LLMConfig{
				Model:   "test-model",
				BaseURL: server.URL,
				APIKey:  apiKey,
				Timeout: 5,
			})

			_, errResult = client.ChatCompletion(context.Background(), []ChatMessage{
				{Role: "user", Content: "hello"},
			})

		case 2:
			// Scenario: server returns invalid URL (connection refused)
			client := NewLLMClient(&config.LLMConfig{
				Model:   "test-model",
				BaseURL: "http://127.0.0.1:1", // port 1 is unlikely to be open
				APIKey:  apiKey,
				Timeout: 2,
			})

			_, errResult = client.ChatCompletion(context.Background(), []ChatMessage{
				{Role: "user", Content: "hello"},
			})
		}

		// The call must have returned an error
		if errResult == nil {
			rt.Fatalf("expected an error for scenario %d, got nil", scenario)
		}

		// The error message must NOT contain the API key
		errMsg := errResult.Error()
		if strings.Contains(errMsg, apiKey) {
			rt.Fatalf("error message exposes API key!\n  api_key: %q\n  error:   %q\n  scenario: %d",
				apiKey, errMsg, scenario)
		}
	})
}
