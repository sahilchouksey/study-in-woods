# PYQ Knowledge Base System Design

**Date**: December 12, 2025  
**Status**: 🚧 In Design  
**Purpose**: Enable semantic search and extraction of Previous Year Questions (PYQs) from scanned/image-based PDFs

---

## 🎯 Problem Statement

**Current Issue**:
- PYQ PDFs are often scanned/image-based (no extractable text)
- Traditional PDF text extraction fails: `insufficient text extracted from PDF (only 0 characters)`
- Cannot extract questions, answers, or metadata from these PDFs

**Solution**:
- Use DigitalOcean's Knowledge Base + Agent system for RAG (Retrieval-Augmented Generation)
- Upload PDFs directly to knowledge base (handles OCR automatically)
- Query the knowledge base with AI agent to extract structured PYQ data
- Support batch uploads (multiple PDFs at once)

---

## 🏗️ Architecture

### High-Level Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    User Uploads PYQ PDF(s)                   │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  1. Upload PDF to DigitalOcean Spaces (existing)             │
│  2. Create Document record (existing)                        │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  3. Get/Create Knowledge Base for Subject                    │
│     - One KB per subject (e.g., "MCA-301-PYQs")             │
│     - Reuse existing KB if available                         │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  4. Add PDF as Data Source to Knowledge Base                 │
│     - Upload PDF to KB (DO handles OCR)                      │
│     - Trigger indexing job                                   │
│     - Wait for indexing to complete                          │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  5. Get/Create AI Agent for PYQ Extraction                   │
│     - One agent per subject (e.g., "MCA-301-PYQ-Agent")     │
│     - Attach knowledge base to agent                         │
│     - Configure with PYQ extraction instructions             │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  6. Query Agent to Extract PYQs                              │
│     - Send structured extraction prompt                      │
│     - Agent searches KB and returns JSON                     │
│     - Parse and validate response                            │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  7. Save Extracted PYQs to Database                          │
│     - Create PYQPaper record                                 │
│     - Create PYQQuestion records                             │
│     - Link to document and subject                           │
└─────────────────────────────────────────────────────────────┘
```

---

## 📊 Database Schema

### Existing Tables (No Changes)

```sql
-- pyq_papers table (existing)
CREATE TABLE pyq_papers (
    id SERIAL PRIMARY KEY,
    subject_id INTEGER NOT NULL,
    document_id INTEGER NOT NULL,
    year INTEGER,
    month VARCHAR(20),
    exam_type VARCHAR(50),
    total_marks INTEGER,
    duration VARCHAR(20),
    total_questions INTEGER,
    instructions TEXT,
    extraction_status VARCHAR(20), -- pending, processing, completed, failed
    extraction_error TEXT,
    raw_extraction TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP
);

-- pyq_questions table (existing)
CREATE TABLE pyq_questions (
    id SERIAL PRIMARY KEY,
    paper_id INTEGER NOT NULL,
    question_number VARCHAR(20),
    question_text TEXT NOT NULL,
    marks INTEGER,
    section VARCHAR(50),
    difficulty VARCHAR(20),
    topics TEXT, -- JSON array of topic IDs
    answer_text TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP
);
```

### New Table: Knowledge Base Tracking

```sql
CREATE TABLE subject_knowledge_bases (
    id SERIAL PRIMARY KEY,
    subject_id INTEGER NOT NULL UNIQUE,
    kb_uuid VARCHAR(255) NOT NULL,
    kb_name VARCHAR(255) NOT NULL,
    agent_uuid VARCHAR(255),
    agent_name VARCHAR(255),
    status VARCHAR(50), -- active, indexing, failed
    total_documents INTEGER DEFAULT 0,
    last_indexed_at TIMESTAMP,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    
    FOREIGN KEY (subject_id) REFERENCES subjects(id) ON DELETE CASCADE
);

CREATE INDEX idx_subject_kb_subject ON subject_knowledge_bases(subject_id);
CREATE INDEX idx_subject_kb_uuid ON subject_knowledge_bases(kb_uuid);
```

---

## 🔧 Implementation Components

### 1. **PYQ Knowledge Base Service** (`services/pyq_kb_service.go`)

**Responsibilities**:
- Manage knowledge bases per subject
- Upload PDFs to knowledge base
- Trigger and monitor indexing jobs
- Manage AI agents for PYQ extraction
- Query agents to extract structured PYQ data

**Key Methods**:

```go
type PYQKnowledgeBaseService struct {
    db          *gorm.DB
    doClient    *digitalocean.Client
    spacesClient *digitalocean.SpacesClient
}

