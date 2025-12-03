# Fleet Management System (FMS)

A learning project exploring message broker architecture and concurrent systems in Go, built with Claude Code as a teaching coach.

## About This Project

This is a **hands-on learning journey** to understand how message brokers work by building one from scratch. I'm using Claude Code's **Learning Mode** as a coach - it guides me through complex topics, provides architectural insights, and checks my work, but **I write all the implementation code myself**.

### Learning Approach

Rather than having Claude Code generate code for me, I'm using it to:
- **Understand patterns** - Learn idiomatic Go patterns and distributed systems concepts
- **Design decisions** - Discuss trade-offs and architectural choices
- **Debug issues** - Identify problems in my code (like deadlocks, race conditions)
- **Best practices** - Learn concurrent programming patterns and testing strategies

This "Learn by Doing" approach helps me deeply understand:
- How pub/sub messaging works under the hood
- Concurrent programming with goroutines and channels
- Thread-safe data structures with mutexes
- Reliability patterns (ACKs, retries, exponential backoff)
- Testing concurrent systems

## What I've Built So Far

### Phase 1 & 2: Foundation (✅ Complete)
- **Pub/Sub MessageBroker** with topic-based routing
- **Task acknowledgment system** - devices send ACKs (Running/Complete/Failed)
- **Task state tracking** - broker monitors lifecycle from publish to completion
- **Comprehensive tests** - 9 tests covering ACK flows and edge cases

**Key Learning:** Goroutine lifecycle management, context hierarchies, channel cleanup

### Phase 3: Retry Logic (✅ Complete)
- **Exponential backoff** - Failed tasks retry with increasing delays (1s → 2s → 4s → 8s)
- **Retry scheduler** - `processRetries()` goroutine checks every second for tasks to retry
- **Attempt tracking** - `processACKs()` manages retry state and max retries enforcement
- **7 comprehensive tests** - Cover first failure, retry increments, max retries, backoff timing

**Key Learning:** Deadlock prevention (collect-then-process pattern), state ownership, concurrent map access, time.Duration math

#### Critical Bugs I Found & Fixed

1. **Deadlock** - `processRetries` called `Publish()` while holding mutex → complete system freeze
   - **Solution:** Collect tasks under lock, release, then publish (never hold locks during external calls)

2. **Lost attempt tracking** - Deleting from retry queue after republish lost retry count
   - **Solution:** Keep tasks in queue as source of truth; remove only on Complete or max retries

3. **Concurrent map panic** - Iterating `retryQueue` without lock while `processACKs` modified it
   - **Solution:** Hold lock for entire iteration while collecting tasks to retry

### Phase 4: Dead Letter Queue (✅ Complete)
- **DLQ storage** - Slice-based storage with dedicated mutex for permanently failed tasks
- **Management APIs** - GetDLQTasks, RequeueFromDLQ, RemoveFromDLQ, ClearDLQ
- **Automatic DLQ population** - Tasks exceeding max retries moved to DLQ with failure metadata
- **7 comprehensive tests** - Cover DLQ operations, thread safety, and requeue functionality

**Key Learning:** Lock ordering to prevent deadlock, defensive copying for thread safety, separate mutexes for independent data structures, resource cleanup ownership patterns

#### Critical Bugs I Found & Fixed

1. **Lock ordering deadlock** - RequeueFromDLQ held dlqMu then tried to acquire mu while other code did opposite
   - **Solution:** Collect-then-process - release dlqMu before acquiring mu (never hold both simultaneously)

2. **Unsafe slice exposure** - GetDLQTasks returned internal slice, allowing external modification
   - **Solution:** Always return defensive copies; never expose internal mutable state

3. **Double-close panic** - Tests closed ackChan AND called Shutdown() which also closes it
   - **Solution:** Resource cleanup in one canonical place (Shutdown owns channel lifecycle)

4. **Missing read locks** - Accessed DLQ slice during iteration without holding mutex
   - **Solution:** Even reads need locks when data can be modified concurrently

## Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│                          MessageBroker                               │
│  ┌────────────┐  ┌────────────┐  ┌──────────────┐  ┌────────────┐  │
│  │Task Queue  │  │ ACK Chan   │  │ Retry Queue  │  │    DLQ     │  │
│  │            │  │            │  │  (backoff)   │  │ (permanent)│  │
│  └────────────┘  └────────────┘  └──────────────┘  └────────────┘  │
│         │              ▲                 ▲               ▲           │
│         │ Publish      │ ACK             │               │           │
│         ▼              │                 │               │           │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │  processACKs()            processRetries()                     │ │
│  │  - Track lifecycle        - Check every 1s                     │ │
│  │  - Manage retries         - Republish tasks                    │ │
│  │  - Move to DLQ on max     - Exponential backoff                │ │
│  └────────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────┘
         │                                         ▲
         │ Task                                    │ ACK
         ▼                                         │
