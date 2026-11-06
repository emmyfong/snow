package clauderuntime

import (
	"encoding/json"
	"errors"
	"fmt"
)

func (c *Client) read() {
	defer func() {
		c.mu.Lock()
		clear(c.tools)
		c.mu.Unlock()
		close(c.frames)
		close(c.done)
	}()
	for frame := range c.stream.Frames() {
		forward, err := c.control(frame)
		if err != nil {
			c.err = errors.Join(err, c.stream.Stop())
			return
		}
		if !forward {
			continue
		}
		select {
		case c.frames <- frame:
		case <-c.stream.Done():
			c.err = c.stream.Err()
			return
		}
	}
	<-c.stream.Done()
	c.err = c.stream.Err()
}

func (c *Client) control(frame Frame) (bool, error) {
	switch frame.Type {
	case "control_response":
		var message struct {
			Response controlReply `json:"response"`
		}
		if json.Unmarshal(frame.Raw, &message) != nil || !validRequestID(message.Response.RequestID) ||
			(message.Response.Subtype != "success" && message.Response.Subtype != "error") {
			return false, fmt.Errorf("%w: invalid control response identity", ErrProtocol)
		}
		c.mu.Lock()
		reply := c.calls[message.Response.RequestID]
		c.mu.Unlock()
		if reply != nil {
			select {
			case reply <- message.Response:
			default:
				return false, fmt.Errorf("%w: duplicate control response", ErrProtocol)
			}
		}
		return false, nil
	case "control_request":
		var message struct {
			Request Permission `json:"request"`
		}
		if json.Unmarshal(frame.Raw, &message) != nil || !validRequestID(frame.RequestID) || message.Request.Subtype == "" {
			return false, fmt.Errorf("%w: invalid permission identity", ErrProtocol)
		}
		var input map[string]json.RawMessage
		if message.Request.Subtype != "can_use_tool" || message.Request.Tool == "" ||
			json.Unmarshal(message.Request.Input, &input) != nil || input == nil {
			return false, fmt.Errorf("%w: unsupported runtime control request", ErrProtocol)
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.seen[frame.RequestID] {
			return false, fmt.Errorf("%w: duplicate permission identity", ErrProtocol)
		}
		if len(c.tools) >= 64 || len(c.seen) >= 16384 {
			return false, ErrBackpressure
		}
		c.tools[frame.RequestID] = message.Request
		c.seen[frame.RequestID] = true
	case "control_cancel_request":
		if !validRequestID(frame.RequestID) {
			return false, fmt.Errorf("%w: missing cancellation identity", ErrProtocol)
		}
		c.mu.Lock()
		delete(c.tools, frame.RequestID)
		c.mu.Unlock()
	}
	return true, nil
}

func validRequestID(id string) bool { return len(id) > 0 && len(id) <= 256 }
