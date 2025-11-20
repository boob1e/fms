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

### Go Idioms - MANDATORY

**CRITICAL: Always prioritize idiomatic Go patterns in every response.**

When providing suggestions, code examples, or discussing architecture:
1. **Reference Go standard library patterns first** - If stdlib does it, that's the idiomatic way
2. **Favor simplicity over cleverness** - Go values clarity and readability
3. **Use descriptive names over abbreviations** - Except for well-known conventions (ctx, err, etc.)
4. **Provide the "Go way" as the primary recommendation** - Mention alternatives only if asked

**Specific Go Idioms to Follow:**
- Interfaces: Small, focused interfaces (often single-method); named with `-er` suffix
- Constants: Prefix with type name when there are multiple similar types (`http.MethodGet`, `TypeSprinkler`)
- Errors: Return errors as last return value; use `errors.New()` or `fmt.Errorf()`
- Concurrency: Use channels for communication, mutexes for state; `sync.RWMutex` for read-heavy operations
- Naming: Exported (PascalCase), unexported (camelCase); avoid stuttering (`fleet.Manager` not `fleet.FleetManager`)
- File organization: Name files after primary type or concern; flat structure until complexity demands subdirectories
- Nil checks: Two-value map lookups (`val, ok := map[key]`), type assertions, channel receives
- Resource cleanup: Use `defer` for cleanup operations immediately after acquisition
- Dependency injection: Use interfaces, not concrete types; constructor functions return concrete types

**When Unsure:**
- Check Go standard library for similar patterns
- Favor the approach used in `net/http`, `database/sql`, `context` packages
- Prioritize readability and maintainability over brevity

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
