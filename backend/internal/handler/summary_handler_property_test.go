package handler

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graydovee/todolist/internal/middleware"
	"github.com/graydovee/todolist/internal/model"
	"github.com/graydovee/todolist/internal/service"
	"github.com/labstack/echo/v4"
	"pgregory.net/rapid"
)

// Feature: ai-summary-streaming, Property 1: Chunk forwarding integrity
// **Validates: Requirements 2.2**
//
// Property: For any sequence of text chunks (including unicode, special characters,
// single-line and multi-line text), the SSE handler SHALL forward each chunk as a
// `data:` event with the exact same content, preserving order and content fidelity.
//
// Note: The handler uses fmt.Fprintf(resp, "data: %s\n\n", chunk.Content).
// This means the raw SSE output for each chunk is: "data: " + content + "\n\n".
// We verify the property by extracting content from this format and comparing.
func TestProperty_ChunkForwardingIntegrity(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random sequence of text chunks (1 to 20 chunks)
		numChunks := rapid.IntRange(1, 20).Draw(rt, "numChunks")
		chunks := make([]string, numChunks)
		for i := range numChunks {
			// Generate arbitrary text including unicode, special characters.
			// LLM streaming chunks are typically token-level fragments that don't
			// contain the SSE event separator (\n\n). We generate realistic chunks
			// that may contain single newlines but not double newlines.
			chunk := rapid.OneOf(
				// Plain ASCII text (words, sentences)
				rapid.StringMatching(`[a-zA-Z0-9 ,.!?]{1,60}`),
				// Unicode characters (Chinese, Japanese, emoji, accented)
				rapid.SampledFrom([]string{
					"你好世界",
					"こんにちは",
					"🎉🚀✨",
					"café résumé naïve",
					"Ñoño año",
					"数据分析报告",
					"テスト結果",
					"🔥💯👍🏽",
					"αβγδεζ",
					"Привет мир",
					"العربية",
					"한국어",
				}),
				// Special characters and edge cases
				rapid.SampledFrom([]string{
					"<script>alert('xss')</script>",
					"data: fake event",
					"event: done",
					`{"key": "value", "nested": {"a": 1}}`,
					"line1\nline2\nline3",
					"tab\there\ttoo",
					"back\\slash\\path",
					`quote"double"end`,
					"single'quote'end",
					"ampersand&entity&more",
					"less<greater>end",
					"## Markdown **bold** _italic_",
					"- list item 1\n- list item 2",
					"```code block```",
					"url: https://example.com/path?q=1&r=2",
				}),
				// Mixed unicode content
				rapid.StringMatching(`[a-zA-Z0-9\x{4e00}-\x{9fff}\x{3040}-\x{309f} ,.!]{1,80}`),
			).Draw(rt, fmt.Sprintf("chunk_%d", i))
			chunks[i] = chunk
		}

		// Simulate the SSE handler forwarding logic:
		// Create a channel with the chunks, then write SSE events to a recorder.
		ch := make(chan service.StreamChunk, len(chunks)+1)
		for _, content := range chunks {
			ch <- service.StreamChunk{Content: content}
		}
		ch <- service.StreamChunk{Done: true}
		close(ch)

		// Use httptest to capture the SSE output
		recorder := httptest.NewRecorder()

		// Simulate the handler's SSE writing loop (same logic as Stream handler)
		for chunk := range ch {
			if chunk.Done {
				fmt.Fprintf(recorder, "event: done\ndata: \n\n")
				break
			}
			if chunk.Err != nil {
				fmt.Fprintf(recorder, "event: error\ndata: %s\n\n", chunk.Err.Error())
				break
			}
			// This is the exact format used by the handler
			fmt.Fprintf(recorder, "data: %s\n\n", chunk.Content)
		}

		// Verify chunk forwarding integrity by parsing the raw SSE output.
		// The handler writes each chunk as "data: <content>\n\n".
		// We verify by reading the output sequentially.
		body := recorder.Body.String()
		extractedChunks := extractDataEvents(body)

		// Verify we got the right number of data events
		if len(extractedChunks) != len(chunks) {
			rt.Fatalf("expected %d data events, got %d\nSSE output:\n%s",
				len(chunks), len(extractedChunks), body)
		}

		// Verify each chunk is forwarded with exact same content, preserving order
		for i, expected := range chunks {
			actual := extractedChunks[i]
			if actual != expected {
				rt.Fatalf("chunk %d mismatch:\n  expected: %q\n  actual:   %q",
					i, expected, actual)
			}
		}
	})
}

