package services

import (
	"fmt"
	"os"
	"testing"
)

func TestExtractPagesForDebug(t *testing.T) {
	// Read the PDF file
	pdfContent, err := os.ReadFile("../../frm_download_file.pdf")
	if err != nil {
		t.Fatalf("Failed to read PDF: %v", err)
	}

	extractor := NewPDFExtractor()

	// Extract pages 9 and 10
	for _, pageNum := range []int{9, 10} {
		text, err := extractor.ExtractPageRange(pdfContent, pageNum, pageNum)
		if err != nil {
			t.Logf("Error extracting page %d: %v", pageNum, err)
			continue
		}
		fmt.Printf("\n\n========== PAGE %d RAW TEXT ==========\n", pageNum)
		fmt.Println(text)
		fmt.Printf("========== END PAGE %d (length: %d chars) ==========\n\n", pageNum, len(text))
	}
}
