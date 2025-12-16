package services

import (
	"fmt"
	"os"
	"testing"
)

func TestExtractAllPagesForDebug(t *testing.T) {
	// Read the PDF file
	pdfPath := "/Users/sahilchouksey/Documents/fun/study-in-woods/frm_download_file.pdf"
	pdfContent, err := os.ReadFile(pdfPath)
	if err != nil {
		t.Fatalf("Failed to read PDF: %v", err)
	}

	extractor := NewPDFExtractor()
	
	// Get total pages
	totalPages, err := extractor.GetPageCount(pdfContent)
	if err != nil {
		t.Fatalf("Failed to get page count: %v", err)
	}
	
	fmt.Printf("PDF has %d pages\n\n", totalPages)

	// Extract ALL pages
	for pageNum := 1; pageNum <= totalPages; pageNum++ {
		text, err := extractor.ExtractPageRange(pdfContent, pageNum, pageNum)
		if err != nil {
			fmt.Printf("\n\n========== PAGE %d ERROR ==========\n", pageNum)
			fmt.Printf("Error: %v\n", err)
			fmt.Printf("========== END PAGE %d ==========\n\n", pageNum)
			continue
		}
		fmt.Printf("\n\n")
		fmt.Printf("================================================================================\n")
		fmt.Printf("                              PAGE %d\n", pageNum)
		fmt.Printf("                         (length: %d chars)\n", len(text))
		fmt.Printf("================================================================================\n\n")
		fmt.Println(text)
		fmt.Printf("\n================================================================================\n")
		fmt.Printf("                           END PAGE %d\n", pageNum)
		fmt.Printf("================================================================================\n")
	}
}
