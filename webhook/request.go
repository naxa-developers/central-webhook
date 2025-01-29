package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/hotosm/central-webhook/parser"
)

// SendRequest parses the request content JSON from the PostgreSQL notification
// and sends the JSON payload to an external API endpoint.
func SendRequest(
	log *slog.Logger,
	ctx context.Context,
	apiEndpoint string,
	eventJson parser.ProcessedEvent,
	apiKey *string,
) {
	// Marshal the payload to JSON
	marshaledPayload, err := json.Marshal(eventJson)
	if err != nil {
		log.Error("failed to marshal payload to JSON", "error", err)
		return
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiEndpoint, bytes.NewBuffer(marshaledPayload))
	if err != nil {
		log.Error("failed to create HTTP request", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	// Add X-API-Key header if apiKey is provided
	if apiKey != nil {
		req.Header.Set("X-API-Key", *apiKey)
	}

	// Send the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Error("failed to send HTTP request", "error", err)
		return
	}
	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("failed to read response body", "error", err)
		return
	}
	respBodyString := string(respBodyBytes)
	defer resp.Body.Close()

	// Check the response status
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		log.Info("webhook called successfully", "status", resp.StatusCode, "endpoint", apiEndpoint)
	} else {
		log.Error(
			"failed to call webhook",
			"endpoint", apiEndpoint,
			"requestPayload", eventJson,
			"responseCode", resp.StatusCode,
			"responseBody", respBodyString)
	}
}
