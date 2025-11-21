# Task Acknowledgment and Retry System Spec

## Overview
This spec defines a reliable task processing system with acknowledgments, completion tracking, automatic retries, and dead letter queue (DLQ) for the Fleet pub/sub messaging system.

## Goals
1. **Visibility**: Broker knows when tasks are received, processing, completed, or failed
2. **Reliability**: Failed tasks are automatically retried with exponential backoff
3. **Observability**: Permanently failed tasks are isolated in a DLQ for investigation
4. **Incremental**: Build in stages, each stage adds value independently

## Architecture Overview

```
┌──────────────────────────────────────────────────────────────┐
│                         MessageBroker                        │
│  ┌────────────┐  ┌────────────┐  ┌────────────────────────┐ │
│  │ Task Queue │  │  ACK Chan  │  │  Dead Letter Queue     │ │
│  │            │  │            │  │  (Permanently Failed)  │ │
│  └────────────┘  └────────────┘  └────────────────────────┘ │
│         │              ▲                      ▲              │
│         │ Publish      │ ACK                  │              │
│         ▼              │                      │              │
│  ┌─────────────────────┴──────────────────────┴──────────┐  │
│  │            Retry Manager                              │  │
│  │  - Tracks task attempts                               │  │
│  │  - Exponential backoff                                │  │
│  │  - Max retry limit                                    │  │
│  └───────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
         │                                      ▲
         │ Task                                 │ ACK
         ▼                                      │
┌─────────────────────────────────────────────────────────┐
│                    FleetDevice (Sprinkler)              │
│  ┌──────────────┐           ┌─────────────────────┐    │
│  │   HandleTask │──────────▶│  Send ACK to Broker │    │
│  │              │           │  (Task.Status)      │    │
│  └──────────────┘           └─────────────────────┘    │
└─────────────────────────────────────────────────────────┘
```

## Data Structures

### TaskAck (New)
```go
type TaskAck struct {
    TaskID    uuid.UUID
    Status    TaskStatus  // Reuse existing: Queued, Running, Complete, Failed
    DeviceID  string      // Which device sent this ACK
    Timestamp time.Time
    Error     string      // Error message if Status == Failed
}
```

### TaskRetryState (New)
```go
type TaskRetryState struct {
    Task          Task
    Attempts      int
    MaxRetries    int
    LastAttempt   time.Time
    NextRetry     time.Time
    BackoffFactor time.Duration
}
```

### Updated MessageBroker
```go
type MessageBroker struct {
    subscribers map[string][]Subscriber
    ackChan     chan TaskAck           // NEW: Receive ACKs from devices
    retryQueue  map[uuid.UUID]*TaskRetryState  // NEW: Track retry state
    dlq         []Task                 // NEW: Dead letter queue
    mu          sync.RWMutex
}
```

## Implementation Phases

### Phase 1: ACK/NACK Infrastructure (Foundation)
**Goal**: Enable devices to send acknowledgments back to broker

**Changes Required**:
1. Add `TaskAck` type to `fleet/messaging_models.go`
2. Add `ackChan chan TaskAck` to `MessageBroker`
3. Add `GetACKChannel() chan TaskAck` method to Broker interface
4. Create `processACKs()` goroutine in MessageBroker to handle incoming ACKs
5. Update `Sprinkler.HandleTask()` to send ACKs at key lifecycle points

**Example Usage**:
```go
// In Sprinkler.HandleTask():
ackChan := s.broker.GetACKChannel()

// Send ACK when task starts
ackChan <- TaskAck{
    TaskID:    task.ID,
    Status:    Running,
    DeviceID:  "sprinkler-zone-a",
    Timestamp: time.Now(),
}

// Send ACK when task completes
ackChan <- TaskAck{
    TaskID:    task.ID,
    Status:    Complete,
    DeviceID:  "sprinkler-zone-a",
    Timestamp: time.Now(),
}
```

**Testing**: Verify ACKs are logged by broker

---

### Phase 2: Completion Tracking (Observability) ✅ COMPLETE
**Goal**: Broker tracks task lifecycle from publish to completion

**Changes Required**:
1. Add `taskStates map[uuid.UUID]*TaskState` to MessageBroker
2. Create `TaskState` struct to track task journey
3. Update `Publish()` to initialize task state
4. Update `processACKs()` to update task state based on ACKs
5. Add `GetTaskStatus(taskID uuid.UUID) TaskStatus` query method

**Data Structure**:
```go
type TaskState struct {
    Task         Task
    PublishedAt  time.Time
    ReceivedAt   *time.Time
    StartedAt    *time.Time
    CompletedAt  *time.Time
    Status       TaskStatus
    DeviceID     string
}
```

**Testing**: Verify broker can report task status after publishing

---

### Phase 3: Retry Logic (Reliability)
**Goal**: Automatically retry failed tasks with exponential backoff

**Changes Required**:
1. Add `TaskRetryState` to track retry attempts
2. Add `retryQueue map[uuid.UUID]*TaskRetryState` to MessageBroker
3. Create `processRetries()` goroutine that:
   - Checks retry queue every second
   - Re-publishes tasks that are past their `NextRetry` time
   - Increments attempt counter
   - Calculates next backoff delay
