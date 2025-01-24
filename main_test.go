package main

// // TODO FIXME
// import (
// 	"context"
// 	"encoding/json"
// 	"log/slog"
// 	"os"
// 	"sync"
// 	"testing"
// 	"net/http"
// 	"net/http/httptest"
// 	"time"

// 	"github.com/matryer/is"

// 	"github.com/hotosm/central-webhook/parser"
// 	"github.com/hotosm/central-webhook/db"
// )

// func TestSetupWebhook(t *testing.T) {
// 	dbUri := os.Getenv("CENTRAL_WEBHOOK_DB_URI")
// 	if len(dbUri) == 0 {
// 		// Default
// 		dbUri = "postgresql://odk:odk@db:5432/odk?sslmode=disable"
// 	}

// 	is := is.New(t)
// 	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
// 	ctx := context.Background()
// 	ctx, cancel := context.WithCancel(ctx)
// 	wg := sync.WaitGroup{}
// 	dbPool, err := db.InitPool(ctx, log, dbUri)
// 	is.NoErr(err)

// 	// Get connection and defer close
// 	conn, err := dbPool.Acquire(ctx)
// 	is.NoErr(err)
// 	defer conn.Release()

// 	// Create entity_defs table
// 	_, err = conn.Exec(ctx, `DROP TABLE IF EXISTS entity_defs;`)
// 	is.NoErr(err)
// 	entityTableCreateSql := `
// 		CREATE TABLE entity_defs (
// 			id int4,
// 			"entityId" int4,
// 			"createdAt" timestamptz,
// 			"current" bool,
// 			"data" jsonb,
// 			"creatorId" int4,
// 			"label" text
// 		);
// 	`
// 	_, err = conn.Exec(ctx, entityTableCreateSql)
// 	is.NoErr(err)

// 	// Create audits_test table
// 	_, err = conn.Exec(ctx, `DROP TABLE IF EXISTS audits_test;`)
// 	is.NoErr(err)
// 	auditTableCreateSql := `
// 		CREATE TABLE audits_test (
// 			"actorId" int,
// 			action varchar,
// 			details jsonb
// 		);
// 	`
// 	_, err = conn.Exec(ctx, auditTableCreateSql)
// 	is.NoErr(err)

// 	// Insert an entity record
// 	entityInsertSql := `
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
// 	`
// 	_, err = conn.Exec(ctx, entityInsertSql)
// 	is.NoErr(err)

// 	// Mock webhook server
// 	webhookReceived := make(chan bool, 1)
// 	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		defer r.Body.Close()

// 		var payload parser.ProcessedEvent
// 		err := json.NewDecoder(r.Body).Decode(&payload)
// 		// This is where the actual payload is inspected
// 		is.NoErr(err)
// 		is.Equal("xxx", payload.ID)  // Check if the ID matches
// 		is.Equal(`{"status": "0", "task_id": "26", "version": "1"}`, payload.Data)  // Check the data

// 		webhookReceived <- true
// 		w.WriteHeader(http.StatusOK)
// 	}))
// 	defer mockServer.Close()

// 	// Run Webhook trigger in background
// 	go func() {
// 		err := SetupWebhook(log, ctx, dbPool, mockServer.URL, mockServer.URL)
// 		is.NoErr(err)
// 	}()

// 	// Insert an audit record (trigger event)
// 	auditInsertSql := `
// 		INSERT INTO audits_test ("actorId", action, details)
// 		VALUES (1, 'entity.update.version', '{"entityDefId": 1001, "entityId": 1000, "entity": {"uuid": "xxx", "dataset": "test"}}');
// 	`
// 	_, err = conn.Exec(ctx, auditInsertSql)
// 	is.NoErr(err)

// 	// Wait for the webhook to be received
// 	select {
// 	case <-webhookReceived:
// 		// Success
// 	case <-time.After(3 * time.Second):
// 		t.Fatalf("Test timed out waiting for webhook")
// 	}

// 	// Cleanup
// 	conn.Exec(ctx, `DROP TABLE IF EXISTS audits_test;`)
// 	cancel()
// 	wg.Wait()
// }
