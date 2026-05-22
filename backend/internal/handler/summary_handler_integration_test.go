package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graydovee/todolist/internal/middleware"
	"github.com/graydovee/todolist/internal/model"
	"github.com/labstack/echo/v4"
)

// Integration tests for end-to-end SSE flow and language parameter pass-through.
// **Validates: Requirements 1.1, 1.2, 1.3, 5.1**

// TestIntegration_SSEMultilineContent verifies that multi-line content is correctly
// formatted as SSE events using writeSSEData, and that parsing the output
// reconstructs the original content.
func TestIntegration_SSEMultilineContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "single line",
			content: "Hello, world!",
		},
		{
			name:    "two lines",
			content: "Line one\nLine two",
		},
		{
			name:    "markdown with headings and lists",
			content: "## Summary\n\n- Item 1\n- Item 2\n\n### Details\n\nSome text here.",
		},
		{
			name:    "consecutive newlines (blank lines)",
			content: "Paragraph 1\n\n\nParagraph 2",
		},
		{
			name:    "unicode content with newlines",
			content: "## 数据分析报告\n\n### 概述\n\n本周完成了5个任务。\n\n### 详情\n\n- 任务1：完成\n- 任务2：进行中",
		},
		{
			name:    "code block in markdown",
			content: "```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```",
		},
		{
			name:    "empty string",
			content: "",
		},
		{
			name:    "only newlines",
			content: "\n\n\n",
		},
		{
			name:    "trailing newline",
			content: "content\n",
		},
		{
			name:    "leading newline",
			content: "\ncontent",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Write content using writeSSEData
			var buf bytes.Buffer
			writeSSEData(&buf, tc.content)
			output := buf.String()

			// Verify output ends with "\n\n" (event terminator)
			if !strings.HasSuffix(output, "\n\n") {
				t.Fatalf("SSE output should end with '\\n\\n', got: %q", output)
			}

			// Parse the SSE output: extract text after "data: " prefix from each line
			// Remove the trailing empty line (event terminator)
			withoutTerminator := output[:len(output)-1]
			rawLines := strings.Split(withoutTerminator, "\n")

			// Remove trailing empty element from split
			if len(rawLines) > 0 && rawLines[len(rawLines)-1] == "" {
				rawLines = rawLines[:len(rawLines)-1]
			}

			// Extract content after "data: " prefix
			var parsedLines []string
			for _, line := range rawLines {
				if !strings.HasPrefix(line, "data: ") {
					t.Fatalf("each SSE line should start with 'data: ', got: %q\nfull output: %q", line, output)
				}
				parsedLines = append(parsedLines, strings.TrimPrefix(line, "data: "))
			}

			// Reconstruct original content by joining with "\n"
			reconstructed := strings.Join(parsedLines, "\n")

			// Verify round-trip
			if reconstructed != tc.content {
				t.Fatalf("round-trip mismatch:\n  original:      %q\n  reconstructed: %q\n  SSE output:    %q",
					tc.content, reconstructed, output)
			}
		})
	}
}

// TestIntegration_SSEMultilineContentEachLineHasDataPrefix verifies that every line
// in the SSE output for multi-line content has the "data: " prefix, per SSE spec.
func TestIntegration_SSEMultilineContentEachLineHasDataPrefix(t *testing.T) {
	// Content with 5 lines including an empty line
	content := "## Title\n\nParagraph text\n- item 1\n- item 2"
	expectedLines := strings.Split(content, "\n")

	var buf bytes.Buffer
	writeSSEData(&buf, content)
	output := buf.String()

	// Remove the trailing "\n" (event terminator)
	withoutTerminator := output[:len(output)-1]
	rawLines := strings.Split(withoutTerminator, "\n")
	if len(rawLines) > 0 && rawLines[len(rawLines)-1] == "" {
		rawLines = rawLines[:len(rawLines)-1]
	}

	// Verify we have the correct number of data lines
	if len(rawLines) != len(expectedLines) {
		t.Fatalf("expected %d data lines, got %d\noutput: %q", len(expectedLines), len(rawLines), output)
	}

	// Verify each line has "data: " prefix
	for i, line := range rawLines {
		if !strings.HasPrefix(line, "data: ") {
			t.Fatalf("line %d missing 'data: ' prefix: %q", i, line)
		}
		// Verify content matches
		extracted := strings.TrimPrefix(line, "data: ")
		if extracted != expectedLines[i] {
			t.Fatalf("line %d content mismatch: expected %q, got %q", i, expectedLines[i], extracted)
		}
	}
}

