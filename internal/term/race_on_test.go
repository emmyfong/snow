//go:build race

package term

// raceDetector reports a test binary built with -race, which slows the code
// too much for timing budgets.
const raceDetector = true
