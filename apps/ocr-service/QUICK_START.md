# OCR Service - Quick Start Guide

## Ports

- **Go API Backend:** Port 8080
- **OCR Service:** Port 8081

## Quick Commands

### Using Makefile (Recommended)

From `apps/api/` directory:

```bash
# Start both API and OCR service
make dev

# Start only OCR service
make ocr-dev

# Start only API (without OCR)
make api-dev

# Stop OCR service
make ocr-stop

# Install OCR dependencies
make ocr-install

# Stop everything
make stop
```

### Manual Commands

```bash
# Install dependencies (one-time)
cd apps/ocr-service
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt

# Start OCR service
python main.py

# Test OCR service
python test_ocr.py
```

## Testing

### 1. Health Check

```bash
curl http://localhost:8081/health
```

**Expected Response:**
```json
{
  "status": "healthy",
  "service": "ocr"
}
```

### 2. Test with Sample PDF

```bash
cd apps/ocr-service
python test_ocr.py
```

**Expected Output:**
```
[SUCCESS] OCR processing completed!
[INFO] Page count: 2
[INFO] Text length: 1932 characters
[SUCCESS] Output saved to: output/mca-301-data-mining-dec-2024_ocr.json
[SUCCESS] Text saved to: output/mca-301-data-mining-dec-2024_ocr.txt
```

### 3. Manual API Test

```bash
# Upload a file
curl -X POST http://localhost:8081/ocr/file \
  -F "file=@data/mca-301-data-mining-dec-2024.pdf"

# Process from URL
curl -X POST http://localhost:8081/ocr/url \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com/document.pdf"}'
```

## Troubleshooting

### Port Already in Use

```bash
# Kill process on port 8081
lsof -ti:8081 | xargs kill -9

# Or use the make command
make ocr-stop
```

### OCR Service Won't Start

```bash
# Check logs
tail -f /tmp/ocr.log

# Verify dependencies
cd apps/ocr-service
source venv/bin/activate
pip list | grep docling
```

### Test Output Not Generated

1. Make sure OCR service is running: `curl http://localhost:8081/health`
2. Check if PDF exists: `ls -la data/`
3. Run test with verbose output: `python test_ocr.py`

## Integration with Go Backend

The Go API automatically uses OCR when you upload PDFs:

```bash
# Upload PDF to your API
curl -X POST http://localhost:8080/api/v1/subjects/1/documents \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -F "file=@test.pdf" \
  -F "type=notes"
```

**What happens:**
1. PDF saved to database
2. Uploaded to Spaces
3. OCR runs automatically (synchronous)
4. Text stored in `documents.ocr_text` field
5. Response returned with OCR data

## File Structure

```
ocr-service/
├── main.py              # FastAPI service (port 8081)
├── test_ocr.py         # Test script
├── requirements.txt     # Dependencies
├── data/               # Input PDFs (for testing)
├── output/             # OCR results (generated)
├── README.md           # Full documentation
└── QUICK_START.md      # This file
```

## Next Steps

1. ✅ Test OCR service: `python test_ocr.py`
2. ✅ Start development: `make dev` (from apps/api/)
3. ✅ Upload a PDF through your API
4. ✅ Verify OCR text in database

## Support

- **Full docs:** See `README.md`
- **API docs:** http://localhost:8081/docs (when running)
- **VPS deployment:** See `README_VPS.md`