// GetOrCreateKnowledgeBase gets or creates a KB for a subject
func (s *PYQKnowledgeBaseService) GetOrCreateKnowledgeBase(ctx context.Context, subjectID uint) (*SubjectKnowledgeBase, error)

// AddPDFToKnowledgeBase uploads a PDF to the subject's KB
func (s *PYQKnowledgeBaseService) AddPDFToKnowledgeBase(ctx context.Context, subjectID uint, pdfPath string, fileName string) error

// TriggerIndexing starts indexing job for a KB
func (s *PYQKnowledgeBaseService) TriggerIndexing(ctx context.Context, kbUUID string) (*digitalocean.IndexingJob, error)

// WaitForIndexing waits for indexing to complete
func (s *PYQKnowledgeBaseService) WaitForIndexing(ctx context.Context, jobUUID string, timeout time.Duration) error

// GetOrCreateAgent gets or creates an AI agent for a subject
func (s *PYQKnowledgeBaseService) GetOrCreateAgent(ctx context.Context, subjectID uint, kbUUID string) (*digitalocean.Agent, error)

// ExtractPYQsFromKnowledgeBase queries the agent to extract PYQs
func (s *PYQKnowledgeBaseService) ExtractPYQsFromKnowledgeBase(ctx context.Context, agentUUID string, year int, month string) (*PYQExtractionResult, error)
```

### 2. **Updated PYQ Service** (`services/pyq_service.go`)

**New Method**:

```go
// ExtractPYQFromDocumentViaKB extracts PYQ using knowledge base + agent
func (s *PYQService) ExtractPYQFromDocumentViaKB(ctx context.Context, documentID uint) (*model.PYQPaper, error) {
    // 1. Get document
    // 2. Get/create knowledge base for subject
    // 3. Add PDF to knowledge base
    // 4. Trigger indexing
    // 5. Wait for indexing to complete
    // 6. Get/create agent
    // 7. Query agent to extract PYQs
    // 8. Save to database
}
```

### 3. **Batch Upload Handler** (`handlers/pyq/pyq.go`)

**New Endpoint**: `POST /api/v1/subjects/:subject_id/pyqs/batch-upload`

```go
func (h *PYQHandler) BatchUploadPYQs(c *fiber.Ctx) error {
    // 1. Parse multiple file uploads
    // 2. Validate files
    // 3. Upload each to Spaces
    // 4. Create document records
    // 5. Add all to knowledge base
    // 6. Trigger single indexing job
    // 7. Extract PYQs from all documents
    // 8. Return results
}
```

---

## 📝 Extraction Prompt Design

### System Instructions for Agent

```
You are a PYQ (Previous Year Question) extraction expert. Your task is to analyze exam papers and extract structured information.

For each document in the knowledge base, extract:
1. Paper metadata (year, month, exam type, total marks, duration)
2. All questions with their question numbers, text, marks, and sections
3. Any answer keys or solutions if present

Output ONLY valid JSON in this exact format:
{
  "year": 2024,
  "month": "December",
  "exam_type": "End Semester",
  "total_marks": 100,
  "duration": "3 hours",
  "instructions": "Answer all questions...",
  "questions": [
    {
      "question_number": "1",
      "question_text": "Explain data mining...",
      "marks": 10,
      "section": "A",
      "difficulty": "medium",
      "topics": ["data mining", "introduction"],
      "answer_text": "Data mining is..." (if available)
    }
  ]
}
```

### User Query for Extraction

```
Extract all previous year questions from the exam paper uploaded for [Subject Name] in [Month] [Year].

Include:
- All question numbers and text
- Marks for each question
- Section information (A, B, C, etc.)
- Any answer keys or solutions

Return the data in the specified JSON format.
```

---

## 🔄 Workflow Examples

### Example 1: Single PDF Upload

```
User uploads: "mca-301-data-mining-dec-2024.pdf"

1. Upload to Spaces ✓
2. Create Document record ✓
3. Get KB for MCA-301 (create if not exists)
   → KB UUID: "kb-abc123"
   → KB Name: "MCA-301-Data-Mining-PYQs"
