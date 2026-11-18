package client

import (
	"errors"
	"io"

	"golang.org/x/sys/windows"
)

func writeTerminalSequence(out io.Writer, data string) error {
	if file, ok := out.(interface{ Fd() uintptr }); ok {
		handle := windows.Handle(file.Fd())
		var mode uint32
		// Bubble Tea restores the Windows console mode when it exits.
		if windows.GetConsoleMode(handle, &mode) == nil && mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING == 0 {
			if err := windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
				return err
			}
			_, err := io.WriteString(out, data)
			return errors.Join(err, windows.SetConsoleMode(handle, mode))
		}
	}
	_, err := io.WriteString(out, data)
	return err
}