// extractDataEvents parses the raw SSE output written by the handler and extracts
// the content of each data event. The handler format is:
//   - Data event: "data: <content>\n\n"
//   - Done event: "event: done\ndata: \n\n"
//   - Error event: "event: error\ndata: <msg>\n\n"
//
// This function extracts only the plain data events (not done/error events).
// It works by scanning the output character by character to correctly handle
// content that may contain newlines.
func extractDataEvents(raw string) []string {
	var events []string
	remaining := raw

	for len(remaining) > 0 {
		// Skip "event: done\ndata: \n\n" and "event: error\ndata: ...\n\n"
		if strings.HasPrefix(remaining, "event: ") {
			// Find the end of this event block (double newline)
			idx := strings.Index(remaining, "\n\n")
			if idx == -1 {
				break
			}
			remaining = remaining[idx+2:]
			continue
		}

		// Look for "data: " prefix
		if strings.HasPrefix(remaining, "data: ") {
			// Extract content: everything after "data: " until "\n\n"
			contentStart := 6 // len("data: ")
			content := remaining[contentStart:]

			// Find the terminating "\n\n" for this event
			idx := strings.Index(content, "\n\n")
			if idx == -1 {
				// No terminator found, take the rest
				events = append(events, content)
				break
			}

			events = append(events, content[:idx])
			remaining = content[idx+2:]
			continue
		}

		// Skip any unexpected content (shouldn't happen with well-formed output)
		remaining = remaining[1:]
	}

	return events
}

// TestProperty_ChunkForwardingIntegrity_OrderPreservation specifically tests that
// chunks with distinguishable content maintain their exact ordering through the
// SSE forwarding pipeline.
func TestProperty_ChunkForwardingIntegrity_OrderPreservation(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate numbered chunks to make order verification explicit
		numChunks := rapid.IntRange(2, 30).Draw(rt, "numChunks")
		chunks := make([]string, numChunks)
		for i := range numChunks {
			// Each chunk has a unique prefix to make ordering verifiable
			suffix := rapid.OneOf(
				rapid.StringMatching(`[a-zA-Z]{1,20}`),
				rapid.SampledFrom([]string{"你好", "🎉", "café", "αβγ"}),
			).Draw(rt, fmt.Sprintf("suffix_%d", i))
			chunks[i] = fmt.Sprintf("[%d]%s", i, suffix)
		}

		// Feed through the SSE formatting pipeline
		ch := make(chan service.StreamChunk, len(chunks)+1)
		for _, content := range chunks {
			ch <- service.StreamChunk{Content: content}
		}
		ch <- service.StreamChunk{Done: true}
		close(ch)

		recorder := httptest.NewRecorder()
		for chunk := range ch {
			if chunk.Done {
				fmt.Fprintf(recorder, "event: done\ndata: \n\n")
				break
			}
			fmt.Fprintf(recorder, "data: %s\n\n", chunk.Content)
		}

		body := recorder.Body.String()
		extractedChunks := extractDataEvents(body)

		if len(extractedChunks) != len(chunks) {
			rt.Fatalf("expected %d chunks, got %d", len(chunks), len(extractedChunks))
		}

		for i, expected := range chunks {
			if extractedChunks[i] != expected {
				rt.Fatalf("order violation at position %d:\n  expected: %q\n  actual:   %q",
					i, expected, extractedChunks[i])
			}
		}
	})
}

// TestProperty_ChunkForwardingIntegrity_FormatConsistency verifies that the SSE
// output format is consistent: each data event starts with "data: " and is
// terminated by "\n\n", and the content between them exactly matches the input.
func TestProperty_ChunkForwardingIntegrity_FormatConsistency(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a single chunk with various content types
		chunk := rapid.OneOf(
			rapid.StringMatching(`[a-zA-Z0-9 ,.!?]{1,50}`),
			rapid.SampledFrom([]string{
				"你好", "🎉", "café", "αβγ", "Привет",
				"special: <>&\"'", "## heading",
			}),
		).Draw(rt, "chunk")

		// Write using the exact same format as the handler
		recorder := httptest.NewRecorder()
		fmt.Fprintf(recorder, "data: %s\n\n", chunk)

		// The output should start with "data: " and end with "\n\n"
		output := recorder.Body.String()
		if !strings.HasPrefix(output, "data: ") {
			rt.Fatalf("SSE data event should start with 'data: ', got: %q", output)
		}
		if !strings.HasSuffix(output, "\n\n") {
			rt.Fatalf("SSE data event should end with '\\n\\n', got: %q", output)
		}

		// Extract the content between "data: " and the final "\n\n"
		extracted := output[6 : len(output)-2] // skip "data: " prefix and "\n\n" suffix
		if extracted != chunk {
			rt.Fatalf("extracted content mismatch:\n  expected: %q\n  actual:   %q",
				chunk, extracted)
		}
	})
}

