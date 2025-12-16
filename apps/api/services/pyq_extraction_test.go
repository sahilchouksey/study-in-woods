package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/sahilchouksey/go-init-setup/services/digitalocean"
)

// ExpectedQuestion represents the expected extraction result for a question
type ExpectedQuestion struct {
	QuestionNumber string
	QuestionText   string
	Marks          int
	IsCompulsory   bool
	HasChoices     bool
	MinTextLength  int // Minimum expected text length (for validation)
}

// ExpectedPYQResult represents the expected extraction result
type ExpectedPYQResult struct {
	Year         int
	Month        string
	ExamType     string
	TotalMarks   int
	Duration     string
	Instructions string
	Questions    []ExpectedQuestion
}

// TestPYQExtractionWithOCR tests the full extraction pipeline with OCR
// This test requires:
// 1. OCR_SERVICE_URL to be set (or running locally on :8081)
// 2. DO_INFERENCE_API_KEY to be set
func TestPYQExtractionWithOCR(t *testing.T) {
	// Skip if not in integration test mode
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run")
	}

	inferenceAPIKey := os.Getenv("DO_INFERENCE_API_KEY")
	if inferenceAPIKey == "" {
		t.Skip("DO_INFERENCE_API_KEY not set")
	}

	// Read the test PDF - try multiple paths since test can run from different dirs
	possiblePaths := []string{
		"tests/testdata/MCA-302-AI-MAY-2024.pdf",          // from apps/api
		"../tests/testdata/MCA-302-AI-MAY-2024.pdf",       // from apps/api/services
		"../../tests/testdata/MCA-302-AI-MAY-2024.pdf",    // from deeper
		"apps/api/tests/testdata/MCA-302-AI-MAY-2024.pdf", // from repo root
	}
	var pdfContent []byte
	var readErr error
	for _, pdfPath := range possiblePaths {
		pdfContent, readErr = os.ReadFile(pdfPath)
		if readErr == nil {
			t.Logf("Found test PDF at: %s", pdfPath)
			break
		}
	}
	err := readErr
	if err != nil {
		t.Fatalf("Failed to read test PDF: %v", err)
	}

	// Step 1: OCR the PDF
	ocrClient := NewOCRClient()
	ctx := context.Background()

	ocrResp, err := ocrClient.ProcessPDFFile(ctx, pdfContent, "MCA-302-AI-MAY-2024.pdf")
	if err != nil {
		t.Fatalf("OCR failed: %v", err)
	}

	t.Logf("OCR extracted %d pages, %d chars of text", ocrResp.PageCount, len(ocrResp.Text))
	t.Logf("OCR Text Preview (first 1000 chars):\n%s", truncateStr(ocrResp.Text, 1000))

	// Step 2: Extract questions using LLM
	inferenceClient := digitalocean.NewInferenceClient(digitalocean.InferenceConfig{
		APIKey: inferenceAPIKey,
	})

	result, rawResponse, err := extractPYQWithLLM(ctx, inferenceClient, ocrResp.Text)
	if err != nil {
		t.Fatalf("LLM extraction failed: %v", err)
	}

	t.Logf("Raw LLM Response:\n%s", truncateStr(rawResponse, 2000))

	// Step 3: Validate the extraction
	expected := getExpectedMCA302AIResult()
	validateExtraction(t, result, expected)
}

// TestPYQExtractionPromptImprovement tests the improved prompt
func TestPYQExtractionPromptImprovement(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=true to run")
	}

	inferenceAPIKey := os.Getenv("DO_INFERENCE_API_KEY")
	if inferenceAPIKey == "" {
		t.Skip("DO_INFERENCE_API_KEY not set")
	}

	// Simulated OCR text (what the OCR would extract from the PDF)
	// This is based on the actual PDF content
	ocrText := getMCA302AIOCRText()

	ctx := context.Background()
	inferenceClient := digitalocean.NewInferenceClient(digitalocean.InferenceConfig{
		APIKey: inferenceAPIKey,
	})

	result, rawResponse, err := extractPYQWithImprovedPrompt(ctx, inferenceClient, ocrText)
	if err != nil {
		t.Fatalf("LLM extraction failed: %v", err)
	}

	t.Logf("Raw LLM Response:\n%s", truncateStr(rawResponse, 3000))

	// Validate
	expected := getExpectedMCA302AIResult()
	validateExtraction(t, result, expected)
}

