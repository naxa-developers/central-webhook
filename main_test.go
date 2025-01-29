package main

// import (
// 	"context"
// 	"encoding/json"
// 	"log/slog"
// 	"net/http"
// 	"net/http/httptest"
// 	"os"
// 	"sync"
// 	"testing"
// 	"time"

// 	"github.com/matryer/is"

// 	"github.com/hotosm/central-webhook/db"
// 	"github.com/hotosm/central-webhook/parser"
// )

// func TestSetupWebhook(t *testing.T) {
// 	dbUri := os.Getenv("CENTRAL_WEBHOOK_DB_URI")
// 	if len(dbUri) == 0 {
// 		dbUri = "postgresql://odk:odk@db:5432/odk?sslmode=disable"
// 	}

// 	is := is.New(t)
// 	wg := sync.WaitGroup{}
// 	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
// 		Level: slog.LevelDebug,
// 	}))
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()

// 	dbPool, err := db.InitPool(ctx, log, dbUri)
// 	is.NoErr(err)
// 	defer dbPool.Close()

// 	conn, err := dbPool.Acquire(ctx)
// 	is.NoErr(err)
// 	defer conn.Release()

// 	// Create test tables
// 	conn.Exec(ctx, `DROP TABLE IF EXISTS entity_defs;`)
// 	conn.Exec(ctx, `DROP TABLE IF EXISTS audits;`)
// 	createTables := []string{
// 		`CREATE TABLE IF NOT EXISTS entity_defs (
// 			id SERIAL PRIMARY KEY,
// 			"entityId" INT,
// 			"createdAt" TIMESTAMPTZ,
// 			"current" BOOL,
// 			"data" JSONB,
// 			"creatorId" INT,
// 			"label" TEXT
// 		);`,
// 		`CREATE TABLE IF NOT EXISTS audits (
// 			"actorId" INT,
// 			action VARCHAR,
// 			details JSONB
// 		);`,
// 	}
// 	for _, sql := range createTables {
// 		_, err := conn.Exec(ctx, sql)
// 		is.NoErr(err)
// 	}

// 	// Insert an entity record
// 	log.Info("inserting entity details record")
// 	_, err = conn.Exec(ctx, `
// 		INSERT INTO public.entity_defs (
// 			id, "entityId","createdAt","current","data","creatorId","label"
// 		) VALUES (
// 		 	1001,
// 			900,
// 			'2025-01-10 16:23:40.073',
// 			true,
// 			'{"status": "0", "task_id": "26", "version": "1"}',
// 			5,
// 			'Task 26 Feature 904487737'
// 		);
// 	`)
// 	is.NoErr(err)
// 	log.Info("entity record inserted")

// 	// Mock webhook server
// 	webhookReceived := make(chan bool, 1)
// 	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		defer r.Body.Close()
// 		var payload parser.ProcessedEvent
// 		err := json.NewDecoder(r.Body).Decode(&payload)
// 		is.NoErr(err)

// 		log.Info("payload received", "payload", payload)
// 		is.Equal(payload.ID, "xxx") // Validate Entity ID

// 		// Convert the payload.Data to map[string]string for comparison
// 		actualData, ok := payload.Data.(map[string]interface{})
// 		is.True(ok) // Ensure the type assertion succeeded

// 		expectedData := map[string]interface{}{
// 			"status":  "0",
// 			"task_id": "26",
// 			"version": "1",
// 		}
// 		is.Equal(actualData, expectedData) // Validate Entity data

// 		webhookReceived <- true
// 		w.Header().Set("Content-Type", "application/json")
// 		w.WriteHeader(http.StatusOK)
// 	}))
// 	defer mockServer.Close()

// 	// Start webhook listener
// 	wg.Add(1)
// 	go func() {
// 		defer wg.Done()
// 		log.Info("starting webhook listener")
// 		err := SetupWebhook(log, ctx, dbPool, nil, mockServer.URL, mockServer.URL, mockServer.URL)
// 		if err != nil && ctx.Err() == nil {
// 			log.Error("webhook listener error", "error", err)
// 		}
// 	}()

// 	// Wait for the listener to initialize
// 	log.Info("waiting for listener to initialize")
// 	time.Sleep(300 * time.Millisecond) // Wait for the listener to be fully set up

// 	// Insert an audit log to trigger the webhook
// 	log.Info("inserting audit log")
// 	_, err = conn.Exec(ctx, `
// 		INSERT INTO audits ("actorId", action, details)
// 		VALUES (
// 			1,
// 			'entity.update.version',
// 			'{"entityDefId": 1001, "entityId": 1000, "entity": {"uuid": "xxx", "dataset": "test"}}'
// 		);
// 	`)
// 	is.NoErr(err)

// 	// Wait for webhook response or timeout
// 	select {
// 	case <-webhookReceived:
// 		log.Info("webhook received successfully")
// 	case <-time.After(3 * time.Second):
// 		t.Fatalf("test timed out waiting for webhook")
// 	}

// 	// Allow some time for final webhook processing
// 	time.Sleep(100 * time.Millisecond)

// 	// Cleanup
// 	log.Info("cleaning up...")
// 	cancel()
// 	wg.Wait()
// 	conn.Exec(ctx, `DROP TABLE IF EXISTS entity_defs;`)
// 	conn.Exec(ctx, `DROP TABLE IF EXISTS audits;`)
// 	conn.Release()
// 	dbPool.Close()
// }
