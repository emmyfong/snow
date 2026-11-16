package clauderuntime

import "testing"

func TestCapabilityRecord(t *testing.T) {
	got := Supported()
	want := Capabilities{Version: "2.1.312", Interrupt: controlInterrupt, StopTask: controlStopTask,
		Initialize: controlInitialize, Permissions: permissionRequestSubtype}
	if got != want || got.Version != ReferenceVersion {
		t.Fatalf("Supported() = %+v, want %+v", got, want)
	}
}
