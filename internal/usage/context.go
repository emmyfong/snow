package usage

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/pkg/api"
)

// Context returns the context pane state of loaded's conversation.
func Context(loaded *convstore.Conversation) api.ContextState {
	p := parse(loaded)
	st := api.ContextState{
		Folder:       loaded.Meta.Folder,
		Conversation: loaded.Meta.ID,
		Title:        title(loaded),
		Model:        cut(p.model),
	}
	if loaded.Meta.Runtime != nil {
		st.Runtime = loaded.Meta.Runtime.Kind
	}
	st.Occupancy = occupancy(p)
	e := evidence(p)
	st.Sources, st.Reads = e.sources, e.reads
	st.Notes = contextNotes(p, st, e)
	return st
}

// occupancy returns how full the model's context window was. Only the
// direct-API path reports it; a runtime's hidden prompt is never estimated.
func occupancy(p *parsed) api.ContextOccupancy {
	if p.runtime {
		return api.ContextOccupancy{
			Reason: "This runtime does not report how much of its context window it used. Snow does not estimate the hidden prompt.",
		}
	}
	var last *reply
	for _, t := range p.turns {
		for i := range t.replies {
			if t.replies[i].usage != nil {
				last = &t.replies[i]
			}
		}
	}
	if last == nil {
		return api.ContextOccupancy{Reason: "No reply with usage has been saved yet."}
	}
	window := 0
	// The requested name carries the provider, which the price list needs;
	// the vendor's reported ID alone may not.
	name := last.requested
	if name == "" {
		name = last.model
	}
	if parsed, err := provider.ParseModelName(name); err == nil {
		window = provider.ContextWindow(parsed, 0)
	}
	return api.ContextOccupancy{
		Known:  true,
		Tokens: last.usage.Input + last.usage.CacheRead + last.usage.CacheWrite,
		Window: window,
		Source: "the last prompt of the direct-API reply",
	}
}

// callKind is what a tool call did to a file.
type callKind int

const (
	callOther callKind = iota
	callRead
	callChanged
)

// classify names the tools that read or change one file, in Snow's own
// names and those of Claude Code.
func classify(name string) callKind {
	switch strings.ToLower(name) {
	case "read":
		return callRead
	case "write", "edit", "multiedit", "notebookedit":
		return callChanged
	}
	return callOther
}

// callPath returns the file and range a tool call names.
func callPath(c *call) (file, rng string) {
	var in struct {
		Path     string `json:"path"`
		FilePath string `json:"file_path"`
		Offset   int    `json:"offset"`
		Limit    int    `json:"limit"`
	}
	if len(c.input) > 0 {
		_ = json.Unmarshal(c.input, &in)
	}
	file = in.Path
	if file == "" {
		file = in.FilePath
	}
	file = cleanPath(file)
	switch {
	case in.Offset > 0 && in.Limit > 0:
		rng = fmt.Sprintf("%d-%d", in.Offset, in.Offset+in.Limit-1)
	case in.Offset > 0:
		rng = fmt.Sprintf("%d+", in.Offset)
	}
	return file, rng
}

// cleanPath makes a tool's path safe to show and one cell wide per rune.
func cleanPath(p string) string {
	p = strings.TrimSpace(convstore.OneLine(p))
	p = strings.ReplaceAll(p, `\`, "/")
	return cut(p)
}

// evidence is the pane's observable context sources and repeated reads.
type evidenceResult struct {
	sources     []api.ContextSource
	reads       []api.ContextRead
	moreSources int
	moreReads   int
}

// evidence walks the successful file tool calls in order. A read's result
// text is compared with the read before it at the same path and range;
// the comparison is evidence, not a claim that the file changed.
func evidence(p *parsed) evidenceResult {
	sources := map[string]*api.ContextSource{}
	type readTotal struct {
		read     api.ContextRead
		prev     string
		havePrev bool
	}
	reads := map[string]*readTotal{}

	for _, t := range p.turns {
		for _, c := range t.calls {
			kind := classify(c.name)
			if kind == callOther || !c.done || c.isError {
				continue
			}
			file, rng := callPath(c)
			if file == "" {
				continue
			}
			s := sources[file]
			if s == nil {
				s = &api.ContextSource{Path: file, FirstTurn: t.number}
				sources[file] = s
			}
			s.LastTurn = t.number
			if kind == callChanged {
				s.Changed = true
				continue
			}
			s.Reads++
			key := file + "\x00" + rng
			r := reads[key]
			if r == nil {
				r = &readTotal{read: api.ContextRead{Path: file, Range: rng, FirstTurn: t.number, FirstAt: c.at}}
				reads[key] = r
			}
			r.read.Reads++
			if r.havePrev {
				if c.result == r.prev {
					r.read.Same++
				} else {
					r.read.Changed++
				}
			}
			r.prev, r.havePrev = c.result, true
			r.read.LastTurn, r.read.LastAt = t.number, c.at
		}
	}

	out := evidenceResult{sources: make([]api.ContextSource, 0, len(sources))}
	for _, s := range sources {
		out.sources = append(out.sources, *s)
	}
	slices.SortFunc(out.sources, func(a, b api.ContextSource) int {
		if d := b.Reads - a.Reads; d != 0 {
			return d
		}
		return strings.Compare(a.Path, b.Path)
	})
	if len(out.sources) > api.MaxContextSources {
		out.moreSources = len(out.sources) - api.MaxContextSources
		out.sources = out.sources[:api.MaxContextSources]
	}

	for _, r := range reads {
		if r.read.Reads < 2 {
			continue
		}
		switch {
		case r.read.Changed == 0:
			r.read.Result = "unchanged"
		case r.read.Same == 0:
			r.read.Result = "changed"
		default:
			r.read.Result = "mixed"
		}
		r.read.Evidence = "tool result text"
		out.reads = append(out.reads, r.read)
	}
	slices.SortFunc(out.reads, func(a, b api.ContextRead) int {
		if d := b.Reads - a.Reads; d != 0 {
			return d
		}
		if d := b.Changed - a.Changed; d != 0 {
			return d
		}
		if d := strings.Compare(a.Path, b.Path); d != 0 {
			return d
		}
		return strings.Compare(a.Range, b.Range)
	})
	if len(out.reads) > api.MaxContextReads {
		out.moreReads = len(out.reads) - api.MaxContextReads
		out.reads = out.reads[:api.MaxContextReads]
	}
	return out
}

// contextNotes explains the limits of what the records prove.
func contextNotes(p *parsed, st api.ContextState, e evidenceResult) []string {
	var out []string
	if p.runtime {
		out = append(out, "This runtime does not report context occupancy or the hidden prompt's content.")
	}
	if st.Occupancy.Known && st.Occupancy.Window == 0 {
		out = append(out, "The model's context window is unknown; only the prompt's token count is shown.")
	}
	if len(st.Sources) == 0 {
		out = append(out, "No successful file reads or changes are saved for this conversation.")
	}
	out = append(out, "Per-source token counts are unavailable: tool calls do not report the tokens their content added.")
	out = append(out, "Repeated reads compare tool result text, not a saved file version. A file change can justify a second read.")
	if e.moreSources > 0 {
		out = append(out, fmt.Sprintf("%d more sources are not shown.", e.moreSources))
	}
	if e.moreReads > 0 {
		out = append(out, fmt.Sprintf("%d more repeated reads are not shown.", e.moreReads))
	}
	return out
}
