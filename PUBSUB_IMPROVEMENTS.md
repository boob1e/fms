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

- [x] **Step 3 & 4: Implement task handling with non-blocking execution**
  - ✅ Implemented `TaskHandler` interface for polymorphic behavior
  - ✅ Self-injection pattern (Strategy Pattern) for device-specific handlers
  - ✅ Task status updates (Queued → Running → Complete/Failed)
  - ✅ Error handling with error return values and logging
  - ✅ Non-blocking execution via goroutines
  - ✅ Async completion tracking using channels
  - ✅ Context-based cancellation for stopping long-running tasks
  - ✅ Race-free shared state with mutex protection
  - ✅ Task lifecycle logging (received, failed, completed)
  - ✅ Defer pattern for guaranteed cleanup
  - Updated: `fleet/fleet_models.go:29-54`, `fleet/irrigation_device.go:46-126`

## Remaining Steps

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
