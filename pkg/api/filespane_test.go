package api

import (
	"errors"
	"testing"
)

func TestFilesStateValidate(t *testing.T) {
	ok := FilesState{
		Folder:   "/src",
		Expanded: []string{"", "a"},
		Folders: []FilesFolder{{Path: "", Entries: []FilesEntry{
			{Name: "a", Path: "a", Dir: true},
			{Name: "b.go", Path: "b.go", Mark: "M"},
		}}},
		Selected: "b.go",
	}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid state: %v", err)
	}
	tests := map[string]FilesState{
		"escape":       {Folders: []FilesFolder{{Path: "../x"}}},
		"absolute":     {Folders: []FilesFolder{{Path: "", Entries: []FilesEntry{{Name: "b", Path: "/b"}}}}},
		"slash name":   {Folders: []FilesFolder{{Path: "", Entries: []FilesEntry{{Name: "a/b", Path: "a/b"}}}}},
		"long mark":    {Folders: []FilesFolder{{Path: "", Entries: []FilesEntry{{Name: "b", Path: "b", Mark: "MM"}}}}},
		"unknown mark": {Folders: []FilesFolder{{Path: "", Entries: []FilesEntry{{Name: "b", Path: "b", Mark: "Z"}}}}},
		"bad selected": {Selected: "/b"},
	}
	for name, st := range tests {
		if err := st.Validate(); !errors.Is(err, ErrInvalidSidePane) {
			t.Errorf("%s.Validate() = %v, want ErrInvalidSidePane", name, err)
		}
	}
}