4. Update `processACKs()` to add failed tasks to retry queue
5. Add retry configuration to MessageBroker

**Retry Configuration**:
```go
type RetryConfig struct {
    MaxRetries      int           // Default: 3
    InitialBackoff  time.Duration // Default: 1 second
    MaxBackoff      time.Duration // Default: 30 seconds
    BackoffFactor   float64       // Default: 2.0 (exponential)
}
```

**Exponential Backoff Algorithm**:
```go
nextBackoff = min(
    InitialBackoff * (BackoffFactor ^ attempts),
    MaxBackoff
)
```

**Example Timeline**:
- Attempt 1: Fail → Retry after 1s
- Attempt 2: Fail → Retry after 2s
- Attempt 3: Fail → Retry after 4s
- Attempt 4: Fail → Move to DLQ

**Testing**:
- Simulate task failure and verify retry happens
- Verify backoff delays increase exponentially
- Verify max retries is respected

---

### Phase 4: Dead Letter Queue (Failure Handling)
**Goal**: Isolate tasks that fail all retry attempts for manual investigation

**Changes Required**:
1. Add `dlq []Task` to MessageBroker
2. Add `dlqMu sync.RWMutex` for thread-safe DLQ access
3. Update `processACKs()` to move exhausted retries to DLQ
4. Add DLQ query/management methods:
   - `GetDLQTasks() []Task` - View failed tasks
   - `RequeueFromDLQ(taskID uuid.UUID) error` - Retry manually
   - `RemoveFromDLQ(taskID uuid.UUID) error` - Acknowledge failure
   - `ClearDLQ()` - Clear all DLQ tasks

**DLQ Task Structure**:
```go
type DLQEntry struct {
    Task          Task
    FailureReason string
    Attempts      int
    LastError     string
    AddedAt       time.Time
}
```

**Logging**:
- Log when task enters DLQ with full context
- Include all retry attempt errors
- Suggest investigation steps

**Testing**:
- Verify task enters DLQ after max retries
- Verify DLQ can be queried
- Verify manual requeue works

---

## Integration Points

### Broker Interface Updates
```go
type Broker interface {
    // Existing
    Subscribe(topic string) Subscriber
    Unsubscribe(topic string, ch Subscriber)
    Publish(ctx context.Context, topic string, task Task) error

    // NEW: ACK and tracking
    GetACKChannel() chan TaskAck
    GetTaskStatus(taskID uuid.UUID) (TaskStatus, error)

    // NEW: DLQ management
    GetDLQTasks() []DLQEntry
    RequeueFromDLQ(taskID uuid.UUID) error
    RemoveFromDLQ(taskID uuid.UUID) error
}
```

### Device Implementation Requirements

All devices implementing `TaskHandler` should:
1. Send ACK when task starts (Status: Running)
2. Send ACK when task completes (Status: Complete)
3. Send ACK when task fails (Status: Failed, include error message)

**Example in Sprinkler**:
```go
func (s *Sprinkler) HandleTask(task Task) error {
    ackChan := s.broker.GetACKChannel()

    // ACK: Task received and starting
    ackChan <- TaskAck{
        TaskID:    task.ID,
        Status:    Running,
        DeviceID:  s.deviceID,
        Timestamp: time.Now(),
    }

    // ... execute task ...

    // ACK: Task completed or failed
    if err != nil {
        ackChan <- TaskAck{
            TaskID:    task.ID,
            Status:    Failed,
            DeviceID:  s.deviceID,
            Timestamp: time.Now(),
            Error:     err.Error(),
        }
        return err
    }

    ackChan <- TaskAck{
        TaskID:    task.ID,
        Status:    Complete,
        DeviceID:  s.deviceID,
        Timestamp: time.Now(),
    }
    return nil
}
```

## Configuration

### Broker Configuration
```go
type BrokerConfig struct {
    RetryConfig RetryConfig
    DLQEnabled  bool
    ACKTimeout  time.Duration  // How long to wait for ACK before considering task lost
}

func NewMessageBrokerWithConfig(config BrokerConfig) *MessageBroker {
    // ...
}
```

## Monitoring and Observability

### Metrics to Track
- Tasks published
- Tasks acknowledged (by status)
- Tasks in retry queue
- Tasks in DLQ
- Average retry count
- ACK latency (time from publish to first ACK)

### Log Events
```go
// Structured logging examples:
log.Printf("[ACK] Task %s: %s → %s (device: %s)", taskID, oldStatus, newStatus, deviceID)
log.Printf("[RETRY] Task %s: Attempt %d/%d, next retry in %s", taskID, attempt, maxRetries, backoff)
log.Printf("[DLQ] Task %s: Max retries exceeded, moved to DLQ", taskID)
```

## Error Handling

