package fleet

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestNewTaskAck verifies the TaskAck helper creates correct acknowledgments
func TestNewTaskAck(t *testing.T) {
	taskID := uuid.New()
	deviceID := uuid.New()
	status := Running

	ack := NewTaskAck(taskID, status, deviceID)

	if ack.TaskID != taskID {
		t.Errorf("Expected TaskID %v, got %v", taskID, ack.TaskID)
	}
	if ack.Status != status {
		t.Errorf("Expected Status %v, got %v", status, ack.Status)
	}
	if ack.DeviceID != deviceID {
		t.Errorf("Expected DeviceID %v, got %v", deviceID, ack.DeviceID)
	}
	if ack.Error != "" {
		t.Errorf("Expected empty Error, got %v", ack.Error)
	}
	if time.Since(ack.Timestamp) > time.Second {
		t.Error("Timestamp should be recent")
	}
}

// TestNewErrTaskAck verifies error acknowledgment creation
func TestNewErrTaskAck(t *testing.T) {
	taskID := uuid.New()
	deviceID := uuid.New()
	errorMsg := "connection timeout"

	ack := NewErrTaskAck(taskID, deviceID, errorMsg)

	if ack.Status != Failed {
		t.Errorf("Expected Status Failed, got %v", ack.Status)
	}
	if ack.Error != errorMsg {
		t.Errorf("Expected Error %q, got %q", errorMsg, ack.Error)
	}
	if ack.TaskID != taskID {
		t.Errorf("Expected TaskID %v, got %v", taskID, ack.TaskID)
	}
}

// TestMessageBroker_GetACKChannel verifies ACK channel is accessible
func TestMessageBroker_GetACKChannel(t *testing.T) {
	broker := NewMessageBroker()
	defer close(broker.ackChan)

	ackChan := broker.GetACKChannel()

	if ackChan == nil {
		t.Fatal("ACK channel should not be nil")
	}

	// Verify we can send to the channel
	testAck := NewTaskAck(uuid.New(), Running, uuid.New())
	select {
	case ackChan <- testAck:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("Should be able to send ACK to channel")
	}
}

// TestMessageBroker_ProcessACKs verifies ACK processing updates task state
func TestMessageBroker_ProcessACKs(t *testing.T) {
	broker := NewMessageBroker()
	defer close(broker.ackChan)

	// Publish a task to initialize state
	task := Task{
		ID:          uuid.New(),
		Instruction: "test",
	}
	ctx := context.Background()
	sub := broker.Subscribe("test-topic")
	defer broker.Unsubscribe("test-topic", sub)

	err := broker.Publish(ctx, "test-topic", task)
	if err != nil {
		t.Fatalf("Failed to publish task: %v", err)
	}

	// Drain the subscriber channel
	select {
	case <-sub:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Task should be received by subscriber")
	}

	deviceID := uuid.New()
	ackChan := broker.GetACKChannel()

	// Send Running ACK
	runningAck := NewTaskAck(task.ID, Running, deviceID)
	ackChan <- runningAck

	// Give processACKs time to process
	time.Sleep(50 * time.Millisecond)

	// Verify state was updated
	state, err := broker.GetTaskStatus(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task status: %v", err)
	}
	if state.Status != Running {
		t.Errorf("Expected status Running, got %v", state.Status)
	}
	if state.StartedAt == nil {
		t.Error("StartedAt should be set after Running ACK")
	}
	if state.DeviceID != deviceID.String() {
		t.Errorf("Expected DeviceID %v, got %v", deviceID.String(), state.DeviceID)
	}

	// Send Complete ACK
	completeAck := NewTaskAck(task.ID, Complete, deviceID)
	ackChan <- completeAck

	time.Sleep(50 * time.Millisecond)

	// Verify completion was tracked
	state, err = broker.GetTaskStatus(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task status: %v", err)
	}
	if state.Status != Complete {
		t.Errorf("Expected status Complete, got %v", state.Status)
	}
	if state.CompletedAt == nil {
		t.Error("CompletedAt should be set after Complete ACK")
	}
}

