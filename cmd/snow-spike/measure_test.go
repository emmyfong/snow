package main

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/charmbracelet/x/vt"
)

// TestMeasureScrollbackMemory reports heap bytes per scrollback line. It
// asserts nothing; run with -v and read the log.
func TestMeasureScrollbackMemory(t *testing.T) {
	for _, width := range []int{80, 200} {
		for _, fill := range []string{"short", "full"} {
			t.Run(fmt.Sprintf("%dcols_%s", width, fill), func(t *testing.T) {
				line := strings.Repeat("x", 20)
				if fill == "full" {
					line = strings.Repeat("x", width)
				}
				const lines = vt.DefaultScrollbackSize
				before := heap()
				emu := vt.NewEmulator(width, 24)
				for range lines + 24 {
					_, _ = emu.WriteString(line + "\r\n")
				}
				after := heap()
				if got := emu.ScrollbackLen(); got != lines {
					t.Fatalf("scrollback holds %d lines, want %d", got, lines)
				}
				perLine := float64(after-before) / float64(lines+24)
				t.Logf("width %d, %s lines: %.0f bytes per line, %.1f MiB for %d lines",
					width, fill, perLine, float64(after-before)/(1<<20), lines)
				runtime.KeepAlive(emu)
			})
		}
	}
}

func heap() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// TestWideCharWidths reports how many cells vt gives each sample. Compare the
// result with what the outer terminal draws for the same text.
func TestWideCharWidths(t *testing.T) {
	samples := []struct{ name, text string }{
		{"ascii", "abc"},
		{"cjk", "漢字"},
		{"emoji", "😀"},
		{"emoji skin tone", "👍🏽"},
		{"emoji zwj family", "👨‍👩‍👧"},
		{"flag", "🇨🇦"},
		{"combining accent", "é"},
		{"arrow", "→"},
		{"ellipsis", "…"},
		{"check mark", "✓"},
		{"warning text style", "⚠"},
		{"warning emoji style", "⚠️"},
		{"box drawing", "┌─┐"},
	}
	t.Logf("%-20s %-8s %s", "sample", "cells", "text")
	for _, s := range samples {
		emu := vt.NewEmulator(80, 4)
		_, _ = emu.WriteString(s.text)
		t.Logf("%-20s %-8d %s", s.name, emu.CursorPosition().X, s.text)
	}
}
