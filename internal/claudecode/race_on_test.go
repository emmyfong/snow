//go:build race

package claudecode

// raceDetector reports a test binary built with -race, which slows the code
// too much for timing budgets.
const raceDetector = true
