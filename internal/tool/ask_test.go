package tool

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// fakeUser answers every question with its reply and keeps what it was
// asked.
type fakeUser struct {
	reply Reply
	err   error
	asked []Question
}

func (u *fakeUser) Ask(_ context.Context, q Question) (Reply, error) {
	u.asked = append(u.asked, q)
	return u.reply, u.err
}

func TestAskRejectsBadInput(t *testing.T) {
	long := func(n int) string { return strings.Repeat("x", n) }
	opts := func(labels ...string) string {
		var parts []string
		for _, l := range labels {
			b, _ := json.Marshal(map[string]string{"label": l})
			parts = append(parts, string(b))
		}
		return "[" + strings.Join(parts, ",") + "]"
	}
	tests := []struct {
		name, in string
	}{
		{"no question", `{"question":" ","options":` + opts("a", "b") + `}`},
		{"long question", `{"question":"` + long(askMaxQuestion+1) + `","options":` + opts("a", "b") + `}`},
		{"one option", `{"question":"q","options":` + opts("a") + `}`},
		{"ten options", `{"question":"q","options":` + opts("1", "2", "3", "4", "5", "6", "7", "8", "9", "10") + `}`},
		{"blank label", `{"question":"q","options":` + opts("a", " ") + `}`},
		{"long label", `{"question":"q","options":` + opts("a", long(askMaxLabel+1)) + `}`},
		{"two-line label", `{"question":"q","options":` + opts("a", "b\nc") + `}`},
		{"same label", `{"question":"q","options":` + opts("Yes", "yes") + `}`},
		{"other label", `{"question":"q","options":` + opts("a", "other") + `}`},
		{"long description", `{"question":"q","options":[{"label":"a"},{"label":"b","description":"` + long(askMaxDesc+1) + `"}]}`},
		{"long preview", `{"question":"q","options":[{"label":"a"},{"label":"b","preview":"` + long(askMaxPreview+1) + `"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &fakeUser{}
			r := Builtin().Call(context.Background(), Env{User: u}, "ask", json.RawMessage(tt.in), nil)
			if !r.IsError || !strings.Contains(r.Content, ErrBadQuestion.Error()) {
				t.Fatalf("result = %+v, want a bad question error", r)
			}
			if len(u.asked) != 0 {
				t.Fatalf("asked %v; a bad question must not reach the user", u.asked)
			}
		})
	}
}

func TestAskReply(t *testing.T) {
	const in = `{"question":" Which? ","options":[{"label":"A","description":"first","preview":"code"},{"label":"B"}],"multi":true}`
	tests := []struct {
		name  string
		reply Reply
		want  string
	}{
		{"one", Reply{Picked: []string{"A"}}, `The user picked "A".`},
		{"two", Reply{Picked: []string{"A", "B"}}, `The user picked "A", "B".`},
		{"none", Reply{}, "The user picked none of the answers."},
		{"other", Reply{Other: "neither"}, "The user typed their own answer:\nneither"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &fakeUser{reply: tt.reply}
			r := Builtin().Call(context.Background(), Env{User: u}, "ask", json.RawMessage(in), nil)
			if r.IsError || r.Content != tt.want {
				t.Fatalf("result = %+v, want %q", r, tt.want)
			}
			want := Question{Text: "Which?", Multi: true, Options: []Option{{Label: "A", Description: "first", Preview: "code"}, {Label: "B"}}}
			if len(u.asked) != 1 || u.asked[0].Text != want.Text || !u.asked[0].Multi || len(u.asked[0].Options) != 2 || u.asked[0].Options[0] != want.Options[0] {
				t.Fatalf("asked %+v, want %+v", u.asked, want)
			}
		})
	}
}

func TestAskErrors(t *testing.T) {
	const in = `{"question":"q","options":[{"label":"a"},{"label":"b"}]}`
	if r := run(t, Env{}, "ask", in); !r.IsError || r.Content != ErrNoUser.Error() {
		t.Fatalf("no user: %+v", r)
	}
	gone := errors.New("interrupted")
	if r := run(t, Env{User: &fakeUser{err: gone}}, "ask", in); !r.IsError || r.Content != gone.Error() {
		t.Fatalf("ask error: %+v", r)
	}
}
