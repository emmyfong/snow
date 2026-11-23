package git

import (
	"context"
	"errors"
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
