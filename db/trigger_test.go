package db

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/matryer/is"
)

// Note: these tests assume you have a postgres server listening on db:5432
// with username odk and password odk.
//
// The easiest way to ensure this is to run the tests with docker compose:
// docker compose run --rm webhook

func TestEntityTrigger(t *testing.T) {
	dbUri := os.Getenv("CENTRAL_WEBHOOK_DB_URI")
	if len(dbUri) == 0 {
		// Default
		dbUri = "postgresql://odk:odk@db:5432/odk?sslmode=disable"
	}

	is := is.New(t)
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	wg := sync.WaitGroup{}
	pool, err := InitPool(ctx, log, dbUri)
	is.NoErr(err)

	// Get connection and defer close
	conn, err := pool.Acquire(ctx)
	is.NoErr(err)
	defer conn.Release()

	// Create entity_defs table
	_, err = conn.Exec(ctx, `DROP TABLE IF EXISTS entity_defs;`)
	is.NoErr(err)
	entityTableCreateSql := `
		CREATE TABLE entity_defs (
			id int4,
			"entityId" int4,
			"createdAt" timestamptz,
			"current" bool,
			"data" jsonb,
			"creatorId" int4,
			"label" text
		);
	`
	_, err = conn.Exec(ctx, entityTableCreateSql)
	is.NoErr(err)

	// Create audits_test table
	_, err = conn.Exec(ctx, `DROP TABLE IF EXISTS audits_test;`)
	is.NoErr(err)
	auditTableCreateSql := `
		CREATE TABLE audits_test (
			"actorId" int,
			action varchar,
			details jsonb
		);
	`
	_, err = conn.Exec(ctx, auditTableCreateSql)
	is.NoErr(err)

	// Insert an entity record
	entityInsertSql := `
		INSERT INTO public.entity_defs (
			id, "entityId","createdAt","current","data","creatorId","label"
		) VALUES (
		 	1001,
			900,
			'2025-01-10 16:23:40.073',
			true,
			'{"status": "0", "task_id": "26", "version": "1"}',
			5,
			'Task 26 Feature 904487737'
		);
	`
	_, err = conn.Exec(ctx, entityInsertSql)
	is.NoErr(err)

	// Create audit trigger
	err = CreateTrigger(ctx, pool, "audits_test")
	is.NoErr(err)

	// Create listener
	listener := NewListener(pool)
	err = listener.Connect(ctx)
	is.NoErr(err)

	// Create notifier
	n := NewNotifier(log, listener)
	wg.Add(1)
	go func() {
		n.Run(ctx)
		wg.Done()
	}()
	sub := n.Listen("odk-events")

	// Insert an audit record
	auditInsertSql := `
		INSERT INTO audits_test ("actorId", action, details)
		VALUES (1, 'entity.update.version', '{"entityDefId": 1001}');
	`
	_, err = conn.Exec(ctx, auditInsertSql)
	is.NoErr(err)

	// Validate the notification content
	wg.Add(1)
	out := make(chan string)
	go func() {
		<-sub.EstablishedC()
		msg := <-sub.NotificationC() // Get the notification

		log.Info("Notification received", "raw", msg)

		out <- string(msg) // Send it to the output channel
		close(out)
		wg.Done()
	}()

	// Process the notification
	var notification map[string]interface{}
	for msg := range out {
		err := json.Unmarshal([]byte(msg), &notification)
		is.NoErr(err) // Ensure the JSON payload is valid
		log.Info("Parsed notification", "notification", notification)
	}

	// Validate the JSON content
	is.Equal(notification["dml_action"], "INSERT")            // Ensure action is correct
	is.Equal(notification["action"], "entity.update.version") // Ensure action is correct
	is.True(notification["details"] != nil)                   // Ensure details key exists
	is.True(notification["data"] != nil)                      // Ensure data key exists

	// Check nested JSON value for entityDefId in details
	details, ok := notification["details"].(map[string]interface{})
	is.True(ok)                                     // Ensure details is a valid map
	is.Equal(details["entityDefId"], float64(1001)) // Ensure entityDefId has the correct value

	// Check nested JSON value for status in data
	data, ok := notification["data"].(map[string]interface{})
	is.True(ok)                   // Ensure data is a valid map
	is.Equal(data["status"], "0") // Ensure `status` has the correct value

	// Cleanup
	conn.Exec(ctx, `DROP TABLE IF EXISTS audits_test;`)
	conn.Exec(ctx, `DROP TABLE IF EXISTS entity_defs;`)
	cancel()
	sub.Unlisten(ctx) // uses background ctx anyway
	listener.Close(ctx)
	wg.Wait()
}

