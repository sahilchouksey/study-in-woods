package pdfvalidation

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"strings"

	"github.com/ledongthuc/pdf"
)

// PDFLimits defines the validation limits for PDF uploads
type PDFLimits struct {
	MaxFileSizeMB    int    // Maximum file size in MB
	MaxPages         int    // Maximum number of pages
	DocumentTypeName string // For error messages (e.g., "syllabus", "PYQ paper")
}

// Default limits
var (
	DefaultLimits = PDFLimits{
		MaxFileSizeMB:    50,
		MaxPages:         50,
		DocumentTypeName: "document",
	}

	SyllabusLimits = PDFLimits{
		MaxFileSizeMB:    50,
		MaxPages:         50,
		DocumentTypeName: "syllabus",
	}

	PYQLimits = PDFLimits{
		MaxFileSizeMB:    50,
		MaxPages:         50,
		DocumentTypeName: "PYQ paper",
	}

	NotesLimits = PDFLimits{
		MaxFileSizeMB:    100,
		MaxPages:         2000,
		DocumentTypeName: "notes",
	}

	BookLimits = PDFLimits{
		MaxFileSizeMB:    100,
		MaxPages:         2000,
		DocumentTypeName: "textbook",
	}
)

// ValidationResult contains the result of PDF validation
type ValidationResult struct {
	Valid     bool
	PageCount int
	FileSize  int64
	Error     string
}

// ValidatePDFFile validates a PDF file against the given limits
// Returns the validation result with page count if valid
func ValidatePDFFile(file *multipart.FileHeader, limits PDFLimits) (*ValidationResult, error) {
	result := &ValidationResult{
		FileSize: file.Size,
	}

	// 1. Validate file size
	maxSize := int64(limits.MaxFileSizeMB) * 1024 * 1024
	if file.Size > maxSize {
		result.Error = fmt.Sprintf("File size exceeds maximum allowed size of %dMB", limits.MaxFileSizeMB)
		return result, nil
	}

	// 2. Validate file extension
	filename := strings.ToLower(file.Filename)
	if !strings.HasSuffix(filename, ".pdf") {
		result.Error = "Only PDF files are supported"
		return result, nil
	}

	// 3. Open file and read content
	fileContent, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer fileContent.Close()

	content, err := io.ReadAll(fileContent)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// 4. Validate PDF header
	if !bytes.HasPrefix(content, []byte("%PDF-")) {
		result.Error = "Invalid PDF file: missing PDF header"
		return result, nil
	}

	// 5. Get page count
	pageCount, err := getPDFPageCount(content)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to read PDF: %v", err)
		return result, nil
	}

	result.PageCount = pageCount

	// 6. Validate page count
	if pageCount > limits.MaxPages {
		result.Error = fmt.Sprintf("PDF has %d pages, which exceeds the maximum of %d pages for %s",
			pageCount, limits.MaxPages, limits.DocumentTypeName)
		return result, nil
	}

	if pageCount == 0 {
		result.Error = "PDF has no pages"
		return result, nil
	}

	result.Valid = true
	return result, nil
}

// ValidatePDFBytes validates PDF content bytes against the given limits
func ValidatePDFBytes(content []byte, limits PDFLimits) (*ValidationResult, error) {
	result := &ValidationResult{
		FileSize: int64(len(content)),
	}

	// 1. Validate file size
	maxSize := int64(limits.MaxFileSizeMB) * 1024 * 1024
	if result.FileSize > maxSize {
		result.Error = fmt.Sprintf("File size exceeds maximum allowed size of %dMB", limits.MaxFileSizeMB)
		return result, nil
	}

	// 2. Validate PDF header
	if !bytes.HasPrefix(content, []byte("%PDF-")) {
		result.Error = "Invalid PDF file: missing PDF header"
		return result, nil
	}

	// 3. Get page count
	pageCount, err := getPDFPageCount(content)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to read PDF: %v", err)
		return result, nil
	}

	result.PageCount = pageCount

	// 4. Validate page count
	if pageCount > limits.MaxPages {
		result.Error = fmt.Sprintf("PDF has %d pages, which exceeds the maximum of %d pages for %s",
			pageCount, limits.MaxPages, limits.DocumentTypeName)
		return result, nil
	}

	if pageCount == 0 {
		result.Error = "PDF has no pages"
		return result, nil
	}

	result.Valid = true
	return result, nil
}

// sanitizePDF removes trailing garbage data from PDFs
func sanitizePDF(content []byte) []byte {
	if len(content) == 0 {
		return content
	}

	if !bytes.HasPrefix(content, []byte("%PDF-")) {
		return content
	}

	eofMarker := []byte("%%EOF")
	lastEOF := bytes.LastIndex(content, eofMarker)

	if lastEOF == -1 {
		return content
	}

	pdfEnd := lastEOF + len(eofMarker)

	for pdfEnd < len(content) && (content[pdfEnd] == '\n' || content[pdfEnd] == '\r') {
		pdfEnd++
	}

	if pdfEnd < len(content) {
		return content[:pdfEnd]
	}

	return content
}

// getPDFPageCount returns the number of pages in a PDF
func getPDFPageCount(content []byte) (int, error) {
	content = sanitizePDF(content)
	reader := bytes.NewReader(content)

	pdfReader, err := pdf.NewReader(reader, int64(len(content)))
	if err != nil {
		return 0, fmt.Errorf("failed to parse PDF: %w", err)
	}

	return pdfReader.NumPage(), nil
}

// GetPageCountFromFile gets the page count from a multipart file header
// This is useful when you need to check page count without full validation
func GetPageCountFromFile(file *multipart.FileHeader) (int, error) {
	fileContent, err := file.Open()
	if err != nil {
		return 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer fileContent.Close()

	content, err := io.ReadAll(fileContent)
	if err != nil {
		return 0, fmt.Errorf("failed to read file: %w", err)
	}

	return getPDFPageCount(content)
}