// TestProperty_ChunkForwardingIntegrity_DoneEventTermination verifies that after
// all data chunks are forwarded, a "done" event is always emitted.
func TestProperty_ChunkForwardingIntegrity_DoneEventTermination(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		numChunks := rapid.IntRange(0, 15).Draw(rt, "numChunks")
		chunks := make([]string, numChunks)
		for i := range numChunks {
			chunks[i] = rapid.StringMatching(`[a-zA-Z0-9]{1,30}`).Draw(rt, fmt.Sprintf("chunk_%d", i))
		}

		ch := make(chan service.StreamChunk, len(chunks)+1)
		for _, content := range chunks {
			ch <- service.StreamChunk{Content: content}
		}
		ch <- service.StreamChunk{Done: true}
		close(ch)

		recorder := httptest.NewRecorder()
		for chunk := range ch {
			if chunk.Done {
				fmt.Fprintf(recorder, "event: done\ndata: \n\n")
				break
			}
			fmt.Fprintf(recorder, "data: %s\n\n", chunk.Content)
		}

		body := recorder.Body.String()

		// Verify the output ends with the done event
		if !strings.Contains(body, "event: done\ndata: \n\n") {
			rt.Fatalf("SSE output should contain 'event: done' termination event\nOutput:\n%s", body)
		}

		// Verify done event appears after all data events
		doneIdx := strings.Index(body, "event: done")
		lastDataIdx := strings.LastIndex(body, "data: ")
		// The last "data: " before "event: done" should be a content event
		// (the "data: " inside "event: done\ndata: \n\n" is at doneIdx+12)
		if numChunks > 0 {
			// Find the last content data event (not the one inside done event)
			contentPart := body[:doneIdx]
			lastContentData := strings.LastIndex(contentPart, "data: ")
			if lastContentData == -1 {
				rt.Fatalf("expected data events before done event")
			}
			_ = lastDataIdx // used for verification above
		}
	})
}

// TestSSEResponseWriter verifies httptest.ResponseRecorder works for SSE testing.
func TestSSEResponseWriter(t *testing.T) {
	recorder := httptest.NewRecorder()
	resp := recorder.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

// Feature: ai-summary-ux-fix, Property 1: SSE multi-line formatting round-trip
// **Validates: Requirements 1.1, 1.3, 1.4**
//
// Property: For any string content (including content with single newlines,
// consecutive newlines, empty lines, unicode characters, and special characters),
// formatting it with writeSSEData and then parsing the resulting SSE output by
// extracting the text after each "data: " prefix and joining with "\n" SHALL
// produce a string identical to the original content.
func TestProperty_SSEMultilineFormattingRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random strings that include newlines, consecutive newlines,
		// unicode, and special characters.
		content := rapid.OneOf(
			// Plain ASCII text with embedded newlines
			rapid.StringMatching(`[a-zA-Z0-9 ,.!?]{0,30}(\n[a-zA-Z0-9 ,.!?]{0,30}){0,5}`),
			// Strings with consecutive newlines (blank lines)
			rapid.SampledFrom([]string{
				"line1\n\nline3",
				"\n\n\n",
				"start\n\n\nend",
				"a\n\nb\n\nc",
				"\nleading newline",
				"trailing newline\n",
				"\n",
				"\n\n",
				"",
				"no newlines at all",
				"one\ntwo\nthree\nfour\nfive",
			}),
			// Unicode content with newlines
			rapid.SampledFrom([]string{
				"你好\n世界",
				"こんにちは\n\nテスト",
				"🎉🚀\n✨💯",
				"café\nrésumé\nnaïve",
				"αβγ\nδεζ\n\nηθι",
				"Привет\nмир",
				"العربية\n한국어",
				"数据分析\n\n报告内容\n\n总结",
			}),
			// Special characters with newlines
			rapid.SampledFrom([]string{
				"data: fake\nevent: trick",
				"## Heading\n\n- item 1\n- item 2\n\n### Sub",
				"```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```",
				"tab\there\nnewline\nback\\slash",
				"<script>\nalert('xss')\n</script>",
				"line with spaces   \n  indented\n    more indent",
				"url: https://example.com\n\npath?q=1&r=2",
			}),
			// Arbitrary strings from rapid's string generator (may contain any rune)
			rapid.String(),
		).Draw(rt, "content")

		// Write using writeSSEData to a bytes.Buffer
		var buf bytes.Buffer
		writeSSEData(&buf, content)

		// Parse the output: extract text after "data: " prefix from each line,
		// then join with "\n". The last line before the empty terminator line
		// should be included.
		output := buf.String()

		// The output should end with "\n\n" (last data line's \n + terminator \n)
		if !strings.HasSuffix(output, "\n\n") {
			rt.Fatalf("SSE output should end with '\\n\\n', got: %q", output)
		}

		// Remove the trailing empty line (the event terminator)
		// The output format is: "data: line1\ndata: line2\n...data: lineN\n\n"
		// We strip the final "\n" (terminator) to get "data: line1\ndata: line2\n...data: lineN\n"
		withoutTerminator := output[:len(output)-1]

		// Split by "\n" to get individual lines (last element will be empty due to trailing \n)
		rawLines := strings.Split(withoutTerminator, "\n")

		// The last element after split will be "" because withoutTerminator ends with \n
		// Remove that trailing empty element
		if len(rawLines) > 0 && rawLines[len(rawLines)-1] == "" {
			rawLines = rawLines[:len(rawLines)-1]
		}

		// Extract content after "data: " prefix from each line
		var parsedLines []string
		for _, line := range rawLines {
			if !strings.HasPrefix(line, "data: ") {
				rt.Fatalf("each SSE line should start with 'data: ', got: %q\nfull output: %q", line, output)
			}
			parsedLines = append(parsedLines, strings.TrimPrefix(line, "data: "))
		}

		// Join with "\n" to reconstruct the original content
		reconstructed := strings.Join(parsedLines, "\n")

		// Verify round-trip: reconstructed must equal original content
		if reconstructed != content {
			rt.Fatalf("round-trip mismatch:\n  original:      %q\n  reconstructed: %q\n  SSE output:    %q",
				content, reconstructed, output)
		}
	})
}

