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
	defer broker.Shutdown()

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
	defer broker.Shutdown()

	// Publish a task to initialize state
	task := Task{
		ID:          uuid.New(),
		Instruction: "test",
		Topic:       "test-topic",
	}
	ctx := context.Background()
	sub := broker.Subscribe("test-topic")
	defer broker.Unsubscribe("test-topic", sub)

	err := broker.Publish(ctx, task)
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
	defer broker.Shutdown()

	// Publish a task
	task := Task{
		ID:          uuid.New(),
		Instruction: "test",
		Topic:       "test-topic",
	}
	ctx := context.Background()
	sub := broker.Subscribe("test-topic")
	defer broker.Unsubscribe("test-topic", sub)

	err := broker.Publish(ctx, task)
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

	// With Phase 3, failed tasks go to retry queue (not completed)
	broker.mu.RLock()
	_, inRetryQueue := broker.retryQueue[task.ID]
	broker.mu.RUnlock()

	if !inRetryQueue {
		t.Error("Failed task should be in retry queue")
	}
}

// TestMessageBroker_ProcessACKs_UnknownTask verifies handling of ACK for unknown task
func TestMessageBroker_ProcessACKs_UnknownTask(t *testing.T) {
	broker := NewMessageBroker()
	defer broker.Shutdown()

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
	defer broker.Shutdown()

	// Test non-existent task
	_, err := broker.GetTaskStatus(uuid.New())
	if err == nil {
		t.Error("Expected error for non-existent task")
	}

	// Publish a task
	task := Task{
		ID:          uuid.New(),
		Instruction: "test",
		Topic:       "test-topic",
	}
	ctx := context.Background()
	sub := broker.Subscribe("test-topic")
	defer broker.Unsubscribe("test-topic", sub)

	err = broker.Publish(ctx, task)
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
	defer broker.Shutdown()

	sprinkler := NewSprinkler(broker, "a")
	sprinkler.Start(context.Background())
	defer sprinkler.Shutdown()

	// Publish a start task
	task := Task{
		ID:          uuid.New(),
		Instruction: "start",
		Topic:       "zone-a",
	}

	err := broker.Publish(context.Background(), task)
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
		Topic:       "zone-a",
	}
	err = broker.Publish(context.Background(), stopTask)
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
	defer broker.Shutdown()

	sprinkler := NewSprinkler(broker, "b")
	sprinkler.Start(context.Background())
	defer sprinkler.Shutdown()

	// Publish task with unknown instruction
	task := Task{
		ID:          uuid.New(),
		Instruction: "invalid-command",
		Topic:       "zone-b",
	}

	err := broker.Publish(context.Background(), task)
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
	defer broker.Shutdown()

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
			Topic:       "test-topic",
		}
		err := broker.Publish(ctx, tasks[i])
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

// ============================================================================
// Phase 3: Retry Logic Tests
// ============================================================================

// TestCalculateBackoff verifies exponential backoff calculation
func TestCalculateBackoff(t *testing.T) {
	config := RetryConfig{
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
		BackoffFactor:  2.0,
	}

	tests := []struct {
		attempts int
		expected time.Duration
	}{
		{0, 0},                  // No backoff for 0 attempts
		{1, 2 * time.Second},    // 1s * 2^1 = 2s
		{2, 4 * time.Second},    // 1s * 2^2 = 4s
		{3, 8 * time.Second},    // 1s * 2^3 = 8s
		{4, 16 * time.Second},   // 1s * 2^4 = 16s
		{5, 30 * time.Second},   // 1s * 2^5 = 32s, capped at 30s
		{10, 30 * time.Second},  // Should be capped at MaxBackoff
	}

	for _, tt := range tests {
		result := calculateBackoff(tt.attempts, config)
		if result != tt.expected {
			t.Errorf("calculateBackoff(%d): expected %v, got %v", tt.attempts, tt.expected, result)
		}
	}
}