// extractPYQWithLLM extracts PYQ data using the CURRENT prompt (for comparison)
func extractPYQWithLLM(ctx context.Context, client *digitalocean.InferenceClient, documentText string) (*PYQExtractionResult, string, error) {
	maxChars := 50000
	if len(documentText) > maxChars {
		documentText = documentText[:maxChars] + "\n\n[Document truncated due to length]"
	}

	userPrompt := fmt.Sprintf("Extract the question paper information from the following document:\n\n%s", documentText)

	response, err := client.StructuredCompletion(
		ctx,
		pyqExtractionPrompt,
		userPrompt,
		"pyq_extraction",
		"Structured extraction of previous year question paper with questions and choices",
		pyqExtractionSchema,
		digitalocean.WithInferenceMaxTokens(8192),
		digitalocean.WithInferenceTemperature(0),
	)
	if err != nil {
		return nil, "", fmt.Errorf("structured completion failed: %w", err)
	}

	var result PYQExtractionResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, response, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, response, nil
}

// extractPYQWithImprovedPrompt uses the IMPROVED prompt
func extractPYQWithImprovedPrompt(ctx context.Context, client *digitalocean.InferenceClient, documentText string) (*PYQExtractionResult, string, error) {
	maxChars := 50000
	if len(documentText) > maxChars {
		documentText = documentText[:maxChars] + "\n\n[Document truncated due to length]"
	}

	userPrompt := fmt.Sprintf("Extract the question paper information from the following document:\n\n%s", documentText)

	response, err := client.StructuredCompletion(
		ctx,
		improvedPYQExtractionPrompt,
		userPrompt,
		"pyq_extraction",
		"Structured extraction of previous year question paper with questions and choices",
		pyqExtractionSchema,
		digitalocean.WithInferenceMaxTokens(8192),
		digitalocean.WithInferenceTemperature(0),
	)
	if err != nil {
		return nil, "", fmt.Errorf("structured completion failed: %w", err)
	}

	var result PYQExtractionResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, response, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, response, nil
}

// validateExtraction validates the extraction result against expected values
func validateExtraction(t *testing.T, result *PYQExtractionResult, expected ExpectedPYQResult) {
	t.Helper()

	// Validate paper metadata
	if result.Year != expected.Year {
		t.Errorf("Year mismatch: got %d, want %d", result.Year, expected.Year)
	}
	if result.Month != expected.Month {
		t.Errorf("Month mismatch: got %s, want %s", result.Month, expected.Month)
	}
	if result.TotalMarks != expected.TotalMarks {
		t.Errorf("TotalMarks mismatch: got %d, want %d", result.TotalMarks, expected.TotalMarks)
	}
	if !strings.Contains(strings.ToLower(result.Duration), "three") && !strings.Contains(result.Duration, "3") {
		t.Errorf("Duration should contain 'three' or '3': got %s", result.Duration)
	}

	// Validate question count
	if len(result.Questions) != len(expected.Questions) {
		t.Errorf("Question count mismatch: got %d, want %d", len(result.Questions), len(expected.Questions))
	}

	// Validate each question
	questionMap := make(map[string]PYQQuestionExtraction)
	for _, q := range result.Questions {
		questionMap[q.QuestionNumber] = q
	}

	for _, eq := range expected.Questions {
		q, exists := questionMap[eq.QuestionNumber]
		if !exists {
			t.Errorf("Missing question %s", eq.QuestionNumber)
			continue
		}

		// Check question text is not empty
		if len(q.QuestionText) < eq.MinTextLength {
			t.Errorf("Question %s text too short or empty: got %d chars (%q), want at least %d",
				eq.QuestionNumber, len(q.QuestionText), truncateStr(q.QuestionText, 50), eq.MinTextLength)
		}

		// Check marks
		if q.Marks != eq.Marks {
			t.Errorf("Question %s marks mismatch: got %d, want %d", eq.QuestionNumber, q.Marks, eq.Marks)
		}

		// Check is_compulsory
		if q.IsCompulsory != eq.IsCompulsory {
			t.Errorf("Question %s is_compulsory mismatch: got %v, want %v", eq.QuestionNumber, q.IsCompulsory, eq.IsCompulsory)
		}

		// Check key phrases in question text
		if eq.QuestionText != "" {
			// Check that at least some key words are present
			keyWords := extractKeyWords(eq.QuestionText)
			matchCount := 0
			for _, kw := range keyWords {
				if strings.Contains(strings.ToLower(q.QuestionText), strings.ToLower(kw)) {
					matchCount++
				}
			}
			if matchCount < len(keyWords)/2 {
				t.Errorf("Question %s text missing key content.\nGot: %s\nWant keywords from: %s",
					eq.QuestionNumber, q.QuestionText, eq.QuestionText)
			}
		}
	}

	// Print summary
	t.Logf("\n=== Extraction Summary ===")
	t.Logf("Year: %d, Month: %s, Total Marks: %d", result.Year, result.Month, result.TotalMarks)
	t.Logf("Duration: %s", result.Duration)
	t.Logf("Instructions: %s", truncateStr(result.Instructions, 100))
	t.Logf("Questions extracted: %d", len(result.Questions))
	for _, q := range result.Questions {
		t.Logf("  %s: %s (marks: %d, compulsory: %v)",
			q.QuestionNumber, truncateStr(q.QuestionText, 60), q.Marks, q.IsCompulsory)
	}
}

