# Self-Injection Pattern Explanation

Great question! This is confusing because it **looks circular** but isn't. Let me break it down:

## What's Actually Happening

```go
s := &Sprinkler{
    FleetDevice: FleetDevice{
        broker: broker,
        inbox:  ch,
        handler: nil,  // Initially nil
    },
}

s.handler = s  // Store pointer to the WHOLE Sprinkler in FleetDevice's handler field
```

### The Memory Layout

```
┌─────────────────────────────┐
│      Sprinkler Object       │
│  ┌──────────────────────┐   │
│  │   FleetDevice        │   │
│  │   - broker           │   │
│  │   - inbox            │   │
│  │   - handler  ────────┼───┼──┐ (points back to parent Sprinkler)
│  └──────────────────────┘   │   │
│  - IsActive                  │ ◄─┘
│  - PressureReading           │
│  - HandleTask() method       │
└─────────────────────────────┘
```

## How It Works

When `FleetDevice.listen()` runs:

```go
func (f *FleetDevice) listen(ctx context.Context) {
    case task := <-f.inbox:
        f.handler.HandleTask(task)
        // f is just the FleetDevice part
        // f.handler points to the FULL Sprinkler
        // So this calls Sprinkler.HandleTask()
}
```

## The Pattern Name

This is the **Strategy Pattern** (from Gang of Four design patterns), specifically implemented as:

1. **Self-Delegation Pattern** - object delegates to itself through an interface
2. **Polymorphic Self-Reference** - storing `this`/`self` in a parent field

It's Go's workaround for the lack of virtual methods/inheritance.

## Why It Works

- `s` (type `*Sprinkler`) implements `TaskHandler` interface
- `handler` field accepts any `TaskHandler`
- Therefore `s.handler = s` is valid (pointer to Sprinkler stored as TaskHandler interface)
- The interface contains a pointer to the **complete Sprinkler object**, not just the FleetDevice part

## Classical OOP Equivalent

In Java/C++, you'd just do:
```java
class FleetDevice {
    void listen() {
        this.handleTask(task);  // Virtual dispatch automatically calls Sprinkler.handleTask()
    }
    void handleTask(Task task) { /* default */ }
}

class Sprinkler extends FleetDevice {
    @Override
    void handleTask(Task task) { /* override */ }
}
```

Go requires explicit delegation because embedded structs aren't true inheritance.
