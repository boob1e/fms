# Pub/Sub Messaging Improvements Spec

## Overview
This document tracks improvements to the Fleet pub/sub messaging system in `fleet/messaging_models.go` and related components.

## Progress

- [x] **Step 1: Add message delivery error handling and logging**
  - ✅ Added context parameter to `Publish` method
  - ✅ Returns error for delivery failures
  - ✅ Implements best-effort broadcast (attempts all subscribers)
  - ✅ Tracks and logs delivery metrics (success, timeout, channel full)
  - ✅ Structured logging with DEBUG/ERROR/WARN/INFO levels
  - Updated: `fleet/messaging_models.go:56-102`

- [x] **Step 5: Add broker initialization in main.go**
  - ✅ Created FMS struct to hold broker and db
  - ✅ Initialized MessageBroker using `fleet.NewMessageBroker()`
  - ✅ Broker is available via FMS struct for dependency injection
  - ✅ Implemented proper DI pattern (no globals)
  - Updated: `main.go:16-35`

- [x] **Minor Issue: Fixed health endpoint typo**
  - ✅ Changed `/heatlh` to `/health`
  - Updated: `main.go:49`

- [x] **Step 2: Fix the channel closure race condition**
  - ✅ Removed `close(ch)` from Unsubscribe method
  - ✅ Lets GC handle channel cleanup (idiomatic Go)
  - ✅ No race condition - Publish can't send to closed channel
  - ✅ Follows "don't close unless signaling" principle
  - Updated: `fleet/messaging_models.go:39-52`

## Remaining Steps

### Step 3: Make task handlers non-blocking

**Location**: `fleet/irrigation_device.go:39-45` (Sprinkler.StartWater)

**Problem**:
```go
func (s *Sprinkler) StartWater() {
    s.IsActive = true
    for s.IsActive == true {  // Blocks the listener goroutine
        time.Sleep(1 * time.Second)
        log.Println("watering crops")
    }
}
```

**Impact**: When `handleTask` calls `StartWater`, the entire listener goroutine blocks, preventing processing of subsequent tasks.

**Solution**: Task handlers should spawn goroutines for long-running work
```go
func (f *FleetDevice) handleTask(task Task) {
    go f.executeTask(task)  // Non-blocking
}
```

**Considerations**:
- How to track running tasks?
- How to limit concurrent task execution?
- How to handle task cancellation?

**Related files**:
- `fleet/fleet_models.go:36-38` (handleTask method)
- All device implementations that inherit from FleetDevice

---

### Step 4: Implement the handleTask method properly

**Location**: `fleet/fleet_models.go:36-38`

**Current state**:
```go
func (f *FleetDevice) handleTask(task Task) {
    // Empty - needs implementation
}
```

**Requirements**:
- Update task status (Queued → Running → Complete/Failed)
- Delegate to device-specific behavior (polymorphism/interface)
- Handle errors gracefully
- Log task lifecycle events
- Non-blocking execution (see Step 3)

**Design options**:
1. FleetDevice has a TaskHandler interface/callback
2. Subclasses override handleTask
3. Task includes a callback/handler function

---

### Step 6: Add message acknowledgment/completion tracking

**Status**: Optional enhancement

**Rationale**: Currently no way to know if a subscriber successfully processed a task vs just received it.

**Features to consider**:
- ACK/NACK messages back to broker
- Task completion confirmation
- Retry logic for failed tasks
- Dead letter queue for permanently failed tasks

**Implementation complexity**: Medium-High

**Files to modify**:
- `fleet/messaging_models.go` - Add ACK methods
- `fleet/fleet_models.go` - Send ACKs from handleTask
- New: Task queue/retry logic

---

## Additional Issues Found

### Minor Issue: Typo in health endpoint
**Location**: `main.go:25`
```go
app.Get("/heatlh", func(c fiber.Ctx) error {  // Should be "/health"
```

### Architecture Questions
1. Should `FleetDevice.listen` have a maximum task queue size?
2. Should there be metrics/monitoring for queue depths?
3. Should tasks have TTL (time to live)?
4. Should there be priority levels for tasks?

---

## Testing Plan (Future)

- [ ] Unit tests for MessageBroker
- [ ] Race condition testing (`go test -race`)
- [ ] Load testing (many publishers, many subscribers)
- [ ] Chaos testing (slow subscribers, crashes during publish)
- [ ] Integration tests with actual devices

---

## Notes

- Best practices: Use context for cancellation throughout
- Consider structured logging library (e.g., `slog`, `zap`) for production
- Monitor channel depths in production for capacity planning
- Document expected behavior when subscribers are slow/stuck
