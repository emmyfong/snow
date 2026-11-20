package chat

import "testing"

// TestPathDetect finds the paths in text that lie below the conversation's
// folder, and the line a path names.
func TestPathDetect(t *testing.T) {
	const folder = "/work/project"
	tests := []struct {
		name string
		text string
		fold string
		want []pathToken
	}{
		{
			name: "relative",
			text: "see internal/server/chat.go here",
			want: []pathToken{{Path: "internal/server/chat.go"}},
		},
		{
			name: "dot folder",
			text: "see .github/workflows/ci.yml now",
			want: []pathToken{{Path: ".github/workflows/ci.yml"}},
		},
		{
			name: "agent skill folder",
			text: "read .claude/skills/tui/SKILL.md:20",
			want: []pathToken{{Path: ".claude/skills/tui/SKILL.md", Line: 20}},
		},
		{
			name: "relative with line",
			text: "at internal/server/chat.go:12, the list",
			want: []pathToken{{Path: "internal/server/chat.go", Line: 12}},
		},
		{
			name: "line and column",
			text: "at internal/server/chat.go:12:5 here",
			want: []pathToken{{Path: "internal/server/chat.go", Line: 12}},
		},
		{
			name: "line then colon",
			text: "at internal/server/chat.go:12: undefined",
			want: []pathToken{{Path: "internal/server/chat.go", Line: 12}},
		},
		{
			name: "absolute inside",
			text: "/work/project/internal/server/chat.go",
			want: []pathToken{{Path: "internal/server/chat.go"}},
		},
		{
			name: "absolute outside",
			text: "/work/other/internal/server/chat.go",
		},
		{
			name: "url",
			text: "https://example.com/internal/server/chat.go",
		},
		{
			name: "text with dots",
			text: "see main.go and README.md",
		},
		{
			name: "host and path",
			text: "github.com/snow/snow/blob/main/README.md",
		},
		{
			name: "parent escape",
			text: "../other/chat.go",
		},
		{
			name: "punctuation",
			text: "`internal/server/chat.go`, then.",
			want: []pathToken{{Path: "internal/server/chat.go"}},
		},
		{
			name: "two paths",
			text: "a/internal/x.go and b/internal/y.go:3",
			want: []pathToken{{Path: "a/internal/x.go"}, {Path: "b/internal/y.go", Line: 3}},
		},
		{
			name: "windows drive",
			fold: "C:/work/project",
			text: "C:/WORK/PROJECT/internal/server/chat.go:4",
			want: []pathToken{{Path: "internal/server/chat.go", Line: 4}},
		},
		{
			name: "windows drive line and column",
			fold: "C:/work/project",
			text: "C:/WORK/PROJECT/internal/server/chat.go:3:7",
			want: []pathToken{{Path: "internal/server/chat.go", Line: 3}},
		},
		{
			name: "windows unicode folder",
			fold: "C:/work/Ärger",
			text: "c:/WORK/ärger/pkg/x.go",
			want: []pathToken{{Path: "pkg/x.go"}},
		},
		{
			name: "windows outside",
			fold: "C:/work/project",
			text: "D:/work/project/internal/server/chat.go",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := tt.fold
			if root == "" {
				root = folder
			}
			got := pathsIn(tt.text, root)
			if len(got) != len(tt.want) {
				t.Fatalf("pathsIn(%q) = %+v, want %+v", tt.text, got, tt.want)
			}
			for i := range got {
				if got[i].Path != tt.want[i].Path || got[i].Line != tt.want[i].Line {
					t.Fatalf("pathsIn(%q)[%d] = %+v, want %+v", tt.text, i, got[i], tt.want[i])
				}
			}
		})
	}
}
