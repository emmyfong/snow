package git

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/gittest"
)

func TestConflictMarkers(t *testing.T) {
	dir := conflictedRepo(t)
	r := Repo{Path: dir}

	got, err := r.ConflictMarkers(context.Background(), false, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "a.txt" {
		t.Fatalf("markers %q, want [a.txt]", got)
	}

	write(t, dir, "a.txt", "resolved with trailing spaces  \n")
	got, err = r.ConflictMarkers(context.Background(), false, "a.txt")
	if err != nil || len(got) != 0 {
		t.Fatalf("resolved file markers %q, %v; want no markers and no error", got, err)
	}
}

func TestConflictMarkersUntracked(t *testing.T) {
	dir := repo(t)
	write(t, dir, "new.txt", "<<<<<<< HEAD\nlocal\n=======\nremote\n>>>>>>> remote\n")

	got, err := (Repo{Path: dir}).ConflictMarkers(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "new.txt" {
		t.Fatalf("markers %q, want [new.txt]", got)
	}
}

func TestGitStageRefusesConflictMarkers(t *testing.T) {
	for _, tt := range []struct {
		name  string
		stage func(context.Context, Repo) error
	}{
		{name: "stage", stage: func(ctx context.Context, r Repo) error { return r.Stage(ctx, "a.txt") }},
		{name: "stage all", stage: func(ctx context.Context, r Repo) error { return r.StageAll(ctx) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := conflictedRepo(t)
			r := Repo{Path: dir}
			before := gittest.Run(t, dir, "ls-files", "--stage", "--", "a.txt")

			err := tt.stage(context.Background(), r)
			if !errors.Is(err, ErrConflictMarkers) || !strings.Contains(err.Error(), "a.txt") {
				t.Fatalf("stage error %v, want conflict-marker error naming a.txt", err)
			}
			if after := gittest.Run(t, dir, "ls-files", "--stage", "--", "a.txt"); after != before {
				t.Fatalf("index changed:\nbefore %q\nafter  %q", before, after)
			}
		})
	}
}

func TestGitCommitRefusesConflictMarkers(t *testing.T) {
	dir := repo(t)
	write(t, dir, "a.txt", "<<<<<<< HEAD\nlocal\n=======\nremote\n>>>>>>> remote\n")
	gittest.Run(t, dir, "add", "--", "a.txt")
	head := gittest.Run(t, dir, "rev-parse", "HEAD")

	_, err := (Repo{Path: dir}).Commit(context.Background(), "finish merge")
	if !errors.Is(err, ErrConflictMarkers) || !strings.Contains(err.Error(), "a.txt") {
		t.Fatalf("commit error %v, want conflict-marker error naming a.txt", err)
	}
	if got := gittest.Run(t, dir, "rev-parse", "HEAD"); got != head {
		t.Fatalf("HEAD moved from %s to %s", head, got)
	}
}

func conflictedRepo(t *testing.T) string {
	t.Helper()
	dir := repo(t)
	gittest.Run(t, dir, "checkout", "-q", "-b", "other")
	write(t, dir, "a.txt", "other\n")
	gittest.Run(t, dir, "add", "--", "a.txt")
	gittest.Run(t, dir, "commit", "-m", "other")
	gittest.Run(t, dir, "checkout", "-q", "main")
	write(t, dir, "a.txt", "main\n")
	gittest.Run(t, dir, "add", "--", "a.txt")
	gittest.Run(t, dir, "commit", "-m", "main")
	if _, err := (Repo{Path: dir}).Run(context.Background(), dir, "merge", "other"); err == nil {
		t.Fatal("merge succeeded, want a content conflict")
	}
	return dir
}

func TestGitStageRefusesConflictMarkersWithFixedPatternConfig(t *testing.T) {
	dir := repo(t)
	write(t, dir, "a.txt", "<<<<<<< HEAD\nlocal\n=======\nremote\n>>>>>>> remote\n")
	before := gittest.Run(t, dir, "ls-files", "--stage", "--", "a.txt")
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "grep.patternType")
	t.Setenv("GIT_CONFIG_VALUE_0", "fixed")

	err := (Repo{Path: dir}).Stage(context.Background(), "a.txt")
	if !errors.Is(err, ErrConflictMarkers) || !strings.Contains(err.Error(), "a.txt") {
		t.Fatalf("Stage error = %v, want conflict-marker refusal naming a.txt", err)
	}
	if got := gittest.Run(t, dir, "ls-files", "--stage", "--", "a.txt"); got != before {
		t.Fatalf("Stage changed the index with grep.patternType=fixed: before=%q after=%q", before, got)
	}
}

func TestGitCommitRefusesUnmergedMarkersFromSubfolder(t *testing.T) {
	root := repo(t)
	write(t, root, "sub/a.txt", "base\n")
	write(t, root, "outside.txt", "base\n")
	gittest.Run(t, root, "add", "sub/a.txt", "outside.txt")
	gittest.Run(t, root, "commit", "-q", "-m", "add subfolder files")
	gittest.Run(t, root, "checkout", "-q", "-b", "other")
	write(t, root, "sub/a.txt", "other sub\n")
	write(t, root, "outside.txt", "other outside\n")
	gittest.Run(t, root, "commit", "-qam", "other edits")
	gittest.Run(t, root, "checkout", "-q", "main")
	write(t, root, "sub/a.txt", "main sub\n")
	write(t, root, "outside.txt", "main outside\n")
	gittest.Run(t, root, "commit", "-qam", "main edits")
	if _, err := (Repo{Path: root}).Run(context.Background(), root, "merge", "other"); err == nil {
		t.Fatal("merge succeeded, want conflicts")
	}

	sub := filepath.Join(root, "sub")
	r := Repo{Path: sub}
	_, err := r.Commit(context.Background(), "finish merge")
	if !errors.Is(err, ErrConflictMarkers) || !strings.Contains(err.Error(), "sub/a.txt") || !strings.Contains(err.Error(), "outside.txt") {
		t.Fatalf("Commit error = %v, want conflict-marker refusal naming both files", err)
	}
}