// TestIntegration_LanguageParameterPassThrough verifies that the CreateSummary handler
// accepts valid language values ("Chinese", "English", "") without returning a language
// validation error. Since the handler calls the service after validation, we verify
// by checking that the handler does NOT return the specific language validation error.
// A panic or other error means validation passed (the service is nil in tests).
func TestIntegration_LanguageParameterPassThrough(t *testing.T) {
	tests := []struct {
		name     string
		language string
	}{
		{
			name:     "language Chinese is accepted",
			language: "Chinese",
		},
		{
			name:     "language English is accepted",
			language: "English",
		},
		{
			name:     "empty language is accepted",
			language: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Build request body with valid dates and the language value
			var reqBody string
			if tc.language == "" {
				reqBody = `{"start_date":"2024-01-01","end_date":"2024-01-31"}`
			} else {
				reqBody = `{"start_date":"2024-01-01","end_date":"2024-01-31","language":"` + tc.language + `"}`
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/summaries", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			e := echo.New()
			c := e.NewContext(req, rec)
			c.Set(middleware.UserContextKey, &model.User{ID: 1})

			// Create handler with nil service. The handler will pass language validation
			// and then panic when calling the nil service. We recover from the panic
			// to verify that language validation did NOT reject the request.
			h := &SummaryHandler{summaryService: nil}

			func() {
				defer func() {
					if r := recover(); r != nil {
						// Panic means we passed validation and reached the service call.
						// This is the expected behavior for valid language values.
					}
				}()
				_ = h.Create(c)
			}()

			// If the handler returned a response (didn't panic), check it's not a
			// language validation error
			body := rec.Body.String()
			if strings.Contains(body, "invalid language value") {
				t.Fatalf("language %q should be accepted but got language validation error: %s",
					tc.language, body)
			}
		})
	}
}

// TestIntegration_LanguageParameterRejection verifies that invalid language values
// are rejected with HTTP 400.
func TestIntegration_LanguageParameterRejection(t *testing.T) {
	invalidLanguages := []string{
		"chinese", // lowercase
		"english", // lowercase
		"French",  // unsupported
		"中文",      // Chinese characters
		"CHINESE", // uppercase
		"Spanish", // unsupported
		"auto",    // not a valid value
		"zh",      // frontend value, not backend value
		"en",      // frontend value, not backend value
	}

	for _, lang := range invalidLanguages {
		t.Run("reject_"+lang, func(t *testing.T) {
			reqBody := `{"start_date":"2024-01-01","end_date":"2024-01-31","language":"` + lang + `"}`

			req := httptest.NewRequest(http.MethodPost, "/api/v1/summaries", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			e := echo.New()
			c := e.NewContext(req, rec)
			c.Set(middleware.UserContextKey, &model.User{ID: 1})

			h := &SummaryHandler{summaryService: nil}
			_ = h.Create(c)

			// Should return 400 with language validation error
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400 for language %q, got %d", lang, rec.Code)
			}

			body := rec.Body.String()
			if !strings.Contains(body, "invalid language value") {
				t.Fatalf("expected language validation error for %q, got: %s", lang, body)
			}
		})
	}
}

// TestIntegration_SSEFullStreamFlow simulates the full SSE handler streaming flow:
// multi-line content chunks are sent through a channel, formatted as SSE events,
// and the output is verified to be parseable back to the original content.
func TestIntegration_SSEFullStreamFlow(t *testing.T) {
	// Simulate a realistic LLM streaming scenario with multi-line chunks
	chunks := []string{
		"## 工作总结\n\n",
		"### 本周完成\n\n- 完成了用户认证模块\n- 修复了3个bug\n",
		"### 进行中\n\n- 数据库优化\n- API文档更新\n",
		"\n### 建议\n\n1. 增加单元测试覆盖率\n2. 优化CI/CD流程\n",
	}

	// Write each chunk as an SSE event using writeSSEData
	var fullOutput bytes.Buffer
	for _, chunk := range chunks {
		writeSSEData(&fullOutput, chunk)
	}
	// Write done event
	fullOutput.WriteString("event: done\ndata: \n\n")

	output := fullOutput.String()

	// Verify the output contains the done event
	if !strings.Contains(output, "event: done\ndata: \n\n") {
		t.Fatalf("output should contain done event")
	}

	// Parse all data events (excluding the done event)
	// Split by the event terminator pattern and extract data events
	var reconstructedChunks []string
	remaining := output

	for len(remaining) > 0 {
		// Skip event: lines (done/error events)
		if strings.HasPrefix(remaining, "event: ") {
			idx := strings.Index(remaining, "\n\n")
			if idx == -1 {
				break
			}
			remaining = remaining[idx+2:]
			continue
		}

		// Look for a data event block (one or more "data: " lines followed by empty line)
		if strings.HasPrefix(remaining, "data: ") {
			// Find the event terminator (empty line = "\n\n" after the last data line)
			// But we need to handle multi-line data events where each line starts with "data: "
			var dataLines []string
			for strings.HasPrefix(remaining, "data: ") {
				// Find end of this line
				nlIdx := strings.Index(remaining, "\n")
				if nlIdx == -1 {
					dataLines = append(dataLines, strings.TrimPrefix(remaining, "data: "))
					remaining = ""
					break
				}
				dataLines = append(dataLines, strings.TrimPrefix(remaining[:nlIdx], "data: "))
				remaining = remaining[nlIdx+1:]
			}
			// Skip the event terminator (empty line)
			if strings.HasPrefix(remaining, "\n") {
				remaining = remaining[1:]
			}
			// Reconstruct the chunk content
			reconstructedChunks = append(reconstructedChunks, strings.Join(dataLines, "\n"))
			continue
		}

		// Skip unexpected content
		remaining = remaining[1:]
	}

	// Verify we got the right number of chunks
	if len(reconstructedChunks) != len(chunks) {
		t.Fatalf("expected %d chunks, got %d\nreconstructed: %v\noutput:\n%s",
			len(chunks), len(reconstructedChunks), reconstructedChunks, output)
	}

	// Verify each chunk matches
	for i, expected := range chunks {
		if reconstructedChunks[i] != expected {
			t.Fatalf("chunk %d mismatch:\n  expected: %q\n  actual:   %q",
				i, expected, reconstructedChunks[i])
		}
	}
}
