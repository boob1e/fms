# Object Construction Patterns in Go

## Overview

This document compares three common patterns for object creation: Factory, Builder, and Option patterns. These patterns solve different problems in object construction and configuration.

---

## Pattern Comparison

| Pattern | Purpose | Go Idiom | Best For |
|---------|---------|----------|----------|
| **Factory** | Create different types | ✅ Idiomatic | Polymorphic object creation |
| **Builder** | Step-by-step construction | ❌ Not idiomatic | Complex objects (Java/C# style) |
| **Option** | Flexible configuration | ✅✅ Most idiomatic | Optional parameters |

---

## 1. Factory Pattern

### Purpose
Encapsulates object creation logic, returns different types based on input.

### Problem Solved
"I need to create different types of objects based on some criteria without exposing creation complexity"

### Structure
```go
type Factory struct {
    // Dependencies needed for creating objects
    broker Broker
    db     *gorm.DB
    config *Config
}

func (f *Factory) CreateObject(objectType string, params map[string]interface{}) (Interface, error) {
    switch objectType {
    case "typeA":
        return f.createTypeA(params)
    case "typeB":
        return f.createTypeB(params)
    default:
        return nil, fmt.Errorf("unknown type: %s", objectType)
    }
}
```

### Example: Device Factory
```go
type DeviceFactory struct {
    broker Broker
    db     *gorm.DB
    appCtx context.Context
}

func NewDeviceFactory(broker Broker, db *gorm.DB, appCtx context.Context) *DeviceFactory {
    return &DeviceFactory{
        broker: broker,
        db:     db,
        appCtx: appCtx,
    }
}

func (f *DeviceFactory) CreateDevice(deviceType string, config string) (Device, error) {
    ctx, cancel := context.WithCancel(f.appCtx)

    switch deviceType {
    case "sprinkler":
        var cfg SprinklerConfig
        json.Unmarshal([]byte(config), &cfg)
        return NewSprinkler(ctx, f.broker, cfg.Zone), nil

    case "tractor":
        var cfg TractorConfig
        json.Unmarshal([]byte(config), &cfg)
        return NewTractor(ctx, f.broker, cfg.Model), nil

    case "drone":
        var cfg DroneConfig
        json.Unmarshal([]byte(config), &cfg)
        return NewDrone(ctx, f.broker, cfg.MaxAltitude), nil

    default:
        return nil, fmt.Errorf("unknown device type: %s", deviceType)
    }
}
```

### Usage
```go
factory := NewDeviceFactory(broker, db, appCtx)
device, err := factory.CreateDevice("sprinkler", `{"zone": "a"}`)
```

### Characteristics
**Pros:**
- ✅ Hides construction complexity
- ✅ Returns interface, not concrete type
- ✅ Centralizes object creation logic
- ✅ Easy to add new types

**Cons:**
- ❌ Can become complex with many types
- ❌ Requires updating switch for new types

### When to Use
- Creating multiple related types
- Complex or varying creation logic by type
- Want to decouple client from concrete types
- Type selection based on runtime input

### Real-World Examples
```go
// Database drivers
db, err := sql.Open("postgres", connString)  // Returns PostgresDB
db, err := sql.Open("mysql", connString)     // Returns MySQLDB

// HTTP client transport
transport := http.DefaultTransport  // Factory for different protocols
```

---

## 2. Builder Pattern

### Purpose
Constructs complex objects step-by-step with many optional parameters using fluent interface.

### Problem Solved
"I need to create an object with many optional fields without a huge constructor with dozens of parameters"

### Structure
```go
type Builder struct {
    object *ComplexObject
}

func NewBuilder(required1, required2 string) *Builder {
    return &Builder{
        object: &ComplexObject{
            Required1: required1,
            Required2: required2,
            // Set defaults
            Optional1: defaultValue1,
            Optional2: defaultValue2,
        },
    }
}

func (b *Builder) WithOptional1(value string) *Builder {
    b.object.Optional1 = value
    return b  // Return self for chaining
}

func (b *Builder) WithOptional2(value int) *Builder {
    b.object.Optional2 = value
    return b
}

func (b *Builder) Build() *ComplexObject {
    // Validation before returning
    if err := b.object.Validate(); err != nil {
        panic(err)  // Or return error
    }
    return b.object
}
```

### Example: Sprinkler Builder
```go
type SprinklerBuilder struct {
    sprinkler *Sprinkler
}

func NewSprinklerBuilder(ctx context.Context, broker Broker, zone string) *SprinklerBuilder {
    return &SprinklerBuilder{
        sprinkler: &Sprinkler{
            FleetDevice: FleetDevice{
                ID:     uuid.New(),
                broker: broker,
                inbox:  broker.Subscribe("irrigation-" + zone),
            },
            Zone:        zone,
            MaxPressure: 100.0,  // Default
            MinPressure: 20.0,   // Default
            FlowRate:    5.0,    // Default
        },
    }
}

func (b *SprinklerBuilder) WithMaxPressure(pressure float64) *SprinklerBuilder {
    b.sprinkler.MaxPressure = pressure
    return b
}

func (b *SprinklerBuilder) WithMinPressure(pressure float64) *SprinklerBuilder {
    b.sprinkler.MinPressure = pressure
    return b
}

func (b *SprinklerBuilder) WithFlowRate(rate float64) *SprinklerBuilder {
    b.sprinkler.FlowRate = rate
    return b
}

func (b *SprinklerBuilder) WithAutoShutoff(enabled bool) *SprinklerBuilder {
    b.sprinkler.AutoShutoff = enabled
    return b
}

func (b *SprinklerBuilder) Build() *Sprinkler {
    if b.sprinkler.MaxPressure < b.sprinkler.MinPressure {
        panic("max pressure must be >= min pressure")
    }
    return b.sprinkler
}
```

### Usage
```go
// With many options
sprinkler := NewSprinklerBuilder(ctx, broker, "zone-a").
    WithMaxPressure(120.0).
    WithMinPressure(30.0).
    WithFlowRate(7.5).
    WithAutoShutoff(true).
    Build()

// With few options - uses defaults
sprinkler := NewSprinklerBuilder(ctx, broker, "zone-b").
    WithMaxPressure(80.0).
    Build()
```

### Characteristics
**Pros:**
- ✅ Readable, self-documenting code
- ✅ Optional parameters easy to skip
- ✅ Fluent interface (method chaining)
- ✅ Can validate before building
- ✅ IDE autocomplete friendly

**Cons:**
- ❌ Verbose (many methods)
- ❌ Mutable during construction
- ❌ Not idiomatic Go (prefer Option pattern)
- ❌ Extra struct for builder

### When to Use
- Many optional parameters (5+)
- Need validation before object is complete
- Coming from Java/C# background
- **In Go: Use Option pattern instead**

---

## 3. Option Pattern (Functional Options)

### Purpose
Provide flexible, extensible configuration for constructors using functional programming concepts.

### Problem Solved
Same as Builder pattern, but using idiomatic Go style with variadic functions.

### Structure
```go
// Option is a function that modifies the object
type Option func(*Object)

// Constructor accepts variadic options
func NewObject(required1, required2 string, opts ...Option) *Object {
    // Create with defaults
    obj := &Object{
        Required1: required1,
        Required2: required2,
        Optional1: defaultValue1,
        Optional2: defaultValue2,
    }

    // Apply options
    for _, opt := range opts {
        opt(obj)
    }

    // Validate after options applied
    if err := obj.Validate(); err != nil {
        panic(err)
    }

    return obj
}

// Option functions (factory for options)
func WithOptional1(value string) Option {
    return func(o *Object) {
        o.Optional1 = value
    }
}

func WithOptional2(value int) Option {
    return func(o *Object) {
        o.Optional2 = value
    }
}
```

### Example: Sprinkler with Options
```go
// Option type
type SprinklerOption func(*Sprinkler)

// Constructor with variadic options
func NewSprinkler(ctx context.Context, broker Broker, zone string, opts ...SprinklerOption) *Sprinkler {
    // Create with defaults
    s := &Sprinkler{
        FleetDevice: FleetDevice{
            ID:     uuid.New(),
            broker: broker,
            inbox:  broker.Subscribe("irrigation-" + zone),
        },
        Zone:        zone,
        MaxPressure: 100.0,  // Default
        MinPressure: 20.0,   // Default
        FlowRate:    5.0,    // Default
    }

    // Apply all options
    for _, opt := range opts {
        opt(s)
    }

    // Validate
    if s.MaxPressure < s.MinPressure {
        panic("invalid pressure range")
    }

    // Start device
    s.handler = s
    s.wg.Add(1)
    go s.listen(ctx)

    return s
}

// Option functions
func WithMaxPressure(pressure float64) SprinklerOption {
    return func(s *Sprinkler) {
        s.MaxPressure = pressure
    }
}

func WithMinPressure(pressure float64) SprinklerOption {
    return func(s *Sprinkler) {
        s.MinPressure = pressure
    }
}

func WithFlowRate(rate float64) SprinklerOption {
    return func(s *Sprinkler) {
        s.FlowRate = rate
    }
}

func WithAutoShutoff(enabled bool) SprinklerOption {
    return func(s *Sprinkler) {
        s.AutoShutoff = enabled
    }
}

// Compound options
func WithPressureRange(min, max float64) SprinklerOption {
    return func(s *Sprinkler) {
        s.MinPressure = min
        s.MaxPressure = max
    }
}
```

### Usage
```go
// With many options
sprinkler := NewSprinkler(ctx, broker, "zone-a",
    WithMaxPressure(120.0),
    WithMinPressure(30.0),
    WithFlowRate(7.5),
    WithAutoShutoff(true),
)

// With few options
sprinkler := NewSprinkler(ctx, broker, "zone-b",
    WithMaxPressure(80.0),
)

// With compound options
sprinkler := NewSprinkler(ctx, broker, "zone-c",
    WithPressureRange(30.0, 120.0),
    WithAutoShutoff(true),
)

// No options - all defaults
sprinkler := NewSprinkler(ctx, broker, "zone-d")

// Composable options
opts := []SprinklerOption{
    WithMaxPressure(100),
    WithAutoShutoff(true),
}
if production {
    opts = append(opts, WithEmergencyContact("555-1234"))
}
sprinkler := NewSprinkler(ctx, broker, zone, opts...)
```

### Characteristics
**Pros:**
- ✅✅ Most idiomatic Go pattern
- ✅ Flexible and extensible
- ✅ Backward compatible (add options without breaking callers)
- ✅ Options are composable
- ✅ Less boilerplate than Builder
- ✅ Constructor returns immutable object
- ✅ Used by many Go standard/third-party libraries

**Cons:**
- ❌ Slightly less discoverable than Builder methods (no IDE autocomplete)
- ❌ Options can be applied in any order (may need careful validation)

### When to Use
- **Default choice for Go projects**
- Need optional configuration
- Want backward compatibility
- Library/package development
- 3+ optional parameters

### Real-World Examples

**gRPC:**
```go
conn, err := grpc.Dial(address,
    grpc.WithInsecure(),
    grpc.WithBlock(),
    grpc.WithTimeout(10*time.Second),
)
```

**Zap Logger:**
```go
logger, err := zap.NewProduction(
    zap.AddCaller(),
    zap.AddStacktrace(zapcore.ErrorLevel),
    zap.Fields(zap.String("service", "fms")),
)
```

**HTTP Client:**
```go
client := &http.Client{
    Timeout: 10 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:       10,
        IdleConnTimeout:    30 * time.Second,
        DisableCompression: true,
    },
}
```

---

## Combining Patterns

Patterns can be used together for maximum flexibility.

### Factory + Option Pattern

```go
type DeviceFactory struct {
    broker Broker
    db     *gorm.DB
    appCtx context.Context
}

// Factory creates devices with options
func (f *DeviceFactory) CreateSprinkler(zone string, opts ...SprinklerOption) *Sprinkler {
    return NewSprinkler(f.appCtx, f.broker, zone, opts...)
}

func (f *DeviceFactory) CreateTractor(model string, opts ...TractorOption) *Tractor {
    return NewTractor(f.appCtx, f.broker, model, opts...)
}

// Usage
factory := NewDeviceFactory(broker, db, appCtx)

sprinkler := factory.CreateSprinkler("zone-a",
    WithMaxPressure(120),
    WithAutoShutoff(true),
)

tractor := factory.CreateTractor("John Deere 8R",
    WithGPSEnabled(true),
    WithFuelCapacity(500),
)
```

---

## Pattern Selection Guide

### Decision Tree

```
Do you need to create different types based on runtime input?
├─ YES → Factory Pattern
└─ NO
    ├─ Do you have 5+ optional parameters?
    │   ├─ YES (in Go) → Option Pattern ✅
    │   └─ YES (in Java/C#) → Builder Pattern
    └─ NO → Simple constructor
```

### Use Case Matrix

| Scenario | Pattern | Example |
|----------|---------|---------|
| Create different types | Factory | `factory.CreateDevice(type)` |
| 5+ optional params (Go) | Option | `New(required, WithX(), WithY())` |
| 5+ optional params (Java/C#) | Builder | `Builder().WithX().Build()` |
| Need validation between steps | Builder | `Builder().Step1().Validate().Step2()` |
| Backward compatibility | Option | Add new options without breaking |
| 1-2 optional params | Simple | Use pointer or boolean param |

---

## Recommendations for FMS Project

### Device Creation
**Use Option Pattern:**
```go
func NewSprinkler(ctx context.Context, broker Broker, zone string, opts ...SprinklerOption) *Sprinkler
```

**Rationale:**
- Idiomatic Go
- Flexible configuration
- Easy to add new options
- Backward compatible

### FMS Initialization
**Use Option Pattern:**
```go
func NewFMS(opts ...FMSOption) (*FMS, error)
```

**Rationale:**
- Clean up main()
- Flexible for different environments (test, dev, prod)
- Easy to provide defaults
- Testable

### Multi-Type Device Creation
**Use Factory + Option:**
```go
type DeviceService struct {
    factory *DeviceFactory
}

func (s *DeviceService) CreateDevice(deviceType string, config map[string]interface{}) (Device, error) {
    // Factory for type selection + Options for configuration
}
```

**Rationale:**
- Factory handles type selection
- Options handle configuration
- Best of both worlds

---

## Anti-Patterns to Avoid

### ❌ Pointer to Interface
```go
// Wrong
func NewService(broker *Broker) *Service

// Right
func NewService(broker Broker) *Service
```

### ❌ Too Many Constructor Parameters
```go
// Wrong - hard to remember order
func NewSprinkler(ctx, broker, zone, maxP, minP, flow, shutoff, contact, schedule, firmware string)

// Right - use options
func NewSprinkler(ctx context.Context, broker Broker, zone string, opts ...SprinklerOption)
```

### ❌ Mixing Patterns Unnecessarily
```go
// Wrong - confusing
func NewBuilder(opts ...Option) *Builder  // Don't mix Builder + Option

// Right - pick one
func New(opts ...Option) *Object          // Option pattern
func NewBuilder() *Builder                 // Builder pattern
```

---

## References

- **Option Pattern**: Dave Cheney - "Functional options for friendly APIs" (2014)
- **Builder Pattern**: Gang of Four "Design Patterns" (1994)
- **Factory Pattern**: Gang of Four "Design Patterns" (1994)
- **Go Best Practices**: Rob Pike - "Go Proverbs"
- **Related Specs**:
  - `SELF_INJECTION_PATTERN.md` - Strategy pattern for polymorphism
  - `DATABASE_MODELING_SPEC.md` - Database design patterns

---

## Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2025-11-19 | Prefer Option pattern over Builder in Go | More idiomatic, less boilerplate, better backward compatibility |
| 2025-11-19 | Use Factory for device type selection | Centralized type switching, clean separation |
| 2025-11-19 | Combine Factory + Option for flexibility | Factory for type, Options for config |
