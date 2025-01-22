package parser

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
)

// Define the nested structs for the details field
type OdkEntityRef struct {
	Uuid    string `json:"uuid"` // Use string for UUID
	Dataset string `json:"dataset"`
}

type OdkEntityDetails struct {
	Entity      OdkEntityRef `json:"entity"`
	EntityId    int          `json:"entityId"`
	EntityDefId int          `json:"entityDefId"`
}

type OdkNewSubmissionDetails struct {
	InstanceId      string `json:"instanceId"` // Use string for UUID
	SubmissionId    int    `json:"submissionId"`
	SubmissionDefId int    `json:"submissionDefId"`
}

// OdkAuditLog represents the main structure for the audit log (returned by pg_notify)
type OdkAuditLog struct {
	Notes   *string     `json:"notes"`   // Pointer to handle null values
	Action  string      `json:"action"`
	ActeeID string      `json:"acteeId"` // Use string for UUID
	ActorID int         `json:"actorId"` // Integer for the actor ID
	Claimed *bool       `json:"claimed"` // Pointer for nullable boolean
	Details interface{} `json:"details"` // Use an interface to handle different detail types
	Data    interface{} `json:"data"`    // Use an interface to handle different data types
}

// ProcessedEvent represents the final parsed event structure (to send to the webhook API)
type ProcessedEvent struct {
	ID   string      `json:"id"`   // Entity UUID or Submission InstanceID
	Data interface{} `json:"data"` // The actual entity data or wrapped submission XML
}

// ParseJsonString converts the pg_notify string to OdkAuditLog
func ParseJsonString(log *slog.Logger, data []byte) (*OdkAuditLog, error) {
	if len(data) == 0 {
		return nil, errors.New("empty input data")
	}

	var parsedData OdkAuditLog
	if err := json.Unmarshal(data, &parsedData); err != nil {
		log.Error("Failed to parse JSON data", "error", err, "data", string(data))
		return nil, err
	}
	log.Debug("Parsed notification data", "data", parsedData)
	return &parsedData, nil
}

// ParseEventJson parses the JSON data and extracts the relevant ID and data fields
func ParseEventJson(log *slog.Logger, ctx context.Context, data []byte) (*ProcessedEvent, error) {
	// Convert the raw pg_notify string to an OdkAuditLog
	rawLog, err := ParseJsonString(log, data)
	if err != nil {
		return nil, err
	}

	// Prepare the result structure
	var processedEvent ProcessedEvent

	// Parse the details field based on the action
	switch rawLog.Action {
	case "entity.update.version":
		var entityDetails OdkEntityDetails
		if err := parseDetails(rawLog.Details, &entityDetails); err != nil {
			log.Error("Failed to parse entity.update.version details", "error", err)
			return nil, err
		}
		processedEvent.ID = entityDetails.Entity.Uuid
		processedEvent.Data = rawLog.Data

	case "submission.create":
		var submissionDetails OdkNewSubmissionDetails
		if err := parseDetails(rawLog.Details, &submissionDetails); err != nil {
			log.Error("Failed to parse submission.create details", "error", err)
			return nil, err
		}
		processedEvent.ID = submissionDetails.InstanceId

		// Parse the raw XML data
		rawData, ok := rawLog.Data.(map[string]interface{})
		if !ok {
			log.Error("Invalid data type for submission.create", "data", rawLog.Data)
			return nil, errors.New("invalid data type for submission.create")
		}
		processedEvent.Data = rawData

	default:
		// No nothing if the event type is not supported
		log.Warn("Unsupported action type", "action", rawLog.Action)
		return nil, nil
	}

	log.Debug("Parsed event successfully", "processedEvent", processedEvent)
	return &processedEvent, nil
}

// parseDetails helps to unmarshal the details field into the appropriate structure
func parseDetails(details interface{}, target interface{}) error {
	detailsBytes, err := json.Marshal(details)
	if err != nil {
		return err
	}
	return json.Unmarshal(detailsBytes, target)
}
