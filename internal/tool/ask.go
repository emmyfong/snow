package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Limits of the ask tool, so a question fits the chat and the protocol.
// Keep askMaxOptions and askMaxLabel in step with api.MaxChatPicked and
// api.MaxChatLabel, which bound a reply.
const (
	askMinOptions  = 2
	askMaxOptions  = 9 // the user picks one with a digit key
	askMaxQuestion = 500
	askMaxLabel    = 60
	askMaxDesc     = 200
	askMaxPreview  = 2000
)

// otherLabel is the choice the chat always adds to a question, for an
// answer the user types. A model's option may not take it.
const otherLabel = "Other"

// ErrBadQuestion is the cause of an error for an ask input the chat
// cannot show.
var ErrBadQuestion = errors.New("bad question")

type askTool struct{}

func init() { register(askTool{}) }

func (askTool) Def() Def {
	option := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"label":       prop("string", "a short answer, unique in the list"),
			"description": prop("string", "one line on what this answer means"),
			"preview":     prop("string", "optional longer text the user sees while this answer is highlighted, such as a code sample"),
		},
		"required":             []string{"label"},
		"additionalProperties": false,
	}
	return Def{
		Name: "ask",
		Description: "Ask the user a question with answers to pick from, and wait for the answer. " +
			"Use it when you need a decision you cannot make from the code. " +
			"The user can always type another answer instead.",
		Schema: schema(map[string]any{
			"question": prop("string", "the question, in one or two sentences"),
			"options": map[string]any{
				"type": "array", "items": option, "minItems": askMinOptions, "maxItems": askMaxOptions,
				"description": "the answers to pick from; do not add an \"" + otherLabel + "\" answer",
			},
			"multi": prop("boolean", "true lets the user pick more than one answer"),
		}, "question", "options"),
	}
}

func (askTool) Run(ctx context.Context, env Env, raw json.RawMessage) (string, error) {
	var in struct {
		Question string `json:"question"`
		Options  []struct {
			Label       string `json:"label"`
			Description string `json:"description"`
			Preview     string `json:"preview"`
		} `json:"options"`
		Multi bool `json:"multi"`
	}
	if err := input(raw, &in); err != nil {
		return "", err
	}
	q := Question{Text: strings.TrimSpace(in.Question), Multi: in.Multi}
	for _, o := range in.Options {
		q.Options = append(q.Options, Option{
			Label: strings.TrimSpace(o.Label), Description: strings.TrimSpace(o.Description), Preview: o.Preview,
		})
	}
	if err := checkQuestion(q); err != nil {
		return "", err
	}
	if env.User == nil {
		return "", ErrNoUser
	}
	r, err := env.User.Ask(ctx, q)
	if err != nil {
		return "", err
	}
	return replyText(r), nil
}

// checkQuestion returns an error wrapping ErrBadQuestion when q breaks a
// limit of the ask tool.
func checkQuestion(q Question) error {
	bad := func(format string, a ...any) error {
		return fmt.Errorf("%w: %s", ErrBadQuestion, fmt.Sprintf(format, a...))
	}
	switch n := len(q.Options); {
	case q.Text == "":
		return bad("the question is empty")
	case utf8.RuneCountInString(q.Text) > askMaxQuestion:
		return bad("the question is over %d characters", askMaxQuestion)
	case n < askMinOptions || n > askMaxOptions:
		return bad("%d options; give %d to %d", n, askMinOptions, askMaxOptions)
	}
	seen := map[string]bool{}
	for i, o := range q.Options {
		key := strings.ToLower(o.Label)
		switch {
		case o.Label == "":
			return bad("option %d has no label", i+1)
		case utf8.RuneCountInString(o.Label) > askMaxLabel:
			return bad("label %q is over %d characters", o.Label, askMaxLabel)
		case strings.ContainsAny(o.Label, "\r\n\t"):
			return bad("label %q is not one line", o.Label)
		case key == strings.ToLower(otherLabel):
			return bad("the chat adds %q by itself", otherLabel)
		case seen[key]:
			return bad("label %q is given twice", o.Label)
		case utf8.RuneCountInString(o.Description) > askMaxDesc:
			return bad("the description of %q is over %d characters", o.Label, askMaxDesc)
		case utf8.RuneCountInString(o.Preview) > askMaxPreview:
			return bad("the preview of %q is over %d characters", o.Label, askMaxPreview)
		}
		seen[key] = true
	}
	return nil
}

// replyText is the tool result the model gets for r.
func replyText(r Reply) string {
	if r.Other != "" {
		return "The user typed their own answer:\n" + r.Other
	}
	if len(r.Picked) == 0 {
		return "The user picked none of the answers."
	}
	quoted := make([]string, len(r.Picked))
	for i, p := range r.Picked {
		quoted[i] = strconv.Quote(p)
	}
	return "The user picked " + strings.Join(quoted, ", ") + "."
}
