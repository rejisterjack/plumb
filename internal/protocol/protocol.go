// Package protocol is the JSON message format shared by the relay and the CLI.
// Request and response bodies are raw bytes encoded as standard base64 so a
// webhook signature computed over the original payload still matches.
package protocol

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"
)

const (
	TypeRegister    = "register"
	TypeRegistered  = "registered"
	TypeRequest     = "request"
	TypeResponse    = "response"
	TypeReplay      = "replay"
	TypeReplayBatch = "replayBatch"
	TypePing        = "ping"
	TypePong        = "pong"
	TypeError       = "error"

	ModeTunnel = "tunnel"
	ModeReplay = "replay"
)

// Message is one WebSocket text frame. Unused fields are omitted.
type Message struct {
	Type        string      `json:"type"`
	ID          string      `json:"id,omitempty"`
	Path        string      `json:"path,omitempty"`
	Token       string      `json:"token,omitempty"`
	Mode        string      `json:"mode,omitempty"`
	Method      string      `json:"method,omitempty"`
	URL         string      `json:"url,omitempty"`
	Host        string      `json:"host,omitempty"`
	OriginalURL string      `json:"originalUrl,omitempty"`
	Headers     http.Header `json:"headers,omitempty"`
	Body        string      `json:"body,omitempty"`
	Status      int         `json:"status,omitempty"`
	Error       string      `json:"error,omitempty"`
	Last        int         `json:"last,omitempty"`
	TimeoutMs   int64       `json:"timeoutMs,omitempty"`
	Items       []Item      `json:"items,omitempty"`
}

// Item is one buffered webhook inside a replayBatch.
type Item struct {
	ID          string      `json:"id"`
	Method      string      `json:"method"`
	URL         string      `json:"url"`
	Host        string      `json:"host,omitempty"`
	OriginalURL string      `json:"originalUrl,omitempty"`
	Headers     http.Header `json:"headers,omitempty"`
	Body        string      `json:"body,omitempty"`
	Time        time.Time   `json:"time"`
}

// EncodeBody encodes a raw webhook body for the wire. An empty body is an
// empty string so it can be omitted.
func EncodeBody(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

// DecodeBody reverses EncodeBody. An empty string decodes to a nil slice.
func DecodeBody(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(s)
}

// Marshal encodes a message as JSON.
func Marshal(msg Message) ([]byte, error) {
	return json.Marshal(msg)
}

// Unmarshal decodes a message and canonicalizes header names.
func Unmarshal(b []byte) (Message, error) {
	var msg Message
	if err := json.Unmarshal(b, &msg); err != nil {
		return Message{}, err
	}
	msg.Headers = canonicalize(msg.Headers)
	for i := range msg.Items {
		msg.Items[i].Headers = canonicalize(msg.Items[i].Headers)
	}
	return msg, nil
}

func canonicalize(h http.Header) http.Header {
	if len(h) == 0 {
		return nil
	}
	out := make(http.Header, len(h))
	for k, vs := range h {
		for _, v := range vs {
			out.Add(k, v)
		}
	}
	return out
}

// ValidSlug reports whether path is safe to use as a tunnel name and a
// filename for the local control socket.
func ValidSlug(s string) bool {
	if len(s) < 1 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}