// getExpectedMCA302AIResult returns the expected extraction for MCA-302 AI May 2024
func getExpectedMCA302AIResult() ExpectedPYQResult {
	return ExpectedPYQResult{
		Year:         2024,
		Month:        "May",
		ExamType:     "End Semester",
		TotalMarks:   70,
		Duration:     "Three Hours",
		Instructions: "Attempt any five questions",
		Questions: []ExpectedQuestion{
			{QuestionNumber: "1a", QuestionText: "Define Artificial Intelligence (AI). What is an AI technique?", Marks: 7, IsCompulsory: false, MinTextLength: 30},
			{QuestionNumber: "1b", QuestionText: "Discuss Basic list manipulation functions in LISP programming", Marks: 7, IsCompulsory: false, MinTextLength: 30},
			{QuestionNumber: "2a", QuestionText: "Explain Heuristic Search Techniques (HST) technique with the help of suitable example. Write algorithm for Heuristic Search Techniques", Marks: 7, IsCompulsory: false, MinTextLength: 40},
			{QuestionNumber: "2b", QuestionText: "What is Constraint satisfaction problems in the field of Artificial Intelligence? Explain", Marks: 7, IsCompulsory: false, MinTextLength: 30},
			{QuestionNumber: "3a", QuestionText: "What do you mean by Knowledge Representations? Discuss primitive rules in First order predicate calculus", Marks: 7, IsCompulsory: false, MinTextLength: 40},
			{QuestionNumber: "3b", QuestionText: "Discuss Horn's clauses and Semantic networks in the field of Artificial Intelligence using real life example", Marks: 7, IsCompulsory: false, MinTextLength: 40},
			{QuestionNumber: "4a", QuestionText: "Discuss the Game playing Minimax search procedure under the definition of Neural Network using suitable examples", Marks: 7, IsCompulsory: false, MinTextLength: 40},
			{QuestionNumber: "4b", QuestionText: "Discuss the role of Natural Language Processing in Artificial Intelligence using a suitable example", Marks: 7, IsCompulsory: false, MinTextLength: 40},
			{QuestionNumber: "5a", QuestionText: "Discuss about the Knowledge, Reasoning and Learning in AI using suitable examples", Marks: 7, IsCompulsory: false, MinTextLength: 40},
			{QuestionNumber: "5b", QuestionText: "Discuss about the different Stages in Data Processing using related example", Marks: 7, IsCompulsory: false, MinTextLength: 40},
			{QuestionNumber: "6a", QuestionText: "Describe the four categories under which AI is classified with examples", Marks: 7, IsCompulsory: false, MinTextLength: 40},
			{QuestionNumber: "6b", QuestionText: "Discuss the concept of a frame in AI. How is it useful in Knowledge representation?", Marks: 7, IsCompulsory: false, MinTextLength: 40},
			{QuestionNumber: "7a", QuestionText: "Explain and justify the usage of Bayes Classifier. Define Utility theorem", Marks: 7, IsCompulsory: false, MinTextLength: 30},
			{QuestionNumber: "7b", QuestionText: "What is need and use of the concept of Image and face recognition Learning in the field of Artificial Intelligence?", Marks: 7, IsCompulsory: false, MinTextLength: 40},
			{QuestionNumber: "8a", QuestionText: "Discuss in detail about the following: i) A* Algorithm, ii) Hill Climbing, iii) ANN, iv) Types of Learning", Marks: 7, IsCompulsory: false, HasChoices: true, MinTextLength: 40},
			{QuestionNumber: "8b", QuestionText: "Describe in detail about the Production system in AI using a suitable example", Marks: 7, IsCompulsory: false, MinTextLength: 40},
		},
	}
}