### Edge Cases to Handle
1. **ACK for unknown task**: Log warning, ignore
2. **Duplicate ACKs**: Use latest status update
3. **Out-of-order ACKs**: Status can only progress forward (Running → Complete, not Complete → Running)
4. **Device crashes**: ACK timeout mechanism (Phase 5 - future enhancement)
5. **Broker restarts**: In-memory state lost (Phase 6 - persistence - future enhancement)

## Testing Strategy

### Unit Tests
- [ ] TaskAck serialization/deserialization
- [ ] Retry backoff calculation
- [ ] DLQ add/remove operations
- [ ] Status transition validation

### Integration Tests
- [ ] End-to-end: Publish → ACK → Complete
- [ ] Failure path: Publish → Fail → Retry → Complete
- [ ] DLQ path: Publish → Fail × 4 → DLQ
- [ ] Concurrent ACKs from multiple devices

### Load Tests
- [ ] 1000 tasks/sec with ACKs
- [ ] Retry queue under load
- [ ] DLQ growth rate

## Future Enhancements (Not in this spec)

### Phase 5: ACK Timeout
- Detect when device crashes (no ACK received within timeout)
- Automatically retry or reassign task

### Phase 6: Persistence
- Persist retry queue and DLQ to disk/database
- Survive broker restarts

### Phase 7: Priority Queues
- High-priority tasks jump retry queue

### Phase 8: Metrics Dashboard
- Web UI to visualize task states, retry rates, DLQ

## Implementation Checklist

- [x] **Phase 1**: ACK/NACK Infrastructure ✅ COMPLETE
  - [x] Add TaskAck type (`task.go:37-43`)
  - [x] Add ackChan to MessageBroker (initialized with buffer size 100)
  - [x] Implement processACKs() goroutine (`broker.go:124-156`)
  - [x] Update Sprinkler to send ACKs (Running, Complete, Failed)
  - [x] Created helper functions: `NewTaskAck()` and `NewErrTaskAck()` (`task.go:45-58`)
  - [x] Removed `Task.Status` field (Task is now immutable command)
  - [x] Removed error return from `TaskHandler.HandleTask()`
  - [x] Write tests (9 comprehensive tests in `broker_test.go`)
  - [x] Fixed goroutine lifecycle management (device context + WaitGroup tracking)
  - [x] Fixed topic tracking for proper cleanup (`worker.go:42`)

- [x] **Phase 2**: Completion Tracking ✅ COMPLETE
  - [x] Add TaskState type (`task.go:27-35`)
  - [x] Track task lifecycle in broker (`broker.go:26, 66-72, 133-151`)
  - [x] Implement GetTaskStatus() query (`broker.go:158-167`)
  - [x] Tests included in Phase 1 test suite

- [ ] **Phase 3**: Retry Logic
  - [ ] Add TaskRetryState type
  - [ ] Implement processRetries() goroutine
  - [ ] Exponential backoff algorithm
  - [ ] Integrate with processACKs()
  - [ ] Write tests

- [ ] **Phase 4**: Dead Letter Queue
  - [ ] Add DLQ to broker
  - [ ] Implement DLQ management methods
  - [ ] Move exhausted retries to DLQ
  - [ ] Add DLQ logging
  - [ ] Write tests

## Success Criteria

✅ **Phase 1**: Broker logs show ACKs being received from devices
✅ **Phase 2**: Can query task status and see lifecycle progression
✅ **Phase 3**: Failed tasks automatically retry with increasing backoff
✅ **Phase 4**: Tasks that fail all retries appear in DLQ

## Notes

- Start with Phase 1, validate it works, then move to Phase 2
- Each phase should be fully tested before moving to next
- Phases 1-2 are low complexity, Phases 3-4 are medium complexity
- **Design Decision**: Removed `Task.Status` field - Task is now an immutable command
  - Status exists only in `TaskAck` and `TaskState` (broker tracking)
  - This prevents confusion about "source of truth" for task status
- Leverage existing context cancellation patterns from Step 4
- Helper functions (`NewTaskAck`, `NewErrTaskAck`) prevent common mistakes

### Phase 1 & 2 Implementation Improvements

During test development, critical issues were uncovered and fixed:

1. **Goroutine Lifecycle Management** (`sprinkler.go:64-100`)
   - Fixed: `handleStartTask` goroutines were not tracked by WaitGroup
   - Solution: Track all spawned goroutines with `s.wg.Add/Done()`
   - Impact: Prevents resource leaks and ensures clean shutdown

2. **Device Context Hierarchy** (`worker.go:38-63`)
   - Added: Device-level context derived from Start() context
   - Benefit: Automatic cancellation of all device operations on Shutdown()
   - Pattern: Parent context → device context → task context hierarchy

3. **Topic Storage for Cleanup** (`worker.go:42`)
   - Fixed: Hardcoded "irrigation-zone" in Shutdown didn't match actual subscriptions
   - Solution: Store subscription topic in device struct
   - Impact: Proper unsubscription prevents channel leaks

4. **Context Cancellation in ACK Logic** (`sprinkler.go:88-89`)
   - Added: Skip ACK for `context.Canceled` errors during shutdown
   - Prevents: Spurious failure ACKs when device is deliberately shut down
