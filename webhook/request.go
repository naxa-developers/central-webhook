package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type OdkAuditDetails struct {
	UserAgent   string  `json:"userAgent"`
	Failures    int     `json:"failures"`
	LoggedAt    string  `json:"loggedAt"`
	Processed   string  `json:"processed"`
	LastFailure *string `json:"lastFailure"` // Pointer for optional/nullable values
}

type OdkAuditLog struct {
	ID      int             `json:"id"`
	Notes   *string         `json:"notes"` // Pointer to handle null values
	Action  string          `json:"action"`
	ActeeID string          `json:"acteeId"` // Use string for UUID
	ActorID int             `json:"actorId"` // Integer for the actor ID
	Claimed *bool           `json:"claimed"` // Pointer for nullable boolean
	Details OdkAuditDetails `json:"details"` // Nested struct
}

func ParseEventJson(log *slog.Logger, ctx context.Context, data []byte) (*OdkAuditLog, error) {
	var parsedData OdkAuditLog
	if err := json.Unmarshal([]byte(data), &parsedData); err != nil {
		log.Error("Failed to parse JSON data", "error", err, "data", data)
		return nil, err
	}
	log.Debug("Parsed notification data", "data", parsedData)
	return &parsedData, nil // Return a pointer to parsedData
}

// SendRequest parses the request content JSON from the PostgreSQL notification
// and sends the JSON payload to an external API endpoint.
func SendRequest(log *slog.Logger, ctx context.Context, apiEndpoint string, parsedData OdkAuditLog) {
	// Marshal the parsed data back to JSON for sending
	payload, err := json.Marshal(parsedData)
	if err != nil {
		log.Error("Failed to marshal parsed data to JSON", "error", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiEndpoint, bytes.NewBuffer(payload))
	if err != nil {
		log.Error("Failed to create HTTP request", "error", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Error("Failed to send HTTP request", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Info("Webhook called successfully", "status", resp.Status)
	} else {
		log.Error("Failed to call webhook", "status", resp.Status)
	}
}
