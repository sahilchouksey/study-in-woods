package services

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	"github.com/ledongthuc/pdf"
)

// PageRange represents a range of pages (1-indexed, inclusive)
type PageRange struct {
	Start int
	End   int
}

// ChunkConfig holds configuration for chunking PDFs
type ChunkConfig struct {
	PagesPerChunk int // Default: 4
	OverlapPages  int // Default: 1
}

// DefaultChunkConfig returns the default chunking configuration
func DefaultChunkConfig() ChunkConfig {
	return ChunkConfig{
		PagesPerChunk: 4,
		OverlapPages:  1,
	}
}

// PDFExtractor handles PDF text extraction using ledongthuc/pdf (MIT license)
type PDFExtractor struct{}

// NewPDFExtractor creates a new PDF extractor
func NewPDFExtractor() *PDFExtractor {
	return &PDFExtractor{}
}

// sanitizePDF fixes common PDF issues like trailing garbage data
// Many PDFs downloaded from web have HTML or other data appended after %%EOF
// This function truncates the content at the last valid %%EOF marker
func sanitizePDF(content []byte) []byte {
	if len(content) == 0 {
		return content
	}

	// Check if content starts with PDF header
	if !bytes.HasPrefix(content, []byte("%PDF-")) {
		return content // Not a PDF, return as-is
	}

	// Find the last occurrence of %%EOF (valid PDF end marker)
	eofMarker := []byte("%%EOF")
	lastEOF := bytes.LastIndex(content, eofMarker)

	if lastEOF == -1 {
		// No %%EOF found - PDF is likely truncated, return as-is and let parser handle it
		return content
	}

	// Calculate where the PDF should end (%%EOF + marker length + optional newline)
	pdfEnd := lastEOF + len(eofMarker)

	// Allow for trailing newlines after %%EOF (valid per PDF spec)
	for pdfEnd < len(content) && (content[pdfEnd] == '\n' || content[pdfEnd] == '\r') {
		pdfEnd++
	}

	// If there's significant extra content after %%EOF, truncate it
	if pdfEnd < len(content) {
		extraBytes := len(content) - pdfEnd
		if extraBytes > 10 { // More than just whitespace
			log.Printf("PDF Sanitizer: Removing %d bytes of trailing garbage after %%EOF", extraBytes)
			return content[:pdfEnd]
		}
	}

	return content
}

// ExtractText extracts text from PDF bytes
// Returns the extracted text or an error if extraction fails
func (p *PDFExtractor) ExtractText(content []byte) (string, error) {
	if len(content) == 0 {
		return "", fmt.Errorf("empty PDF content")
	}

	// Try to sanitize PDF if it has trailing garbage (common with web downloads)
	content = sanitizePDF(content)

	// Create a bytes.Reader which implements io.ReaderAt
	reader := bytes.NewReader(content)

	// Create PDF reader
	pdfReader, err := pdf.NewReader(reader, int64(len(content)))
	if err != nil {
		return "", fmt.Errorf("failed to parse PDF: %w", err)
	}

	numPages := pdfReader.NumPage()
	if numPages == 0 {
		return "", fmt.Errorf("PDF has no pages")
	}

	log.Printf("PDF Extractor: Processing PDF with %d pages", numPages)

	var textBuilder strings.Builder

	for i := 1; i <= numPages; i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			log.Printf("PDF Extractor: Page %d is null, skipping", i)
			continue
		}

		// Try to extract text by row for better structure preservation
		rows, err := page.GetTextByRow()
		if err != nil {
			log.Printf("PDF Extractor: Row extraction failed for page %d, trying plain text: %v", i, err)
			// Fallback to plain text if row extraction fails
			text, plainErr := page.GetPlainText(nil)
			if plainErr != nil {
				log.Printf("PDF Extractor: Plain text extraction also failed for page %d: %v", i, plainErr)
				continue
			}
			textBuilder.WriteString(text)
			textBuilder.WriteString("\n")
			continue
		}

		// Build text from rows - this preserves document structure better
		for _, row := range rows {
			var rowText strings.Builder
			for _, word := range row.Content {
				rowText.WriteString(word.S)
			}
			line := strings.TrimSpace(rowText.String())
			if line != "" {
				textBuilder.WriteString(line)
				textBuilder.WriteString("\n")
			}
		}
		textBuilder.WriteString("\n") // Separate pages
	}

	extracted := strings.TrimSpace(textBuilder.String())

	// Validate we got meaningful content
	if len(extracted) < 50 {
		return "", fmt.Errorf("insufficient text extracted from PDF (only %d characters) - PDF may be scanned/image-based and requires OCR", len(extracted))
	}

	log.Printf("PDF Extractor: Successfully extracted %d characters from %d pages", len(extracted), numPages)

	return extracted, nil
}

