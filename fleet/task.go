package fleet

import (
	"time"

	"github.com/google/uuid"
)

type TaskStatus string

const (
	Queued   TaskStatus = "Queued"
	Running  TaskStatus = "Running"
	Complete TaskStatus = "Complete"
	Failed   TaskStatus = "Failed"
)

type Task struct {
	ID          uuid.UUID
	Instruction string
}

type TaskHandler interface {
	HandleTask(task Task)
}

type TaskState struct {
	Task        Task
	PublishedAt time.Time
	ReceivedAt  *time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	Status      TaskStatus
	DeviceID    string
}

type TaskAck struct {
	TaskID    uuid.UUID
	Status    TaskStatus
	DeviceID  uuid.UUID
	Timestamp time.Time
	Error     string
}

func NewTaskAck(tid uuid.UUID, s TaskStatus, did uuid.UUID) TaskAck {
	return TaskAck{
		TaskID:    tid,
		Status:    s,
		DeviceID:  did,
		Timestamp: time.Now(),
	}
}

func NewErrTaskAck(tid uuid.UUID, did uuid.UUID, errorMsg string) TaskAck {
	ta := NewTaskAck(tid, Failed, did)
	ta.Error = errorMsg
	return ta
}