// TestMessageBroker_ProcessACKs_Failed verifies failed task ACK handling
func TestMessageBroker_ProcessACKs_Failed(t *testing.T) {
	broker := NewMessageBroker()
	defer close(broker.ackChan)

	// Publish a task
	task := Task{
		ID:          uuid.New(),
		Instruction: "test",
	}
	ctx := context.Background()
	sub := broker.Subscribe("test-topic")
	defer broker.Unsubscribe("test-topic", sub)

	err := broker.Publish(ctx, "test-topic", task)
	if err != nil {
		t.Fatalf("Failed to publish task: %v", err)
	}

	// Drain subscriber
	<-sub

	deviceID := uuid.New()
	ackChan := broker.GetACKChannel()

	// Send Failed ACK with error
	failedAck := NewErrTaskAck(task.ID, deviceID, "motor malfunction")
	ackChan <- failedAck

	time.Sleep(50 * time.Millisecond)

	// Verify failure was tracked
	state, err := broker.GetTaskStatus(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task status: %v", err)
	}
	if state.Status != Failed {
		t.Errorf("Expected status Failed, got %v", state.Status)
	}
	if state.CompletedAt == nil {
		t.Error("CompletedAt should be set after Failed ACK")
	}
}

// TestMessageBroker_ProcessACKs_UnknownTask verifies handling of ACK for unknown task
func TestMessageBroker_ProcessACKs_UnknownTask(t *testing.T) {
	broker := NewMessageBroker()
	defer close(broker.ackChan)

	unknownTaskID := uuid.New()
	deviceID := uuid.New()
	ackChan := broker.GetACKChannel()

	// Send ACK for task that was never published
	ack := NewTaskAck(unknownTaskID, Complete, deviceID)
	ackChan <- ack

	time.Sleep(50 * time.Millisecond)

	// Should not panic or error, just log warning
	_, err := broker.GetTaskStatus(unknownTaskID)
	if err == nil {
		t.Error("Expected error for unknown task ID")
	}
}

// TestMessageBroker_GetTaskStatus verifies task status retrieval
func TestMessageBroker_GetTaskStatus(t *testing.T) {
	broker := NewMessageBroker()
	defer close(broker.ackChan)

	// Test non-existent task
	_, err := broker.GetTaskStatus(uuid.New())
	if err == nil {
		t.Error("Expected error for non-existent task")
	}

	// Publish a task
	task := Task{
		ID:          uuid.New(),
		Instruction: "test",
	}
	ctx := context.Background()
	sub := broker.Subscribe("test-topic")
	defer broker.Unsubscribe("test-topic", sub)

	err = broker.Publish(ctx, "test-topic", task)
	if err != nil {
		t.Fatalf("Failed to publish task: %v", err)
	}

	// Get initial state
	state, err := broker.GetTaskStatus(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task status: %v", err)
	}

	if state.Status != Queued {
		t.Errorf("Expected initial status Queued, got %v", state.Status)
	}
	if state.Task.ID != task.ID {
		t.Errorf("Expected task ID %v, got %v", task.ID, state.Task.ID)
	}
	if state.PublishedAt.IsZero() {
		t.Error("PublishedAt should be set")
	}
	if state.StartedAt != nil {
		t.Error("StartedAt should be nil initially")
	}
	if state.CompletedAt != nil {
		t.Error("CompletedAt should be nil initially")
	}
}

