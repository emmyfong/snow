package clauderuntime

import (
	"bufio"
	"encoding/json"
	"fmt"
)

func (s *Stream) read() {
	defer close(s.readDone)
	scanner := bufio.NewScanner(s.output)
	scanner.Buffer(make([]byte, 64<<10), MaxFrameBytes+1)
	for scanner.Scan() {
		data := scanner.Bytes()
		if len(data) == 0 {
			continue
		}
		var frame Frame
		if len(data) > MaxFrameBytes || json.Unmarshal(data, &frame) != nil || frame.Type == "" {
			s.finish(fmt.Errorf("%w: missing or malformed envelope", ErrProtocol))
			return
		}
		frame.Raw = append(json.RawMessage(nil), data...)
		select {
		case <-s.stopping:
			return
		case s.frames <- frame:
		}
	}
	select {
	case <-s.stopping:
		return
	default:
	}
	if scanner.Err() != nil {
		// Scanner errors can include untrusted text. Keep the error bounded and
		// never include a raw frame, which may contain private tool arguments.
		s.finish(fmt.Errorf("%w: cannot read complete frame", ErrProtocol))
		return
	}
	s.finish(fmt.Errorf("%w: runtime closed its output", ErrClosed))
}