4. Add PDF to KB
   → Data Source UUID: "ds-xyz789"
5. Trigger indexing
   → Job UUID: "job-def456"
6. Wait for indexing (polling every 5s, max 2 min)
   → Status: COMPLETED
7. Get Agent for MCA-301 (create if not exists)
   → Agent UUID: "agent-ghi012"
   → Attached to KB: "kb-abc123"
8. Query agent:
   → Prompt: "Extract PYQs from December 2024 exam"
   → Response: { year: 2024, questions: [...] }
9. Save to database:
   → PYQPaper ID: 1
   → 10 PYQQuestions created
```

### Example 2: Batch Upload (3 PDFs)

```
User uploads:
- "mca-301-dec-2024.pdf"
- "mca-301-may-2024.pdf"
- "mca-301-dec-2023.pdf"

1. Upload all 3 to Spaces ✓
2. Create 3 Document records ✓
3. Get KB for MCA-301 (reuse existing)
   → KB UUID: "kb-abc123"
4. Add all 3 PDFs to KB
   → Data Sources: "ds-1", "ds-2", "ds-3"
5. Trigger single indexing job
   → Job UUID: "job-batch-1"
6. Wait for indexing
   → Status: COMPLETED
7. Get Agent (reuse existing)
   → Agent UUID: "agent-ghi012"
8. Query agent 3 times (one per document):
   → Extract Dec 2024 → 10 questions
   → Extract May 2024 → 12 questions
   → Extract Dec 2023 → 8 questions
9. Save to database:
   → 3 PYQPapers created
   → 30 PYQQuestions created
```

---

## ⚙️ Configuration

### Environment Variables

```bash
# Existing
DO_API_TOKEN=your_api_token
DO_SPACES_KEY=your_spaces_key
DO_SPACES_SECRET=your_spaces_secret
DO_SPACES_BUCKET=your_bucket
DO_SPACES_REGION=nyc3

# New (optional, for custom settings)
PYQ_KB_EMBEDDING_MODEL=text-embedding-3-small  # Default embedding model
PYQ_INDEXING_TIMEOUT=120s  # Max wait time for indexing
PYQ_AGENT_MODEL=llama3.3-70b-instruct  # Model for PYQ extraction
```

### Knowledge Base Naming Convention

```
Format: {SubjectCode}-PYQs
Examples:
- MCA-301-PYQs
- MCA-302-PYQs
- CS-101-PYQs
```

### Agent Naming Convention

```
Format: {SubjectCode}-PYQ-Agent
Examples:
- MCA-301-PYQ-Agent
- MCA-302-PYQ-Agent
- CS-101-PYQ-Agent
```

---

## 🧪 Testing Strategy

### Unit Tests

```go
// Test KB creation
func TestGetOrCreateKnowledgeBase(t *testing.T)

// Test PDF upload to KB
func TestAddPDFToKnowledgeBase(t *testing.T)

// Test indexing wait logic
func TestWaitForIndexing(t *testing.T)

// Test agent creation
func TestGetOrCreateAgent(t *testing.T)

// Test PYQ extraction
func TestExtractPYQsFromKnowledgeBase(t *testing.T)
```

### Integration Tests

```bash
# 1. Upload single PYQ PDF
curl -X POST http://localhost:8080/api/v1/subjects/161/pyqs/upload \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@mca-301-dec-2024.pdf"

# 2. Upload batch PYQs
curl -X POST http://localhost:8080/api/v1/subjects/161/pyqs/batch-upload \
  -H "Authorization: Bearer $TOKEN" \
  -F "files=@dec-2024.pdf" \
  -F "files=@may-2024.pdf" \
  -F "files=@dec-2023.pdf"

# 3. Verify extraction
curl http://localhost:8080/api/v1/subjects/161/pyqs \
  -H "Authorization: Bearer $TOKEN"
