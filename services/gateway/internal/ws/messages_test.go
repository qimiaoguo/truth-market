package ws

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Message type constants (contract that implementation must satisfy)
// ---------------------------------------------------------------------------

const (
	expectedMessageTypeSubscribe   = "subscribe"
	expectedMessageTypeUnsubscribe = "unsubscribe"
	expectedMessageTypeEvent       = "event"
	expectedMessageTypeError       = "error"
	expectedMessageTypePong        = "pong"
)

// ---------------------------------------------------------------------------
// Tests for ParseWSMessage
// ---------------------------------------------------------------------------

func TestParseWSMessage_Subscribe(t *testing.T) {
	raw := []byte(`{"type":"subscribe","topic":"market:mkt-1"}`)

	msg, err := ParseWSMessage(raw)

	require.NoError(t, err)
	assert.Equal(t, expectedMessageTypeSubscribe, msg.Type)
	assert.Equal(t, "market:mkt-1", msg.Topic)
	assert.Nil(t, msg.Payload, "subscribe messages should have no payload")
}

func TestParseWSMessage_SubscribeUserTopic(t *testing.T) {
	raw := []byte(`{"type":"subscribe","topic":"user:usr-42"}`)

	msg, err := ParseWSMessage(raw)

	require.NoError(t, err)
	assert.Equal(t, expectedMessageTypeSubscribe, msg.Type)
	assert.Equal(t, "user:usr-42", msg.Topic)
}

func TestParseWSMessage_Unsubscribe(t *testing.T) {
	raw := []byte(`{"type":"unsubscribe","topic":"market:mkt-1"}`)

	msg, err := ParseWSMessage(raw)

	require.NoError(t, err)
	assert.Equal(t, expectedMessageTypeUnsubscribe, msg.Type)
	assert.Equal(t, "market:mkt-1", msg.Topic)
	assert.Nil(t, msg.Payload, "unsubscribe messages should have no payload")
}

func TestParseWSMessage_WithPayload(t *testing.T) {
	raw := []byte(`{"type":"event","topic":"market:mkt-1","payload":{"price":0.75}}`)

	msg, err := ParseWSMessage(raw)

	require.NoError(t, err)
	assert.Equal(t, expectedMessageTypeEvent, msg.Type)
	assert.Equal(t, "market:mkt-1", msg.Topic)
	assert.NotNil(t, msg.Payload)

	// Verify the payload round-trips correctly.
	var payload map[string]float64
	err = json.Unmarshal(msg.Payload, &payload)
	require.NoError(t, err)
	assert.InDelta(t, 0.75, payload["price"], 0.001)
}

func TestParseWSMessage_InvalidJSON_ReturnsError(t *testing.T) {
	raw := []byte(`{this is not json}`)

	msg, err := ParseWSMessage(raw)

	assert.Error(t, err)
	assert.Nil(t, msg)
}

func TestParseWSMessage_EmptyBytes_ReturnsError(t *testing.T) {
	msg, err := ParseWSMessage([]byte{})

	assert.Error(t, err)
	assert.Nil(t, msg)
}

func TestParseWSMessage_MissingType_ReturnsError(t *testing.T) {
	// A message with no "type" field is invalid.
	raw := []byte(`{"topic":"market:mkt-1"}`)

	msg, err := ParseWSMessage(raw)

	assert.Error(t, err, "messages without a type should be rejected")
	assert.Nil(t, msg)
}

// ---------------------------------------------------------------------------
// Tests for NewEventMessage
// ---------------------------------------------------------------------------

func TestNewEventMessage_SerializesCorrectly(t *testing.T) {
	payload := map[string]interface{}{
		"marketId": "mkt-1",
		"price":    0.65,
		"volume":   1000,
	}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	msg := NewEventMessage("market:mkt-1", json.RawMessage(payloadBytes))

	assert.Equal(t, expectedMessageTypeEvent, msg.Type)
	assert.Equal(t, "market:mkt-1", msg.Topic)
	assert.NotNil(t, msg.Payload)

	// Serialize and deserialize to verify full round-trip.
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded WSMessage
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, expectedMessageTypeEvent, decoded.Type)
	assert.Equal(t, "market:mkt-1", decoded.Topic)

	// Verify payload contents survived the round-trip.
	var decodedPayload map[string]interface{}
	err = json.Unmarshal(decoded.Payload, &decodedPayload)
	require.NoError(t, err)
	assert.Equal(t, "mkt-1", decodedPayload["marketId"])
}

func TestNewEventMessage_NilPayload(t *testing.T) {
	msg := NewEventMessage("market:mkt-1", nil)

	assert.Equal(t, expectedMessageTypeEvent, msg.Type)
	assert.Equal(t, "market:mkt-1", msg.Topic)
	assert.Nil(t, msg.Payload)

	// Should still serialize without error.
	data, err := json.Marshal(msg)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

// ---------------------------------------------------------------------------
// Tests for NewErrorMessage
// ---------------------------------------------------------------------------

func TestNewErrorMessage_SerializesCorrectly(t *testing.T) {
	msg := NewErrorMessage("invalid topic format")

	assert.Equal(t, expectedMessageTypeError, msg.Type)
	assert.Empty(t, msg.Topic, "error messages should not have a topic")

	// Serialize to JSON and verify structure.
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded map[string]json.RawMessage
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// The payload should contain the error message.
	assert.Contains(t, string(decoded["payload"]), "invalid topic format")
}

func TestNewErrorMessage_EmptyMessage(t *testing.T) {
	msg := NewErrorMessage("")

	assert.Equal(t, expectedMessageTypeError, msg.Type)

	data, err := json.Marshal(msg)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

// ---------------------------------------------------------------------------
// Tests for WSMessage struct JSON tags
// ---------------------------------------------------------------------------

func TestWSMessage_JSONOmitsEmptyFields(t *testing.T) {
	msg := WSMessage{
		Type: expectedMessageTypePong,
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	// Topic and payload should be omitted due to "omitempty" tags.
	var raw map[string]json.RawMessage
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.Contains(t, raw, "type")
	assert.NotContains(t, raw, "topic", "empty topic should be omitted")
	assert.NotContains(t, raw, "payload", "nil payload should be omitted")
}
