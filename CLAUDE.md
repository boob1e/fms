# Claude Code Preferences

This file documents preferences for how Claude should assist with this project.

## Learning Mode Preferences

**Active Output Style**: Learning

### Implementation Approach

When implementing features:
- **Use "Learn by Doing" format** for implementation tasks involving design decisions
- Add `TODO(human)` markers in code before requesting implementation
- Provide Context, Task, and Guidance structure
- Wait for implementation before proceeding

### When to Provide Complete Code vs Guidance

**Provide complete code examples for:**
- Answering architectural/conceptual questions
- Explaining patterns and best practices
- Reference implementations during discussions

**Use "Learn by Doing" guidance for:**
- Active feature implementation
- Code involving design decisions (error handling, data structures, algorithms)
- Functions/methods 20+ lines with meaningful choices
- Business logic with multiple valid approaches

**Always ask first** before transitioning from discussion to implementation:
- "Would you like to implement this yourself with guidance?"
- Wait for explicit confirmation before using "Learn by Doing" format

## Code Style Preferences

### Go Idioms
- Use idiomatic Go patterns (two-value map lookups, defer for cleanup, etc.)
- Prefer interfaces for dependency injection
- Use proper locking strategies (RLock for reads, Lock for writes)
- Implement defensive copying for internal state protection

### Architecture Patterns
- Strategy Pattern for polymorphic behavior (self-injection)
- Three-layer pattern: Domain → Orchestration → Routing
- Context-based cancellation for long-running operations
- Channel-based async communication

## Project Context

**Current Architecture:**
- Fleet Management System (FMS) for IoT devices
- Pub/sub messaging with MessageBroker
- Task-based command system with ACK/NACK tracking
- Device agents with self-injection pattern

**Naming Conventions:**
- `DeviceAgent` for base fleet device infrastructure
- `TaskHandler` interface for polymorphic task handling
- `TaskState` for broker-side tracking, `TaskAck` for device feedback

## Active Work

See `TASK_ACK_RETRY_SPEC.md` for current implementation phases.
- Phase 1: ✅ Complete (ACK/NACK Infrastructure)
- Phase 2: ✅ Complete (Completion Tracking)
- Phase 3: Paused (Retry Logic)

## Documentation Files

- `PUBSUB_IMPROVEMENTS.md` - Overall pub/sub system roadmap
- `TASK_ACK_RETRY_SPEC.md` - ACK/retry system implementation spec
- `SELF_INJECTION_PATTERN.md` - Strategy pattern explanation
