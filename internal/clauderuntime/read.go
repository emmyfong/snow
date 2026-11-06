package clauderuntime

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"
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
	s.finish(s.closedCause())
}

// notInstalledCode is the exit code of a shell that cannot find the command,
// which the WSL launch script also uses when claude is missing.
const notInstalledCode = 127

// closedCause says why the runtime closed its output. Claude's diagnostic
// output can hold private tool details, so it is never part of the error;
// only the exit code of a missing claude is told apart.
func (s *Stream) closedCause() error {
	select {
	case <-s.process.Done():
		var exit *exec.ExitError
		if errors.As(s.process.Err(), &exit) && exit.ExitCode() == notInstalledCode {
			return fmt.Errorf("%w: Claude Code is not installed in this environment", ErrClosed)
		}
	case <-time.After(time.Second):
	}
	return fmt.Errorf("%w: runtime closed its output", ErrClosed)
}
