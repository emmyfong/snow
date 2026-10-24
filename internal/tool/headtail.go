package tool

import "fmt"

// headTail keeps the first and the last bytes of a stream, up to a limit,
// and counts the bytes it drops between them. A command's end often holds
// its error; its start, what it was doing.
type headTail struct {
	head, tail []byte
	half       int
	dropped    int
}

func newHeadTail(limit int) *headTail { return &headTail{half: limit / 2} }

// Write keeps p's bytes. It never fails.
func (h *headTail) Write(p []byte) (int, error) {
	n := len(p)
	if room := h.half - len(h.head); room > 0 {
		k := min(room, len(p))
		h.head = append(h.head, p[:k]...)
		p = p[k:]
	}
	h.tail = append(h.tail, p...)
	if over := len(h.tail) - h.half; over > 0 {
		h.dropped += over
		h.tail = append(h.tail[:0], h.tail[over:]...)
	}
	return n, nil
}

// String returns the output with a note where bytes were dropped. It ends
// with a newline when there is output.
func (h *headTail) String() string {
	s := string(h.head)
	if h.dropped > 0 {
		s += fmt.Sprintf("\n[%d bytes cut]\n", h.dropped)
	}
	s += string(h.tail)
	if s != "" && s[len(s)-1] != '\n' {
		s += "\n"
	}
	return s
}