// ExtractTextWithPageInfo extracts text with page markers
// Useful for debugging and understanding document structure
func (p *PDFExtractor) ExtractTextWithPageInfo(content []byte) (string, error) {
	if len(content) == 0 {
		return "", fmt.Errorf("empty PDF content")
	}

	// Sanitize PDF to remove trailing garbage
	content = sanitizePDF(content)

	reader := bytes.NewReader(content)
	pdfReader, err := pdf.NewReader(reader, int64(len(content)))
	if err != nil {
		return "", fmt.Errorf("failed to parse PDF: %w", err)
	}

	numPages := pdfReader.NumPage()
	if numPages == 0 {
		return "", fmt.Errorf("PDF has no pages")
	}

	var textBuilder strings.Builder

	for i := 1; i <= numPages; i++ {
		textBuilder.WriteString(fmt.Sprintf("\n===== PAGE %d of %d =====\n\n", i, numPages))

		page := pdfReader.Page(i)
		if page.V.IsNull() {
			textBuilder.WriteString("[Page content unavailable]\n")
			continue
		}

		rows, err := page.GetTextByRow()
		if err != nil {
			text, plainErr := page.GetPlainText(nil)
			if plainErr != nil {
				textBuilder.WriteString("[Failed to extract text]\n")
				continue
			}
			textBuilder.WriteString(text)
			continue
		}

		for _, row := range rows {
			var rowText strings.Builder
			for _, word := range row.Content {
				rowText.WriteString(word.S)
			}
			line := strings.TrimSpace(rowText.String())
			if line != "" {
				textBuilder.WriteString(line)
				textBuilder.WriteString("\n")
			}
		}
	}

	extracted := strings.TrimSpace(textBuilder.String())
	if len(extracted) < 50 {
		return "", fmt.Errorf("insufficient text extracted from PDF (only %d characters)", len(extracted))
	}

	return extracted, nil
}

// ExtractTextFromPDFBytes is a convenience function for one-off extractions
// without needing to create a PDFExtractor instance
func ExtractTextFromPDFBytes(content []byte) (string, error) {
	extractor := NewPDFExtractor()
	return extractor.ExtractText(content)
}

// GetPageCount returns the total number of pages in the PDF
func (p *PDFExtractor) GetPageCount(content []byte) (int, error) {
	if len(content) == 0 {
		return 0, fmt.Errorf("empty PDF content")
	}

	content = sanitizePDF(content)
	reader := bytes.NewReader(content)

	pdfReader, err := pdf.NewReader(reader, int64(len(content)))
	if err != nil {
		return 0, fmt.Errorf("failed to parse PDF: %w", err)
	}

	return pdfReader.NumPage(), nil
}

// ExtractPageRange extracts text from a specific page range (1-indexed, inclusive)
// For example, ExtractPageRange(content, 1, 4) extracts pages 1, 2, 3, and 4
func (p *PDFExtractor) ExtractPageRange(content []byte, startPage, endPage int) (string, error) {
	if len(content) == 0 {
		return "", fmt.Errorf("empty PDF content")
	}

	content = sanitizePDF(content)
	reader := bytes.NewReader(content)

	pdfReader, err := pdf.NewReader(reader, int64(len(content)))
	if err != nil {
		return "", fmt.Errorf("failed to parse PDF: %w", err)
	}

	numPages := pdfReader.NumPage()
	if numPages == 0 {
		return "", fmt.Errorf("PDF has no pages")
	}

	// Validate page range
	if startPage < 1 {
		startPage = 1
	}
	if endPage > numPages {
		endPage = numPages
	}
	if startPage > endPage {
		return "", fmt.Errorf("invalid page range: start=%d, end=%d", startPage, endPage)
	}

	log.Printf("PDF Extractor: Extracting pages %d-%d of %d", startPage, endPage, numPages)

	var textBuilder strings.Builder

	for i := startPage; i <= endPage; i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			log.Printf("PDF Extractor: Page %d is null, skipping", i)
			continue
		}

		// Try to extract text by row for better structure preservation
		rows, err := page.GetTextByRow()
		if err != nil {
			// Fallback to plain text if row extraction fails
			text, plainErr := page.GetPlainText(nil)
			if plainErr != nil {
				log.Printf("PDF Extractor: Failed to extract page %d: %v", i, plainErr)
				continue
			}
			textBuilder.WriteString(text)
			textBuilder.WriteString("\n")
			continue
		}

		// Build text from rows
		for _, row := range rows {
			var rowText strings.Builder
			for _, word := range row.Content {
				rowText.WriteString(word.S)
			}
			line := strings.TrimSpace(rowText.String())
			if line != "" {
				textBuilder.WriteString(line)
				textBuilder.WriteString("\n")
			}
		}
		textBuilder.WriteString("\n") // Separate pages
	}

	extracted := strings.TrimSpace(textBuilder.String())
	log.Printf("PDF Extractor: Extracted %d characters from pages %d-%d", len(extracted), startPage, endPage)

	return extracted, nil
}

// CalculateChunks returns overlapping page ranges for parallel processing
// Example: 12 pages with pagesPerChunk=4, overlap=1 returns:
//   - {1, 4}, {4, 8}, {8, 12}
func (p *PDFExtractor) CalculateChunks(totalPages int, config ChunkConfig) []PageRange {
	if totalPages <= 0 {
		return nil
	}

	// Use defaults if not set
	if config.PagesPerChunk <= 0 {
		config.PagesPerChunk = 4
	}
	if config.OverlapPages < 0 {
		config.OverlapPages = 0
	}

	var chunks []PageRange

	// Calculate effective step (pages to advance between chunks)
	step := config.PagesPerChunk - config.OverlapPages
	if step <= 0 {
		step = 1 // Ensure we make progress
	}

	for start := 1; start <= totalPages; {
		end := start + config.PagesPerChunk - 1
		if end > totalPages {
			end = totalPages
		}

		chunks = append(chunks, PageRange{Start: start, End: end})

		// Move to next chunk
		start += step

		// If we've already covered all pages, stop
		if end >= totalPages {
			break
		}
	}

	log.Printf("PDF Extractor: Calculated %d chunks for %d pages (pagesPerChunk=%d, overlap=%d)",
		len(chunks), totalPages, config.PagesPerChunk, config.OverlapPages)

	return chunks
}
