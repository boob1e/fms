# Multi-Topic Subscription Specification

## Overview
Refactor the Worker subscription model to support multiple topic subscriptions per device, enabling flexible task routing based on zone, worker type, device type, and individual device ID.

## Current State

### Existing Architecture
- **Single Topic Subscription**: Workers subscribe to one topic during construction
- **Topic Format**: `"irrigation-" + zone` (e.g., `"irrigation-zoneA"`)
- **Channel Model**: Single `inbox chan Task` per device
- **Subscription Location**: `NewSprinkler()` constructor
- **Unsubscribe Issue**: Hardcoded topic in `Shutdown()` doesn't match actual subscription

### Code References
- `fleet/sprinkler.go:27` - `broker.Subscribe("irrigation-" + zone)`
- `fleet/worker.go:41` - `inbox chan Task`
- `fleet/worker.go:56` - `broker.Unsubscribe("irrigation-zone", d.inbox)` (bug)

## Requirements

### Functional Requirements

#### FR1: Multiple Topic Types
Workers must be able to receive tasks from multiple topic categories:

1. **Zone Topics**: `"zone-{zone}"` (e.g., `"zone-A"`)
   - Target: All devices in a specific zone
   - Use case: Zone-wide operations (water all sprinklers in zone A)

2. **Worker Type Topics**: `"worker-{type}"` (e.g., `"worker-irrigation"`)
   - Target: All devices of a specific worker type
   - Use case: Type-specific operations (calibrate all irrigation workers)

3. **Device Type Topics**: `"device-{type}"` (e.g., `"device-sprinkler"`)
   - Target: All devices of a specific device type
   - Use case: Device-specific commands (firmware update for all sprinklers)

4. **Individual Device Topics**: `"device-{uuid}"` (e.g., `"device-550e8400-..."`)
   - Target: Single specific device
   - Use case: Direct device commands

#### FR2: Dynamic Subscription Management
- Workers should subscribe to all relevant topics at startup
- Workers should cleanly unsubscribe from all topics at shutdown
- Future: Support runtime subscription changes (add/remove topics)

#### FR3: Task Processing
- Workers should process tasks from any subscribed topic
- Workers may need to know which topic a task came from (context for handling)
- Task handling should remain non-blocking

### Non-Functional Requirements

#### NFR1: Performance
- No significant overhead compared to single-topic model
- Avoid channel/goroutine proliferation
- Maintain low latency for task delivery

#### NFR2: Maintainability
- Clear separation of concerns
- Minimal changes to existing `HandleTask` implementations
- Clean shutdown guarantees (no goroutine leaks)

#### NFR3: Flexibility
- Easy to add new topic types in the future
- Workers can opt-in/out of certain topic types
- Topic naming should be consistent and predictable

## Design Questions

### DQ1: Channel Architecture
**Question**: How should workers handle multiple topic subscriptions?

**Option A: Multiple Channels**
```go
type device struct {
    inboxes map[string]chan Task  // topic -> channel
}
```
- Pros: Direct mapping, easy to track which topic each task came from
- Cons: Complex select logic, harder to manage multiple channels

**Option B: Single Merged Channel**
```go
type device struct {
    inbox        chan Task          // Merged channel
    subscriptions map[string]Subscriber // Track subscriptions
}
```
- Pros: Simple listen loop, no changes to current processing logic
- Cons: Lose topic context unless added to Task, need merge mechanism

**Option C: Single Channel with Topic Metadata**
```go
type TaskEnvelope struct {
    Task  Task
    Topic string
}
type device struct {
    inbox chan TaskEnvelope
}
```
- Pros: Preserves topic context, single channel simplicity
- Cons: Changes Task type across system, affects all existing code

**Decision**: [TODO - to be determined during implementation]
**Rationale**: [TODO]

### DQ2: Topic Determination
**Question**: Who calculates which topics a worker should subscribe to?

**Option A: Worker Self-Determination**
```go
func (s *Sprinkler) GetTopics() []string {
    // Worker knows its own characteristics
}
```
- Pros: Encapsulation, worker controls its subscriptions
- Cons: Worker needs to store zone/type info, couples worker to topic logic

**Option B: External Configuration**
```go
topics := TopicResolver.ResolveTopics(device)
device.SubscribeToAll(topics)
```
- Pros: Centralized topic logic, easy to change topic strategy
- Cons: Extra abstraction, worker needs external dependencies

**Option C: Constructor-Based**
```go
func NewSprinkler(broker Broker, zone string, topics []string) *Sprinkler
```
- Pros: Explicit, caller controls subscriptions
- Cons: Caller needs to know topic calculation logic

**Decision**: [TODO]
**Rationale**: [TODO]

### DQ3: Topic Context in Task Handling
**Question**: Does the worker need to know which topic a task came from?

**Scenario 1**: Task content is self-contained
```go
// Task has all needed info
task := Task{Instruction: "start", Duration: 300}
// Worker doesn't care if it came from zone topic vs device topic
```

**Scenario 2**: Topic provides context
```go
// Behavior differs based on source
if topic == "zone-A" {
    // Coordinate with other devices in zone
} else if topic == "device-{uuid}" {
    // Direct command, execute immediately
}
```

**Decision**: [TODO]
**Rationale**: [TODO]

### DQ4: Interface Changes
**Question**: What changes are needed to the Worker interface?