```

---

## 📈 Performance Considerations

### Indexing Time

- **Small PDF (2-5 pages)**: ~30-60 seconds
- **Medium PDF (10-20 pages)**: ~1-2 minutes
- **Large PDF (50+ pages)**: ~3-5 minutes

### Optimization Strategies

1. **Async Processing**: Don't block user on indexing
   - Return immediately with "processing" status
   - Use background job to poll indexing status
   - Notify user when complete

2. **Batch Indexing**: Add multiple PDFs before triggering indexing
   - More efficient than indexing each PDF separately
   - Single indexing job for all documents

3. **KB Reuse**: One KB per subject (not per document)
   - Reduces API calls
   - Faster subsequent uploads
   - Better semantic search across all PYQs

4. **Agent Reuse**: One agent per subject
   - No need to recreate for each extraction
   - Consistent extraction quality

---

## 🚨 Error Handling

### Common Errors

1. **Indexing Timeout**
   - Retry with longer timeout
   - Check indexing job status manually
   - Notify user to try again later

2. **Agent Query Failure**
   - Retry with exponential backoff
   - Fall back to direct LLM extraction (without KB)
   - Log error for debugging

3. **Invalid JSON Response**
   - Parse with lenient JSON parser
   - Extract partial data if possible
   - Mark as "partial extraction" in database

4. **KB Creation Failure**
   - Check API quota/limits
   - Verify credentials
   - Use existing KB if available

---

## 🔐 Security Considerations

1. **API Key Management**
   - Store DO API token securely
   - Rotate keys periodically
   - Use separate keys for dev/prod

2. **Access Control**
   - Only authenticated users can upload PYQs
   - Subject-level permissions
   - Admin-only batch operations

3. **Data Privacy**
   - PYQs may contain sensitive exam data
   - Ensure KB is private (not public)
   - Delete KB when subject is deleted

---

## 📊 Monitoring & Logging

### Metrics to Track

- KB creation count
- Indexing job success/failure rate
- Average indexing time
- Agent query count
- Extraction success rate
- Questions extracted per document

### Logs

```
[PYQ-KB] Creating knowledge base for subject MCA-301
[PYQ-KB] KB created: kb-abc123
[PYQ-KB] Adding PDF to KB: mca-301-dec-2024.pdf
[PYQ-KB] Data source created: ds-xyz789
[PYQ-KB] Triggering indexing job
[PYQ-KB] Indexing job started: job-def456
[PYQ-KB] Waiting for indexing... (attempt 1/24)
[PYQ-KB] Indexing completed in 45s
[PYQ-KB] Creating agent for subject MCA-301
[PYQ-KB] Agent created: agent-ghi012
[PYQ-KB] Querying agent for PYQ extraction
[PYQ-KB] Extracted 10 questions from document
[PYQ-KB] Saved PYQPaper ID: 1 with 10 questions
```

---

## 🎯 Success Criteria

- ✅ Successfully extract PYQs from scanned/image PDFs
- ✅ Support batch upload of multiple PDFs
- ✅ Indexing completes within 2 minutes for typical PDFs
- ✅ Extraction accuracy > 90% for well-formatted exams
- ✅ One KB per subject (efficient resource usage)
- ✅ Async processing doesn't block user
- ✅ Clear error messages for failures

---

## 🚀 Implementation Plan

1. **Phase 1**: Core KB Service (2-3 hours)
   - Implement `PYQKnowledgeBaseService`
   - KB creation and management
   - PDF upload to KB
   - Indexing job management

2. **Phase 2**: Agent Integration (1-2 hours)
   - Agent creation and management
   - Attach KB to agent
   - Query agent for extraction

3. **Phase 3**: PYQ Service Integration (1 hour)
   - Update `ExtractPYQFromDocument` to use KB
   - Add async processing support
   - Error handling and retries

4. **Phase 4**: Batch Upload (1 hour)
   - New batch upload endpoint
   - Multi-file handling
   - Batch indexing

5. **Phase 5**: Testing & Documentation (1 hour)
   - Integration tests
   - API documentation
   - User guide

**Total Estimated Time**: 6-8 hours

---

## 📚 References

- [DigitalOcean AI Platform API Docs](https://docs.digitalocean.com/products/gradient-ai-platform/)
- [Knowledge Base API](https://docs.digitalocean.com/reference/api/api-reference/#tag/Knowledge-Bases)
- [Agent API](https://docs.digitalocean.com/reference/api/api-reference/#tag/Agents)
- [Chat Completions API](https://docs.digitalocean.com/reference/api/api-reference/#tag/Chat-Completions)

---

**Status**: Ready for implementation! 🚀
