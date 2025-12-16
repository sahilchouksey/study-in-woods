# SSE API Reference
## Syllabus Extraction Progress Streaming

**Date**: December 14, 2025  
**Version**: 2.0  
**Base URL**: `/api/v2`  
**Authentication**: Required (JWT Bearer Token)

---

## Table of Contents

1. [Authentication](#authentication)
2. [Endpoints](#endpoints)
3. [Event Schema Reference](#event-schema-reference)
4. [Error Codes](#error-codes)
5. [Rate Limiting](#rate-limiting)
6. [Examples](#examples)

---

## Authentication

All endpoints require JWT authentication.

### Method 1: Authorization Header (Recommended for non-SSE)

```http
Authorization: Bearer <jwt_token>
```

### Method 2: Query Parameter (For SSE EventSource)

```http
GET /api/v2/documents/123/extract-syllabus?stream=true&token=<jwt_token>
```

**Note**: EventSource API doesn't support custom headers, so token must be passed as query parameter for SSE endpoints.

### Method 3: Cookie-Based (Most Secure for SSE)

```http
Cookie: auth_token=<jwt_token>
```

Backend validates JWT from cookie when `withCredentials: true` is set on EventSource.

---

## Endpoints

### 1. Start Extraction with Streaming

**Endpoint**: `GET /api/v2/documents/:document_id/extract-syllabus`

**Method**: GET

**Authentication**: Required

**Query Parameters**:
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `stream` | boolean | No | `false` | Enable SSE streaming |
| `token` | string | Conditional | - | JWT token (required if using EventSource) |

**Request Example**:
```http
GET /api/v2/documents/123/extract-syllabus?stream=true&token=eyJhbGc... HTTP/1.1
Host: api.example.com
Accept: text/event-stream
```

**Response Headers**:
```http
HTTP/1.1 200 OK
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
Transfer-Encoding: chunked
X-Accel-Buffering: no
```

**Response Body** (SSE Stream):
```
event: started
data: {"type":"started","job_id":"123_1734181800","progress":0,...}

event: progress
data: {"type":"progress","job_id":"123_1734181800","progress":10,...}

event: progress
data: {"type":"progress","job_id":"123_1734181800","progress":22,...}

event: complete
data: {"type":"complete","job_id":"123_1734181800","progress":100,...}
```

**Error Responses**:
| Status Code | Description | Response Body |
|-------------|-------------|---------------|
| 400 | Invalid document ID | `{"success":false,"error":{"code":"BAD_REQUEST","message":"Invalid document ID"}}` |
| 401 | Not authenticated | `{"success":false,"error":{"code":"UNAUTHORIZED","message":"User not authenticated"}}` |
| 403 | Access denied | `{"success":false,"error":{"code":"FORBIDDEN","message":"You don't have permission to access this document"}}` |
| 404 | Document not found | `{"success":false,"error":{"code":"NOT_FOUND","message":"Document not found"}}` |
| 409 | Active job exists | `{"success":false,"error":{"code":"CONFLICT","message":"You already have an active extraction job: 123_1734181800"}}` |
| 500 | Server error | `{"success":false,"error":{"code":"INTERNAL_ERROR","message":"Failed to create job"}}` |

---

### 2. Get Job Status

**Endpoint**: `GET /api/v2/extraction-jobs/:job_id`

**Method**: GET

**Authentication**: Required

**Path Parameters**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `job_id` | string | Yes | Job ID (format: `{document_id}_{timestamp}`) |

**Request Example**:
```http
GET /api/v2/extraction-jobs/123_1734181800 HTTP/1.1
Host: api.example.com
Authorization: Bearer eyJhbGc...
```

**Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "job_id": "123_1734181800",
    "user_id": 456,
    "document_id": 123,
    "status": "processing",
    "progress": 45,
    "current_phase": "extraction",
    "message": "Processing chunk 3 of 6...",
    "total_chunks": 6,
    "completed_chunks": 2,
    "failed_chunks": 0,
    "started_at": "2025-12-14T10:30:00Z",
    "updated_at": "2025-12-14T10:31:00Z"
  }
}
```

**Job Status Values**:
- `pending`: Job created, not started
- `processing`: Extraction in progress
- `completed`: Extraction successful
- `failed`: Extraction failed
- `cancelled`: Job cancelled by user

**Error Responses**:
| Status Code | Description |
|-------------|-------------|
| 401 | Not authenticated |
| 403 | Not authorized to view this job |
| 404 | Job not found or expired |

---

### 3. Reconnect to Existing Job

**Endpoint**: `GET /api/v2/extraction-jobs/:job_id/stream`

**Method**: GET

**Authentication**: Required

**Path Parameters**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `job_id` | string | Yes | Job ID to reconnect to |

**Query Parameters**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `token` | string | Conditional | JWT token (if using EventSource) |

**Request Example**:
```http
GET /api/v2/extraction-jobs/123_1734181800/stream?token=eyJhbGc... HTTP/1.1
Host: api.example.com
Accept: text/event-stream
```

**Behavior**:
- If job is **processing**: Sends current state, then closes (no live updates in v1)
- If job is **completed**: Sends `complete` event immediately
- If job is **failed**: Sends `error` event immediately
- If job is **not found**: Returns 404

**Response** (200 OK):
```
event: progress
data: {"type":"progress","job_id":"123_1734181800","progress":45,...}
```

**Note**: In v1, reconnection only sends current state snapshot. For live updates, use WebSocket (future enhancement).

---

### 4. Upload and Extract with Streaming

**Endpoint**: `POST /api/v2/semesters/:semester_id/syllabus/upload`

**Method**: POST

**Authentication**: Required

**Path Parameters**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `semester_id` | integer | Yes | Semester ID |

**Query Parameters**:
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `stream` | boolean | No | `false` | Enable SSE streaming |

**Request Body** (multipart/form-data):
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `file` | file | Yes | PDF file (max 50MB) |

**Request Example**:
```http
POST /api/v2/semesters/789/syllabus/upload?stream=true HTTP/1.1
Host: api.example.com
Authorization: Bearer eyJhbGc...
Content-Type: multipart/form-data; boundary=----WebKitFormBoundary

------WebKitFormBoundary
Content-Disposition: form-data; name="file"; filename="syllabus.pdf"
Content-Type: application/pdf

<binary data>
------WebKitFormBoundary--
```

**Response** (SSE Stream if `stream=true`):
```
event: started
data: {"type":"started","job_id":"456_1734181900",...}

event: progress
data: {"type":"progress","progress":10,...}

event: complete
data: {"type":"complete","progress":100,...}
```

**Response** (JSON if `stream=false`):
```json
{
  "success": true,
  "message": "Extraction started",
  "data": {
    "job_id": "456_1734181900",
    "status": "processing",
    "progress": 0
  }
}
```

---

## Event Schema Reference

### Base Event Structure

All events share these common fields:

```typescript
interface BaseEvent {
  type: 'started' | 'progress' | 'warning' | 'complete' | 'error';
  job_id: string;
  progress: number;        // 0-100
  phase: string;
  message: string;
  timestamp: string;       // ISO 8601 format
}
```

---

### Event Type: `started`

**Sent**: Once at the beginning of extraction

```json
{
  "type": "started",
  "job_id": "123_1734181800",
  "progress": 0,
  "phase": "initializing",
  "message": "Starting syllabus extraction...",
  "timestamp": "2025-12-14T10:30:00.000Z"
}
```

**Fields**:
| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Always `"started"` |
| `job_id` | string | Unique job identifier |
| `progress` | number | Always `0` |
| `phase` | string | Always `"initializing"` |
| `message` | string | User-friendly status message |
| `timestamp` | string | ISO 8601 timestamp |

---

### Event Type: `progress`

**Sent**: Multiple times during extraction (15-20 events typical)

```json
{
  "type": "progress",
  "job_id": "123_1734181800",
  "progress": 34,
  "phase": "extraction",
  "message": "Processing chunk 2 of 6...",
  "total_chunks": 6,
  "completed_chunks": 2,
  "current_chunk": 2,
  "timestamp": "2025-12-14T10:30:45.123Z"
}
```

**Fields**:
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | Always `"progress"` |
| `job_id` | string | Yes | Job identifier |
| `progress` | number | Yes | Progress percentage (0-100) |
| `phase` | string | Yes | Current phase (see below) |
| `message` | string | Yes | Status message |
| `total_chunks` | number | No | Total chunks (only in extraction phase) |
| `completed_chunks` | number | No | Completed chunks |
| `current_chunk` | number | No | Current chunk being processed |
| `timestamp` | string | Yes | ISO 8601 timestamp |

**Phases**:
| Phase | Progress Range | Description |
|-------|----------------|-------------|
| `download` | 0-5% | Downloading PDF from storage |
| `chunking` | 5-10% | Analyzing document structure |
| `extraction` | 10-70% | Processing chunks with AI |
| `merge` | 70-75% | Merging extracted content |
| `save` | 75-95% | Saving to database |
| `complete` | 95-100% | Finalizing |

---

### Event Type: `warning`

**Sent**: When recoverable errors occur (e.g., chunk retry)

```json
{
  "type": "warning",
  "job_id": "123_1734181800",
  "progress": 34,
  "phase": "extraction",
  "message": "Chunk 2 failed, retrying (attempt 1/3)...",
  "error_type": "llm_timeout",
  "error_message": "LLM request timed out after 60s",
  "retry_count": 1,
  "max_retries": 3,
  "recoverable": true,
  "timestamp": "2025-12-14T10:31:20.456Z"
}
```

**Fields**:
| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Always `"warning"` |
| `job_id` | string | Job identifier |
| `progress` | number | Current progress (unchanged) |
| `phase` | string | Current phase |
| `message` | string | Warning message |
| `error_type` | string | Error category (see below) |
| `error_message` | string | Detailed error description |
| `retry_count` | number | Current retry attempt |
| `max_retries` | number | Maximum retry attempts |
| `recoverable` | boolean | Always `true` for warnings |
| `timestamp` | string | ISO 8601 timestamp |

**Error Types**:
| Type | Description | Recoverable |
|------|-------------|-------------|
| `network` | Network connectivity issue | Yes |
| `llm` | LLM API error (timeout, rate limit) | Yes |
| `timeout` | Request timeout | Yes |
| `database` | Database error | No |
| `pdf` | PDF extraction error | No |
| `validation` | Validation error | No |
| `unknown` | Unknown error | Maybe |

---

### Event Type: `complete`

**Sent**: Once when extraction succeeds

```json
{
  "type": "complete",
  "job_id": "123_1734181800",
  "progress": 100,
  "phase": "complete",
  "message": "Extraction completed successfully (12 subjects)",
  "result_syllabus_ids": [456, 457, 458, 459, 460, 461, 462, 463, 464, 465, 466, 467],
  "elapsed_ms": 68000,
  "timestamp": "2025-12-14T10:32:10.789Z"
}
```

**Fields**:
| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Always `"complete"` |
| `job_id` | string | Job identifier |
| `progress` | number | Always `100` |
| `phase` | string | Always `"complete"` |
| `message` | string | Success message |
| `result_syllabus_ids` | number[] | Array of created syllabus IDs |
| `elapsed_ms` | number | Total extraction time in milliseconds |
| `timestamp` | string | ISO 8601 timestamp |

**After this event**: EventSource should be closed by client.

---

### Event Type: `error`

**Sent**: Once when fatal error occurs

```json
{
  "type": "error",
  "job_id": "123_1734181800",
  "progress": 45,
  "phase": "extraction",
  "message": "Extraction failed after maximum retries",
  "error_type": "llm_timeout",
  "error_message": "Chunk 3 failed after 3 retry attempts: LLM service unavailable",
  "recoverable": false,
  "timestamp": "2025-12-14T10:32:00.123Z"
}
```

**Fields**:
| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Always `"error"` |
| `job_id` | string | Job identifier |
| `progress` | number | Progress when error occurred |
| `phase` | string | Phase when error occurred |
| `message` | string | User-friendly error message |
| `error_type` | string | Error category |
| `error_message` | string | Detailed error description |
| `recoverable` | boolean | Always `false` for fatal errors |
| `timestamp` | string | ISO 8601 timestamp |

**After this event**: EventSource should be closed by client.

---

## Error Codes

### HTTP Error Responses

All non-SSE endpoints return errors in this format:

```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message",
    "details": "Optional detailed information"
  }
}
```

### Error Code Reference

| Code | HTTP Status | Description | Retry? |
|------|-------------|-------------|--------|
| `BAD_REQUEST` | 400 | Invalid request parameters | No |
| `UNAUTHORIZED` | 401 | Missing or invalid authentication | No |
| `FORBIDDEN` | 403 | Insufficient permissions | No |
| `NOT_FOUND` | 404 | Resource not found | No |
| `CONFLICT` | 409 | Active job already exists | Yes (after completion) |
| `VALIDATION_ERROR` | 422 | Request validation failed | No |
| `TOO_MANY_REQUESTS` | 429 | Rate limit exceeded | Yes (after delay) |
| `INTERNAL_ERROR` | 500 | Server error | Yes |
| `SERVICE_UNAVAILABLE` | 503 | Service temporarily down | Yes |

---

## Rate Limiting

### Limits

| Endpoint | Limit | Window | Scope |
|----------|-------|--------|-------|
| All API endpoints | 100 requests | 1 minute | Per API key |
| SSE connections | 5 concurrent | - | Per user |
| Extraction jobs | 1 active | - | Per user |

### Rate Limit Headers

```http
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1734182400
```

### Rate Limit Exceeded Response

```json
{
  "success": false,
  "error": {
    "code": "TOO_MANY_REQUESTS",
    "message": "Rate limit exceeded. Try again in 45 seconds."
  }
}
```

**Response Headers**:
```http
HTTP/1.1 429 Too Many Requests
Retry-After: 45
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1734182445
```

---

## Examples

### Example 1: Complete Extraction Flow (curl)

```bash
# Start extraction with streaming
curl -N -H "Authorization: Bearer YOUR_TOKEN" \
  "https://api.example.com/api/v2/documents/123/extract-syllabus?stream=true"

# Output:
# event: started
# data: {"type":"started","job_id":"123_1734181800","progress":0,...}
#
# event: progress
# data: {"type":"progress","progress":5,"phase":"download",...}
#
# event: progress
# data: {"type":"progress","progress":10,"phase":"chunking",...}
#
# event: progress
# data: {"type":"progress","progress":22,"phase":"extraction","total_chunks":6,"completed_chunks":1,...}
#
# event: progress
# data: {"type":"progress","progress":34,"phase":"extraction","total_chunks":6,"completed_chunks":2,...}
#
# event: warning
# data: {"type":"warning","error_type":"llm_timeout","retry_count":1,...}
#
# event: progress
# data: {"type":"progress","progress":34,"phase":"extraction","total_chunks":6,"completed_chunks":2,...}
#
# event: progress
# data: {"type":"progress","progress":46,"phase":"extraction","total_chunks":6,"completed_chunks":3,...}
#
# event: progress
# data: {"type":"progress","progress":58,"phase":"extraction","total_chunks":6,"completed_chunks":4,...}
#
# event: progress
# data: {"type":"progress","progress":70,"phase":"extraction","total_chunks":6,"completed_chunks":6,...}
#
# event: progress
# data: {"type":"progress","progress":75,"phase":"merge",...}
#
# event: progress
# data: {"type":"progress","progress":95,"phase":"save",...}
#
# event: complete
# data: {"type":"complete","progress":100,"result_syllabus_ids":[456,457,458],...}
```

---

### Example 2: Check Job Status

```bash
# Get current job status
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "https://api.example.com/api/v2/extraction-jobs/123_1734181800"

# Response:
{
  "success": true,
  "data": {
    "job_id": "123_1734181800",
    "user_id": 456,
    "document_id": 123,
    "status": "processing",
    "progress": 45,
    "current_phase": "extraction",
    "message": "Processing chunk 3 of 6...",
    "total_chunks": 6,
    "completed_chunks": 2,
    "started_at": "2025-12-14T10:30:00Z",
    "updated_at": "2025-12-14T10:31:00Z"
  }
}
```

---

### Example 3: Reconnect to Job

```bash
# Reconnect to existing job
curl -N -H "Authorization: Bearer YOUR_TOKEN" \
  "https://api.example.com/api/v2/extraction-jobs/123_1734181800/stream"

# If job is still processing:
# event: progress
# data: {"type":"progress","progress":45,"phase":"extraction",...}

# If job is completed:
# event: complete
# data: {"type":"complete","progress":100,"result_syllabus_ids":[456,457,458],...}

# If job failed:
# event: error
# data: {"type":"error","error_message":"Extraction failed",...}
```

---

### Example 4: Upload and Extract

```bash
# Upload PDF and extract with streaming
curl -X POST \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -F "file=@syllabus.pdf" \
  "https://api.example.com/api/v2/semesters/789/syllabus/upload?stream=true"

# Output: (same SSE stream as Example 1)
```

---

### Example 5: Error Handling

```bash
# Try to start extraction when one is already active
curl -H "Authorization: Bearer YOUR_TOKEN" \
  "https://api.example.com/api/v2/documents/123/extract-syllabus?stream=true"

# Response:
HTTP/1.1 409 Conflict
Content-Type: application/json

{
  "success": false,
  "error": {
    "code": "CONFLICT",
    "message": "You already have an active extraction job: 123_1734181800. Please wait for it to complete or reconnect to it."
  }
}
```

---

## Versioning

### Current Version: v2

**Base URL**: `/api/v2`

**Changes from v1**:
- Added SSE streaming support
- Added job tracking
- Added reconnection capability
- Improved error handling
- Added retry logic with warnings

### Backward Compatibility

v1 endpoints remain available:
- `POST /api/v1/documents/:id/extract-syllabus` (non-streaming)
- `POST /api/v1/semesters/:id/syllabus/upload` (non-streaming)

**Migration Path**:
1. Update frontend to use v2 endpoints
2. Add SSE support
3. Test thoroughly
4. Deprecate v1 (6 months notice)

---

## Changelog

### Version 2.0 (2025-12-14)
- ✨ Added SSE streaming support
- ✨ Added job tracking and status endpoints
- ✨ Added reconnection capability
- ✨ Added retry logic with warning events
- ✨ Added per-user concurrency control
- 🐛 Fixed progress calculation accuracy
- 📝 Improved error messages

### Version 1.0 (2025-01-01)
- Initial release
- Basic extraction endpoints
- Synchronous processing only

---

## Support

### Documentation
- Implementation Plan: `SSE_IMPLEMENTATION_PLAN.md`
- Frontend Guide: `FRONTEND_INTEGRATION_GUIDE.md`
- This API Reference: `SSE_API_REFERENCE.md`

### Contact
- Technical Issues: Create GitHub issue
- Questions: Contact backend team

---

**Document Version**: 1.0  
**Last Updated**: December 14, 2025  
**Status**: ✅ Production Ready
