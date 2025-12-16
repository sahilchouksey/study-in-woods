package sse

import (
	"bufio"
	"encoding/json"
	"fmt"
)

// Event represents an SSE event to be sent to clients
type Event struct {
	// Event is the SSE event type (e.g., "progress", "error", "complete")
	// If empty, no "event:" line will be written
	Event string

	// Data is the payload to send (will be JSON-encoded if not a string)
	Data interface{}

	// ID is an optional event ID for reconnection support
	ID string

	// Retry is an optional reconnection time in milliseconds
	Retry int
}

// Send writes an SSE event to the given writer and flushes immediately
func Send(w *bufio.Writer, event Event) error {
	// Write event ID if provided
	if event.ID != "" {
		if _, err := fmt.Fprintf(w, "id: %s\n", event.ID); err != nil {
			return fmt.Errorf("failed to write event ID: %w", err)
		}
	}

	// Write retry time if provided
	if event.Retry > 0 {
		if _, err := fmt.Fprintf(w, "retry: %d\n", event.Retry); err != nil {
			return fmt.Errorf("failed to write retry: %w", err)
		}
	}

	// Write event type if provided
	if event.Event != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", event.Event); err != nil {
			return fmt.Errorf("failed to write event type: %w", err)
		}
	}

	// Write data
	var dataStr string
	switch v := event.Data.(type) {
	case string:
		dataStr = v
	case []byte:
		dataStr = string(v)
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to marshal event data: %w", err)
		}
		dataStr = string(data)
	}

	if _, err := fmt.Fprintf(w, "data: %s\n\n", dataStr); err != nil {
		return fmt.Errorf("failed to write event data: %w", err)
	}

	return w.Flush()
}

// SendProgress sends a progress event with the given data
func SendProgress(w *bufio.Writer, data interface{}) error {
	return Send(w, Event{
		Event: "progress",
		Data:  data,
	})
}

// SendStarted sends a started event
func SendStarted(w *bufio.Writer, data interface{}) error {
	return Send(w, Event{
		Event: "started",
		Data:  data,
	})
}

// SendWarning sends a warning event (e.g., for retries)
func SendWarning(w *bufio.Writer, data interface{}) error {
	return Send(w, Event{
		Event: "warning",
		Data:  data,
	})
}

// SendComplete sends a completion event with the given result
func SendComplete(w *bufio.Writer, data interface{}) error {
	return Send(w, Event{
		Event: "complete",
		Data:  data,
	})
}

// SendError sends an error event
func SendError(w *bufio.Writer, err error) error {
	return Send(w, Event{
		Event: "error",
		Data: map[string]interface{}{
			"type":    "error",
			"message": err.Error(),
		},
	})
}

// SendErrorWithDetails sends an error event with additional details
func SendErrorWithDetails(w *bufio.Writer, errType, message string, details interface{}) error {
	data := map[string]interface{}{
		"type":    "error",
		"error":   errType,
		"message": message,
	}
	if details != nil {
		data["details"] = details
	}
	return Send(w, Event{
		Event: "error",
		Data:  data,
	})
}

// SendKeepAlive sends a comment (: ping) to keep the connection alive
// Useful for long-running operations to prevent proxy timeouts
func SendKeepAlive(w *bufio.Writer) error {
	if _, err := fmt.Fprintf(w, ": ping\n\n"); err != nil {
		return fmt.Errorf("failed to write keepalive: %w", err)
	}
	return w.Flush()
}