// Test a new submission event type
func TestSubmissionTrigger(t *testing.T) {
	dbUri := os.Getenv("CENTRAL_WEBHOOK_DB_URI")
	if len(dbUri) == 0 {
		// Default
		dbUri = "postgresql://odk:odk@db:5432/odk?sslmode=disable"
	}

	is := is.New(t)
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	wg := sync.WaitGroup{}
	pool, err := InitPool(ctx, log, dbUri)
	is.NoErr(err)

	// Get connection and defer close
	conn, err := pool.Acquire(ctx)
	is.NoErr(err)
	defer conn.Release()

	// Create submission_defs table
	_, err = conn.Exec(ctx, `DROP TABLE IF EXISTS submission_defs;`)
	is.NoErr(err)
	submissionTableCreateSql := `
		CREATE TABLE submission_defs (
			id int4,
			"submissionId" int4,
			xml text,
			"formDefId" int4,
			"submitterId" int4,
			"createdAt" timestamptz
		);
	`
	_, err = conn.Exec(ctx, submissionTableCreateSql)
	is.NoErr(err)

	// Create audits_test table
	_, err = conn.Exec(ctx, `DROP TABLE IF EXISTS audits_test;`)
	is.NoErr(err)
	auditTableCreateSql := `
		CREATE TABLE audits_test (
			"actorId" int,
			action varchar,
			details jsonb
		);
	`
	_, err = conn.Exec(ctx, auditTableCreateSql)
	is.NoErr(err)

	// Insert an submission record
	submissionInsertSql := `
		INSERT INTO submission_defs (
			id,
			"submissionId",
			xml,
			"formDefId",
			"submitterId",
			"createdAt"
		) VALUES (
		 	1,
            2,
			'<data id="xxx">',
			7,
			5,
			'2025-01-10 16:23:40.073'
		);
	`
	_, err = conn.Exec(ctx, submissionInsertSql)
	is.NoErr(err)

	// Create audit trigger
	err = CreateTrigger(ctx, pool, "audits_test")
	is.NoErr(err)

	// Create listener
	listener := NewListener(pool)
	err = listener.Connect(ctx)
	is.NoErr(err)

	// Create notifier
	n := NewNotifier(log, listener)
	wg.Add(1)
	go func() {
		n.Run(ctx)
		wg.Done()
	}()
	sub := n.Listen("odk-events")

	// Insert an audit record
	auditInsertSql := `
		INSERT INTO audits_test ("actorId", action, details)
		VALUES (5, 'submission.create', '{"submissionDefId": 1}');
	`
	_, err = conn.Exec(ctx, auditInsertSql)
	is.NoErr(err)

	// Validate the notification content
	wg.Add(1)
	out := make(chan string)
	go func() {
		<-sub.EstablishedC()
		msg := <-sub.NotificationC() // Get the notification

		log.Info("Notification received", "raw", msg)

		out <- string(msg) // Send it to the output channel
		close(out)
		wg.Done()
	}()

	// Process the notification
	var notification map[string]interface{}
	for msg := range out {
		err := json.Unmarshal([]byte(msg), &notification)
		is.NoErr(err) // Ensure the JSON payload is valid
		log.Info("Parsed notification", "notification", notification)
	}

	// Validate the JSON content
	is.Equal(notification["dml_action"], "INSERT")        // Ensure action is correct
	is.Equal(notification["action"], "submission.create") // Ensure action is correct
	is.True(notification["details"] != nil)               // Ensure details key exists
	is.True(notification["data"] != nil)                  // Ensure data key exists

	// Check nested JSON value for submissionDefId in details
	details, ok := notification["details"].(map[string]interface{})
	is.True(ok)                                      // Ensure details is a valid map
	is.Equal(details["submissionDefId"], float64(1)) // Ensure submissionDefId has the correct value

	// Check nested JSON value for status in data
	data, ok := notification["data"].(map[string]interface{})
	is.True(ok)                              // Ensure data is a valid map
	is.Equal(data["xml"], `<data id="xxx">`) // Ensure `xml` has the correct value

	// Cleanup
	conn.Exec(ctx, `DROP TABLE IF EXISTS submission_defs;`)
	conn.Exec(ctx, `DROP TABLE IF EXISTS audits_test;`)
	cancel()
	sub.Unlisten(ctx) // uses background ctx anyway
	listener.Close(ctx)
	wg.Wait()
}

// Test an unsupported event type and ensure nothing is triggered
func TestNoTrigger(t *testing.T) {
	dbUri := os.Getenv("CENTRAL_WEBHOOK_DB_URI")
	if len(dbUri) == 0 {
		// Default
		dbUri = "postgresql://odk:odk@db:5432/odk?sslmode=disable"
	}

	is := is.New(t)
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	pool, err := InitPool(ctx, log, dbUri)
	is.NoErr(err)

	// Get connection and defer close
	conn, err := pool.Acquire(ctx)
	is.NoErr(err)
	defer conn.Release()

	// Create audits_test table
	_, err = conn.Exec(ctx, `DROP TABLE IF EXISTS audits_test;`)
	is.NoErr(err)
	auditTableCreateSql := `
		CREATE TABLE audits_test (
			"actorId" int,
			action varchar,
			details jsonb
		);
	`
	_, err = conn.Exec(ctx, auditTableCreateSql)
	is.NoErr(err)

	// Create audit trigger
	err = CreateTrigger(ctx, pool, "audits_test")
	is.NoErr(err)

	// Create listener
	listener := NewListener(pool)
	err = listener.Connect(ctx)
	is.NoErr(err)

	// Create notifier
	n := NewNotifier(log, listener)
	go func() {
		n.Run(ctx)
	}()
	sub := n.Listen("odk-events")

	// Insert an audit record
	auditInsertSql := `
		INSERT INTO audits_test ("actorId", action, details)
		VALUES (1, 'invalid.event', '{"submissionDefId": 5}');
	`
	_, err = conn.Exec(ctx, auditInsertSql)
	is.NoErr(err)

	// Ensure that no event is fired for incorrect event type
	out := make(chan string)
	go func() {
		<-sub.EstablishedC()
		msg := <-sub.NotificationC() // Get the notification

		log.Info("Notification received", "raw", msg)

		out <- string(msg) // Send it to the output channel
		close(out)
	}()

	// Validate that no event was triggered for invalid event type
	select {
	case msg := <-out:
		// If a message was received, we failed the test since no event should be fired
		t.Fatalf("Unexpected message received: %s", msg)
	case <-time.After(1 * time.Second):
		// No message should have been received within the timeout
		log.Info("No event triggered for invalid event type")
	}

	// Cleanup
	conn.Exec(ctx, `DROP TABLE IF EXISTS audits_test;`)
	cancel()
	sub.Unlisten(ctx) // uses background ctx anyway
	listener.Close(ctx)
}
