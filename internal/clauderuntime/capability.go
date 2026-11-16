package clauderuntime

// ReferenceVersion is the Claude Code version used by the protocol fixtures.
// It is a research reference, not a validated or certified minimum.
const ReferenceVersion = "2.1.312"

// Capabilities lists the Claude Code controls Snow sends or reads.
type Capabilities struct {
	Version     string
	Interrupt   string
	StopTask    string
	Initialize  string
	Permissions string
}

// Supported reports the controls implemented by this package.
func Supported() Capabilities {
	return Capabilities{
		Version: ReferenceVersion, Interrupt: controlInterrupt, StopTask: controlStopTask,
		Initialize: controlInitialize, Permissions: permissionRequestSubtype,
	}
}