┌──────────────────────────────────────────────────────────────────┐
│                   FleetDevice (Sprinkler)                        │
│  ┌──────────────┐              ┌─────────────────────┐           │
│  │  HandleTask  │─────────────▶│  Send ACK to Broker │           │
│  │              │              │  (Running/Complete) │           │
│  └──────────────┘              └─────────────────────┘           │
└──────────────────────────────────────────────────────────────────┘
```

## Running Tests

```bash
# All tests (26 total: ACKs, retry logic, DLQ)
go test ./fleet/

# Retry logic tests only
go test -v -run "Test.*Retry|TestCalculateBackoff" ./fleet/

# DLQ tests only
go test -v -run "Test.*DLQ" ./fleet/

# With coverage
go test -cover ./fleet/
```

## Project Structure

```
fleet/
├── broker.go           # MessageBroker implementation (ACK, retry, DLQ)
├── task.go            # Task, TaskAck, TaskState, RetryConfig, DLQEntry types
├── device_agent.go    # Base device with self-injection pattern
├── sprinkler.go       # Example TaskHandler implementation
└── broker_test.go     # Comprehensive test suite (26 tests)

specs/
└── completed/         # ✅ Completed implementation specs
    ├── TASK_ACK_RETRY_SPEC_COMPLETE.md
    ├── MULTI_TOPIC_SUBSCRIPTION_SPEC_COMPLETE.md
    └── PUBSUB_IMPROVEMENTS_COMPLETE.md

Root documentation:
├── README.md                      # Project overview & learning journey
├── CLAUDE.md                      # Claude Code preferences & configuration
├── SELF_INJECTION_PATTERN.md     # Strategy pattern reference
├── CONSTRUCTION_PATTERNS_SPEC.md  # Device construction patterns (active)
└── DATABASE_MODELING_SPEC.md      # Database schema design (active)
```

## Design Patterns I've Learned

### 1. Strategy Pattern (Self-Injection)
Devices inject themselves as `TaskHandler` to enable polymorphic behavior:
```go
type TaskHandler interface {
    HandleTask(task Task)
}

// Sprinkler implements TaskHandler and injects itself
sprinkler := NewSprinkler(broker, "zone-a")
deviceAgent.InjectHandler(sprinkler)  // Self-injection
```

### 2. Collect-Then-Process (Deadlock Prevention)
```go
// Collect under lock
b.mu.Lock()
var tasksToRetry []Task
for _, t := range b.retryQueue {
    if time.Now().After(t.NextRetry) {
        tasksToRetry = append(tasksToRetry, t.Task)
    }
}
b.mu.Unlock()

// Process without lock
for _, task := range tasksToRetry {
    b.Publish(ctx, task)  // Safe - no lock held
}
```

### 3. Context Cancellation Hierarchy
```go
// Parent context → device context → task context
deviceCtx := context.WithCancel(parentCtx)
taskCtx := context.WithTimeout(deviceCtx, 30*time.Second)
```

### 4. Defensive Copying (Thread Safety)
```go
// WRONG - exposes internal state
func (b *MessageBroker) GetDLQTasks() []DLQEntry {
    b.dlqMu.RLock()
    defer b.dlqMu.RUnlock()
    return b.dlq  // ❌ Caller can modify internal slice!
}

// CORRECT - returns copy
func (b *MessageBroker) GetDLQTasks() []DLQEntry {
    b.dlqMu.RLock()
    defer b.dlqMu.RUnlock()
    cpy := make([]DLQEntry, len(b.dlq))
    copy(cpy, b.dlq)
    return cpy  // ✅ Safe - external modifications don't affect internal state
}
```

## What I'm Learning About

- **Concurrent programming:** Goroutines, channels, mutexes, race conditions
- **Distributed systems:** ACKs, retries, idempotency, failure handling
- **Testing:** Table-driven tests, timing-sensitive tests, concurrent test scenarios
- **Go idioms:** Interfaces, error handling, context patterns, builder patterns
- **System design:** Separation of concerns, state machines, event-driven architecture

## Lessons Learned

### Concurrency Is Hard
- Deadlocks are subtle and tests reveal them
- Race conditions require careful mutex placement
- Never hold locks while calling functions that acquire locks

### State Management Matters
- Clear ownership prevents bugs (retry queue owns attempt count)
- Deleting state too early breaks logic
- Source of truth must be explicit

### Testing Reveals Truth
- Time-based tests need generous margins
- Concurrent tests expose race conditions
- Integration tests catch design flaws unit tests miss

## Resources & Inspiration

- **Go Concurrency Patterns** - Rob Pike's talks
- **Building Microservices** - Sam Newman
- **Designing Data-Intensive Applications** - Martin Kleppmann
- **Claude Code Learning Mode** - Interactive coaching for complex topics

## Contributing

This is a personal learning project, but feedback on my code is welcome! Feel free to:
- Point out bugs or anti-patterns
- Suggest better approaches
- Share resources on message brokers or Go concurrency

---

**Built with:** Go 1.21+ | Claude Code (Learning Mode)
**License:** MIT
**Status:** Active Learning Project 🚀