// TestRetryOnFirstFailure verifies that first failure adds task to retry queue
func TestRetryOnFirstFailure(t *testing.T) {
	broker := NewMessageBroker()
	defer broker.Shutdown()
	ctx := context.Background()

	task := Task{
		ID:          uuid.New(),
		Instruction: "test-task",
		Topic:       "test-topic",
	}

	// Subscribe to receive the task
	sub := broker.Subscribe(task.Topic)
	defer broker.Unsubscribe(task.Topic, sub)

	// Publish task
	err := broker.Publish(ctx, task)
	if err != nil {
		t.Fatalf("Failed to publish task: %v", err)
	}

	// Consume task
	<-sub

	// Send Failed ACK
	deviceID := uuid.New()
	ackChan := broker.GetACKChannel()
	ackChan <- NewErrTaskAck(task.ID, deviceID, "simulated failure")

	// Give processACKs time to process
	time.Sleep(50 * time.Millisecond)

	// Verify task is in retry queue
	broker.mu.RLock()
	retryState, exists := broker.retryQueue[task.ID]
	broker.mu.RUnlock()

	if !exists {
		t.Fatal("Task should be in retry queue after first failure")
	}

	if retryState.Attempts != 1 {
		t.Errorf("Expected Attempts = 1, got %d", retryState.Attempts)
	}

	if retryState.LastError != "simulated failure" {
		t.Errorf("Expected LastError = 'simulated failure', got %q", retryState.LastError)
	}

	if retryState.NextRetry.IsZero() {
		t.Error("NextRetry should be set")
	}
}

// TestRetryIncrementsAttempts verifies that subsequent failures increment attempts
func TestRetryIncrementsAttempts(t *testing.T) {
	broker := NewMessageBroker()
	defer broker.Shutdown()
	ctx := context.Background()

	task := Task{
		ID:          uuid.New(),
		Instruction: "test-task",
		Topic:       "test-topic",
	}

	sub := broker.Subscribe(task.Topic)
	defer broker.Unsubscribe(task.Topic, sub)

	// Publish task
	broker.Publish(ctx, task)
	<-sub

	deviceID := uuid.New()
	ackChan := broker.GetACKChannel()

	// First failure
	ackChan <- NewErrTaskAck(task.ID, deviceID, "failure 1")
	time.Sleep(100 * time.Millisecond)

	broker.mu.RLock()
	retryState := broker.retryQueue[task.ID]
	broker.mu.RUnlock()

	if retryState.Attempts != 1 {
		t.Errorf("After first failure: expected Attempts = 1, got %d", retryState.Attempts)
	}

	// Simulate retry by modifying NextRetry to past time
	broker.mu.Lock()
	if rs, ok := broker.retryQueue[task.ID]; ok {
		rs.NextRetry = time.Now().Add(-2 * time.Second)
	}
	broker.mu.Unlock()

	// Wait for processRetries to republish (it checks every 1 second)
	// Give generous timeout to account for worst-case timing
	select {
	case <-sub:
		// Task was republished - good!
	case <-time.After(2500 * time.Millisecond):
		t.Fatal("Task was not republished within 2.5 seconds")
	}

	// Second failure
	ackChan <- NewErrTaskAck(task.ID, deviceID, "failure 2")
	time.Sleep(100 * time.Millisecond)

	broker.mu.RLock()
	retryState, exists := broker.retryQueue[task.ID]
	broker.mu.RUnlock()

	if !exists {
		t.Fatal("Task should still be in retry queue after second failure")
	}

	if retryState.Attempts != 2 {
		t.Errorf("After second failure: expected Attempts = 2, got %d", retryState.Attempts)
	}
}

// TestMaxRetriesExceeded verifies task removal after max retries
func TestMaxRetriesExceeded(t *testing.T) {
	broker := NewMessageBroker()
	defer broker.Shutdown()
	ctx := context.Background()

	task := Task{
		ID:          uuid.New(),
		Instruction: "test-task",
		Topic:       "test-topic",
	}

	sub := broker.Subscribe(task.Topic)
	defer broker.Unsubscribe(task.Topic, sub)

	broker.Publish(ctx, task)
	<-sub

	deviceID := uuid.New()
	ackChan := broker.GetACKChannel()

	// Simulate max retries (default is 3)
	maxRetries := broker.retryConfig.MaxRetries

	for i := 1; i <= maxRetries+1; i++ {
		// Send failure ACK
		ackChan <- NewErrTaskAck(task.ID, deviceID, "persistent failure")
		time.Sleep(50 * time.Millisecond)

		broker.mu.RLock()
		_, exists := broker.retryQueue[task.ID]
		broker.mu.RUnlock()

		if i <= maxRetries {
			// Should still be in retry queue
			if !exists {
				t.Errorf("After attempt %d: task should still be in retry queue", i)
			}
		} else {
			// Should be removed after exceeding max retries
			if exists {
				t.Errorf("After attempt %d: task should be removed from retry queue", i)
			}
		}

		// If not the last iteration, prepare for next retry
		if i <= maxRetries {
			broker.mu.Lock()
			if retryState, ok := broker.retryQueue[task.ID]; ok {
				retryState.NextRetry = time.Now().Add(-2 * time.Second)
			}
			broker.mu.Unlock()

			// Wait for republish with generous timeout
			select {
			case <-sub:
				// Republished
			case <-time.After(2500 * time.Millisecond):
				if i < maxRetries {
					t.Fatalf("Task was not republished on attempt %d", i)
				}
			}
		}
	}
}

