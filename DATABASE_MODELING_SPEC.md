# Database Modeling Specification: Device Agent Polymorphism

## Problem Statement

Model a polymorphic device system in a relational database where:
- Multiple device types exist (Sprinkler, Tractor, Drone, etc.)
- Each device type has shared fields (serial_number, status, etc.)
- Each device type has unique fields (zone for sprinklers, GPS for tractors)
- System requirements:
  - Query all devices regardless of type
  - Reference "any device" from other tables (tasks, maintenance logs)
  - Add new device types regularly without breaking existing code
  - Support type-agnostic messaging/task system
  - Future: Support Actor model implementation

## Approaches Considered

### Approach 1: Concrete Table Inheritance
**Description**: Each device type gets its own table with all fields (shared + specific)

**Schema Example**:
```sql
CREATE TABLE sprinklers (
    id UUID PRIMARY KEY,
    serial_number VARCHAR(50),  -- Duplicated
    status VARCHAR(20),          -- Duplicated
    zone VARCHAR(10),            -- Specific
    pressure_reading FLOAT       -- Specific
);

CREATE TABLE tractors (
    id UUID PRIMARY KEY,
    serial_number VARCHAR(50),  -- Duplicated
    status VARCHAR(20),          -- Duplicated
    model VARCHAR(50),           -- Specific
    fuel_level FLOAT             -- Specific
);
```

**Pros**:
- ✅ Simple queries (no joins)
- ✅ Best query performance
- ✅ Type safety at database level
- ✅ Independent schema evolution

