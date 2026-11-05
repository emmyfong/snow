package roletext

import "testing"

// TestNoInstructionOnlyReadOnly: only a read-only mode reads "read-only";
// an instruction-only role says it is not enforced.
func TestNoInstructionOnlyReadOnly(t *testing.T) {
	tests := []struct {
		mode        string
		readOnly    bool
		long, short string
	}{
		{"plan", true, "read-only: plan mode, enforced", "read-only"},
		{"acceptEdits", false, "acceptEdits mode, enforced", "acceptEdits"},
		{"", false, "instructions only, not enforced", "not enforced"},
	}
	for _, tt := range tests {
		if got := Limits(tt.mode, tt.readOnly); got != tt.long {
			t.Errorf("Limits(%q, %v) = %q, want %q", tt.mode, tt.readOnly, got, tt.long)
		}
		if got := Short(tt.mode, tt.readOnly); got != tt.short {
			t.Errorf("Short(%q, %v) = %q, want %q", tt.mode, tt.readOnly, got, tt.short)
		}
	}
}
