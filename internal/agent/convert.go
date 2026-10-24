package agent

import (
	"time"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/provider"
)

// history turns loaded records into the messages a provider gets. Records
// that are not part of the exchange, such as renames, are left out, and
// messages of the same role in a row are joined, as vendors want turns to
// alternate.
func history(recs []convstore.Record) []provider.Message {
	var msgs []provider.Message
	add := func(role provider.Role, blocks ...provider.Block) {
		if n := len(msgs); n > 0 && msgs[n-1].Role == role {
			msgs[n-1].Content = append(msgs[n-1].Content, blocks...)
			return
		}
		msgs = append(msgs, provider.Message{Role: role, Content: blocks})
	}
	for _, r := range recs {
		switch r := r.(type) {
		case *convstore.User:
			add(provider.RoleUser, &provider.Text{Text: r.Text})
		case *convstore.Assistant:
			if blocks := toProvider(r.Blocks); len(blocks) > 0 {
				add(provider.RoleAssistant, blocks...)
			}
		case *convstore.ToolResult:
			add(provider.RoleUser, &provider.ToolResult{ToolUseID: r.ToolUseID, Content: r.Content, IsError: r.IsError})
		}
	}
	return msgs
}

// toProvider turns saved blocks into provider blocks.
func toProvider(blocks []convstore.Block) []provider.Block {
	out := make([]provider.Block, 0, len(blocks))
	for _, b := range blocks {
		switch b.Kind {
		case convstore.BlockText:
			out = append(out, &provider.Text{Text: b.Text})
		case convstore.BlockThinking:
			out = append(out, &provider.Thinking{Text: b.Text, Signature: b.Signature})
		case convstore.BlockRedactedThinking:
			out = append(out, &provider.RedactedThinking{Data: b.Data})
		case convstore.BlockToolUse:
			out = append(out, &provider.ToolUse{ID: b.ID, Name: b.Name, Input: b.Input})
		}
	}
	return out
}

// fromProvider turns the blocks of a reply into blocks to save.
func fromProvider(blocks []provider.Block) []convstore.Block {
	out := make([]convstore.Block, 0, len(blocks))
	for _, b := range blocks {
		switch b := b.(type) {
		case *provider.Text:
			out = append(out, convstore.Block{Kind: convstore.BlockText, Text: b.Text})
		case *provider.Thinking:
			out = append(out, convstore.Block{Kind: convstore.BlockThinking, Text: b.Text, Signature: b.Signature})
		case *provider.RedactedThinking:
			out = append(out, convstore.Block{Kind: convstore.BlockRedactedThinking, Data: b.Data})
		case *provider.ToolUse:
			out = append(out, convstore.Block{Kind: convstore.BlockToolUse, ID: b.ID, Name: b.Name, Input: b.Input})
		}
	}
	return out
}

// assistantRecord is the record of a whole reply.
func assistantRecord(at time.Time, r reply) *convstore.Assistant {
	a := &convstore.Assistant{At: at, Model: r.reported, Blocks: fromProvider(r.blocks), Stop: convstore.StopReason(r.stop)}
	if r.usage != (provider.Usage{}) {
		a.Usage = &convstore.Usage{Input: r.usage.Input, Output: r.usage.Output, CacheRead: r.usage.CacheRead, CacheWrite: r.usage.CacheWrite}
	}
	return a
}
