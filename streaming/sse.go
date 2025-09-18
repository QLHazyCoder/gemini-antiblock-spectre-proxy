package streaming

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gemini-antiblock/logger"
)

// SSELineIterator reads SSE lines from a reader
func SSELineIterator(reader io.Reader, ch chan<- string) {
	defer close(ch)

	scanner := bufio.NewScanner(reader)
	lineCount := 0

	logger.LogDebug("Starting SSE line iteration")

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) != "" {
			lineCount++
			logger.LogDebug(fmt.Sprintf("SSE Line %d: %s", lineCount,
				func() string {
					if len(line) > 200 {
						return line[:200] + "..."
					}
					return line
				}()))
			ch <- line
		}
	}

	if err := scanner.Err(); err != nil {
		logger.LogError("Error reading SSE stream:", err)
	}

	logger.LogDebug(fmt.Sprintf("SSE stream ended. Total lines processed: %d", lineCount))
}

// IsDataLine checks if a line is a data line
func IsDataLine(line string) bool {
	return strings.HasPrefix(line, "data: ")
}

// IsBlockedLine checks if a line contains blocking information
func IsBlockedLine(line string) bool {
	return strings.Contains(line, "blockReason")
}

// ExtractFinishReason extracts finish reason from a line
func ExtractFinishReason(line string) string {
	if !strings.Contains(line, "finishReason") {
		return ""
	}

	idx := strings.Index(line, "{")
	if idx == -1 {
		return ""
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(line[idx:]), &data); err != nil {
		logger.LogDebug("Failed to extract finishReason from line:", err)
		return ""
	}

	if candidates, ok := data["candidates"].([]interface{}); ok && len(candidates) > 0 {
		if candidate, ok := candidates[0].(map[string]interface{}); ok {
			if finishReason, ok := candidate["finishReason"].(string); ok {
				logger.LogDebug("Extracted finishReason:", finishReason)
				return finishReason
			}
		}
	}

	return ""
}

// LineContent represents parsed content from a data line
type LineContent struct {
	Text      string
	IsThought bool
}

// ParseLineContent parses a data line to extract text content and thought status
func ParseLineContent(line string) LineContent {
	if !IsDataLine(line) {
		return LineContent{}
	}

	idx := strings.Index(line, "{")
	if idx == -1 {
		return LineContent{}
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(line[idx:]), &data); err != nil {
		logger.LogDebug("Failed to parse content from data line:", err)
		return LineContent{}
	}

	candidates, ok := data["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		return LineContent{}
	}

	candidate, ok := candidates[0].(map[string]interface{})
	if !ok {
		return LineContent{}
	}

	content, ok := candidate["content"].(map[string]interface{})
	if !ok {
		return LineContent{}
	}

	parts, ok := content["parts"].([]interface{})
	if !ok || len(parts) == 0 {
		return LineContent{}
	}

	part, ok := parts[0].(map[string]interface{})
	if !ok {
		return LineContent{}
	}

	text, _ := part["text"].(string)
	thought, _ := part["thought"].(bool)

	if thought {
		logger.LogDebug("Extracted thought chunk. This will be tracked.")
	} else if text != "" {
		logger.LogDebug(fmt.Sprintf("Extracted text chunk (%d chars): %s", len(text),
			func() string {
				if len(text) > 100 {
					return text[:100] + "..."
				}
				return text
			}()))
	}

	return LineContent{
		Text:      text,
		IsThought: thought,
	}
}

// RemoveDoneTokenFromLine removes [done] token from SSE data line if present
func RemoveDoneTokenFromLine(line string, shouldRemove bool) string {
	if !IsDataLine(line) || !shouldRemove {
		return line
	}

	idx := strings.Index(line, "{")
	if idx == -1 {
		return line
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(line[idx:]), &data); err != nil {
		logger.LogDebug("Failed to process line for [done] token removal:", err)
		return line
	}

	candidates, ok := data["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		return line
	}

	candidate, ok := candidates[0].(map[string]interface{})
	if !ok {
		return line
	}

	content, ok := candidate["content"].(map[string]interface{})
	if !ok {
		return line
	}

	parts, ok := content["parts"].([]interface{})
	if !ok || len(parts) == 0 {
		return line
	}

	part, ok := parts[0].(map[string]interface{})
	if !ok {
		return line
	}

	text, hasText := part["text"].(string)
	thought, _ := part["thought"].(bool)

	if !hasText || thought {
		return line
	}

	// Remove the longest suffix of "[done]" from the text
	// This handles cases where [done] is split across chunks
	originalText := strings.TrimSpace(text)
	modifiedText := originalText

	// Try to remove each possible suffix of "[done]"
	doneToken := "[done]"
	for i := len(doneToken); i > 0; i-- {
		suffix := doneToken[len(doneToken)-i:]
		if strings.HasSuffix(originalText, suffix) {
			modifiedText = strings.TrimSuffix(originalText, suffix)
			logger.LogDebug(fmt.Sprintf("Removed [done] token suffix '%s' from text content. Original length: %d, Modified length: %d", suffix, len(originalText), len(modifiedText)))
			break
		}
	}

	if originalText != modifiedText {
		part["text"] = modifiedText

		modifiedData, err := json.Marshal(data)
		if err != nil {
			logger.LogDebug("Failed to marshal modified data:", err)
			return line
		}

		return line[:idx] + string(modifiedData)
	}

	return line
}
