package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Example parsed JSON
// {"action":"entity.update.version","actorId":1,"details":{"var1":"test"},"dml_action":"INSERT"}}

func CreateTrigger(ctx context.Context, dbPool *pgxpool.Pool, tableName string) error {
	// This trigger runs on the `audits` table by default, and creates a new event
	// in the odk-events queue when a new event is created in the table

	if tableName == "" {
		tableName = "audits" // default table
	}

	// SQL for creating the function
	createFunctionSQL := `
		CREATE OR REPLACE FUNCTION new_audit_log() RETURNS trigger AS
		$$
			DECLARE
				js jsonb;
			BEGIN
				SELECT to_jsonb(NEW.*) INTO js;
				js := jsonb_set(js, '{dml_action}', to_jsonb(TG_OP));
				PERFORM pg_notify('odk-events', js::text);
				RETURN NEW;
			END;
		$$ LANGUAGE 'plpgsql';
	`

	// SQL for dropping the existing trigger
	dropTriggerSQL := fmt.Sprintf(`
		DROP TRIGGER IF EXISTS new_audit_log_trigger
		ON %s;
	`, tableName)

	// SQL for creating the new trigger
	createTriggerSQL := fmt.Sprintf(`
		CREATE TRIGGER new_audit_log_trigger
			BEFORE INSERT OR UPDATE ON %s
			FOR EACH ROW
				EXECUTE FUNCTION new_audit_log();
	`, tableName)

	// Acquire a connection from the pool, close after all statements executed
	conn, err := dbPool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, createFunctionSQL); err != nil {
		return fmt.Errorf("failed to create function: %w", err)
	}
	if _, err := conn.Exec(ctx, dropTriggerSQL); err != nil {
		return fmt.Errorf("failed to drop trigger: %w", err)
	}
	if _, err := conn.Exec(ctx, createTriggerSQL); err != nil {
		return fmt.Errorf("failed to create trigger: %w", err)
	}

	return err
}