// getMCA302AIOCRText returns simulated OCR text for testing without OCR service
func getMCA302AIOCRText() string {
	return `Total No. of Questions : 8]                    [Total No. of Printed Pages : 4

                                Roll No .................................

                                        MCA-302
                    M.C.A. III Semester (Two Year Course)
                            Examination, May 2024
                            Artificial Intelligence
                                Time : Three Hours
                                Maximum Marks : 70

Note: i)    Attempt any five questions.
            किन्हीं पाँच प्रश्नों को हल कीजिए।
      ii)   All questions carry equal marks.
            सभी प्रश्नों के समान अंक हैं।
      iii)  In case of any doubt or dispute the English version
            question should be treated as final.
            किसी भी प्रकार के संदेह अथवा विवाद की स्थिति में अंग्रेजी भाषा
            के प्रश्न को अंतिम माना जायेगा।

1.  a)  Define Artificial Intelligence (AI). What is an AI
        technique?
        आर्टिफिशियल इंटेलिजेंस को (AI) परिभाषित करें। AI तकनीक क्या
        है?

    b)  Discuss Basic list manipulation functions in LISP
        programming.
        LISP प्रोग्रामिंग में बुनियादी सूची हेरफेर कार्यों पर चर्चा करें।

                                                                        [2]

2.  a)  Explain Heuristic Search Techniques (HST) technique
        with the help of suitable example. Write algorithm for
        Heuristic Search Techniques.
        उपयुक्त उदाहरण की सहायता से अनुमानी खोज तकनीक (HST)
        तकनीक को समझाइए। अनुमानी खोज तकनीकों के लिए एल्गोरिथम
        लिखें।

    b)  What is Constraint satisfaction problems in the field of
        Artificial Intelligence? Explain.
        आर्टिफिशियल इंटेलिजेंस के क्षेत्र में बाधा संतुष्टि समस्याएँ क्या हैं?
        व्याख्या करें।

3.  a)  What do you mean by Knowledge Representations?
        Discuss primitive rules in First order predicate calculus.
        ज्ञान निरूपण से आप क्या समझते हैं? प्रथम क्रम विधेय कलन में
        आदिम नियमों पर चर्चा करें।

    b)  Discuss Horn's clauses and Semantic networks in the field
        of Artificial Intelligence using real life example.
        वास्तविक जीवन के उदाहरण का उपयोग करके आर्टिफिशियल
        इंटेलिजेंस के क्षेत्र में हॉर्न के खंड और सिमेंटिक नेटवर्क पर चर्चा करें।

4.  a)  Discuss the Game playing Minimax search procedure
        under the definition of Neural Network using suitable
        examples.
        उपयुक्त उदाहरणों का उपयोग करते हुए न्यूरल नेटवर्क की परिभाषा
        के तहत गेम प्लेइंग मिनिमैक्स खोज प्रक्रिया पर चर्चा करें।

    b)  Discuss the role of Natural Language Processing in
        Artificial Intelligence using a suitable example.
        एक उपयुक्त उदाहरण का उपयोग करके कृत्रिम बुद्धिमत्ता में प्राकृतिक
        भाषा प्रसंस्करण की भूमिका पर चर्चा करें।

MCA-302                                                                 Contd...

                                                                        [3]

5.  a)  Discuss about the Knowledge, Reasoning and Learning
        in AI using suitable examples.
        उपयुक्त उदाहरणों का उपयोग करके AI में ज्ञान, तर्क और सीखने के
        बारे में चर्चा करें।

    b)  Discuss about the different Stages in Data Processing
        using related example.
        संबंधित उदाहरण का उपयोग करके डाटा प्रोसेसिंग में विभिन्न चरणों
        के बारे में चर्चा करें।

6.  a)  Describe the four categories under which AI is classified
        with examples.
        उन चार श्रेणियों का वर्णन करें जिनके अंतर्गत AI को उदाहरण सहित
        वर्गीकृत किया गया है।

    b)  Discuss the concept of a frame in AI. How is it useful in
        Knowledge representation?
        AI में एक फ्रेम की अवधारणा पर चर्चा करें। यह ज्ञान निरूपण में
        किस प्रकार उपयोगी है?

7.  a)  Explain and justify the usage of Bayes Classifier. Define
        Utility theorem.
        बेयस क्लासिफायर के उपयोग को समझाइए और उचित ठहराएँ।
        उपयोगिता प्रमेय को परिभाषित करें।

    b)  What is need and use of the concept of Image and face
        recognition Learning in the field of Artificial Intelligence?
        आर्टिफिशियल इंटेलिजेंस के क्षेत्र में छवि और चेहरा पहचान सीखने
        की अवधारणा और उपयोग क्या है?

MCA-302                                 PTO

                                                                        [4]

8.  a)  Discuss in detail about the following.
        i)   A* Algorithm
        ii)  Hill Climbing
        iii) ANN
        iv)  Types of Learning
        निम्नलिखित के बारे में विस्तार से चर्चा करें।
        i)   A* एल्गोरिथम
        ii)  पहाड़ी पर चढ़ना
        iii) ANN
        iv)  सीखने के प्रकार

    b)  Describe in detail about the Production system in AI using
        a suitable example.
        एक उपयुक्त उदाहरण का उपयोग करके AI में उत्पादन प्रणाली के
        बारे में विस्तार से वर्णन करें।

                                ******

MCA-302
`
}

