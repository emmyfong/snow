package clauderuntime

func (s *conversationState) drainRuntimeFrames() {
	if s.worker == nil {
		return
	}
	client := s.worker.client.Load()
	if client == nil {
		return
	}
	// Control acknowledgements are correlated on another reader. Fold earlier
	// queued lifecycle frames before checking which owned tasks remain live.
	for range 256 {
		select {
		case frame, ok := <-client.Frames():
			if !ok {
				return
			}
			if err := s.frame(frame); err != nil && s.failure == nil {
				s.failure = err
			}
		default:
			return
		}
	}
}