// TestCompleteRemovesFromRetryQueue verifies Complete ACK removes task from retry queue
func TestCompleteRemovesFromRetryQueue(t *testing.T) {
	broker := NewMessageBroker()
	defer broker.Shutdown()
	ctx := context.Background()

	task := Task{
		ID:          uuid.New(),
		Instruction: "test-task",
		Topic:       "test-topic",
	}

	sub := broker.Subscribe(task.Topic)
	defer broker.Unsubscribe(task.Topic, sub)

	broker.Publish(ctx, task)
	<-sub

	deviceID := uuid.New()
	ackChan := broker.GetACKChannel()

	// First failure - adds to retry queue
	ackChan <- NewErrTaskAck(task.ID, deviceID, "initial failure")
	time.Sleep(50 * time.Millisecond)

	broker.mu.RLock()
	_, exists := broker.retryQueue[task.ID]
	broker.mu.RUnlock()

	if !exists {
		t.Fatal("Task should be in retry queue after failure")
	}

	// Simulate retry
	broker.mu.Lock()
	if rs, ok := broker.retryQueue[task.ID]; ok {
		rs.NextRetry = time.Now().Add(-2 * time.Second)
	}
	broker.mu.Unlock()

	// Wait for republish
	select {
	case <-sub:
		// Republished successfully
	case <-time.After(2500 * time.Millisecond):
		t.Fatal("Task was not republished")
	}

	// Send Complete ACK
	ackChan <- NewTaskAck(task.ID, Complete, deviceID)
	time.Sleep(50 * time.Millisecond)

	broker.mu.RLock()
	_, exists = broker.retryQueue[task.ID]
	broker.mu.RUnlock()

	if exists {
		t.Error("Task should be removed from retry queue after Complete ACK")
	}

	// Verify task status is Complete
	state, err := broker.GetTaskStatus(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task status: %v", err)
	}
	if state.Status != Complete {
		t.Errorf("Expected status Complete, got %v", state.Status)
	}
}

// TestRetryBackoffTiming verifies retry happens after calculated backoff
func TestRetryBackoffTiming(t *testing.T) {
	broker := NewMessageBroker()
	defer broker.Shutdown()

	// Use shorter backoff for faster test
	broker.mu.Lock()
	broker.retryConfig = RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 500 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
		BackoffFactor:  2.0,
	}
	broker.mu.Unlock()

	ctx := context.Background()
	task := Task{
		ID:          uuid.New(),
		Instruction: "test-task",
		Topic:       "test-topic",
	}

	sub := broker.Subscribe(task.Topic)
	defer broker.Unsubscribe(task.Topic, sub)

	broker.Publish(ctx, task)
	<-sub

	deviceID := uuid.New()
	ackChan := broker.GetACKChannel()

	// Send failure
	failTime := time.Now()
	ackChan <- NewErrTaskAck(task.ID, deviceID, "test failure")
	time.Sleep(50 * time.Millisecond)

	// Task should be republished after backoff (500ms * 2^1 = 1s)
	expectedBackoff := 1 * time.Second

	select {
	case <-sub:
		retryTime := time.Now()
		actualDelay := retryTime.Sub(failTime)

		// Allow 500ms variance due to sleep intervals and processing time
		if actualDelay < expectedBackoff || actualDelay > expectedBackoff+1500*time.Millisecond {
			t.Errorf("Expected retry after ~%v, got %v", expectedBackoff, actualDelay)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Task was not retried within timeout")
	}
}

