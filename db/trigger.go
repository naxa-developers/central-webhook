package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Example parsed JSON
// {"action":"entity.update.version","actorId":1,"details":{"entityDefId":1001,...},"dml_action":"INSERT"}}

func CreateTrigger(ctx context.Context, dbPool *pgxpool.Pool, tableName string) error {
	// This trigger runs on the `audits` table by default, and creates a new event
	// in the odk-events queue when a new event is created in the table

	if tableName == "" {
		// default table (this is configurable for easier tests mainly)
		tableName = "audits"
	}

	// SQL for creating the function
	createFunctionSQL := `
		CREATE OR REPLACE FUNCTION new_audit_log() RETURNS trigger AS
		$$
		DECLARE
			js jsonb;
			action_type text;
			result_data jsonb;
		BEGIN
			-- Serialize the NEW row into JSONB
			SELECT to_jsonb(NEW.*) INTO js;

			-- Add the DML action (INSERT/UPDATE)
			js := jsonb_set(js, '{dml_action}', to_jsonb(TG_OP));

			-- Extract the action type from the NEW row
			action_type := NEW.action;

			-- Handle different action types with a CASE statement
			CASE action_type
				WHEN 'entity.update.version' THEN
					SELECT entity_defs.data
					INTO result_data
					FROM entity_defs
					WHERE entity_defs.id = (NEW.details->>'entityDefId')::int;

					-- Merge the entity details into the JSON data key
					js := jsonb_set(js, '{data}', result_data, true);

					-- Notify the odk-events queue
					PERFORM pg_notify('odk-events', js::text);

				WHEN 'submission.create' THEN
					SELECT jsonb_build_object('xml', submission_defs.xml)
					INTO result_data
					FROM submission_defs
					WHERE submission_defs.id = (NEW.details->>'submissionDefId')::int;

					-- Merge the submission XML into the JSON data key
					js := jsonb_set(js, '{data}', result_data, true);

					-- Notify the odk-events queue
					PERFORM pg_notify('odk-events', js::text);

				WHEN 'submission.update' THEN
					SELECT jsonb_build_object('instanceId', submission_defs."instanceId")
					INTO result_data
					FROM submission_defs
					WHERE submission_defs.id = (NEW.details->>'submissionDefId')::int;

					-- Merge the instanceId into the existing 'details' key in JSON
					js := jsonb_set(js, '{details}', (js->'details') || result_data, true);

					-- Notify the odk-events queue
					PERFORM pg_notify('odk-events', js::text);

				ELSE
					-- Skip pg_notify for unsupported actions & insert as normal
					RETURN NEW;
			END CASE;

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