Current:
```go
type Worker interface {
    TaskHandler
    GetID() uuid.UUID
    Start(ctx context.Context)
    Shutdown()
}
```

Possible additions:
- `GetTopics() []string` - Return current subscriptions
- `Subscribe(topic string) error` - Add topic at runtime
- `Unsubscribe(topic string) error` - Remove topic at runtime
- `GetSubscribedTopics() []string` - Query current subscriptions

**Decision**: [TODO]
**Rationale**: [TODO]

### DQ5: Subscription Lifecycle
**Question**: When and how should subscriptions be managed?

**Option A: All-At-Once (Constructor)**
```go
func NewSprinkler(...) *Sprinkler {
    topics := []string{"zone-A", "worker-irrigation", "device-sprinkler"}
    for _, topic := range topics {
        ch := broker.Subscribe(topic)
        // Store/merge channels
    }
}
```

**Option B: Lazy/Deferred (After Construction)**
```go
sprinkler := NewSprinkler(...)
sprinkler.Subscribe("zone-A")
sprinkler.Subscribe("worker-irrigation")
sprinkler.Start(ctx)
```

**Option C: Service-Managed**
```go
// DeviceService orchestrates subscriptions
s.registry.Register(sprinkler, req.Zone)
s.subscriptionManager.ConfigureTopics(sprinkler, zone, workerType, deviceType)
```

**Decision**: [TODO]
**Rationale**: [TODO]

## Implementation Considerations

### IC1: Channel Merging Pattern
If using single merged channel, how to combine multiple topic subscriptions?

**Pattern 1: Fan-in with goroutines**
```go
// One goroutine per topic subscription
for _, topic := range topics {
    ch := broker.Subscribe(topic)
    go func(ch chan Task) {
        for task := range ch {
            mergedInbox <- task
        }
    }(ch)
}
```
- Pros: Simple, automatic forwarding
- Cons: N goroutines per device, shutdown complexity

**Pattern 2: Select-based multiplexing**
```go
// Single goroutine, dynamic select (requires reflection or code generation)
cases := make([]reflect.SelectCase, len(subscriptions))
// ... build select cases ...
reflect.Select(cases)
```
- Pros: Single goroutine
- Cons: Complex, uses reflection, hard to maintain

### IC2: Shutdown Cleanup
Need to track all subscriptions for proper cleanup:

```go
type device struct {
    subscriptions []subscription  // Track for cleanup
}

type subscription struct {
    topic string
    ch    Subscriber
}

func (d *device) Shutdown() {
    for _, sub := range d.subscriptions {
        d.broker.Unsubscribe(sub.topic, sub.ch)
    }
}
```

### IC3: Topic Naming Convention
Establish consistent topic naming:

```
zone-{zoneName}           // e.g., zone-A, zone-north-field
worker-{workerType}       // e.g., worker-irrigation, worker-harvesting
device-{deviceType}       // e.g., device-sprinkler, device-pump
device-{uuid}             // e.g., device-550e8400-e29b-41d4-a716-446655440000
```

Should these be constants?
```go
const (
    TopicPrefixZone   = "zone-"
    TopicPrefixWorker = "worker-"
    TopicPrefixDevice = "device-"
)

func ZoneTopic(zone string) string {
    return TopicPrefixZone + zone
}
```

### IC4: Backward Compatibility
Current code expects:
- `NewSprinkler(broker Broker, zone string)` signature
- Single topic subscription
- Direct channel access in tests

Migration strategy:
1. Add new multi-topic support alongside existing single-topic
2. Deprecate old pattern
3. Migrate existing devices
4. Remove old code

Or: Breaking change with full refactor?

## Success Criteria

### SC1: Functional Success
- [ ] Worker can subscribe to 4+ topics simultaneously
- [ ] Tasks published to any subscribed topic reach the worker
- [ ] Worker processes tasks from all topics correctly
- [ ] Clean shutdown unsubscribes from all topics
- [ ] No goroutine leaks after shutdown

### SC2: Code Quality
- [ ] Clear, maintainable code
- [ ] Minimal changes to existing HandleTask implementations
- [ ] Proper error handling for subscription failures
- [ ] Thread-safe subscription management

### SC3: Testing
- [ ] Unit tests for multi-topic subscription
- [ ] Integration tests for task delivery across topic types
- [ ] Shutdown cleanup verification
- [ ] Performance benchmarks (no significant regression)

## Open Questions

1. Should workers be able to subscribe/unsubscribe at runtime, or only during construction/shutdown?
2. Do we need priority queuing (e.g., device-specific tasks take precedence over zone-wide tasks)?
3. Should there be a maximum number of topics per worker?
4. How do we handle subscription failures (topic doesn't exist, broker unavailable)?
5. Should the broker support topic patterns/wildcards (e.g., `"zone-*"` for all zones)?

## Next Steps

1. [ ] Answer design questions (DQ1-DQ5)
2. [ ] Choose channel architecture approach
3. [ ] Define new interfaces/types needed
4. [ ] Create implementation plan
5. [ ] Write tests for new behavior
6. [ ] Implement changes
7. [ ] Update documentation

## References

- Current implementation: `fleet/worker.go`, `fleet/sprinkler.go`, `fleet/broker.go`
- Related specs: `SELF_INJECTION_PATTERN.md`, `TASK_ACK_RETRY_SPEC.md`
- Go patterns: Fan-in, worker pools, pub/sub
