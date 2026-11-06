package clauderuntime

import (
	"fmt"
	"slices"

	"github.com/emmyfong/snow/pkg/api"
)

func (s *conversationState) taskEvent(message runtimeMessage) error {
	if message.TaskID == "" || message.Ambient {
		return nil
	}
	previous, known := s.taskRuns[message.TaskID]
	if known && previous.Run != "" && message.RunID != "" && previous.Run > message.RunID {
		return nil // the runtime documents run IDs as chronologically sortable
	}
	switch message.Subtype {
	case "task_started":
		if known && previous.Run == message.RunID && !terminalTask(previous.State) {
			return nil
		}
		if message.ParentTaskID == message.TaskID {
			return fmt.Errorf("%w: task names itself as its parent", ErrProtocol)
		}
		if !known && len(s.taskRuns) >= 256 {
			for id, task := range s.taskRuns {
				if terminalTask(task.State) {
					delete(s.taskRuns, id)
					break
				}
			}
			if len(s.taskRuns) >= 256 {
				return ErrBackpressure
			}
		}
		s.taskRuns[message.TaskID] = api.RuntimeTask{ID: message.TaskID, Run: message.RunID,
			Parent: message.ParentTaskID, Kind: message.TaskType, Depth: message.SpawnDepth,
			Description: message.Description, State: "running"}
	case "task_updated", "task_notification":
		if !known {
			return nil
		}
		status := message.Status
		if message.Subtype == "task_updated" {
			status = message.Patch.Status
		}
		if status == "" || previous.State == "stopping" && !terminalTask(status) {
			return nil
		}
		previous.State = status
		s.taskRuns[message.TaskID] = previous
	}
	s.publishRuntime()
	return nil
}

func (s *conversationState) publishRuntime() {
	info := *s.snapshot.Runtime
	info.Tasks = make([]api.RuntimeTask, 0, len(s.taskRuns))
	for _, task := range s.taskRuns {
		info.Tasks = append(info.Tasks, task)
	}
	slices.SortFunc(info.Tasks, func(a, b api.RuntimeTask) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	s.emit(api.ChatEvent{Kind: api.ChatEventRuntime, Runtime: &info})
}

func terminalTask(status string) bool {
	switch status {
	case "completed", "failed", "killed", "stopped":
		return true
	default:
		return false
	}
}

func (s *conversationState) stopTask(id, run string) error {
	task, ok := s.taskRuns[id]
	if !ok || task.Run != run || terminalTask(task.State) || s.worker == nil {
		return ErrRequest
	}
	if task.Run == "" {
		return fmt.Errorf("%w: this runtime did not identify the task run; child stop is unavailable", ErrControl)
	}
	for _, active := range s.taskRuns {
		if !terminalTask(active.State) && (active.Depth > 1 && active.Parent == "" || active.Parent != "" && s.taskRuns[active.Parent].ID == "") {
			return fmt.Errorf("%w: Claude did not report enough task ownership to stop descendants safely", ErrControl)
		}
	}
	ids, err := s.descendants(id, map[string]bool{})
	if err != nil {
		return err
	}
	if err := s.worker.submit(operation{kind: "stop-task", id: id, ids: ids}); err != nil {
		return err
	}
	for _, id := range ids {
		task := s.taskRuns[id]
		task.State = "stopping"
		s.taskRuns[id] = task
	}
	s.publishRuntime()
	return nil
}

func (s *conversationState) descendants(id string, visiting map[string]bool) ([]string, error) {
	if visiting[id] {
		return nil, fmt.Errorf("%w: cyclic task ownership", ErrProtocol)
	}
	visiting[id] = true
	var ids []string
	for child, task := range s.taskRuns {
		if task.Parent == id && !terminalTask(task.State) {
			nested, err := s.descendants(child, visiting)
			if err != nil {
				return nil, err
			}
			ids = append(ids, nested...)
		}
	}
	return append(ids, id), nil
}
