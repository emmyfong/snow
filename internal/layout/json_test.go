package layout

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestTreeJSONRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		tree *Tree
	}{
		{"one pane", New(1)},
		{"nested", &Tree{Root: h(half, leaf(1), v(half, leaf(2), leaf(3)))}},
		{"three children", &Tree{Root: v([]float64{0.25, 0.25, 0.5}, leaf(4), leaf(7), leaf(2))}},
		{"uneven ratios", &Tree{Root: h([]float64{1.0 / 3, 2.0 / 3}, leaf(1), leaf(2))}},
		{"zoomed", &Tree{Root: h(half, leaf(1), leaf(2)), Zoomed: 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.tree)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			var got Tree
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatalf("Unmarshal(%s): %v", b, err)
			}
			if !reflect.DeepEqual(&got, tt.tree) {
				t.Fatalf("round trip = %s, want %s", dump(t, &got), b)
			}
		})
	}
}

func TestTreeJSONShape(t *testing.T) {
	tr := &Tree{Root: h(half, leaf(1), leaf(2)), Zoomed: 2}
	b, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `{"root":{"axis":"horizontal","ratios":[0.5,0.5],"children":[{"pane":1},{"pane":2}]},"zoomed":2}`
	if string(b) != want {
		t.Fatalf("Marshal = %s, want %s", b, want)
	}
}

func TestTreeJSONInvalid(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"no root", `{}`},
		{"leaf without a pane", `{"root":{}}`},
		{"negative pane", `{"root":{"pane":-1}}`},
		{"leaf with children", `{"root":{"pane":1,"children":[{"pane":2}]}}`},
		{"split with a pane", `{"root":{"pane":3,"axis":"vertical","ratios":[0.5,0.5],"children":[{"pane":1},{"pane":2}]}}`},
		{"split without an axis", `{"root":{"ratios":[0.5,0.5],"children":[{"pane":1},{"pane":2}]}}`},
		{"one-child split", `{"root":{"axis":"vertical","ratios":[1],"children":[{"pane":1}]}}`},
		{"ratio count differs", `{"root":{"axis":"vertical","ratios":[1],"children":[{"pane":1},{"pane":2}]}}`},
		{"ratios do not sum to one", `{"root":{"axis":"vertical","ratios":[0.5,0.25],"children":[{"pane":1},{"pane":2}]}}`},
		{"zero ratio", `{"root":{"axis":"vertical","ratios":[1,0],"children":[{"pane":1},{"pane":2}]}}`},
		{"duplicate pane", `{"root":{"axis":"vertical","ratios":[0.5,0.5],"children":[{"pane":1},{"pane":1}]}}`},
		{"zoomed pane missing", `{"root":{"pane":1},"zoomed":2}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Tree
			if err := json.Unmarshal([]byte(tt.in), &got); !errors.Is(err, ErrInvalidTree) {
				t.Fatalf("Unmarshal(%s) err = %v, want ErrInvalidTree", tt.in, err)
			}
		})
	}
}

func TestAxisJSONUnknown(t *testing.T) {
	in := `{"root":{"axis":"diagonal","ratios":[0.5,0.5],"children":[{"pane":1},{"pane":2}]}}`
	var got Tree
	err := json.Unmarshal([]byte(in), &got)
	if !errors.Is(err, ErrInvalidAxis) || !errors.Is(err, ErrInvalidTree) {
		t.Fatalf("Unmarshal err = %v, want ErrInvalidAxis and ErrInvalidTree", err)
	}
}