// TestSprinkler_ACK_Lifecycle is an integration test verifying the full ACK flow
func TestSprinkler_ACK_Lifecycle(t *testing.T) {
	broker := NewMessageBroker()
	defer close(broker.ackChan)

	sprinkler := NewSprinkler(broker, "zone-a")
	sprinkler.Start(context.Background())
	defer sprinkler.Shutdown()

	// Publish a start task
	task := Task{
		ID:          uuid.New(),
		Instruction: "start",
	}

	err := broker.Publish(context.Background(), "irrigation-zone-a", task)
	if err != nil {
		t.Fatalf("Failed to publish task: %v", err)
	}

	// Wait for Running ACK
	time.Sleep(100 * time.Millisecond)

	state, err := broker.GetTaskStatus(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task status: %v", err)
	}
	if state.Status != Running {
		t.Errorf("Expected status Running, got %v", state.Status)
	}
	if state.StartedAt == nil {
		t.Error("StartedAt should be set")
	}

	// Send stop task
	stopTask := Task{
		ID:          uuid.New(),
		Instruction: "stop",
	}
	err = broker.Publish(context.Background(), "irrigation-zone-a", stopTask)
	if err != nil {
		t.Fatalf("Failed to publish stop task: %v", err)
	}

	// Wait for Complete ACK
	time.Sleep(100 * time.Millisecond)

	stopState, err := broker.GetTaskStatus(stopTask.ID)
	if err != nil {
		t.Fatalf("Failed to get stop task status: %v", err)
	}
	if stopState.Status != Complete {
		t.Errorf("Expected stop task status Complete, got %v", stopState.Status)
	}
}

// TestSprinkler_ACK_UnknownInstruction verifies failed ACK for unknown instruction
func TestSprinkler_ACK_UnknownInstruction(t *testing.T) {
	broker := NewMessageBroker()
	defer close(broker.ackChan)

	sprinkler := NewSprinkler(broker, "zone-b")
	sprinkler.Start(context.Background())
	defer sprinkler.Shutdown()

	// Publish task with unknown instruction
	task := Task{
		ID:          uuid.New(),
		Instruction: "invalid-command",
	}

	err := broker.Publish(context.Background(), "irrigation-zone-b", task)
	if err != nil {
		t.Fatalf("Failed to publish task: %v", err)
	}

	// Wait for Failed ACK
	time.Sleep(100 * time.Millisecond)

	state, err := broker.GetTaskStatus(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task status: %v", err)
	}
	if state.Status != Failed {
		t.Errorf("Expected status Failed, got %v", state.Status)
	}
}

// TestMessageBroker_ConcurrentACKs verifies thread-safe ACK processing
func TestMessageBroker_ConcurrentACKs(t *testing.T) {
	broker := NewMessageBroker()
	defer close(broker.ackChan)

	ctx := context.Background()
	sub := broker.Subscribe("test-topic")
	defer broker.Unsubscribe("test-topic", sub)

	// Publish multiple tasks
	numTasks := 10
	tasks := make([]Task, numTasks)
	for i := 0; i < numTasks; i++ {
		tasks[i] = Task{
			ID:          uuid.New(),
			Instruction: "test",
		}
		err := broker.Publish(ctx, "test-topic", tasks[i])
		if err != nil {
			t.Fatalf("Failed to publish task %d: %v", i, err)
		}
	}

	// Drain subscriber
	for i := 0; i < numTasks; i++ {
		<-sub
	}

	ackChan := broker.GetACKChannel()
	deviceID := uuid.New()

	// Send ACKs concurrently
	for i := 0; i < numTasks; i++ {
		go func(task Task) {
			ackChan <- NewTaskAck(task.ID, Running, deviceID)
			time.Sleep(10 * time.Millisecond)
			ackChan <- NewTaskAck(task.ID, Complete, deviceID)
		}(tasks[i])
	}

	// Wait for all ACKs to process
	time.Sleep(200 * time.Millisecond)

	// Verify all tasks reached Complete status
	for i, task := range tasks {
		state, err := broker.GetTaskStatus(task.ID)
		if err != nil {
			t.Errorf("Failed to get status for task %d: %v", i, err)
			continue
		}
		if state.Status != Complete {
			t.Errorf("Task %d: expected Complete, got %v", i, state.Status)
		}
	}
}
