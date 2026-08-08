// Package observability writes safe, structured application events to stdout.
package observability

import (
	"encoding/json"
	"log"
)

// Event emits one JSON line. Keep identifiers and numeric workload facts, but
// never include subjects, presigned URLs, bucket object keys, or request bodies.
func Event(logger *log.Logger, level, eventType, message string, fields map[string]any) {
	payload := map[string]any{
		"level":      level,
		"event_type": eventType,
		"message":    message,
	}
	for key, value := range fields {
		payload[key] = value
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		logger.Printf("structured log encoding failed event_type=%s", eventType)
		return
	}
	logger.Print(string(encoded))
}