**Cons**:
- ❌ Column duplication across tables
- ❌ Hard to query across types (requires UNION)
- ❌ Schema maintenance burden (add shared field = alter all tables)
- ❌ Referential integrity challenges (can't FK to "any device")

**Use Case**: When device types are very different, rarely queried together, and performance is critical.

---

### Approach 2: Class Table Inheritance ✅ **SELECTED**
**Description**: Base table with shared fields, specific tables with type-specific fields joined by FK

**Schema Example**:
```sql
CREATE TABLE device_agents (
    id UUID PRIMARY KEY,
    serial_number VARCHAR(50) UNIQUE,
    device_type VARCHAR(20),
    status VARCHAR(20)
);

CREATE TABLE irrigation_device_configs (
    device_id UUID PRIMARY KEY,
    zone VARCHAR(10),
    pressure_reading FLOAT,
    FOREIGN KEY (device_id) REFERENCES device_agents(id)
);

CREATE TABLE tractor_configs (
    device_id UUID PRIMARY KEY,
    model VARCHAR(50),
    fuel_level FLOAT,
    FOREIGN KEY (device_id) REFERENCES device_agents(id)
);
```

**Pros**:
- ✅ No column duplication (DRY)
- ✅ Easy cross-type queries (SELECT * FROM device_agents)
- ✅ Easy to add shared fields (alter base table only)
- ✅ Referential integrity (other tables FK to base table)
- ✅ Easy to add new types (just new config table)

**Cons**:
- ❌ Requires joins for full device details
- ❌ Slightly slower queries (join overhead)
- ❌ More complex GORM setup
- ❌ Two inserts per device (base + config)

**Use Case**: When many shared fields, frequent cross-type queries, need referential integrity, and adding types regularly.

---

### Approach 3: Single Table Inheritance
**Description**: One table with all fields for all types, using discriminator column

**Schema Example**:
```sql
CREATE TABLE device_agents (
    id UUID PRIMARY KEY,
    device_type VARCHAR(20),     -- Discriminator
    serial_number VARCHAR(50),
    status VARCHAR(20),
    -- Sprinkler fields (NULL for tractors)
    zone VARCHAR(10),
    pressure_reading FLOAT,
    -- Tractor fields (NULL for sprinklers)
    model VARCHAR(50),
    fuel_level FLOAT
);
```

**Pros**:
- ✅ Simplest queries (no joins)
- ✅ Single insert/update
- ✅ Easy cross-type queries
- ✅ GORM-friendly

**Cons**:
- ❌ Sparse tables (many NULL values)
- ❌ Wasted space
- ❌ Schema bloat (all types = all columns)
- ❌ No type safety

**Use Case**: When device types are very similar (80%+ shared fields) and only 2-4 types exist.

---

## Decision: Approach 2 (Class Table Inheritance)

**Rationale**:

| Requirement | Why Approach 2 Fits |
|------------|---------------------|
| Query all devices | Base table `device_agents` provides this without joins |
| Reference "any device" | Tasks can FK to `device_agents.id` |
| Add types regularly | Just add new config table, base unchanged |
| Type-agnostic messaging | Tasks reference `device_agents.id`, ignore type |
| Actor model support | Base table = Actor registry, natural fit |

**Key Decision Factors**:
1. **Many shared fields** between device types (serial_number, status, firmware, warranty, etc.)
2. **Frequent cross-type queries** ("show all devices", "devices needing maintenance")
3. **Messaging system needs type-agnostic references** (tasks don't care about device type)
4. **Regular addition of device types** (sprinklers → tractors → drones → sensors)
5. **Future Actor model** maps naturally to this structure

---

## Final Schema Design

### Base Table: Device Agents (Actor Registry)
```sql
CREATE TABLE device_agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_type VARCHAR(50) NOT NULL,
    serial_number VARCHAR(100) UNIQUE NOT NULL,
    status VARCHAR(20) DEFAULT 'offline',

    -- Actor model support (future)
    mailbox_size INT DEFAULT 0,
    last_activity_at TIMESTAMP,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_device_agents_type ON device_agents(device_type);
CREATE INDEX idx_device_agents_status ON device_agents(status);
```

**Purpose**:
- Single source of truth for all devices
- Type-agnostic queries
- FK target for tasks, logs, maintenance records
- Actor registry for actor model

---

### Device-Specific Config Tables

#### Irrigation Devices (Sprinklers)
```sql
CREATE TABLE irrigation_device_configs (
    device_id UUID PRIMARY KEY,
    zone VARCHAR(50) NOT NULL,
    pressure_reading FLOAT,
    flow_rate FLOAT,
    is_active BOOLEAN DEFAULT FALSE,

    FOREIGN KEY (device_id) REFERENCES device_agents(id) ON DELETE CASCADE
);
```

#### Tractors
```sql
CREATE TABLE tractor_configs (
    device_id UUID PRIMARY KEY,
    model VARCHAR(100),
    fuel_level FLOAT,
    gps_lat FLOAT,
    gps_lng FLOAT,
    current_implement VARCHAR(50),

    FOREIGN KEY (device_id) REFERENCES device_agents(id) ON DELETE CASCADE
);
```

#### Future: Drones (Example of Easy Extension)
```sql
CREATE TABLE drone_configs (
    device_id UUID PRIMARY KEY,
    battery_level FLOAT,
    altitude FLOAT,
    max_flight_time_minutes INT,
    camera_model VARCHAR(50),

    FOREIGN KEY (device_id) REFERENCES device_agents(id) ON DELETE CASCADE
);
```

**Purpose**:
- Store type-specific state
- Keep base table clean
- Easy to add new types without touching existing tables

---

### Messaging/Task Tables (Type-Agnostic)

#### Tasks
```sql
CREATE TABLE tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id UUID NOT NULL,
    instruction VARCHAR(50) NOT NULL,
    payload JSONB,
    status VARCHAR(20) DEFAULT 'queued',

    published_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    delivered_at TIMESTAMP,

    FOREIGN KEY (device_id) REFERENCES device_agents(id) ON DELETE CASCADE
);

CREATE INDEX idx_tasks_device_id ON tasks(device_id);
CREATE INDEX idx_tasks_status ON tasks(status);
```

**Purpose**:
- Persist pub/sub messages
- Track task lifecycle
- Type-agnostic (works for all device types)

#### Task Acknowledgments
```sql
CREATE TABLE task_acks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL,
    device_id UUID NOT NULL,
    status VARCHAR(20) NOT NULL,
    error_message TEXT,

    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    FOREIGN KEY (device_id) REFERENCES device_agents(id) ON DELETE CASCADE
);

CREATE INDEX idx_task_acks_task_id ON task_acks(task_id);
CREATE INDEX idx_task_acks_device_status ON task_acks(device_id, status);
```

**Purpose**:
- Track task execution status
- Support Phase 1 & 2 ACK infrastructure
- Enable retry logic (Phase 3)

---

### Optional: Actor Mailbox (Future)

```sql
CREATE TABLE actor_mailboxes (
    id BIGSERIAL PRIMARY KEY,
    device_id UUID NOT NULL,
    message JSONB NOT NULL,
    priority INT DEFAULT 0,
    received_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMP,

    FOREIGN KEY (device_id) REFERENCES device_agents(id) ON DELETE CASCADE
);

CREATE INDEX idx_actor_mailboxes_device_id ON actor_mailboxes(device_id);
CREATE INDEX idx_actor_mailboxes_unprocessed ON actor_mailboxes(device_id, processed_at)
    WHERE processed_at IS NULL;
```

**Purpose**:
- Persistent mailbox for Actor model
- Survive actor restarts
- Process messages in order

---

## GORM Models

### Base Model
```go
type DeviceAgent struct {
    ID             uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    DeviceType     string     `gorm:"size:50;not null;index"`
    SerialNumber   string     `gorm:"size:100;uniqueIndex;not null"`
    Status         string     `gorm:"size:20;default:offline;index"`
    MailboxSize    int        `gorm:"default:0"`
    LastActivityAt *time.Time
    CreatedAt      time.Time
    UpdatedAt      time.Time

    // Associations (not in DB)
    Tasks []Task `gorm:"foreignKey:DeviceID"`
}

func (DeviceAgent) TableName() string {
    return "device_agents"
}
```

### Type-Specific Configs
```go
type IrrigationDeviceConfig struct {
    DeviceID        uuid.UUID `gorm:"type:uuid;primaryKey"`
    Zone            string    `gorm:"size:50;not null"`
    PressureReading float64
    FlowRate        float64
    IsActive        bool      `gorm:"default:false"`

    Device DeviceAgent `gorm:"foreignKey:DeviceID;references:ID"`
}

func (IrrigationDeviceConfig) TableName() string {
    return "irrigation_device_configs"
}

type TractorConfig struct {
    DeviceID         uuid.UUID `gorm:"type:uuid;primaryKey"`
    Model            string    `gorm:"size:100"`
    FuelLevel        float64
    GPSLat           float64
    GPSLng           float64
    CurrentImplement string    `gorm:"size:50"`

    Device DeviceAgent `gorm:"foreignKey:DeviceID;references:ID"`
}

func (TractorConfig) TableName() string {
    return "tractor_configs"
}
```

### Messaging Models
```go
type Task struct {
    ID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    DeviceID    uuid.UUID  `gorm:"type:uuid;not null;index"`
    Instruction string     `gorm:"size:50;not null"`
    Payload     string     `gorm:"type:jsonb"`
    Status      string     `gorm:"size:20;default:queued;index"`
    PublishedAt time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
    DeliveredAt *time.Time

    Device DeviceAgent `gorm:"foreignKey:DeviceID"`
    Acks   []TaskAck   `gorm:"foreignKey:TaskID"`
}

func (Task) TableName() string {
    return "tasks"
}

type TaskAck struct {
    ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    TaskID       uuid.UUID `gorm:"type:uuid;not null;index"`
    DeviceID     uuid.UUID `gorm:"type:uuid;not null"`
    Status       string    `gorm:"size:20;not null"`
    ErrorMessage string    `gorm:"type:text"`
    Timestamp    time.Time `gorm:"default:CURRENT_TIMESTAMP"`

    Task   Task        `gorm:"foreignKey:TaskID"`
    Device DeviceAgent `gorm:"foreignKey:DeviceID"`
}

func (TaskAck) TableName() string {
    return "task_acks"
}
```

---

## Common Query Patterns

### Query All Devices (No Join)
```sql
SELECT id, device_type, serial_number, status
FROM device_agents
WHERE status = 'online';
```

### Query Device with Details (Join When Needed)
```sql
-- Sprinkler with full details
SELECT a.*, i.zone, i.pressure_reading, i.is_active
FROM device_agents a
JOIN irrigation_device_configs i ON a.id = i.device_id
WHERE a.id = ?;
```

### Create Task (Type-Agnostic)
```sql
INSERT INTO tasks (device_id, instruction, payload)
VALUES (?, 'start', '{"duration": 300}');
```

### Get Task History for Device
```sql
SELECT t.*, ta.status, ta.timestamp
FROM tasks t
LEFT JOIN task_acks ta ON t.id = ta.task_id
WHERE t.device_id = ?
ORDER BY t.published_at DESC;
```

### Add New Device Type (No Schema Changes to Existing!)
```sql
-- 1. Insert base record
INSERT INTO device_agents (device_type, serial_number)
VALUES ('drone', 'SN-DRONE-001')
RETURNING id;

-- 2. Insert type-specific config
INSERT INTO drone_configs (device_id, battery_level, altitude)
VALUES (?, 95.0, 50.0);
```

---

## Actor Model Support

**How This Schema Supports Actor Model**:

| Actor Concept | Database Mapping |
|--------------|------------------|
| **Actor Registry** | `device_agents` table |
| **Actor ID** | `device_agents.id` |
| **Actor Type** | `device_agents.device_type` |
| **Actor State** | `*_configs` tables |
| **Actor Mailbox** | `tasks` table (or `actor_mailboxes`) |
| **Message Log** | `task_acks` table |

**Benefits**:
1. **Persistent State**: Actor state survives crashes
2. **Persistent Mailbox**: Unprocessed messages survive restarts
3. **Actor Discovery**: Query `device_agents` to find all actors
4. **Type-Agnostic Routing**: Route by `device_id` without knowing type
5. **Message History**: Full audit trail in `tasks` and `task_acks`

**Pattern**:
```go
// Each device is an actor
type DeviceActor struct {
    deviceID uuid.UUID
    mailbox  chan Task
    db       *gorm.DB
}

// Actor loads state from DB on startup
func (a *DeviceActor) LoadState() error {
    var agent DeviceAgent
    return a.db.First(&agent, a.deviceID).Error
}

// Actor processes messages
func (a *DeviceActor) Run(ctx context.Context) {
    for {
        select {
        case task := <-a.mailbox:
            a.processTask(task)
            a.saveState()  // Persist to DB
        case <-ctx.Done():
            return
        }
    }
}
```

---

## Migration Path

### Phase 1: Schema Creation (Foundation)
**Goal**: Create database structure

**Steps**:
1. Create migration file with all tables
2. Run migrations: `device_agents`, `*_configs`, `tasks`, `task_acks`
3. Add indexes for performance
4. Test with sample data

**Validation**:
- All tables created
- Foreign keys enforced
- Indexes present

---

### Phase 2: Data Integration (Connect In-Memory to DB)
**Goal**: Persist in-memory data structures to database

**Steps**:
1. Update `NewSprinkler()` to insert into DB:
   ```go
   func NewSprinkler(ctx context.Context, broker Broker, db *gorm.DB, zone string) (*Sprinkler, error) {
       // Create DeviceAgent record
       agent := DeviceAgent{
           DeviceType: "sprinkler",
           SerialNumber: generateSerialNumber(),
           Status: "online",
       }
       db.Create(&agent)

       // Create config record
       config := IrrigationDeviceConfig{
           DeviceID: agent.ID,
           Zone: zone,
       }
       db.Create(&config)

       // Create in-memory sprinkler as before
       s := &Sprinkler{...}
       return s, nil
   }
   ```

2. Update `Publish()` to save tasks to DB
3. Update `processACKs()` to save ACKs to DB
4. Implement queries for task history

**Validation**:
- Devices persist to DB
- Tasks persist to DB
- ACKs persist to DB
- Can query task history

---

### Phase 3: Actor Model Implementation (Future)
**Goal**: Implement full actor model with supervision

**Steps**:
1. Add `actor_mailboxes` table
2. Implement actor supervisor
3. Add persistent mailbox processing
4. Implement actor recovery on failure
5. Add actor metrics and monitoring

**Validation**:
- Actors restart and recover state
- Unprocessed messages survive restarts
- Actor supervision works

---

## Trade-offs and Considerations

### Performance Considerations
- **Base table queries**: Very fast (no joins)
- **Detail queries**: Requires join, slightly slower
- **Optimization**: Use projections (only select needed fields)
- **Caching**: Consider caching device configs in memory

### Data Integrity
- **Foreign keys**: Enforce referential integrity
- **ON DELETE CASCADE**: Deleting device removes configs, tasks, ACKs
- **Transactions**: Use for multi-table inserts (device + config)

### Future Extensibility
- **Adding fields**: Easy to add to base or config tables
- **Adding types**: Just create new `*_configs` table
- **Adding relationships**: FK to `device_agents.id` works for all types

---

## References

- **Design Patterns**: Martin Fowler's "Patterns of Enterprise Application Architecture" (Class Table Inheritance pattern)
- **Actor Model**: Hewitt, Bishop, Steiger (1973), "A Universal Modular Actor Formalism for Artificial Intelligence"
- **Related Specs**:
  - `PUBSUB_IMPROVEMENTS.md` - Pub/sub messaging system
  - `TASK_ACK_RETRY_SPEC.md` - Task ACK/retry infrastructure
  - `SELF_INJECTION_PATTERN.md` - Strategy pattern for polymorphism in code

---

## Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2025-11-19 | Selected Class Table Inheritance | Balances query complexity with schema maintainability, supports all requirements |
| 2025-11-19 | Use JSONB for task payload | Flexible for device-specific task data without schema changes |
| 2025-11-19 | Separate tasks and task_acks tables | Allows multiple ACKs per task for retry tracking |
| 2025-11-19 | Optional actor_mailboxes for future | Defer until Actor model implementation decided |
