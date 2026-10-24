package providerhttp

// MaxMessage keeps a vendor's message short enough for one line in the UI.
const MaxMessage = 500

// Cut shortens msg to MaxMessage runes and marks the cut with "...".
func Cut(msg string) string {
	r := []rune(msg)
	if len(r) <= MaxMessage {
		return msg
	}
	return string(r[:MaxMessage]) + "..."
}
