//go:build !windows

package main

const probeEnv = "SNOW_TEST_PROBE"

// runProbe has no roles outside Windows; see console_windows_test.go.
func runProbe(string) int { return 2 }
