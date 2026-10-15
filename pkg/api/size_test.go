package api

import (
	"errors"
	"strings"
	"testing"
)

func TestSizeValidate(t *testing.T) {
	tests := []struct {
		cols, rows int
		ok         bool
	}{
		{80, 24, true},
		{MaxCols, MaxRows, true},
		{1, 1, true},
		{0, 24, false},
		{80, -1, false},
		{MaxCols + 1, 24, false},
		{80, MaxRows + 1, false},
		{1 << 40, 3, false},
	}
	for _, tt := range tests {
		r := Resize{Cols: tt.cols, Rows: tt.rows}
		if err := r.Validate(); (err == nil) != tt.ok || (err != nil && !errors.Is(err, ErrBadSize)) {
			t.Errorf("Resize{%d,%d}.Validate() = %v, want ok=%v", tt.cols, tt.rows, err, tt.ok)
		}
		h := Hello{Protocol: ProtocolVersion, Cols: tt.cols, Rows: tt.rows}
		if err := h.Validate(); (err == nil) != tt.ok {
			t.Errorf("Hello{%d,%d}.Validate() = %v, want ok=%v", tt.cols, tt.rows, err, tt.ok)
		}
	}
}

func TestInputValidateHidesTypedText(t *testing.T) {
	in := Input{Pane: 1, Key: &Key{Code: "secret-word", Text: "secret-word"}}
	err := in.Validate()
	if err == nil {
		t.Fatal("invalid key accepted")
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("error %q contains the typed text; servers log these errors", err)
	}
}
