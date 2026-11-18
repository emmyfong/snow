//go:build !windows

package client

import "io"

func writeTerminalSequence(out io.Writer, data string) error {
	_, err := io.WriteString(out, data)
	return err
}
