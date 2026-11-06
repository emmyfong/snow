package session

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/listpick"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

type runtimeControls interface {
	Chat() *api.ChatSnapshot
	RuntimeInput(kind, id, run, text string) *api.ChatInput
	SendNowInput() *api.ChatInput
}

func (m *Model) openRuntimeControls(id int) tea.Cmd {
	controls, ok := m.own[id].(runtimeControls)
	if !ok || controls.Chat().Runtime == nil {
		return m.ShowError("Run controls require a Claude Code chat.")
	}
	snap := controls.Chat()
	generation := snap.Agent
	pick := listpick.New("Claude Code run controls", []ui.Hint{{Key: "Enter", Desc: "choose"}, {Key: "Esc", Desc: "back"}})
	pick.SetDetail("Stop a task with its descendants. Its parent and siblings continue. Stop the run to end all owned work. Queued drafts stay paused after stop.")
	var rows [][]string
	var actions []*api.ChatInput
	add := func(label, detail, kind, task, run string) {
		rows = append(rows, []string{label, detail})
		actions = append(actions, controls.RuntimeInput(kind, task, run, ""))
	}
	if snap.Running && snap.State != api.ChatStateStopping {
		add("Stop run", "all owned tasks", api.ChatInputInterrupt, "", "")
		add("Interrupt and send typed message", "keeps other drafts paused", api.ChatInputSendNow, "", "")
	}
	if !snap.Running && len(snap.Queue) > 0 {
		add("Send queued drafts", fmt.Sprintf("%d queued", len(snap.Queue)), api.ChatInputResumeQueue, "", "")
	}
	for _, task := range snap.Runtime.Tasks {
		switch task.State {
		case "completed", "failed", "killed", "stopped", "stopping":
			continue
		}
		detail := task.State
		if task.Run == "" {
			detail = "stop unavailable: no run identity"
		}
		add("Stop task: "+ui.PlainLine(task.Description), detail, api.ChatInputStopTask, task.ID, task.Run)
	}
	pick.SetRows(rows)
	pick.SetNote("No run or queued drafts. Send a message to begin.")
	return m.showOverlay(&overlay{pane: id, popup: pick, key: pick.Key, click: pick.Click,
		done: func(result picker.Result) tea.Cmd {
			if result != picker.Accepted || pick.Selected() < 0 {
				return nil
			}
			if controls.Chat().Agent != generation {
				return m.ShowError("The chat changed. Open run controls again.")
			}
			in := actions[pick.Selected()]
			if in.Kind == api.ChatInputStopTask && in.Run == "" {
				return m.ShowError("Claude did not identify this task run. Stop the parent run instead.")
			}
			if in.Kind == api.ChatInputSendNow {
				in = controls.SendNowInput()
				if in == nil {
					return m.ShowError("Type a message before choosing Interrupt and send.")
				}
			}
			m.send(in)
			return nil
		}}, "run controls")
}

// runtimeApprovalMode keeps unsupported grant modes out of the native picker.
func (m *Model) runtimeApprovalMode(id int) bool {
	chooser, ok := m.own[id].(runtimeChooser)
	return ok && chooser.RuntimeInfo() != nil
}
