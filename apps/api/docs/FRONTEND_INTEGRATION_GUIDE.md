# Frontend Integration Guide
## SSE-Based Syllabus Extraction Progress Streaming

**Date**: December 14, 2025  
**Version**: 1.0  
**Audience**: Frontend Developers  
**Status**: Single Source of Truth for Frontend Integration

---

## Table of Contents

1. [Overview](#overview)
2. [Quick Start](#quick-start)
3. [Event Types Reference](#event-types-reference)
4. [TypeScript Type Definitions](#typescript-type-definitions)
5. [Implementation Examples](#implementation-examples)
6. [Error Handling](#error-handling)
7. [Testing Guide](#testing-guide)
8. [Troubleshooting](#troubleshooting)

---

## Overview

### What is Server-Sent Events (SSE)?

Server-Sent Events (SSE) is a standard allowing servers to push real-time updates to clients over HTTP. Unlike WebSockets, SSE is:
- **Unidirectional**: Server → Client only
- **HTTP-based**: Works over standard HTTP/HTTPS
- **Auto-reconnecting**: Browser handles reconnection automatically
- **Simple**: Built-in browser API (`EventSource`)

### Why SSE for Syllabus Extraction?

Syllabus extraction takes **60-120 seconds**. SSE provides:
- ✅ Real-time progress updates
- ✅ Better user experience (no "black box" waiting)
- ✅ Error visibility during processing
- ✅ Ability to show detailed status messages

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Your Frontend App                        │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  useExtractionProgress Hook                           │  │
│  │  • Creates EventSource connection                     │  │
│  │  • Listens for SSE events                             │  │
│  │  • Updates React state                                │  │
│  └───────────────────────────────────────────────────────┘  │
│                           │                                  │
│                           ▼                                  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  UI Components                                        │  │
│  │  • Progress bar (0-100%)                              │  │
│  │  • Status message                                     │  │
│  │  • Chunk counter (e.g., "2 of 6 chunks")             │  │
│  │  • Error display                                      │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                           │
                           │ EventSource
                           │ GET /api/v2/documents/:id/extract-syllabus?stream=true
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                    Backend API                              │
│  • Streams SSE events                                       │
│  • Event types: started, progress, warning, complete, error │
└─────────────────────────────────────────────────────────────┘
```

---

## Quick Start

### 1. Install Dependencies

No additional dependencies needed! `EventSource` is built into all modern browsers.

### 2. Basic Usage

```typescript
import { useExtractionProgress } from '@/hooks/useExtractionProgress';

function ExtractionPage({ documentId }: { documentId: number }) {
  const { progress, status, error, isStreaming } = useExtractionProgress(documentId);

  if (error) {
    return <ErrorDisplay error={error} />;
  }

  if (status === 'completed') {
    return <SuccessDisplay />;
  }

  return (
    <div>
      <ProgressBar value={progress} />
      <p>{status}</p>
    </div>
  );
}
```

### 3. That's It!

The hook handles:
- ✅ EventSource connection
- ✅ Event parsing
- ✅ State management
- ✅ Error handling
- ✅ Cleanup on unmount

---

## Event Types Reference

### Event Flow Diagram

```
START
  │
  ▼
┌─────────────┐
│   started   │ ← Initial event (progress: 0%)
└─────────────┘
  │
  ▼
┌─────────────┐
│  progress   │ ← Multiple progress events (5%, 10%, 22%, 34%, ...)
└─────────────┘
  │
  ├─────────────┐
  │             │
  ▼             ▼
┌─────────┐  ┌─────────┐
│ warning │  │ progress│ ← Warning if retry needed
└─────────┘  └─────────┘
  │             │
  └─────────────┘
  │
  ▼
┌─────────────┐
│  complete   │ ← Success (progress: 100%)
└─────────────┘
  OR
┌─────────────┐
│   error     │ ← Fatal error
└─────────────┘
```

### Event Type: `started`

**When**: Extraction begins  
**Progress**: 0%

```json
{
  "type": "started",
  "job_id": "123_1734181800",
  "progress": 0,
  "phase": "initializing",
  "message": "Starting syllabus extraction...",
  "timestamp": "2025-12-14T10:30:00Z"
}
```

**Frontend Action**: Show loading indicator

---

### Event Type: `progress`

**When**: Extraction progresses through phases  
**Progress**: 0-100%  
**Frequency**: ~15-20 events per extraction

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
  "timestamp": "2025-12-14T10:30:45Z"
}
```

**Phases**:
- `download` (0-5%): Downloading PDF
- `chunking` (5-10%): Analyzing document structure
- `extraction` (10-70%): Processing chunks with AI
- `merge` (70-75%): Merging results
- `save` (75-95%): Saving to database
- `complete` (95-100%): Finalizing

**Frontend Action**: Update progress bar and status message

---

### Event Type: `warning`

**When**: Recoverable error occurs (e.g., chunk retry)  
**Progress**: Current progress (unchanged)

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
  "timestamp": "2025-12-14T10:31:20Z"
}
```

**Error Types**:
- `network`: Network connectivity issue
- `llm`: LLM API error (timeout, rate limit, server error)
- `timeout`: Request timeout
- `database`: Database error
- `pdf`: PDF extraction error
- `validation`: Validation error

**Frontend Action**: 
- Show warning toast/notification
- Display retry counter
- Don't stop progress indicator

---

### Event Type: `complete`

**When**: Extraction succeeds  
**Progress**: 100%

```json
{
  "type": "complete",
  "job_id": "123_1734181800",
  "progress": 100,
  "phase": "complete",
  "message": "Extraction completed successfully (12 subjects)",
  "result_syllabus_ids": [456, 457, 458],
  "elapsed_ms": 68000,
  "timestamp": "2025-12-14T10:32:10Z"
}
```

**Frontend Action**:
- Show success message
- Redirect to results page
- Close EventSource connection

---

### Event Type: `error`

**When**: Fatal error (unrecoverable)  
**Progress**: Current progress (frozen)

```json
{
  "type": "error",
  "job_id": "123_1734181800",
  "progress": 45,
  "phase": "extraction",
  "message": "Extraction failed after maximum retries",
  "error_type": "llm_timeout",
  "error_message": "Chunk 3 failed after 3 retry attempts",
  "recoverable": false,
  "timestamp": "2025-12-14T10:32:00Z"
}
```

**Frontend Action**:
- Show error message
- Offer "Retry" button
- Close EventSource connection

---

## TypeScript Type Definitions

### Core Types

```typescript
// types/extraction.ts

export type ExtractionPhase = 
  | 'initializing'
  | 'download'
  | 'chunking'
  | 'extraction'
  | 'merge'
  | 'save'
  | 'complete';

export type ExtractionEventType = 
  | 'started'
  | 'progress'
  | 'warning'
  | 'complete'
  | 'error';

export type ErrorType = 
  | 'network'
  | 'llm'
  | 'timeout'
  | 'database'
  | 'pdf'
  | 'validation'
  | 'unknown';

export interface BaseExtractionEvent {
  type: ExtractionEventType;
  job_id: string;
  progress: number;        // 0-100
  phase: ExtractionPhase;
  message: string;
  timestamp: string;       // ISO 8601
}

export interface StartedEvent extends BaseExtractionEvent {
  type: 'started';
  progress: 0;
}

export interface ProgressEvent extends BaseExtractionEvent {
  type: 'progress';
  total_chunks?: number;
  completed_chunks?: number;
  current_chunk?: number;
}

export interface WarningEvent extends BaseExtractionEvent {
  type: 'warning';
  error_type: ErrorType;
  error_message: string;
  retry_count: number;
  max_retries: number;
  recoverable: true;
}

export interface CompleteEvent extends BaseExtractionEvent {
  type: 'complete';
  progress: 100;
  phase: 'complete';
  result_syllabus_ids: number[];
  elapsed_ms: number;
}

export interface ErrorEvent extends BaseExtractionEvent {
  type: 'error';
  error_type: ErrorType;
  error_message: string;
  recoverable: false;
}

export type ExtractionEvent = 
  | StartedEvent
  | ProgressEvent
  | WarningEvent
  | CompleteEvent
  | ErrorEvent;
```

### Hook State Type

```typescript
export interface ExtractionState {
  // Status
  isStreaming: boolean;
  isComplete: boolean;
  hasError: boolean;
  
  // Progress
  progress: number;        // 0-100
  phase: ExtractionPhase;
  message: string;
  
  // Chunk tracking
  totalChunks?: number;
  completedChunks?: number;
  currentChunk?: number;
  
  // Job info
  jobId?: string;
  
  // Results
  syllabusIds?: number[];
  elapsedMs?: number;
  
  // Error info
  error?: {
    type: ErrorType;
    message: string;
    recoverable: boolean;
  };
  
  // Warnings
  warnings: WarningEvent[];
}
```

---

## Implementation Examples

### Example 1: React Hook (Recommended)

**File**: `src/hooks/useExtractionProgress.ts`

```typescript
import { useState, useEffect, useCallback } from 'react';
import type { ExtractionEvent, ExtractionState, WarningEvent } from '@/types/extraction';

interface UseExtractionProgressOptions {
  documentId: number;
  onComplete?: (syllabusIds: number[]) => void;
  onError?: (error: { type: string; message: string }) => void;
  autoStart?: boolean;
}

export function useExtractionProgress({
  documentId,
  onComplete,
  onError,
  autoStart = true,
}: UseExtractionProgressOptions) {
  const [state, setState] = useState<ExtractionState>({
    isStreaming: false,
    isComplete: false,
    hasError: false,
    progress: 0,
    phase: 'initializing',
    message: 'Waiting to start...',
    warnings: [],
  });

  const startExtraction = useCallback(() => {
    // Get auth token from your auth system
    const token = localStorage.getItem('auth_token');
    if (!token) {
      setState(prev => ({
        ...prev,
        hasError: true,
        error: {
          type: 'validation',
          message: 'Not authenticated',
          recoverable: false,
        },
      }));
      return;
    }

    // Create EventSource with auth header
    const url = `${process.env.NEXT_PUBLIC_API_URL}/api/v2/documents/${documentId}/extract-syllabus?stream=true`;
    
    // Note: EventSource doesn't support custom headers directly
    // You need to pass token as query param or use a proxy
    const urlWithAuth = `${url}&token=${token}`;
    
    const eventSource = new EventSource(urlWithAuth, {
      withCredentials: true,
    });

    setState(prev => ({ ...prev, isStreaming: true }));

    // Handle 'started' event
    eventSource.addEventListener('started', (e) => {
      const event: ExtractionEvent = JSON.parse(e.data);
      setState(prev => ({
        ...prev,
        jobId: event.job_id,
        progress: event.progress,
        phase: event.phase,
        message: event.message,
      }));
    });

    // Handle 'progress' event
    eventSource.addEventListener('progress', (e) => {
      const event: ExtractionEvent = JSON.parse(e.data);
      setState(prev => ({
        ...prev,
        progress: event.progress,
        phase: event.phase,
        message: event.message,
        totalChunks: event.total_chunks,
        completedChunks: event.completed_chunks,
        currentChunk: event.current_chunk,
      }));
    });

    // Handle 'warning' event
    eventSource.addEventListener('warning', (e) => {
      const event: WarningEvent = JSON.parse(e.data);
      setState(prev => ({
        ...prev,
        message: event.message,
        warnings: [...prev.warnings, event],
      }));
    });

    // Handle 'complete' event
    eventSource.addEventListener('complete', (e) => {
      const event: ExtractionEvent = JSON.parse(e.data);
      setState(prev => ({
        ...prev,
        isStreaming: false,
        isComplete: true,
        progress: 100,
        phase: 'complete',
        message: event.message,
        syllabusIds: event.result_syllabus_ids,
        elapsedMs: event.elapsed_ms,
      }));
      
      eventSource.close();
      
      if (onComplete && event.result_syllabus_ids) {
        onComplete(event.result_syllabus_ids);
      }
    });

    // Handle 'error' event
    eventSource.addEventListener('error', (e) => {
      const event: ExtractionEvent = JSON.parse(e.data);
      const error = {
        type: event.error_type,
        message: event.error_message,
        recoverable: event.recoverable,
      };
      
      setState(prev => ({
        ...prev,
        isStreaming: false,
        hasError: true,
        error,
      }));
      
      eventSource.close();
      
      if (onError) {
        onError(error);
      }
    });

    // Handle connection errors
    eventSource.onerror = (e) => {
      console.error('EventSource error:', e);
      
      setState(prev => ({
        ...prev,
        isStreaming: false,
        hasError: true,
        error: {
          type: 'network',
          message: 'Connection lost. Please try again.',
          recoverable: true,
        },
      }));
      
      eventSource.close();
    };

    // Cleanup on unmount
    return () => {
      eventSource.close();
      setState(prev => ({ ...prev, isStreaming: false }));
    };
  }, [documentId, onComplete, onError]);

  useEffect(() => {
    if (autoStart) {
      const cleanup = startExtraction();
      return cleanup;
    }
  }, [autoStart, startExtraction]);

  return {
    ...state,
    startExtraction,
  };
}
```

**Usage:**

```typescript
function ExtractionPage() {
  const { documentId } = useParams();
  const router = useRouter();
  
  const {
    progress,
    phase,
    message,
    isStreaming,
    isComplete,
    hasError,
    error,
    totalChunks,
    completedChunks,
    warnings,
  } = useExtractionProgress({
    documentId: Number(documentId),
    onComplete: (syllabusIds) => {
      // Redirect to results page
      router.push(`/syllabuses?ids=${syllabusIds.join(',')}`);
    },
    onError: (error) => {
      // Show error toast
      toast.error(error.message);
    },
  });

  if (hasError) {
    return (
      <ErrorDisplay 
        error={error} 
        onRetry={() => window.location.reload()}
      />
    );
  }

  if (isComplete) {
    return <SuccessDisplay message="Extraction completed!" />;
  }

  return (
    <div className="extraction-progress">
      <h2>Extracting Syllabus</h2>
      
      {/* Progress Bar */}
      <ProgressBar value={progress} max={100} />
      <p className="progress-text">{progress}%</p>
      
      {/* Status Message */}
      <p className="status-message">{message}</p>
      
      {/* Chunk Counter (if available) */}
      {totalChunks && (
        <p className="chunk-counter">
          Processing chunk {completedChunks} of {totalChunks}
        </p>
      )}
      
      {/* Warnings */}
      {warnings.length > 0 && (
        <div className="warnings">
          {warnings.map((warning, idx) => (
            <div key={idx} className="warning-item">
              ⚠️ {warning.message}
            </div>
          ))}
        </div>
      )}
      
      {/* Phase Indicator */}
      <div className="phase-indicator">
        <PhaseStep name="Download" active={phase === 'download'} complete={progress > 5} />
        <PhaseStep name="Analyze" active={phase === 'chunking'} complete={progress > 10} />
        <PhaseStep name="Extract" active={phase === 'extraction'} complete={progress > 70} />
        <PhaseStep name="Merge" active={phase === 'merge'} complete={progress > 75} />
        <PhaseStep name="Save" active={phase === 'save'} complete={progress > 95} />
      </div>
    </div>
  );
}
```

---

### Example 2: Vanilla JavaScript

```javascript
// vanilla-extraction.js

function startExtraction(documentId, token) {
  const url = `${API_URL}/api/v2/documents/${documentId}/extract-syllabus?stream=true&token=${token}`;
  const eventSource = new EventSource(url, { withCredentials: true });

  // Update UI elements
  const progressBar = document.getElementById('progress-bar');
  const statusText = document.getElementById('status-text');
  const chunkCounter = document.getElementById('chunk-counter');

  eventSource.addEventListener('started', (e) => {
    const data = JSON.parse(e.data);
    statusText.textContent = data.message;
    progressBar.value = 0;
  });

  eventSource.addEventListener('progress', (e) => {
    const data = JSON.parse(e.data);
    progressBar.value = data.progress;
    statusText.textContent = data.message;
    
    if (data.total_chunks) {
      chunkCounter.textContent = `Chunk ${data.completed_chunks}/${data.total_chunks}`;
    }
  });

  eventSource.addEventListener('warning', (e) => {
    const data = JSON.parse(e.data);
    showWarningToast(data.message);
  });

  eventSource.addEventListener('complete', (e) => {
    const data = JSON.parse(e.data);
    progressBar.value = 100;
    statusText.textContent = data.message;
    eventSource.close();
    
    // Redirect to results
    window.location.href = `/syllabuses?ids=${data.result_syllabus_ids.join(',')}`;
  });

  eventSource.addEventListener('error', (e) => {
    const data = JSON.parse(e.data);
    showErrorMessage(data.error_message);
    eventSource.close();
  });

  eventSource.onerror = () => {
    showErrorMessage('Connection lost. Please try again.');
    eventSource.close();
  };
}
```

---

### Example 3: Vue Composition API

```typescript
// composables/useExtractionProgress.ts
import { ref, onUnmounted } from 'vue';
import type { ExtractionState } from '@/types/extraction';

export function useExtractionProgress(documentId: number) {
  const state = ref<ExtractionState>({
    isStreaming: false,
    isComplete: false,
    hasError: false,
    progress: 0,
    phase: 'initializing',
    message: 'Waiting to start...',
    warnings: [],
  });

  let eventSource: EventSource | null = null;

  const startExtraction = () => {
    const token = localStorage.getItem('auth_token');
    const url = `${import.meta.env.VITE_API_URL}/api/v2/documents/${documentId}/extract-syllabus?stream=true&token=${token}`;
    
    eventSource = new EventSource(url, { withCredentials: true });
    state.value.isStreaming = true;

    eventSource.addEventListener('started', (e) => {
      const data = JSON.parse(e.data);
      state.value.jobId = data.job_id;
      state.value.message = data.message;
    });

    eventSource.addEventListener('progress', (e) => {
      const data = JSON.parse(e.data);
      state.value.progress = data.progress;
      state.value.phase = data.phase;
      state.value.message = data.message;
      state.value.totalChunks = data.total_chunks;
      state.value.completedChunks = data.completed_chunks;
    });

    eventSource.addEventListener('warning', (e) => {
      const data = JSON.parse(e.data);
      state.value.warnings.push(data);
    });

    eventSource.addEventListener('complete', (e) => {
      const data = JSON.parse(e.data);
      state.value.isStreaming = false;
      state.value.isComplete = true;
      state.value.progress = 100;
      state.value.syllabusIds = data.result_syllabus_ids;
      eventSource?.close();
    });

    eventSource.addEventListener('error', (e) => {
      const data = JSON.parse(e.data);
      state.value.isStreaming = false;
      state.value.hasError = true;
      state.value.error = {
        type: data.error_type,
        message: data.error_message,
        recoverable: data.recoverable,
      };
      eventSource?.close();
    });

    eventSource.onerror = () => {
      state.value.isStreaming = false;
      state.value.hasError = true;
      state.value.error = {
        type: 'network',
        message: 'Connection lost',
        recoverable: true,
      };
      eventSource?.close();
    };
  };

  onUnmounted(() => {
    eventSource?.close();
  });

  return {
    state,
    startExtraction,
  };
}
```

---

## Error Handling

### Authentication Errors

**Problem**: EventSource doesn't support custom headers (like `Authorization: Bearer <token>`)

**Solutions:**

#### Option 1: Token in Query Parameter (Simplest)
```typescript
const url = `${API_URL}/api/v2/documents/${documentId}/extract-syllabus?stream=true&token=${token}`;
const eventSource = new EventSource(url);
```

**Backend**: Validate token from query parameter

#### Option 2: Cookie-Based Auth (Recommended)
```typescript
const eventSource = new EventSource(url, {
  withCredentials: true, // Send cookies
});
```

**Backend**: Validate JWT from cookie

#### Option 3: Proxy Request (Most Secure)
```typescript
// Frontend: Use fetch with custom headers, then stream
const response = await fetch(url, {
  headers: {
    'Authorization': `Bearer ${token}`,
  },
});

const reader = response.body.getReader();
// Manually parse SSE stream
```

### Network Errors

```typescript
eventSource.onerror = (e) => {
  console.error('EventSource error:', e);
  
  // Check if it's a connection error or server error
  if (eventSource.readyState === EventSource.CLOSED) {
    // Connection closed by server (likely error event sent)
    // Error already handled by 'error' event listener
  } else {
    // Network error (connection lost)
    setState({
      hasError: true,
      error: {
        type: 'network',
        message: 'Connection lost. Please check your internet connection.',
        recoverable: true,
      },
    });
  }
  
  eventSource.close();
};
```

### Timeout Handling

```typescript
const EXTRACTION_TIMEOUT = 5 * 60 * 1000; // 5 minutes

const timeoutId = setTimeout(() => {
  if (state.isStreaming && !state.isComplete) {
    eventSource.close();
    setState({
      hasError: true,
      error: {
        type: 'timeout',
        message: 'Extraction timed out. Please try again.',
        recoverable: true,
      },
    });
  }
}, EXTRACTION_TIMEOUT);

// Clear timeout on completion
eventSource.addEventListener('complete', () => {
  clearTimeout(timeoutId);
});
```

### Retry Logic

```typescript
function useExtractionProgressWithRetry(documentId: number, maxRetries = 3) {
  const [retryCount, setRetryCount] = useState(0);
  
  const { state, startExtraction } = useExtractionProgress({
    documentId,
    onError: (error) => {
      if (error.recoverable && retryCount < maxRetries) {
        // Auto-retry after delay
        setTimeout(() => {
          setRetryCount(prev => prev + 1);
          startExtraction();
        }, 5000); // 5 second delay
      }
    },
  });
  
  return { ...state, retryCount };
}
```

---

## Testing Guide

### Manual Testing with Browser DevTools

1. **Open Network Tab**
   - Filter by "EventStream" or "text/event-stream"
   - You should see the SSE connection

2. **Inspect Events**
   - Click on the EventStream request
   - Go to "EventStream" tab
   - See real-time events as they arrive

3. **Test Disconnection**
   - Throttle network to "Slow 3G"
   - Disconnect network mid-extraction
   - Verify error handling

### Testing with curl

```bash
# Test SSE endpoint
curl -N -H "Authorization: Bearer YOUR_TOKEN" \
  "http://localhost:8080/api/v2/documents/123/extract-syllabus?stream=true"

# Expected output:
# event: started
# data: {"type":"started","job_id":"123_1734181800",...}
#
# event: progress
# data: {"type":"progress","progress":10,...}
#
# ...
```

### Unit Testing React Hook

```typescript
// __tests__/useExtractionProgress.test.ts
import { renderHook, waitFor } from '@testing-library/react';
import { useExtractionProgress } from '@/hooks/useExtractionProgress';

// Mock EventSource
class MockEventSource {
  addEventListener = jest.fn();
  close = jest.fn();
  onerror = null;
}

global.EventSource = MockEventSource as any;

describe('useExtractionProgress', () => {
  it('should start streaming on mount', () => {
    const { result } = renderHook(() => 
      useExtractionProgress({ documentId: 123 })
    );
    
    expect(result.current.isStreaming).toBe(true);
  });
  
  it('should handle progress events', async () => {
    const { result } = renderHook(() => 
      useExtractionProgress({ documentId: 123 })
    );
    
    // Simulate progress event
    const progressHandler = MockEventSource.prototype.addEventListener.mock.calls
      .find(call => call[0] === 'progress')[1];
    
    progressHandler({
      data: JSON.stringify({
        type: 'progress',
        progress: 50,
        message: 'Processing...',
      }),
    });
    
    await waitFor(() => {
      expect(result.current.progress).toBe(50);
    });
  });
});
```

---

## Troubleshooting

### Issue: Events Not Received

**Symptoms**: EventSource connects but no events arrive

**Possible Causes:**
1. **nginx buffering**: Add `X-Accel-Buffering: no` header on backend
2. **CORS**: Ensure CORS allows EventSource connections
3. **Authentication**: Token might be invalid or expired

**Debug Steps:**
```typescript
eventSource.addEventListener('open', () => {
  console.log('EventSource connected');
});

eventSource.addEventListener('message', (e) => {
  console.log('Received event:', e);
});
```

### Issue: Connection Closes Immediately

**Symptoms**: EventSource connects then immediately closes

**Possible Causes:**
1. **401 Unauthorized**: Check authentication
2. **404 Not Found**: Verify URL is correct
3. **500 Server Error**: Check backend logs

**Debug:**
```typescript
eventSource.onerror = (e) => {
  console.log('EventSource state:', eventSource.readyState);
  // 0 = CONNECTING, 1 = OPEN, 2 = CLOSED
};
```

### Issue: Progress Stuck

**Symptoms**: Progress stops updating mid-extraction

**Possible Causes:**
1. **Backend crash**: Check backend logs
2. **Network timeout**: Check network tab
3. **Redis connection lost**: Check Redis status

**Solution**: Implement timeout detection:
```typescript
let lastEventTime = Date.now();

eventSource.addEventListener('progress', () => {
  lastEventTime = Date.now();
});

setInterval(() => {
  if (Date.now() - lastEventTime > 60000) { // 1 minute
    console.error('No events received for 1 minute');
    eventSource.close();
    // Show error to user
  }
}, 10000); // Check every 10 seconds
```

### Issue: Memory Leak

**Symptoms**: Memory usage increases over time

**Cause**: EventSource not closed on component unmount

**Solution**:
```typescript
useEffect(() => {
  const eventSource = new EventSource(url);
  
  // ... event listeners ...
  
  return () => {
    eventSource.close(); // ← CRITICAL
  };
}, []);
```

---

## Best Practices

### 1. Always Close EventSource

```typescript
// ✅ GOOD
useEffect(() => {
  const eventSource = new EventSource(url);
  return () => eventSource.close();
}, []);

// ❌ BAD
useEffect(() => {
  const eventSource = new EventSource(url);
  // No cleanup!
}, []);
```

### 2. Handle All Event Types

```typescript
// ✅ GOOD
eventSource.addEventListener('started', handler);
eventSource.addEventListener('progress', handler);
eventSource.addEventListener('warning', handler);
eventSource.addEventListener('complete', handler);
eventSource.addEventListener('error', handler);
eventSource.onerror = handler;

// ❌ BAD
eventSource.addEventListener('progress', handler);
// Missing other events!
```

### 3. Provide User Feedback

```typescript
// ✅ GOOD
<div>
  <ProgressBar value={progress} />
  <p>{message}</p>
  {totalChunks && <p>Chunk {completedChunks}/{totalChunks}</p>}
  {warnings.map(w => <Warning key={w.timestamp} message={w.message} />)}
</div>

// ❌ BAD
<ProgressBar value={progress} />
// No context for user!
```

### 4. Implement Retry Logic

```typescript
// ✅ GOOD
onError: (error) => {
  if (error.recoverable) {
    showRetryButton();
  } else {
    showFatalError();
  }
}

// ❌ BAD
onError: (error) => {
  alert(error.message); // No retry option!
}
```

### 5. Test Edge Cases

- [ ] Network disconnection mid-extraction
- [ ] Browser refresh during extraction
- [ ] Multiple tabs extracting simultaneously
- [ ] Very slow network (3G)
- [ ] Backend restart during extraction

---

## Summary Checklist

### Implementation Checklist

- [ ] Install/verify EventSource support
- [ ] Create TypeScript types
- [ ] Implement `useExtractionProgress` hook
- [ ] Create UI components (progress bar, status, errors)
- [ ] Handle all event types (started, progress, warning, complete, error)
- [ ] Implement error handling
- [ ] Add retry logic
- [ ] Test with real backend
- [ ] Test edge cases (disconnection, timeout, etc.)
- [ ] Add loading states
- [ ] Add success/error states
- [ ] Implement cleanup on unmount

### Testing Checklist

- [ ] Manual test with browser DevTools
- [ ] Test with curl
- [ ] Unit test hook
- [ ] Integration test with mock backend
- [ ] Test on slow network
- [ ] Test disconnection scenarios
- [ ] Test multiple concurrent extractions
- [ ] Test browser refresh
- [ ] Test timeout scenarios

---

**Document Version**: 1.0  
**Last Updated**: December 14, 2025  
**Status**: ✅ Ready for Frontend Integration

**Questions?** Refer to the [API Reference](./API_REFERENCE.md) for detailed endpoint specifications.