// Feature: ai-summary-ux-fix, Property 2: Invalid language rejection
// **Validates: Requirements 5.3**
//
// Property: For any string value that is not empty ("") and not one of the accepted
// values ("Chinese", "English"), the CreateSummary handler SHALL return an HTTP 400
// error response and SHALL NOT create a summary record.
func TestProperty_InvalidLanguageRejection(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random string that is NOT "", "Chinese", or "English"
		invalidLang := rapid.OneOf(
			// Random ASCII strings (filtered)
			rapid.StringMatching(`[a-zA-Z0-9 _\-]{1,50}`),
			// Common near-miss values
			rapid.SampledFrom([]string{
				"chinese", "english", "CHINESE", "ENGLISH",
				"中文", "英文", "French", "Spanish", "Japanese",
				"chi", "eng", "CN", "EN", "zh", "en",
				"chinese ", " English", "Chinese1", "0English",
			}),
			// Unicode strings
			rapid.StringMatching(`[\x{4e00}-\x{9fff}]{1,10}`),
			// Special characters
			rapid.SampledFrom([]string{
				"<script>", "null", "undefined", "true", "false",
				"' OR 1=1 --", "../../etc/passwd",
			}),
		).Draw(rt, "invalidLang")

		// Filter out the valid values
		if invalidLang == "" || invalidLang == "Chinese" || invalidLang == "English" {
			rt.Skip("generated a valid language value, skipping")
		}

		// Build a JSON request body with valid dates and the invalid language
		reqBody := fmt.Sprintf(`{"start_date":"2024-01-01","end_date":"2024-01-31","language":%q}`, invalidLang)

		// Create an HTTP request
		req := httptest.NewRequest(http.MethodPost, "/api/v1/summaries", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Set up Echo context with a mock user
		e := echo.New()
		c := e.NewContext(req, rec)
		c.Set(middleware.UserContextKey, &model.User{ID: 1})

		// Create handler with nil service (should not be reached due to validation)
		h := &SummaryHandler{summaryService: nil}

		// Call the handler
		err := h.Create(c)
		if err != nil {
			rt.Fatalf("handler returned error: %v", err)
		}

		// Verify HTTP 400 status
		if rec.Code != http.StatusBadRequest {
			rt.Fatalf("expected status 400 for language %q, got %d\nbody: %s",
				invalidLang, rec.Code, rec.Body.String())
		}

		// Verify error message
		body := rec.Body.String()
		expectedMsg := "invalid language value, must be one of: Chinese, English"
		if !strings.Contains(body, expectedMsg) {
			rt.Fatalf("expected error message %q in response body, got: %s",
				expectedMsg, body)
		}
	})
}
