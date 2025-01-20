package db

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/matryer/is"
)

// NB: these tests assume you have a postgres server listening on localhost:5432
// with username postgres and password postgres. You can trivially set this up
// with Docker with the following:
//
// docker run --rm --name postgres -p 5432:5432 \
// -e POSTGRES_PASSWORD=postgres postgres

func TestTrigger(t *testing.T) {
	// TODO this should be a local db in a compose stack
	dbUri := "postgresql://odk:odk@host.docker.internal:5434/odk?sslmode=disable"

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

	// Create audits_test table
	auditCreateSql := `
		CREATE TABLE audits_test (
			"actorId" int,
			action varchar,
			details jsonb
		);
	`
	// Ignore if table already exists
	conn.Exec(ctx, auditCreateSql)

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
		VALUES (1, 'entity.update.version', '{"var1": "test"}');
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
	is.Equal(notification["dml_action"], "INSERT") // Ensure action is correct
	is.True(notification["details"] != nil)        // Ensure details key exists

	// Check nested JSON value
	details, ok := notification["details"].(map[string]interface{})
	is.True(ok)                       // Ensure details is a valid map
	is.Equal(details["var1"], "test") // Ensure var1 has the correct value

	// Cleanup
	cancel()
	// FIXME this doesn't actually drop the test table
	defer conn.Exec(ctx, `DROP TABLE IF EXISTS audits_test;`)
	sub.Unlisten(ctx) // uses background ctx anyway
	listener.Close(ctx)
	wg.Wait()
}