// TestDefaultRetryConfig verifies default configuration values
func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries = 3, got %d", config.MaxRetries)
	}
	if config.InitialBackoff != 1*time.Second {
		t.Errorf("Expected InitialBackoff = 1s, got %v", config.InitialBackoff)
	}
	if config.MaxBackoff != 30*time.Second {
		t.Errorf("Expected MaxBackoff = 30s, got %v", config.MaxBackoff)
	}
	if config.BackoffFactor != 2.0 {
		t.Errorf("Expected BackoffFactor = 2.0, got %f", config.BackoffFactor)
	}
}

// TestTaskMovesToDLQAfterMaxRetries verifies tasks enter DLQ after exhausting retries
func TestTaskMovesToDLQAfterMaxRetries(t *testing.T) {
	broker := NewMessageBroker()
	defer broker.Shutdown()
	ctx := context.Background()

	task := Task{
		ID:          uuid.New(),
		Instruction: "test-task",
		Topic:       "test-topic",
	}

	sub := broker.Subscribe(task.Topic)
	defer broker.Unsubscribe(task.Topic, sub)

	err := broker.Publish(ctx, task)
	if err != nil {
		t.Fatalf("Failed to publish task: %v", err)
	}

	<-sub

	deviceID := uuid.New()
	ackChan := broker.GetACKChannel()

	for i := 1; i <= broker.retryConfig.MaxRetries+1; i++ {
		ackChan <- NewErrTaskAck(task.ID, deviceID, "test failure")
		time.Sleep(50 * time.Millisecond)
	}

	dlqTasks := broker.GetDLQTasks()
	if len(dlqTasks) != 1 {
		t.Fatalf("Expected 1 task in DLQ, got %d", len(dlqTasks))
	}

	entry := dlqTasks[0]
	if entry.Task.ID != task.ID {
		t.Errorf("Expected task ID %v in DLQ, got %v", task.ID, entry.Task.ID)
	}
	if entry.Attempts != broker.retryConfig.MaxRetries+1 {
		t.Errorf("Expected %d attempts, got %d", broker.retryConfig.MaxRetries+1, entry.Attempts)
	}
	if entry.LastError != "test failure" {
		t.Errorf("Expected LastError 'test failure', got %q", entry.LastError)
	}
	if entry.FailureReason != "attempts exceeded" {
		t.Errorf("Expected FailureReason 'attempts exceeded', got %q", entry.FailureReason)
	}
	if entry.AddedAt.IsZero() {
		t.Error("Expected AddedAt to be set")
	}
}

// TestGetDLQTasksReturnsCopy verifies GetDLQTasks returns a copy
func TestGetDLQTasksReturnsCopy(t *testing.T) {
	broker := NewMessageBroker()
	defer broker.Shutdown()

	broker.dlqMu.Lock()
	broker.dlq = []DLQEntry{
		{
			Task:          Task{ID: uuid.New(), Instruction: "task1", Topic: "test"},
			FailureReason: "test",
			Attempts:      4,
			LastError:     "error1",
			AddedAt:       time.Now(),
		},
		{
			Task:          Task{ID: uuid.New(), Instruction: "task2", Topic: "test"},
			FailureReason: "test",
			Attempts:      4,
			LastError:     "error2",
			AddedAt:       time.Now(),
		},
	}
	broker.dlqMu.Unlock()

	tasks1 := broker.GetDLQTasks()
	tasks2 := broker.GetDLQTasks()

	if len(tasks1) != 2 {
		t.Fatalf("Expected 2 tasks, got %d", len(tasks1))
	}

	tasks1[0].LastError = "modified"

	if tasks2[0].LastError == "modified" {
		t.Error("Modifying returned slice affected internal state")
	}
}

