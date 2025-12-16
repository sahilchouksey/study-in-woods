# Simple OCR Service

A lightweight OCR service using IBM's Docling library. Takes a PDF (file or URL) and returns extracted text.

## Features

- **No complexity** - No database, no job tracking, no webhooks
- **2 simple endpoints** - Upload file or provide URL
- **Fast & reliable** - Synchronous processing with Docling OCR

## Installation

```bash
# Create virtual environment
python3 -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate

# Install dependencies
pip install -r requirements.txt
```

## Running the Service

```bash
# Make sure venv is activated
source venv/bin/activate

# Start the service
python main.py
```

Service will start on `http://localhost:8081` (port 8081 - Go API uses 8080)

## API Endpoints

### 1. Health Check
```bash
GET /health
```

### 2. OCR from File Upload
```bash
POST /ocr/file
Content-Type: multipart/form-data

# Example
curl -X POST http://localhost:8081/ocr/file \
  -F "file=@data/mca-301-data-mining-dec-2024.pdf"
```

**Response:**
```json
{
  "text": "Extracted text content...",
  "page_count": 10,
  "filename": "mca-301-data-mining-dec-2024.pdf"
}
```

### 3. OCR from URL
```bash
POST /ocr/url
Content-Type: application/json

# Example
curl -X POST http://localhost:8081/ocr/url \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com/document.pdf"}'
```

**Response:**
```json
{
  "text": "Extracted text content...",
  "page_count": 10,
  "source_url": "https://example.com/document.pdf"
}
```

## Testing

A test script is provided to test the OCR service with a sample PDF:

```bash
# Make sure OCR service is running first
python main.py

# In another terminal, run the test
python test_ocr.py
```

The test will:
1. Check if the service is healthy
2. Process the PDF from `data/` directory
3. Save OCR results to `output/` directory
4. Generate both JSON and TXT outputs

## Integration with Go Backend

The Go backend (`apps/api`) uses this service via the `OCRClient`:

```go
// Process PDF from file bytes
ocrResp, err := ocrClient.ProcessPDFFile(ctx, pdfBytes, filename)

// Process PDF from URL
ocrResp, err := ocrClient.ProcessPDFFromURL(ctx, pdfURL)
```

## Directory Structure

```
ocr-service/
├── main.py              # FastAPI application
├── requirements.txt     # Python dependencies
├── test_ocr.py         # Test script
├── data/               # Test PDFs (input)
├── output/             # OCR results (generated)
└── README.md           # This file
```

## Using with Makefile

From the `apps/api` directory:

```bash
# Start both API and OCR service
make dev

# Start only OCR service
make ocr-dev

# Install OCR dependencies
make ocr-install

# Stop OCR service
make ocr-stop
```

## License

Part of Study in Woods project
