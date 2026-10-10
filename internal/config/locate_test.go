package config

import "testing"

func TestKeyLine(t *testing.T) {
	tests := []struct {
		name string
		data string
		key  []string
		want int
	}{
		{"root key", "a = 1\nb = 2\n", []string{"b"}, 2},
		{"key in table", "[t]\nx = 1\ny = 2\n", []string{"t", "y"}, 3},
		{"table header", "x = 1\n\n[t]\n", []string{"t"}, 3},
		{"same name in another table", "[a]\nx = 1\n[b]\nx = 2\n", []string{"b", "x"}, 4},
		{"dotted key", "[t]\nu.v = 1\n", []string{"t", "u", "v"}, 2},
		{"dotted header", "[t . u]\nv = 1\n", []string{"t", "u", "v"}, 2},
		{"quoted key", "[t]\n\"a.b\" = 1\n", []string{"t", "a.b"}, 2},
		{"literal key", "[t]\n'x y' = 1\n", []string{"t", "x y"}, 2},
		{"escaped quote in key", "\"a\\\"b\" = 1\n", []string{`a"b`}, 1},
		{"array of tables", "[[t]]\nx = 1\n", []string{"t", "x"}, 2},
		{"inline table falls back to parent", "t = { x = 1 }\n", []string{"t", "x"}, 1},
		{"comment and blank lines", "# c\n\n  [t]  # c\n  x=1\n", []string{"t", "x"}, 4},
		{"windows line endings", "[t]\r\nx = 1\r\n", []string{"t", "x"}, 2},
		{"missing", "a = 1\n", []string{"z"}, 0},
		{"value is not a key", "a = \"b = 1\"\n", []string{"b"}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := keyLine(tt.data, tt.key); got != tt.want {
				t.Errorf("keyLine(%q, %q) = %d, want %d", tt.data, tt.key, got, tt.want)
			}
		})
	}
}