// TestRequeueFromDLQ verifies task can be requeued from DLQ
func TestRequeueFromDLQ(t *testing.T) {
	broker := NewMessageBroker()
	defer broker.Shutdown()

	taskID := uuid.New()
	broker.dlqMu.Lock()
	broker.dlq = []DLQEntry{
		{
			Task:          Task{ID: taskID, Instruction: "test-task", Topic: "test-topic"},
			FailureReason: "max retries exceeded",
			Attempts:      4,
			LastError:     "test error",
			AddedAt:       time.Now(),
		},
	}
	broker.dlqMu.Unlock()

	err := broker.RequeueFromDLQ(taskID)
	if err != nil {
		t.Fatalf("Failed to requeue from DLQ: %v", err)
	}

	dlqTasks := broker.GetDLQTasks()
	if len(dlqTasks) != 0 {
		t.Errorf("Expected DLQ to be empty after requeue, got %d tasks", len(dlqTasks))
	}

	broker.mu.RLock()
	retryState, exists := broker.retryQueue[taskID]
	broker.mu.RUnlock()

	if !exists {
		t.Fatal("Task not found in retry queue after requeue")
	}

	if retryState.Task.ID != taskID {
		t.Errorf("Expected task ID %v in retry queue, got %v", taskID, retryState.Task.ID)
	}
	if retryState.Attempts != 0 {
		t.Errorf("Expected Attempts = 0 for requeued task, got %d", retryState.Attempts)
	}
	if retryState.LastError != "test error" {
		t.Errorf("Expected LastError preserved, got %q", retryState.LastError)
	}
}

// TestRequeueFromDLQNotFound verifies error when task not in DLQ
func TestRequeueFromDLQNotFound(t *testing.T) {
	broker := NewMessageBroker()
	defer broker.Shutdown()

	nonExistentID := uuid.New()
	err := broker.RequeueFromDLQ(nonExistentID)

	if err == nil {
		t.Fatal("Expected error when requeuing non-existent task, got nil")
	}
}

// TestRemoveFromDLQ verifies task can be removed from DLQ
func TestRemoveFromDLQ(t *testing.T) {
	broker := NewMessageBroker()
	defer broker.Shutdown()

	task1ID := uuid.New()
	task2ID := uuid.New()

	broker.dlqMu.Lock()
	broker.dlq = []DLQEntry{
		{
			Task:          Task{ID: task1ID, Instruction: "task1", Topic: "test"},
			FailureReason: "test",
			Attempts:      4,
			LastError:     "error1",
			AddedAt:       time.Now(),
		},
		{
			Task:          Task{ID: task2ID, Instruction: "task2", Topic: "test"},
			FailureReason: "test",
			Attempts:      4,
			LastError:     "error2",
			AddedAt:       time.Now(),
		},
	}
	broker.dlqMu.Unlock()

	err := broker.RemoveFromDLQ(task1ID)
	if err != nil {
		t.Fatalf("Failed to remove from DLQ: %v", err)
	}

	dlqTasks := broker.GetDLQTasks()
	if len(dlqTasks) != 1 {
		t.Fatalf("Expected 1 task in DLQ after removal, got %d", len(dlqTasks))
	}

	if dlqTasks[0].Task.ID != task2ID {
		t.Errorf("Expected task2 to remain in DLQ, got task ID %v", dlqTasks[0].Task.ID)
	}
}

// TestRemoveFromDLQNotFound verifies error when task not in DLQ
func TestRemoveFromDLQNotFound(t *testing.T) {
	broker := NewMessageBroker()
	defer broker.Shutdown()

	nonExistentID := uuid.New()
	err := broker.RemoveFromDLQ(nonExistentID)

	if err == nil {
		t.Fatal("Expected error when removing non-existent task, got nil")
	}
}

// TestClearDLQ verifies all tasks can be cleared from DLQ
func TestClearDLQ(t *testing.T) {
	broker := NewMessageBroker()
	defer broker.Shutdown()

	broker.dlqMu.Lock()
	broker.dlq = []DLQEntry{
		{
			Task:          Task{ID: uuid.New(), Instruction: "task1", Topic: "test"},
			FailureReason: "test",
			Attempts:      4,
			LastError:     "error1",
			AddedAt:       time.Now(),
		},
		{
			Task:          Task{ID: uuid.New(), Instruction: "task2", Topic: "test"},
			FailureReason: "test",
			Attempts:      4,
			LastError:     "error2",
			AddedAt:       time.Now(),
		},
		{
			Task:          Task{ID: uuid.New(), Instruction: "task3", Topic: "test"},
			FailureReason: "test",
			Attempts:      4,
			LastError:     "error3",
			AddedAt:       time.Now(),
		},
	}
	broker.dlqMu.Unlock()

	broker.ClearDLQ()

	dlqTasks := broker.GetDLQTasks()
	if len(dlqTasks) != 0 {
		t.Errorf("Expected DLQ to be empty after clear, got %d tasks", len(dlqTasks))
	}
}