// extractKeyWords extracts important words from a question text
func extractKeyWords(text string) []string {
	// Remove common words and extract important terms
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"what": true, "how": true, "why": true, "when": true, "where": true,
		"in": true, "of": true, "to": true, "for": true, "with": true,
		"and": true, "or": true, "using": true, "about": true,
	}

	words := strings.Fields(strings.ToLower(text))
	var keywords []string
	for _, w := range words {
		w = strings.Trim(w, ".,?!()[]")
		if len(w) > 3 && !stopWords[w] {
			keywords = append(keywords, w)
		}
	}
	return keywords
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// improvedPYQExtractionPrompt is the IMPROVED system prompt for better extraction
const improvedPYQExtractionPrompt = `You are an expert at extracting structured information from Indian university examination question papers (Previous Year Questions / PYQs).

CRITICAL: You MUST respond with ONLY valid JSON. No markdown, no explanations, no code blocks. Start with { and end with }.

IMPORTANT - UNDERSTAND INDIAN UNIVERSITY EXAM PAPER STRUCTURE:
1. Papers typically have 8 questions numbered 1-8
2. Each main question has SUB-PARTS labeled a), b) - these are SEPARATE questions to be answered
3. "Attempt any X questions" means questions are OPTIONAL (is_compulsory = false)
4. "All questions carry equal marks" - divide total marks by required questions, then by sub-parts
   Example: 70 marks, attempt 5 questions = 14 marks per question = 7 marks per sub-part (a/b)
5. Papers often have bilingual text (English + Hindi) - extract ONLY the English version
6. Ignore Hindi/Devanagari text completely

CRITICAL EXTRACTION RULES:

1. MARKS CALCULATION:
   - If "All questions carry equal marks" and "Attempt any 5 questions" with 70 total marks:
     * Each question = 70/5 = 14 marks
     * Each sub-part (a, b) = 14/2 = 7 marks
   - ALWAYS calculate and assign marks - NEVER leave as 0

2. SUB-PARTS (a), (b) = Extract as SEPARATE question entries:
   - Question number format: "1a", "1b", "2a", "2b", etc.
   - has_choices = false
   - Each sub-part gets its own marks (typically half of main question)

3. COMPULSORY vs OPTIONAL:
   - "Attempt any X questions" = is_compulsory: false for ALL questions
   - "Attempt all questions" or "Compulsory" = is_compulsory: true

4. QUESTION TEXT - BE COMPLETE:
   - Extract the FULL question text in English
   - Include all parts of multi-part questions
   - If question says "Discuss X and Y", include both X and Y
   - If question has sub-items (i, ii, iii), include them in the text

5. CHOICES (has_choices = true):
   - Only use when question explicitly says "any X of the following" or has "OR" between options
   - Put options in choices array with labels (i, ii, iii or a, b, c)

6. NEVER SKIP OR LEAVE EMPTY:
   - Every question must have question_text - no empty strings allowed
   - Every question must have marks > 0
   - Extract ALL 16 sub-questions (8 questions × 2 sub-parts)

OUTPUT FORMAT:
{
  "year": 2024,
  "month": "May",
  "exam_type": "End Semester Examination",
  "total_marks": 70,
  "duration": "Three Hours",
  "instructions": "Attempt any five questions. All questions carry equal marks.",
  "questions": [
    {
      "question_number": "1a",
      "section_name": "",
      "question_text": "Define Artificial Intelligence (AI). What is an AI technique?",
      "marks": 7,
      "is_compulsory": false,
      "has_choices": false,
      "unit_number": 0,
      "topic_keywords": "AI, artificial intelligence, technique",
      "choices": []
    }
  ]
}

VALIDATION CHECKLIST (verify before output):
✓ All 16 questions extracted (8 main questions × 2 sub-parts each)
✓ No empty question_text fields
✓ All marks fields are > 0 (typically 7 for this paper type)
✓ is_compulsory is false for "Attempt any X" papers
✓ Year and month extracted correctly from header
✓ Only English text extracted (no Hindi/Devanagari)

REMEMBER: Output ONLY the JSON object. Start with { end with }.`
