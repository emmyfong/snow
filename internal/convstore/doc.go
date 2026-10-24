// Package convstore keeps conversations on disk so that nothing the user and
// the agent said is lost: one append-only JSON Lines file per conversation,
// one record per line, each written whole and flushed before Append returns.
//
// A Store is a folder of conversation files. Create starts a conversation
// with its meta record, Writer resumes one, Load reads one back with the
// damage a crash can leave, and List shows the history of a folder.
package convstore
