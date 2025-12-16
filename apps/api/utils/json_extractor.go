package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
)

// ErrNoJSONFound is returned when no valid JSON object/array is found in the input
var ErrNoJSONFound = errors.New("no valid JSON object or array found in response")

// ExtractJSON extracts and validates JSON from LLM responses that may contain
// garbage characters, markdown formatting, or other non-JSON content.
//
// It handles common issues like:
// - Markdown code blocks (```json ... ```)
// - Garbage characters before/after valid JSON
// - Mixed content with valid JSON embedded
//
// Returns the cleaned JSON string or an error if no valid JSON is found.
func ExtractJSON(response string) (string, error) {
	if response == "" {
		return "", ErrNoJSONFound
	}

	// Log the raw response length for debugging
	log.Printf("[JSON Extractor] Input length: %d chars", len(response))
	if len(response) > 200 {
		log.Printf("[JSON Extractor] First 200 chars: %s", response[:200])
	} else {
		log.Printf("[JSON Extractor] Full response: %s", response)
	}

	// Step 1: Try to extract from markdown code blocks first
	cleaned := extractFromMarkdown(response)

	// Step 2: Try to find valid JSON by bracket matching
	jsonStr := extractJSONByBrackets(cleaned)
	if jsonStr != "" {
		// Validate it's actually valid JSON
		if json.Valid([]byte(jsonStr)) {
			log.Printf("[JSON Extractor] Found valid JSON via bracket matching (%d chars)", len(jsonStr))
			return jsonStr, nil
		}
		log.Printf("[JSON Extractor] Bracket matching found invalid JSON")
	}

	// Step 3: Try the original cleaned response
	if json.Valid([]byte(cleaned)) {
		log.Printf("[JSON Extractor] Cleaned response is valid JSON")
		return cleaned, nil
	}

	// Step 4: Aggressive extraction - find first { or [ and last } or ]
	jsonStr = aggressiveExtract(response)
	if jsonStr != "" && json.Valid([]byte(jsonStr)) {
		log.Printf("[JSON Extractor] Aggressive extraction found valid JSON (%d chars)", len(jsonStr))
		return jsonStr, nil
	}

	// Step 5: Try to fix common JSON issues
	jsonStr = tryFixJSON(cleaned)
	if jsonStr != "" && json.Valid([]byte(jsonStr)) {
		log.Printf("[JSON Extractor] Fixed JSON is valid (%d chars)", len(jsonStr))
		return jsonStr, nil
	}

	log.Printf("[JSON Extractor] No valid JSON found in response")
	return "", fmt.Errorf("%w: response length=%d", ErrNoJSONFound, len(response))
}

// ExtractJSONTo extracts JSON from response and unmarshals it into the target
func ExtractJSONTo(response string, target interface{}) error {
	jsonStr, err := ExtractJSON(response)
	if err != nil {
		return err
	}

	// Double-check validity before unmarshaling
	if !json.Valid([]byte(jsonStr)) {
		log.Printf("[JSON Extractor] Warning: extracted string is not valid JSON: %s", jsonStr[:min(200, len(jsonStr))])
		return fmt.Errorf("extracted content is not valid JSON")
	}

	if err := json.Unmarshal([]byte(jsonStr), target); err != nil {
		log.Printf("[JSON Extractor] Unmarshal failed: %v", err)
		return err
	}
	return nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// extractFromMarkdown removes markdown code block formatting
func extractFromMarkdown(s string) string {
	s = strings.TrimSpace(s)

	// Remove ```json or ``` at the start
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}

	// Remove ``` at the end
	if strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(s, "```")
	}

	// Also try regex for inline code blocks
	re := regexp.MustCompile("(?s)```(?:json)?\\s*(.+?)\\s*```")
	matches := re.FindStringSubmatch(s)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	return strings.TrimSpace(s)
}

// extractJSONByBrackets uses bracket matching to find complete JSON
func extractJSONByBrackets(s string) string {
	// Find the first { or [
	startObj := strings.Index(s, "{")
	startArr := strings.Index(s, "[")

	var start int
	var openChar, closeChar rune

	if startObj == -1 && startArr == -1 {
		return ""
	} else if startObj == -1 {
		start = startArr
		openChar = '['
		closeChar = ']'
	} else if startArr == -1 {
		start = startObj
		openChar = '{'
		closeChar = '}'
	} else if startObj < startArr {
		start = startObj
		openChar = '{'
		closeChar = '}'
	} else {
		start = startArr
		openChar = '['
		closeChar = ']'
	}

	// Use bracket matching to find the end
	depth := 0
	inString := false
	escaped := false
	end := -1

	for i := start; i < len(s); i++ {
		c := rune(s[i])

		if escaped {
			escaped = false
			continue
		}

		if c == '\\' && inString {
			escaped = true
			continue
		}

		if c == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if c == openChar {
			depth++
		} else if c == closeChar {
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
	}

	if end == -1 {
		return ""
	}

	return s[start:end]
}

// aggressiveExtract tries to find JSON by looking for first { and last }
func aggressiveExtract(s string) string {
	// Try object first
	firstBrace := strings.Index(s, "{")
	lastBrace := strings.LastIndex(s, "}")

	if firstBrace != -1 && lastBrace != -1 && lastBrace > firstBrace {
		candidate := s[firstBrace : lastBrace+1]
		if json.Valid([]byte(candidate)) {
			return candidate
		}
	}

	// Try array
	firstBracket := strings.Index(s, "[")
	lastBracket := strings.LastIndex(s, "]")

	if firstBracket != -1 && lastBracket != -1 && lastBracket > firstBracket {
		candidate := s[firstBracket : lastBracket+1]
		if json.Valid([]byte(candidate)) {
			return candidate
		}
	}

	return ""
}

// tryFixJSON attempts to fix common JSON issues
func tryFixJSON(s string) string {
	// Remove any trailing garbage after the last }
	lastBrace := strings.LastIndex(s, "}")
	if lastBrace > 0 {
		s = s[:lastBrace+1]
	}

	// Remove any leading garbage before the first {
	firstBrace := strings.Index(s, "{")
	if firstBrace > 0 {
		s = s[firstBrace:]
	}

	// Remove control characters except standard whitespace
	var cleaned strings.Builder
	for _, r := range s {
		// Keep only printable ASCII and standard whitespace
		if r >= 32 && r < 127 || r == '\n' || r == '\r' || r == '\t' {
			cleaned.WriteRune(r)
		}
	}

	return cleaned.String()
}
