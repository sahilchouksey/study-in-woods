package services

import (
	"log"
	"strings"
)

// ValidateAndFixUnitTitle ensures unit titles are concise and don't duplicate raw_text
// Returns the fixed title if changes were made
func ValidateAndFixUnitTitle(title string, rawText string) string {
	originalTitle := title

	// Check if title duplicates raw_text (exact match or very similar)
	if title == rawText {
		log.Printf("SyllabusValidator: Title duplicates raw_text, creating summary")
		title = createTitleFromText(rawText)
	}

	// Check if title is too long (>60 chars)
	if len(title) > 60 {
		log.Printf("SyllabusValidator: Title too long (%d chars), shortening", len(title))
		title = shortenTitle(title)
	}

	// Check word count (should be 3-6 words ideally)
	wordCount := len(strings.Fields(title))
	if wordCount > 8 {
		log.Printf("SyllabusValidator: Title has too many words (%d), truncating", wordCount)
		title = createTitleFromText(title)
	}

	if title != originalTitle {
		log.Printf("SyllabusValidator: Fixed title: '%s' → '%s'", originalTitle, title)
	}

	return title
}

// createTitleFromText creates a concise title from raw text
// Extracts first 3-5 meaningful words, skipping common fillers
func createTitleFromText(text string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return "Unit"
	}

	// Common filler words to skip (in lowercase)
	fillers := map[string]bool{
		"introduction": true,
		"overview":     true,
		"basics":       true,
		"to":           true,
		"and":          true,
		"or":           true,
		"the":          true,
		"a":            true,
		"an":           true,
	}

	var titleWords []string
	for _, word := range words {
		// Stop after collecting 5 meaningful words
		if len(titleWords) >= 5 {
			break
		}

		// Clean punctuation from word
		cleanWord := strings.Trim(word, ",.;:!?-–")

		// Skip empty words
		if cleanWord == "" {
			continue
		}

		// Skip filler words, but keep first word always
		if len(titleWords) == 0 || !fillers[strings.ToLower(cleanWord)] {
			titleWords = append(titleWords, cleanWord)
		}
	}

	if len(titleWords) == 0 {
		return "Unit"
	}

	title := strings.Join(titleWords, " ")

	// If still too long, truncate at 60 chars
	if len(title) > 60 {
		title = title[:60]
		// Try to cut at last space to avoid cutting mid-word
		if lastSpace := strings.LastIndex(title, " "); lastSpace > 30 {
			title = title[:lastSpace]
		}
	}

	return title
}

// shortenTitle shortens a title to fit within 60 characters
// Tries to preserve meaning by cutting at word boundaries
func shortenTitle(title string) string {
	if len(title) <= 60 {
		return title
	}

	// Try to find first 5 words
	words := strings.Fields(title)
	if len(words) > 5 {
		return strings.Join(words[:5], " ")
	}

	// If already 5 words or less but still too long, truncate at 60 chars
	shortened := title[:60]

	// Try to cut at last space to avoid cutting mid-word
	if lastSpace := strings.LastIndex(shortened, " "); lastSpace > 30 {
		shortened = shortened[:lastSpace]
	}

	return shortened
}

// ValidateUnitData performs comprehensive validation on unit data
// Returns true if unit is valid, false if it should be skipped
func ValidateUnitData(unitNumber int, title string, rawText string, topicCount int) bool {
	// Unit must have at least a title or raw_text
	if title == "" && rawText == "" {
		log.Printf("SyllabusValidator: Unit %d has no title or raw_text, skipping", unitNumber)
		return false
	}

	// Warn if no topics extracted (but don't skip - might be intentional)
	if topicCount == 0 && rawText != "" {
		log.Printf("SyllabusValidator: Warning - Unit %d ('%s') has no topics extracted", unitNumber, title)
	}

	return true
}
