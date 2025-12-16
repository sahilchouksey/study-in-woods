# PYQ Crawler Service

A high-class, extensible service for crawling Previous Year Question (PYQ) papers from various university websites using the Factory pattern.

## Architecture

### Factory Pattern
The service uses the **Factory Pattern** to manage multiple crawler instances:

```
CrawlerFactory (Singleton)
    ├── RGPV Crawler
    ├── [Future: AKTU Crawler]
    └── [Future: Other University Crawlers]
```

### Components

1. **PYQCrawlerInterface** (`crawler_interface.go`)
   - Defines the contract for all crawlers
   - Methods: SearchPapers, GetAllPapers, ValidatePDFURL, GetPaperMetadata

2. **RGPV Crawler** (`rgpv_crawler.go`)
   - Implementation for RGPV Online (https://www.rgpvonline.com)
   - Parses HTML to extract paper links
   - Converts `.html` URLs to `.pdf` URLs
   - Extracts metadata from titles

3. **Crawler Factory** (`crawler_factory.go`)
   - Singleton pattern for managing crawler instances
   - Thread-safe registration and retrieval
   - Supports adding new crawlers at runtime

4. **PYQ Crawler Service** (`../pyq_crawler_service.go`)
   - High-level service for searching papers
   - Aggregates results from multiple crawlers
   - Filters and deduplicates results

## API Endpoints

### 1. Search Available PYQs
```
GET /api/v1/subjects/:subject_id/pyqs/search-available
```

**Query Parameters:**
- `course` (optional): Course name (default: "MCA")
- `semester` (optional): Semester number
- `year` (optional): Filter by specific year
- `month` (optional): Filter by exam month
- `source` (optional): Use specific crawler (default: all)
- `limit` (optional): Maximum results (default: 50)

**Response:**
```json
{
  "data": {
    "subject_id": 123,
    "subject_name": "Data Mining",
    "subject_code": "MCA-301",
    "total_found": 45,
    "available_count": 38,
    "ingested_count": 7,
    "available_papers": [
      {
        "title": "MCA-301-DATA-MINING-DEC-2024",
        "source_url": "https://www.rgpvonline.com/papers/mca-301-data-mining-dec-2024.html",
        "pdf_url": "https://www.rgpvonline.com/papers/mca-301-data-mining-dec-2024.pdf",
        "file_type": "pdf",
        "subject_code": "MCA-301",
        "subject_name": "Data Mining",
        "year": 2024,
        "month": "December",
        "exam_type": "End Semester",
        "source_name": "RGPV Online"
      }
    ]
  }
}
```

### 2. Get Crawler Sources
```
GET /api/v1/pyqs/crawler-sources
```

**Response:**
```json
{
  "data": {
    "sources": [
      {
        "name": "rgpv",
        "display_name": "RGPV Online",
        "base_url": "https://www.rgpvonline.com"
      }
    ],
    "count": 1
  }
}
```

### 3. Ingest Crawled PYQ
```
POST /api/v1/subjects/:subject_id/pyqs/ingest
```

**Request Body:**
```json
{
  "pdf_url": "https://www.rgpvonline.com/papers/mca-301-data-mining-dec-2024.pdf",
  "title": "MCA-301-DATA-MINING-DEC-2024",
  "year": 2024,
  "month": "December",
  "exam_type": "End Semester",
  "source_name": "RGPV Online"
}
```

**Response:**
```json
{
  "data": {
    "message": "PYQ ingestion initiated",
    "status": "pending",
    "details": {
      "pdf_url": "...",
      "title": "...",
      "year": 2024,
      "month": "December",
      "subject_id": 123
    }
  }
}
```

## How It Works

### RGPV Crawler Logic

1. **URL Construction:**
   ```
   Course: "MCA" → https://www.rgpvonline.com/mca.html
   ```

2. **HTML Parsing:**
   - Finds all `<a>` tags with `href` containing `/papers/`
   - Extracts paper titles and HTML URLs

3. **PDF URL Conversion:**
   ```
   HTML: https://www.rgpvonline.com/papers/mca-301-data-mining-dec-2024.html
   PDF:  https://www.rgpvonline.com/papers/mca-301-data-mining-dec-2024.pdf
   ```

4. **Metadata Extraction:**
   - Title: `MCA-301-DATA-MINING-DEC-2024`
   - Subject Code: `MCA-301`
   - Subject Name: `Data Mining`
   - Month: `December` (normalized)
   - Year: `2024`
   - Exam Type: `End Semester` (detected)

## Adding New Crawlers

To add a crawler for a new university:

1. **Create Crawler File:**
   ```go
   // aktu_crawler.go
   package pyq_crawler

   type AKTUCrawler struct {
       BaseCrawler
       httpClient *http.Client
   }

   func NewAKTUCrawler() *AKTUCrawler {
       config := CrawlerConfig{
           Name:        "aktu",
           DisplayName: "AKTU",
           BaseURL:     "https://aktu.ac.in",
           // ... other config
       }
       return &AKTUCrawler{
           BaseCrawler: BaseCrawler{Config: config},
           httpClient:  &http.Client{Timeout: 30 * time.Second},
       }
   }

   // Implement PYQCrawlerInterface methods...
   ```

2. **Register in Factory:**
   ```go
   // In GetCrawlerFactory() function
   factory.RegisterCrawler(NewRGPVCrawler())
   factory.RegisterCrawler(NewAKTUCrawler()) // Add this
   ```

## Features

✅ Factory pattern for extensibility  
✅ HTML parsing with golang.org/x/net/html  
✅ Automatic metadata extraction from titles  
✅ PDF URL validation  
✅ Thread-safe crawler management  
✅ No database dependency for crawling  
✅ Support for multiple sources  
✅ Deduplication based on year and month  

## Future Enhancements

- [ ] Download and upload PDF to storage
- [ ] Automatic PYQ extraction after ingestion
- [ ] Periodic background crawling
- [ ] Crawler health monitoring
- [ ] Support for more universities (AKTU, VTU, etc.)
- [ ] OCR support for scanned PDFs
- [ ] Question similarity detection
- [ ] Bulk ingestion API

## Example Usage

### Frontend Implementation

```typescript
// Search available PYQs for a subject
const searchPYQs = async (subjectId: number) => {
  const response = await fetch(
    `/api/v1/subjects/${subjectId}/pyqs/search-available?year=2024`
  );
  const data = await response.json();
  return data.data.available_papers;
};

// Ingest a specific PYQ
const ingestPYQ = async (subjectId: number, paper: any) => {
  const response = await fetch(
    `/api/v1/subjects/${subjectId}/pyqs/ingest`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        pdf_url: paper.pdf_url,
        title: paper.title,
        year: paper.year,
        month: paper.month,
        exam_type: paper.exam_type,
        source_name: paper.source_name,
      }),
    }
  );
  return response.json();
};
```

## Testing

```bash
# Test RGPV crawler directly
curl "https://www.rgpvonline.com/papers/mca-301-data-mining-dec-2024.pdf" -I

# Test API endpoint
curl "http://localhost:3000/api/v1/subjects/123/pyqs/search-available?course=MCA"

# Get available sources
curl "http://localhost:3000/api/v1/pyqs/crawler-sources"
```

## Notes

- The crawler service is **stateless** - no database required for crawling
- Papers are only saved to DB when user chooses to ingest them
- Already ingested papers are filtered out from search results
- The service validates PDF URLs before returning results
