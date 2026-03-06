package ws

import (
	"encoding/json"
	"errors"
)

// Message type constants define the WebSocket message types used by the
// gateway for client communication.
const (
	MessageTypeSubscribe   = "subscribe"
	MessageTypeUnsubscribe = "unsubscribe"
	MessageTypeEvent       = "event"
	MessageTypeError       = "error"
	MessageTypePong        = "pong"
)

// WSMessage represents a structured WebSocket message exchanged between the
// gateway and connected clients.
type WSMessage struct {
	Type    string          `json:"type"`
	Topic   string          `json:"topic,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// ParseWSMessage deserialises raw JSON bytes into a WSMessage. It returns an
// error if the bytes are not valid JSON or if the required Type field is empty.
func ParseWSMessage(data []byte) (*WSMessage, error) {
	if len(data) == 0 {
		return nil, errors.New("empty message")
	}

	var msg WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}

	if msg.Type == "" {
		return nil, errors.New("message type is required")
	}

	return &msg, nil
}

// NewEventMessage creates a WSMessage with type "event" for the given topic
// and payload.
func NewEventMessage(topic string, payload json.RawMessage) *WSMessage {
	return &WSMessage{
		Type:    MessageTypeEvent,
		Topic:   topic,
		Payload: payload,
	}
}

// NewErrorMessage creates a WSMessage with type "error" carrying the given
// error text as a JSON-encoded payload.
func NewErrorMessage(msg string) *WSMessage {
	payload, _ := json.Marshal(map[string]string{"error": msg})
	return &WSMessage{
		Type:    MessageTypeError,
		Payload: json.RawMessage(payload),
	}
}
