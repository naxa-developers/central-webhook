package webhook

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/matryer/is"

	"github.com/hotosm/odk-webhook/parser"
)

func TestSendRequest(t *testing.T) {
	is := is.New(t)
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Set up a mock server
	var receivedPayload parser.OdkAuditLog
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify content type
		is.Equal("application/json", r.Header.Get("Content-Type"))

		// Read and parse request body
		body, err := io.ReadAll(r.Body)
		is.NoErr(err)
		defer r.Body.Close()

		err = json.Unmarshal(body, &receivedPayload)
		is.NoErr(err)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Define test cases
	testCases := []struct {
		name         string
		event        parser.ProcessedEvent
		expectedID   string
		expectedData interface{}
	}{
		{
			name: "Submission Create Event",
			event: parser.ProcessedEvent{
				ID:   "23dc865a-4757-431e-b182-67e7d5581c81",
				Data: "<submission>XML Data</submission>",
			},
			expectedID:   "23dc865a-4757-431e-b182-67e7d5581c81",
			expectedData: "<submission>XML Data</submission>",
		},
		{
			name: "Entity Update Event",
			event: parser.ProcessedEvent{
				ID:   "45fgh789-e32c-56d2-a765-427654321abc",
				Data: "{\"field\":\"value\"}",
			},
			expectedID:   "45fgh789-e32c-56d2-a765-427654321abc",
			expectedData: "{\"field\":\"value\"}",
		},
	}

	// Execute test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Call the SendRequest function
			SendRequest(log, ctx, server.URL, tc.event)

			// Validate the received payload
			is.Equal(tc.expectedID, receivedPayload.ID)
			is.Equal(tc.expectedData, receivedPayload.Data)
		})
	}
}
