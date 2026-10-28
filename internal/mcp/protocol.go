package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
)

const (
	// protocolVersion is the MCP revision snow mcp answers with when the
	// client asks for one it does not know. Snow offers tools only, and
	// their shape is stable across revisions.
	protocolVersion = "2025-06-18"

	// jsonrpcVersion is the only JSON-RPC version MCP uses.
	jsonrpcVersion = "2.0"

	// maxMessage bounds one client message. The longest tool input is a
	// short request, so 8 MiB leaves room and keeps a broken client from
	// growing snow mcp without limit.
	maxMessage = 8 << 20
)

// JSON-RPC error codes. The range is reserved by the specification.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternal       = -32603
	codeCancelled      = -32800
)

// errMessageTooLarge stops the server: after an oversized message the
// stream cannot be resynchronized.
var errMessageTooLarge = errors.New("mcp: message too large")

// request is one message from the client. A request carries an ID and gets
// a response; a notification has no ID and gets none.
type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// response is one reply, or one server-to-client request. Snow only
// replies.
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError is a JSON-RPC error object.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// encode builds one wire line, without its newline.
func encode(id json.RawMessage, result any, e *rpcError) []byte {
	if id == nil {
		id = json.RawMessage("null")
	}
	b, err := json.Marshal(response{JSONRPC: jsonrpcVersion, ID: id, Result: result, Error: e})
	if err != nil {
		// The response types hold only strings, numbers, and slices, so
		// this cannot happen; answer with a plain error rather than die.
		b, _ = json.Marshal(response{JSONRPC: jsonrpcVersion, ID: json.RawMessage("null"), Error: &rpcError{Code: codeInternal, Message: "internal error"}})
	}
	return b
}

// idKey names a request ID for the set of running calls. A client may name
// the same ID as a number or a string, and both forms must match.
func idKey(id json.RawMessage) string {
	var v any
	if err := json.Unmarshal(id, &v); err != nil {
		return string(id)
	}
	switch v := v.(type) {
	case string:
		return v
	default:
		return string(id)
	}
}

// readLine reads one newline-terminated message. A last message without a
// newline still counts, and the next call reports io.EOF. A line longer
// than maxMessage stops the server.
func readLine(r *bufio.Reader) ([]byte, error) {
	var b []byte
	for {
		chunk, err := r.ReadSlice('\n')
		if len(b)+len(chunk) > maxMessage {
			return nil, errMessageTooLarge
		}
		b = append(b, chunk...)
		switch {
		case err == nil:
			return bytes.TrimSpace(b), nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		default:
			if line := bytes.TrimSpace(b); len(line) > 0 {
				return line, nil
			}
			return nil, err
		}
	}
}
