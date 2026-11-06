package convstore

// AppendRuntime writes an ordered batch of external display events with one
// flush. Runtime streaming produces many small text events; flushing each token
// separately would let disk latency determine whether a run can keep up.
// Call it off the server state goroutine, like Append.
func (w *Writer) AppendRuntime(events []RuntimeEvent) error {
	if len(events) == 0 {
		return nil
	}
	var lines []byte
	for i := range events {
		line, err := Encode(&events[i])
		if err != nil {
			return &WriteError{Path: w.path, Err: err}
		}
		lines = append(lines, line...)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ready(); err != nil {
		return err
	}
	return w.write(lines)
}
