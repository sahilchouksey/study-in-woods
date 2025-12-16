# SSE Streaming Implementation Guide

**Date**: December 14, 2025  
**Purpose**: Complete documentation of existing streaming patterns for SSE implementation  
**Author**: Research of existing codebase patterns

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Existing Streaming Implementation](#existing-streaming-implementation)
3. [HTTP Client Configuration](#http-client-configuration)
4. [Streaming Patterns & Best Practices](#streaming-patterns--best-practices)
5. [Fiber SSE Implementation](#fiber-sse-implementation)
6. [Error Handling in Streaming Context](#error-handling-in-streaming-context)
7. [Reusable Patterns](#reusable-patterns)
8. [Implementation Checklist](#implementation-checklist)
9. [Gotchas & Common Mistakes](#gotchas--common-mistakes)

---

## Executive Summary

The codebase already has **production-ready SSE streaming** for chat completions. This guide extracts lessons learned and reusable patterns for implementing SSE in syllabus extraction.

### Key Findings

✅ **Working Implementation**: `services/digitalocean/chat.go::StreamChatCompletion()`  
✅ **Fiber Integration**: `handlers/chat/chats.go::handleStreamMessage()`  
✅ **Callback Pattern**: Clean separation of concerns  
✅ **Error Handling**: Graceful error propagation in streams  
✅ **Connection Pooling**: Optimized HTTP client configuration  

### Architecture Pattern

```
Client Request
    ↓
Fiber Handler (sets SSE headers)
    ↓
SetBodyStreamWriter (Fiber streaming API)
    ↓
Service Layer (StreamMessage)
    ↓
DigitalOcean Client (StreamChatCompletion)
    ↓
Callback Function (for each chunk)
    ↓
bufio.Writer.Flush() → Client
```

---

## Existing Streaming Implementation

### 1. DigitalOcean Chat Streaming (`services/digitalocean/chat.go`)

#### StreamChatCompletion Method

**Location**: `apps/api/services/digitalocean/chat.go:82-155`

```go
// StreamChatCompletion creates a streaming chat completion
func (c *Client) StreamChatCompletion(
    ctx context.Context, 
    req ChatCompletionRequest, 
    callback func(StreamChunk) error,
) error {
    endpoint := fmt.Sprintf("%s/v2/gen-ai/agents/%s/chat/completions", 
        c.baseURL, req.AgentUUID)

    // Force stream to true
    req.Stream = true

    // Build request body
    jsonBody, err := json.Marshal(req)
    if err != nil {
        return fmt.Errorf("failed to marshal request: %w", err)
    }

    // Create HTTP request
    httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
    if err != nil {
        return fmt.Errorf("failed to create request: %w", err)
    }

    // Set headers for SSE
    httpReq.Header.Set("Authorization", "Bearer "+c.apiToken)
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("Accept", "text/event-stream")  // ← Critical header

    // Make request
    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()  // ← Always close response body

    // Check status before streaming
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("streaming failed with status %d: %s", resp.StatusCode, string(body))
    }

    // Read SSE stream line-by-line
    scanner := bufio.NewScanner(resp.Body)
    for scanner.Scan() {
        line := scanner.Text()

        // Skip empty lines and comments
        if line == "" || strings.HasPrefix(line, ":") {
            continue
        }

        // Parse SSE data
        if strings.HasPrefix(line, "data: ") {
            data := strings.TrimPrefix(line, "data: ")

            // Check for stream end signal
            if data == "[DONE]" {
                break
            }

            // Parse JSON chunk
            var chunk StreamChunk
            if err := json.Unmarshal([]byte(data), &chunk); err != nil {
                // Log error but continue streaming (graceful degradation)
                continue
            }

            // Call callback with chunk
            if err := callback(chunk); err != nil {
                return fmt.Errorf("callback error: %w", err)
            }
        }
    }

    // Check for scanner errors
    if err := scanner.Err(); err != nil {
        return fmt.Errorf("stream reading error: %w", err)
    }

    return nil
}
```

#### Key Data Structures

```go
// StreamChunk represents a chunk in a streaming response
type StreamChunk struct {
    ID      string `json:"id"`
    Model   string `json:"model"`
    Choices []struct {
        Index int `json:"index"`
        Delta struct {
            Role    string `json:"role,omitempty"`
            Content string `json:"content,omitempty"`  // ← The actual content chunk
        } `json:"delta"`
        FinishReason string `json:"finish_reason,omitempty"`
    } `json:"choices"`
    Created int `json:"created"`
}
```

#### Pattern Breakdown

| **Aspect** | **Implementation** | **Why It Matters** |
|------------|-------------------|-------------------|
| **Header** | `Accept: text/event-stream` | Tells server to stream SSE format |
| **Scanner** | `bufio.NewScanner(resp.Body)` | Line-by-line reading, low memory |
| **Parsing** | `strings.HasPrefix(line, "data: ")` | Standard SSE format detection |
| **End Signal** | `data == "[DONE]"` | Graceful stream termination |
| **Callback** | `callback(chunk)` | Decoupled processing logic |
| **Error Handling** | Continue on parse errors | Resilience over strict validation |

---

### 2. Service Layer Integration (`services/chat_service.go`)

#### StreamMessage Method

**Location**: `apps/api/services/chat_service.go:242-380`

```go
// StreamCallback is called for each chunk of streamed content
type StreamCallback func(chunk string) error

// StreamMessage sends a message and streams AI response
func (s *ChatService) StreamMessage(
    ctx context.Context, 
    req StreamMessageRequest, 
    callback StreamCallback,
) (*SendMessageResponse, error) {
    result := &SendMessageResponse{}

    // ... validation and setup ...

    // Save user message BEFORE streaming starts
    userMessage := model.ChatMessage{
        SessionID: req.SessionID,
        SubjectID: session.SubjectID,
        UserID:    req.UserID,
        Role:      model.MessageRoleUser,
        Content:   req.Content,
    }

    if err := tx.Create(&userMessage).Error; err != nil {
        tx.Rollback()
        return nil, fmt.Errorf("failed to save user message: %w", err)
    }
    result.UserMessage = &userMessage

    // Commit user message immediately (don't wait for stream)
    if err := tx.Commit().Error; err != nil {
        return nil, fmt.Errorf("failed to commit user message: %w", err)
    }

    // Get conversation history
    var history []model.ChatMessage
    if err := s.db.Where("session_id = ?", req.SessionID).
        Order("created_at ASC").
        Limit(10).
        Find(&history).Error; err != nil {
        return nil, fmt.Errorf("failed to fetch conversation history: %w", err)
    }

    // Build messages for AI
    var messages []digitalocean.ChatMessage
    for _, msg := range history {
        messages = append(messages, digitalocean.ChatMessage{
            Role:    string(msg.Role),
            Content: msg.Content,
        })
    }

    // Stream AI response
    startTime := time.Now()
    var fullContent strings.Builder  // ← Accumulate chunks

    aiReq := digitalocean.ChatCompletionRequest{
        AgentUUID: session.AgentUUID,
        Messages:  messages,
    }

    // Call streaming API with inline callback
    err := s.doClient.StreamChatCompletion(ctx, aiReq, func(chunk digitalocean.StreamChunk) error {
        if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
            content := chunk.Choices[0].Delta.Content
            fullContent.WriteString(content)  // ← Save for DB

            // Call user's callback (Fiber handler)
            if err := callback(content); err != nil {
                return err  // ← Propagate errors up
            }
        }
        return nil
    })

    if err != nil {
        return nil, fmt.Errorf("failed to stream AI response: %w", err)
    }

    responseTime := time.Since(startTime).Milliseconds()

    // Save complete assistant message AFTER streaming completes
    tx = s.db.Begin()
    if tx.Error != nil {
        return nil, fmt.Errorf("failed to begin transaction for assistant message: %w", tx.Error)
    }

    assistantMessage := model.ChatMessage{
        SessionID:    req.SessionID,
        SubjectID:    session.SubjectID,
        UserID:       req.UserID,
        Role:         model.MessageRoleAssistant,
        Content:      fullContent.String(),  // ← Complete content
        ResponseTime: int(responseTime),
        IsStreamed:   true,
    }

    if err := tx.Create(&assistantMessage).Error; err != nil {
        tx.Rollback()
        return nil, fmt.Errorf("failed to save assistant message: %w", err)
    }
    result.AssistantMessage = &assistantMessage

    // Update session statistics
    now := time.Now()
    if err := tx.Model(&session).Updates(map[string]interface{}{
        "message_count":   gorm.Expr("message_count + ?", 2),
        "last_message_at": now,
    }).Error; err != nil {
        tx.Rollback()
        return nil, fmt.Errorf("failed to update session: %w", err)
    }

    if err := tx.Commit().Error; err != nil {
        return nil, fmt.Errorf("failed to commit transaction: %w", err)
    }

    return result, nil
}
```

#### Key Patterns

1. **Early User Message Save**: Commit user message before streaming starts
2. **Content Accumulation**: Use `strings.Builder` to accumulate chunks
3. **Dual Callback**: Inner callback accumulates + calls outer callback
4. **Post-Stream Persistence**: Save complete message after streaming ends
5. **Timing Metrics**: Track response time for analytics

---

### 3. Fiber Handler Integration (`handlers/chat/chats.go`)

#### handleStreamMessage Method

**Location**: `apps/api/handlers/chat/chats.go:284-326`

```go
// handleStreamMessage handles streaming chat responses
func (h *ChatHandler) handleStreamMessage(
    c *fiber.Ctx, 
    sessionID uint, 
    userID uint, 
    content string,
) error {
    // Set headers for SSE
    c.Set("Content-Type", "text/event-stream")
    c.Set("Cache-Control", "no-cache")
    c.Set("Connection", "keep-alive")
    c.Set("Transfer-Encoding", "chunked")

    // Set response to streaming
    c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
        // Send initial event
        fmt.Fprintf(w, "event: start\n")
        fmt.Fprintf(w, "data: {\"status\":\"streaming\"}\n\n")
        w.Flush()

        // Stream message
        result, err := h.chatService.StreamMessage(c.Context(), services.StreamMessageRequest{
            SessionID: sessionID,
            UserID:    userID,
            Content:   content,
        }, func(chunk string) error {
            // Send chunk as SSE
            fmt.Fprintf(w, "event: chunk\n")
            fmt.Fprintf(w, "data: %s\n\n", chunk)
            return w.Flush()  // ← Flush immediately!
        })

        if err != nil {
            // Send error event
            fmt.Fprintf(w, "event: error\n")
            fmt.Fprintf(w, "data: {\"error\":\"%s\"}\n\n", err.Error())
            w.Flush()
            return
        }

        // Send completion event
        fmt.Fprintf(w, "event: done\n")
        fmt.Fprintf(w, "data: {\"user_message_id\":%d,\"assistant_message_id\":%d}\n\n",
            result.UserMessage.ID, result.AssistantMessage.ID)
        w.Flush()
    })

    return nil
}
```

#### SSE Format Breakdown

```
event: start
data: {"status":"streaming"}

event: chunk
data: Hello

event: chunk
data:  world

event: done
data: {"user_message_id":123,"assistant_message_id":124}
```

**Format Rules**:
- Each event has two lines: `event: <type>` and `data: <payload>`
- Events are separated by blank lines (`\n\n`)
- Data can be JSON or plain text
- Custom event names (`start`, `chunk`, `done`, `error`)

#### Critical Fiber API

```go
c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
    // This function controls the entire response lifecycle
    // Write to w using fmt.Fprintf
    // Call w.Flush() to send data immediately
    // Return from function to close connection
})
```

**Key Points**:
- `SetBodyStreamWriter` takes full control of response
- Must use `bufio.Writer` methods
- `w.Flush()` is critical - data won't send without it
- Returning from function closes the connection
- No need to manually set status code (defaults to 200)

---

## HTTP Client Configuration

### 1. DigitalOcean Client Setup (`services/digitalocean/client.go`)

**Location**: `apps/api/services/digitalocean/client.go:35-50`

```go
// NewClient creates a new DigitalOcean API client
func NewClient(config Config) *Client {
    if config.BaseURL == "" {
        config.BaseURL = BaseURL
    }
    if config.Timeout == 0 {
        config.Timeout = DefaultTimeout  // 30s
    }

    return &Client{
        apiToken: config.APIToken,
        baseURL:  config.BaseURL,
        httpClient: &http.Client{
            Timeout: config.Timeout,
        },
    }
}
```

⚠️ **Note**: Basic client - no connection pooling. Sufficient for low-concurrency streaming.

---

### 2. Inference Client (Optimized) (`services/digitalocean/inference.go`)

**Location**: `apps/api/services/digitalocean/inference.go:43-70`

```go
func NewInferenceClient(config InferenceConfig) *InferenceClient {
    if config.BaseURL == "" {
        config.BaseURL = InferenceBaseURL
    }
    if config.Timeout == 0 {
        config.Timeout = DefaultInferenceTimeout  // 300s (5 minutes)
    }
    if config.Model == "" {
        config.Model = DefaultInferenceModel
    }

    return &InferenceClient{
        apiKey:  config.APIKey,
        baseURL: config.BaseURL,
        httpClient: &http.Client{
            Timeout: config.Timeout,
            Transport: &http.Transport{
                MaxIdleConns:        100,              // ← Total max idle connections
                MaxIdleConnsPerHost: 20,               // ← Up from default 2
                MaxConnsPerHost:     0,                // ← 0 = unlimited
                IdleConnTimeout:     90 * time.Second, // ← Keep connections alive
                DisableKeepAlives:   false,            // ← Enable connection reuse
                ForceAttemptHTTP2:   true,             // ← Use HTTP/2 if available
            },
        },
        model: config.Model,
    }
}
```

#### Configuration Explanation

| **Setting** | **Value** | **Impact** | **Why** |
|------------|-----------|-----------|---------|
| `MaxIdleConns` | 100 | Pool of 100 reusable connections | Reduce connection overhead |
| `MaxIdleConnsPerHost` | 20 | Up to 20 concurrent requests to same host | Critical for parallel chunk processing |
| `MaxConnsPerHost` | 0 (unlimited) | No limit on total connections | Prevent bottleneck during bursts |
| `IdleConnTimeout` | 90s | Connections stay alive 90s | Balance between reuse and resource cleanup |
| `DisableKeepAlives` | false | Enable HTTP keep-alive | Reuse TCP connections |
| `ForceAttemptHTTP2` | true | Use HTTP/2 if server supports | Multiplexing, header compression |
| `Timeout` | 300s (5 min) | Request timeout | Long-running LLM inference |

**For SSE Streaming**:
- **DO NOT** set `Timeout` on streaming requests
- Use `context.Context` for cancellation instead
- Timeout applies to entire stream, not per-chunk

---

## Streaming Patterns & Best Practices

### Pattern 1: Callback-Based Streaming

**✅ Pros**:
- Clean separation of concerns
- Easy to test
- Reusable across handlers
- Error propagation is clear

**Example**:

```go
// Client layer
func (c *Client) StreamData(ctx context.Context, callback func(Chunk) error) error {
    // ... setup streaming connection ...
    
    for scanner.Scan() {
        var chunk Chunk
        json.Unmarshal(scanner.Bytes(), &chunk)
        
        if err := callback(chunk); err != nil {
            return err  // Propagate callback errors
        }
    }
    
    return scanner.Err()
}

// Service layer
func (s *Service) ProcessStream(ctx context.Context, userCallback func(string) error) error {
    var accumulated strings.Builder
    
    err := s.client.StreamData(ctx, func(chunk Chunk) error {
        accumulated.WriteString(chunk.Content)
        return userCallback(chunk.Content)  // Chain callbacks
    })
    
    // ... save accumulated data ...
    return err
}

// Handler layer
func (h *Handler) HandleStream(c *fiber.Ctx) error {
    c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
        h.service.ProcessStream(c.Context(), func(content string) error {
            fmt.Fprintf(w, "data: %s\n\n", content)
            return w.Flush()
        })
    })
    return nil
}
```

---

### Pattern 2: Channel-Based Streaming (Alternative)

**Use when**: Multiple consumers, fan-out, or backpressure needed

```go
func (c *Client) StreamDataChan(ctx context.Context) (<-chan Chunk, <-chan error) {
    chunkChan := make(chan Chunk, 10)  // Buffered for backpressure
    errChan := make(chan error, 1)
    
    go func() {
        defer close(chunkChan)
        defer close(errChan)
        
        // ... setup connection ...
        
        for scanner.Scan() {
            var chunk Chunk
            json.Unmarshal(scanner.Bytes(), &chunk)
            
            select {
            case chunkChan <- chunk:
            case <-ctx.Done():
                errChan <- ctx.Err()
                return
            }
        }
        
        if err := scanner.Err(); err != nil {
            errChan <- err
        }
    }()
    
    return chunkChan, errChan
}

// Usage
func (h *Handler) HandleStream(c *fiber.Ctx) error {
    chunkChan, errChan := h.client.StreamDataChan(c.Context())
    
    c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
        for {
            select {
            case chunk, ok := <-chunkChan:
                if !ok {
                    return  // Channel closed
                }
                fmt.Fprintf(w, "data: %s\n\n", chunk.Content)
                w.Flush()
                
            case err := <-errChan:
                fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
                w.Flush()
                return
                
            case <-c.Context().Done():
                return  // Client disconnected
            }
        }
    })
    
    return nil
}
```

**⚠️ Channel Pattern Trade-offs**:
- More complex code
- Goroutine management required
- Better for fan-out scenarios
- Our use case: **Callback pattern is simpler and sufficient**

---

### Pattern 3: SSE Event Types

**Standard Event Types**:

```go
const (
    EventStart    = "start"      // Stream initialization
    EventProgress = "progress"   // Progress updates
    EventChunk    = "chunk"      // Data chunks
    EventDone     = "done"       // Successful completion
    EventError    = "error"      // Error occurred
)

func SendSSEEvent(w *bufio.Writer, eventType, data string) error {
    fmt.Fprintf(w, "event: %s\n", eventType)
    fmt.Fprintf(w, "data: %s\n\n", data)
    return w.Flush()
}

// Usage in handler
c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
    SendSSEEvent(w, EventStart, `{"status":"started"}`)
    
    for chunk := range chunks {
        SendSSEEvent(w, EventChunk, chunk.Content)
    }
    
    SendSSEEvent(w, EventDone, `{"status":"completed"}`)
})
```

---

## Fiber SSE Implementation

### Complete Working Example

```go
package handlers

import (
    "bufio"
    "encoding/json"
    "fmt"
    "github.com/gofiber/fiber/v2"
)

type StreamHandler struct {
    service *services.ExtractionService
}

// SSE headers configuration
func setSSEHeaders(c *fiber.Ctx) {
    c.Set("Content-Type", "text/event-stream")
    c.Set("Cache-Control", "no-cache")
    c.Set("Connection", "keep-alive")
    c.Set("Transfer-Encoding", "chunked")
    c.Set("X-Accel-Buffering", "no")  // Disable nginx buffering
}

// Helper to send SSE events
func sendSSEEvent(w *bufio.Writer, event, data string) error {
    if event != "" {
        fmt.Fprintf(w, "event: %s\n", event)
    }
    fmt.Fprintf(w, "data: %s\n\n", data)
    return w.Flush()
}

func (h *StreamHandler) ExtractWithProgress(c *fiber.Ctx) error {
    documentID := c.Params("document_id")
    
    // Set SSE headers
    setSSEHeaders(c)
    
    // Start streaming
    c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
        // Send start event
        sendSSEEvent(w, "start", `{"status":"starting"}`)
        
        // Stream extraction with progress
        result, err := h.service.ExtractWithProgress(
            c.Context(),
            documentID,
            func(progress ExtractionProgress) error {
                // Send progress event
                data, _ := json.Marshal(progress)
                return sendSSEEvent(w, "progress", string(data))
            },
        )
        
        if err != nil {
            // Send error event
            errData, _ := json.Marshal(map[string]string{
                "error": err.Error(),
            })
            sendSSEEvent(w, "error", string(errData))
            return
        }
        
        // Send completion event
        resultData, _ := json.Marshal(result)
        sendSSEEvent(w, "done", string(resultData))
    })
    
    return nil
}
```

### Frontend Integration (TypeScript/React)

```typescript
// Custom hook for SSE streaming
function useSSEStream<T>(url: string) {
  const [progress, setProgress] = useState<T | null>(null);
  const [result, setResult] = useState<any>(null);
  const [error, setError] = useState<Error | null>(null);
  const [isStreaming, setIsStreaming] = useState(false);

  useEffect(() => {
    const eventSource = new EventSource(url, {
      withCredentials: true,
    });

    setIsStreaming(true);

    eventSource.addEventListener('start', (e) => {
      console.log('Stream started:', e.data);
    });

    eventSource.addEventListener('progress', (e) => {
      const data = JSON.parse(e.data);
      setProgress(data);
    });

    eventSource.addEventListener('done', (e) => {
      const data = JSON.parse(e.data);
      setResult(data);
      setIsStreaming(false);
      eventSource.close();
    });

    eventSource.addEventListener('error', (e) => {
      const data = JSON.parse(e.data);
      setError(new Error(data.error));
      setIsStreaming(false);
      eventSource.close();
    });

    // Handle connection errors
    eventSource.onerror = (e) => {
      console.error('EventSource error:', e);
      setError(new Error('Connection lost'));
      setIsStreaming(false);
      eventSource.close();
    };

    return () => {
      eventSource.close();
      setIsStreaming(false);
    };
  }, [url]);

  return { progress, result, error, isStreaming };
}

// Usage in component
function ExtractionProgress({ documentId }: { documentId: string }) {
  const { progress, result, error, isStreaming } = useSSEStream(
    `/api/v1/documents/${documentId}/extract-stream`
  );

  if (error) {
    return <ErrorDisplay error={error} />;
  }

  if (result) {
    return <SuccessDisplay result={result} />;
  }

  return (
    <div>
      {isStreaming && progress && (
        <ProgressBar 
          value={progress.progress * 100}
          message={progress.message}
        />
      )}
    </div>
  );
}
```

---

## Error Handling in Streaming Context

### 1. Client-Side Error Handling (DigitalOcean API)

```go
// From services/digitalocean/chat.go

// Check status BEFORE starting to stream
if resp.StatusCode != http.StatusOK {
    body, _ := io.ReadAll(resp.Body)
    return fmt.Errorf("streaming failed with status %d: %s", resp.StatusCode, string(body))
}

// Parse errors: log but continue (graceful degradation)
if err := json.Unmarshal([]byte(data), &chunk); err != nil {
    // Log error but continue streaming
    continue
}

// Scanner errors: return after stream completes
if err := scanner.Err(); err != nil {
    return fmt.Errorf("stream reading error: %w", err)
}
```

**Lessons**:
- ✅ Validate status code before streaming
- ✅ Continue on parse errors (some chunks may be malformed)
- ✅ Check scanner errors after loop
- ✅ Use `defer resp.Body.Close()` to prevent leaks

---

### 2. Service Layer Error Handling

```go
// From services/chat_service.go

// Commit user message BEFORE streaming (fail fast)
if err := tx.Create(&userMessage).Error; err != nil {
    tx.Rollback()
    return nil, fmt.Errorf("failed to save user message: %w", err)
}

// Stream with callback error propagation
err := s.doClient.StreamChatCompletion(ctx, aiReq, func(chunk digitalocean.StreamChunk) error {
    // ... process chunk ...
    
    if err := callback(content); err != nil {
        return err  // ← Propagate callback errors up
    }
    return nil
})

if err != nil {
    return nil, fmt.Errorf("failed to stream AI response: %w", err)
}

// Save complete message AFTER successful stream
tx = s.db.Begin()
if err := tx.Create(&assistantMessage).Error; err != nil {
    tx.Rollback()
    return nil, fmt.Errorf("failed to save assistant message: %w", err)
}
```

**Lessons**:
- ✅ Save user input before streaming (immediate feedback)
- ✅ Propagate callback errors (stop stream on flush errors)
- ✅ Save complete result after stream (transactional integrity)
- ✅ Use separate transactions (don't hold locks during streaming)

---

### 3. Handler Layer Error Handling

```go
// From handlers/chat/chats.go

c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
    // Send start event
    fmt.Fprintf(w, "event: start\n")
    fmt.Fprintf(w, "data: {\"status\":\"streaming\"}\n\n")
    w.Flush()
    
    // Stream with error handling
    result, err := h.chatService.StreamMessage(c.Context(), req, func(chunk string) error {
        fmt.Fprintf(w, "event: chunk\n")
        fmt.Fprintf(w, "data: %s\n\n", chunk)
        return w.Flush()  // ← Return flush error to stop stream
    })
    
    if err != nil {
        // Send error event to client
        fmt.Fprintf(w, "event: error\n")
        fmt.Fprintf(w, "data: {\"error\":\"%s\"}\n\n", err.Error())
        w.Flush()
        return  // ← Exit stream writer
    }
    
    // Send success event
    fmt.Fprintf(w, "event: done\n")
    fmt.Fprintf(w, "data: {\"user_message_id\":%d,...}\n\n", ...)
    w.Flush()
})
```

**Lessons**:
- ✅ Always send a `start` event (confirm connection)
- ✅ Send `error` events for failures (client can display)
- ✅ Return from stream writer to close connection
- ✅ No need to return error from handler (stream already started)

---

### 4. Context Cancellation

**⚠️ Not implemented in current code, but recommended:**

```go
func (c *Client) StreamChatCompletion(ctx context.Context, ...) error {
    // ... setup ...
    
    scanner := bufio.NewScanner(resp.Body)
    for scanner.Scan() {
        // Check for context cancellation
        select {
        case <-ctx.Done():
            return ctx.Err()  // Client disconnected
        default:
            // Continue processing
        }
        
        line := scanner.Text()
        // ... process line ...
    }
    
    return scanner.Err()
}
```

**When to check context**:
- Before starting expensive operations
- Between chunks in a stream
- In long-running loops

---

## Reusable Patterns

### 1. SSE Helper Package

**Create**: `apps/api/utils/sse/sse.go`

```go
package sse

import (
    "bufio"
    "encoding/json"
    "fmt"
)

// Event represents an SSE event
type Event struct {
    Event string      `json:"-"`        // Event type (optional)
    Data  interface{} `json:"data"`     // Event data
}

// Send sends an SSE event to the client
func Send(w *bufio.Writer, event Event) error {
    if event.Event != "" {
        fmt.Fprintf(w, "event: %s\n", event.Event)
    }
    
    // Marshal data to JSON
    var dataStr string
    switch v := event.Data.(type) {
    case string:
        dataStr = v
    default:
        data, err := json.Marshal(v)
        if err != nil {
            return err
        }
        dataStr = string(data)
    }
    
    fmt.Fprintf(w, "data: %s\n\n", dataStr)
    return w.Flush()
}

// SendError sends an error event
func SendError(w *bufio.Writer, err error) error {
    return Send(w, Event{
        Event: "error",
        Data: map[string]string{
            "error": err.Error(),
        },
    })
}

// SendProgress sends a progress event
func SendProgress(w *bufio.Writer, progress interface{}) error {
    return Send(w, Event{
        Event: "progress",
        Data:  progress,
    })
}

// SendComplete sends a completion event
func SendComplete(w *bufio.Writer, result interface{}) error {
    return Send(w, Event{
        Event: "done",
        Data:  result,
    })
}
```

**Usage**:

```go
import "github.com/sahilchouksey/go-init-setup/utils/sse"

c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
    sse.Send(w, sse.Event{Event: "start", Data: "Starting extraction"})
    
    err := service.ExtractWithProgress(ctx, func(p Progress) error {
        return sse.SendProgress(w, p)
    })
    
    if err != nil {
        sse.SendError(w, err)
        return
    }
    
    sse.SendComplete(w, result)
})
```

---

### 2. Streaming Callback Pattern

**Interface definition**:

```go
// StreamCallback processes chunks as they arrive
type StreamCallback func(chunk string) error

// ProgressCallback reports progress updates
type ProgressCallback func(progress float64, message string) error

// ChunkProcessor processes structured chunks
type ChunkProcessor func(chunk ExtractionChunk) error
```

**Guidelines**:
- Return `error` to stop streaming
- Return `nil` to continue
- Use closure to capture context (bufio.Writer, accumulators)

---

### 3. Fiber SSE Middleware (Optional)

```go
package middleware

import (
    "github.com/gofiber/fiber/v2"
)

// SSE configures Fiber context for Server-Sent Events
func SSE() fiber.Handler {
    return func(c *fiber.Ctx) error {
        c.Set("Content-Type", "text/event-stream")
        c.Set("Cache-Control", "no-cache")
        c.Set("Connection", "keep-alive")
        c.Set("Transfer-Encoding", "chunked")
        c.Set("X-Accel-Buffering", "no")
        return c.Next()
    }
}
```

**Usage**:

```go
router.Get("/extract-stream/:id", middleware.SSE(), handler.ExtractStream)
```

---

## Implementation Checklist

### Backend (Go)

#### Service Layer
- [ ] Add `ProgressCallback` parameter to extraction method
- [ ] Use `strings.Builder` to accumulate chunks
- [ ] Send progress updates at key stages:
  - [ ] Chunking (10%)
  - [ ] Each chunk completion (10% + n*60%)
  - [ ] Merging (80%)
  - [ ] Saving (90%)
- [ ] Save complete result after stream ends
- [ ] Handle errors gracefully (continue on minor errors)

#### Handler Layer
- [ ] Create new endpoint `/extract-stream/:id`
- [ ] Set SSE headers (Content-Type, Cache-Control, etc.)
- [ ] Use `c.Context().SetBodyStreamWriter()`
- [ ] Send events: `start`, `progress`, `chunk`, `done`, `error`
- [ ] Call service method with callback
- [ ] Flush after each event
- [ ] Handle errors with error events

#### Client Layer (Optional)
- [ ] Create HTTP client with connection pooling
- [ ] Set `Accept: text/event-stream` header
- [ ] Use `bufio.Scanner` for line-by-line reading
- [ ] Parse SSE format (`data:`, `event:`)
- [ ] Handle `[DONE]` signal
- [ ] Return errors from callback to stop stream

### Frontend (TypeScript/React)

- [ ] Create `useSSEStream` hook
- [ ] Handle event types: `start`, `progress`, `chunk`, `done`, `error`
- [ ] Update UI with progress
- [ ] Handle connection errors
- [ ] Close EventSource on unmount
- [ ] Display error states
- [ ] Show success state with results

### Testing

- [ ] Unit test: Service callback integration
- [ ] Integration test: Full streaming flow
- [ ] Test: Client disconnection handling
- [ ] Test: Error propagation
- [ ] Test: Multiple concurrent streams
- [ ] Load test: Performance under load

---

## Gotchas & Common Mistakes

### ❌ Mistake 1: Not Flushing After Each Event

```go
// WRONG
fmt.Fprintf(w, "data: chunk1\n\n")
fmt.Fprintf(w, "data: chunk2\n\n")
w.Flush()  // Too late - chunks batched
```

```go
// CORRECT
fmt.Fprintf(w, "data: chunk1\n\n")
w.Flush()  // Flush immediately

fmt.Fprintf(w, "data: chunk2\n\n")
w.Flush()  // Flush each chunk
```

**Why**: Without flushing, data is buffered and sent in batches, defeating the purpose of streaming.

---

### ❌ Mistake 2: Setting Timeout on Streaming HTTP Client

```go
// WRONG
client := &http.Client{
    Timeout: 30 * time.Second,  // Stream will timeout after 30s
}
```

```go
// CORRECT
client := &http.Client{
    // No timeout - use context for cancellation
}

ctx, cancel := context.WithTimeout(parentCtx, 5*time.Minute)
defer cancel()

req, _ := http.NewRequestWithContext(ctx, ...)
```

**Why**: Timeout applies to entire request, including streaming. Use context for cancellation instead.

---

### ❌ Mistake 3: Holding Database Locks During Streaming

```go
// WRONG
tx := db.Begin()
// ... save user message ...
// Stream for 60 seconds (lock held!)
err := client.StreamData(...)
// ... save result ...
tx.Commit()
```

```go
// CORRECT
tx := db.Begin()
// ... save user message ...
tx.Commit()  // Release lock BEFORE streaming

// Stream without holding locks
err := client.StreamData(...)

tx := db.Begin()  // New transaction for result
// ... save result ...
tx.Commit()
```

**Why**: Holding locks during streaming blocks other requests and can cause deadlocks.

---

### ❌ Mistake 4: Not Handling Client Disconnection

```go
// WRONG
c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
    for chunk := range chunks {
        fmt.Fprintf(w, "data: %s\n\n", chunk)
        w.Flush()
        // No check - continues even if client disconnected
    }
})
```

```go
// CORRECT
c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
    for chunk := range chunks {
        select {
        case <-c.Context().Done():
            return  // Client disconnected
        default:
            fmt.Fprintf(w, "data: %s\n\n", chunk)
            if err := w.Flush(); err != nil {
                return  // Connection error
            }
        }
    }
})
```

**Why**: Client may disconnect mid-stream. Check context and flush errors to avoid wasting resources.

---

### ❌ Mistake 5: Returning Error After Stream Started

```go
// WRONG
func (h *Handler) Stream(c *fiber.Ctx) error {
    c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
        // ... streaming ...
    })
    
    return c.JSON(...)  // Never executed!
}
```

```go
// CORRECT
func (h *Handler) Stream(c *fiber.Ctx) error {
    c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
        // Send all data here
        // Send errors as SSE events
        if err != nil {
            fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
            w.Flush()
        }
    })
    
    return nil  // Handler returns nil
}
```

**Why**: Once `SetBodyStreamWriter` is called, Fiber returns the response. Handler return value is ignored.

---

### ❌ Mistake 6: JSON Marshaling Errors Not Handled

```go
// WRONG
data, _ := json.Marshal(progress)  // Ignoring error
fmt.Fprintf(w, "data: %s\n\n", data)
```

```go
// CORRECT
data, err := json.Marshal(progress)
if err != nil {
    // Send error event
    fmt.Fprintf(w, "event: error\ndata: {\"error\":\"marshal failed\"}\n\n")
    w.Flush()
    return
}
fmt.Fprintf(w, "data: %s\n\n", data)
w.Flush()
```

**Why**: Marshaling can fail (circular references, unsupported types). Always handle errors.

---

### ❌ Mistake 7: Forgetting nginx/Proxy Buffering

```go
// WRONG
c.Set("Content-Type", "text/event-stream")
c.Set("Cache-Control", "no-cache")
// nginx will buffer by default!
```

```go
// CORRECT
c.Set("Content-Type", "text/event-stream")
c.Set("Cache-Control", "no-cache")
c.Set("X-Accel-Buffering", "no")  // ← Disable nginx buffering
```

**Why**: nginx and other proxies buffer responses by default. Must explicitly disable for SSE.

---

### ✅ Best Practices Summary

1. **Always flush after each event**
2. **Use context for cancellation, not HTTP timeout**
3. **Release database locks before streaming**
4. **Check context.Done() and flush errors**
5. **Send errors as SSE events, not handler returns**
6. **Handle JSON marshaling errors**
7. **Disable proxy buffering with X-Accel-Buffering**
8. **Send a `start` event to confirm connection**
9. **Use structured event types (start, progress, done, error)**
10. **Test with client disconnection scenarios**

---

## Quick Reference

### SSE Format Cheat Sheet

```
event: <event-name>
data: <json-or-text>

```

**Rules**:
- Blank line (`\n\n`) separates events
- `event:` line is optional (default type is "message")
- `data:` line is required
- Multiple `data:` lines are concatenated with `\n`

### Fiber Streaming Template

```go
func (h *Handler) StreamEndpoint(c *fiber.Ctx) error {
    // Set SSE headers
    c.Set("Content-Type", "text/event-stream")
    c.Set("Cache-Control", "no-cache")
    c.Set("Connection", "keep-alive")
    c.Set("X-Accel-Buffering", "no")
    
    // Stream response
    c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
        // Send start
        fmt.Fprintf(w, "event: start\ndata: {}\n\n")
        w.Flush()
        
        // Stream data
        err := h.service.ProcessWithCallback(c.Context(), func(chunk string) error {
            fmt.Fprintf(w, "event: chunk\ndata: %s\n\n", chunk)
            return w.Flush()
        })
        
        // Handle error
        if err != nil {
            fmt.Fprintf(w, "event: error\ndata: {\"error\":\"%s\"}\n\n", err.Error())
            w.Flush()
            return
        }
        
        // Send completion
        fmt.Fprintf(w, "event: done\ndata: {}\n\n")
        w.Flush()
    })
    
    return nil
}
```

### Frontend EventSource Template

```typescript
const eventSource = new EventSource('/api/v1/stream', {
  withCredentials: true,
});

eventSource.addEventListener('start', (e) => {
  console.log('Started:', e.data);
});

eventSource.addEventListener('progress', (e) => {
  const data = JSON.parse(e.data);
  setProgress(data);
});

eventSource.addEventListener('done', (e) => {
  const data = JSON.parse(e.data);
  setResult(data);
  eventSource.close();
});

eventSource.addEventListener('error', (e) => {
  const data = JSON.parse(e.data);
  setError(data.error);
  eventSource.close();
});

eventSource.onerror = () => {
  console.error('Connection error');
  eventSource.close();
};
```

---

## Next Steps

1. **Review existing implementation**: Read through `services/digitalocean/chat.go` and `handlers/chat/chats.go`
2. **Create SSE helper package**: Implement reusable SSE utilities
3. **Modify extraction service**: Add progress callback parameter
4. **Create streaming endpoint**: Implement `/extract-stream/:id` handler
5. **Test with Postman/curl**: Verify SSE events are sent correctly
6. **Implement frontend**: Create React hook and UI components
7. **Load test**: Test with multiple concurrent streams

---

**End of Document**
