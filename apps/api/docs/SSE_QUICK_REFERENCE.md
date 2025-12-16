# SSE Streaming Quick Reference Card

**Last Updated**: December 14, 2025

---

## 🚀 Fiber SSE Template (Copy-Paste Ready)

```go
package handlers

import (
    "bufio"
    "fmt"
    "github.com/gofiber/fiber/v2"
)

func (h *Handler) StreamEndpoint(c *fiber.Ctx) error {
    // 1. Set SSE Headers
    c.Set("Content-Type", "text/event-stream")
    c.Set("Cache-Control", "no-cache")
    c.Set("Connection", "keep-alive")
    c.Set("X-Accel-Buffering", "no")
    
    // 2. Start Streaming
    c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
        // 3. Send Start Event
        fmt.Fprintf(w, "event: start\ndata: {\"status\":\"started\"}\n\n")
        w.Flush()
        
        // 4. Stream Data with Callback
        result, err := h.service.ProcessWithCallback(c.Context(), func(chunk string) error {
            fmt.Fprintf(w, "event: chunk\ndata: %s\n\n", chunk)
            return w.Flush()
        })
        
        // 5. Handle Errors
        if err != nil {
            fmt.Fprintf(w, "event: error\ndata: {\"error\":\"%s\"}\n\n", err.Error())
            w.Flush()
            return
        }
        
        // 6. Send Completion
        fmt.Fprintf(w, "event: done\ndata: {\"result_id\":%d}\n\n", result.ID)
        w.Flush()
    })
    
    return nil
}
```

---

## 📋 SSE Event Format

```
event: <event_type>
data: <json_or_text>

```

**Example**:
```
event: progress
data: {"stage":"extracting","progress":0.5,"message":"Half done"}

event: chunk
data: Some content

event: done
data: {"status":"completed","result_id":123}

event: error
data: {"error":"Something went wrong"}
```

**Rules**:
- Blank line (`\n\n`) separates events
- `event:` is optional (defaults to "message")
- `data:` is required
- Can have multiple `data:` lines (joined with `\n`)

---

## 🔧 HTTP Client Configuration

### For Streaming Requests

```go
client := &http.Client{
    // NO timeout - use context instead
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 20,   // ← Increase from default 2
        MaxConnsPerHost:     0,    // ← Unlimited
        IdleConnTimeout:     90 * time.Second,
        DisableKeepAlives:   false,
        ForceAttemptHTTP2:   true,
    },
}

// Use context for timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
req.Header.Set("Accept", "text/event-stream")
```

---

## 🎯 Callback Pattern (3 Layers)

```go
// Layer 1: Handler
c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
    service.Process(func(chunk string) error {
        fmt.Fprintf(w, "data: %s\n\n", chunk)
        return w.Flush()
    })
})

// Layer 2: Service
func (s *Service) Process(callback func(string) error) error {
    var accumulated strings.Builder
    err := client.Stream(func(apiChunk Chunk) error {
        accumulated.WriteString(apiChunk.Content)
        return callback(apiChunk.Content)
    })
    // Save accumulated
    return err
}

// Layer 3: Client
func (c *Client) Stream(callback func(Chunk) error) error {
    scanner := bufio.NewScanner(resp.Body)
    for scanner.Scan() {
        var chunk Chunk
        json.Unmarshal(scanner.Bytes(), &chunk)
        if err := callback(chunk); err != nil {
            return err
        }
    }
    return scanner.Err()
}
```

---

## 🛡️ Error Handling Checklist

- [x] Check HTTP status before streaming
- [x] Use `defer resp.Body.Close()`
- [x] Continue on JSON parse errors (log but don't stop)
- [x] Return error from callback to stop stream
- [x] Check `scanner.Err()` after loop
- [x] Send errors as SSE events (not handler errors)
- [x] Release DB locks before streaming
- [x] Check `context.Done()` for cancellation
- [x] Check `w.Flush()` errors

---

## ⚡ Critical Do's and Don'ts

### ✅ DO

```go
// Flush after each event
fmt.Fprintf(w, "data: chunk\n\n")
w.Flush()  // ← CRITICAL

// Use context for timeout
ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)

// Send errors as SSE events
fmt.Fprintf(w, "event: error\ndata: {...}\n\n")

// Return nil from handler
return nil

// Disable nginx buffering
c.Set("X-Accel-Buffering", "no")
```

### ❌ DON'T

```go
// Don't batch flushes
fmt.Fprintf(w, "data: 1\n\n")
fmt.Fprintf(w, "data: 2\n\n")
w.Flush()  // ❌ Too late

// Don't use HTTP timeout
client := &http.Client{Timeout: 30*time.Second}  // ❌

// Don't return errors after streaming started
c.Context().SetBodyStreamWriter(...)
return c.JSON(...)  // ❌ Never executes

// Don't hold DB locks while streaming
tx := db.Begin()
streamData()  // ❌ Lock held for 60s
tx.Commit()
```

---

## 🌐 Frontend EventSource Template

```typescript
const eventSource = new EventSource('/api/stream', {
  withCredentials: true,
});

eventSource.addEventListener('start', (e) => {
  console.log('Started');
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

## 🧪 Testing with curl

```bash
# Test SSE endpoint
curl -N -H "Accept: text/event-stream" \
     -H "Authorization: Bearer <token>" \
     http://localhost:3000/api/v1/stream

# Output:
# event: start
# data: {"status":"started"}
#
# event: progress
# data: {"progress":0.5}
#
# event: done
# data: {"result_id":123}
```

**Flags**:
- `-N` / `--no-buffer`: Disable output buffering
- `-H`: Set headers

---

## 🔍 Debugging Checklist

**If no data is streaming**:
1. Check `w.Flush()` is called after each write
2. Check `X-Accel-Buffering: no` header is set
3. Check nginx/proxy is not buffering
4. Check client `Accept: text/event-stream` header
5. Verify blank line (`\n\n`) after each event

**If stream disconnects early**:
1. Check HTTP timeout is not set
2. Check context cancellation
3. Check callback error propagation
4. Check database lock timeouts
5. Verify `w.Flush()` error handling

**If frontend not receiving events**:
1. Check CORS headers
2. Check `withCredentials: true`
3. Check event names match
4. Check JSON parsing
5. Verify EventSource is not closed early

---

## 📚 Related Files

- Full Guide: `apps/api/docs/SSE_STREAMING_IMPLEMENTATION_GUIDE.md`
- Summary: `SSE_IMPLEMENTATION_SUMMARY.md`
- Research: `STREAMING_RESEARCH.md`
- Working Code: `apps/api/handlers/chat/chats.go:284-326`

---

## 🎓 Resources

- [SSE Spec (WHATWG)](https://html.spec.whatwg.org/multipage/server-sent-events.html)
- [MDN EventSource](https://developer.mozilla.org/en-US/docs/Web/API/EventSource)
- [Fiber Context API](https://docs.gofiber.io/api/ctx)
- [Go bufio.Scanner](https://pkg.go.dev/bufio#Scanner)

---

**Print this card and keep it handy while implementing SSE!**
