package parser

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/matryer/is"
)

func TestParseJsonString(t *testing.T) {
	is := is.New(t)
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))

	t.Run("Valid JSON", func(t *testing.T) {
		input := []byte(`{"id":"123","action":"entity.update.version","actorId":1,"details":{"entity":{"uuid":"abc","dataset":"test"}},"data":{}}`)
		result, err := ParseJsonString(log, input)
		is.NoErr(err)
		is.Equal("123", result.ID)
		is.Equal("entity.update.version", result.Action)
	})

	t.Run("Empty Input", func(t *testing.T) {
		input := []byte("")
		result, err := ParseJsonString(log, input)
		is.Equal(result, nil)
		is.True(err != nil)
		is.Equal("empty input data", err.Error())
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		input := []byte(`invalid`)
		result, err := ParseJsonString(log, input)
		is.Equal(result, nil)
		is.True(err != nil)
	})
}

func TestParseEventJson(t *testing.T) {
	is := is.New(t)
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	ctx := context.Background()

	t.Run("Entity Update Version", func(t *testing.T) {
		input := []byte(`{
			"id":"123",
			"action":"entity.update.version",
			"actorId":1,
			"details":{"entity":{"uuid":"abc","dataset":"test"}},
			"data":{}
		}`)
		result, err := ParseEventJson(log, ctx, input)
		is.NoErr(err)
		is.Equal("abc", result.ID)
		is.Equal(map[string]interface{}{}, result.Data)
	})

	t.Run("Submission Create", func(t *testing.T) {
		input := []byte(`{
			"id":"456",
			"action":"submission.create",
			"actorId":2,
			"details":{"instanceId":"sub-123","submissionId":789,"submissionDefId":101112},
			"data":{"xml":"<submission></submission>"}
		}`)
		result, err := ParseEventJson(log, ctx, input)
		is.NoErr(err)
		is.Equal("sub-123", result.ID)

		wrappedData, ok := result.Data.(map[string]interface{})
		is.True(ok)
		is.Equal("<submission></submission>", wrappedData["xml"])
	})

	t.Run("Unsupported Action", func(t *testing.T) {
		input := []byte(`{
			"id":"789",
			"action":"unknown.action",
			"actorId":3,
			"details":{},
			"data":{}
		}`)
		result, err := ParseEventJson(log, ctx, input)
		is.Equal(result, nil)
		is.True(err != nil)
		is.Equal("unsupported action type", err.Error())
	})
}
