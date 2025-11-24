package fleet

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestCascadeDelete verifies that deleting RegisteredDevice cascades to IrrigationDevice
func TestCascadeDelete(t *testing.T) {
	// Setup in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// CRITICAL: Enable foreign keys for SQLite
	err = db.Exec("PRAGMA foreign_keys = ON").Error
	require.NoError(t, err)

	// Verify foreign keys are enabled
	var fkEnabled int
	err = db.Raw("PRAGMA foreign_keys").Scan(&fkEnabled).Error
	require.NoError(t, err)
	assert.Equal(t, 1, fkEnabled, "Foreign keys must be enabled for CASCADE to work")

	// Run migrations
	err = db.AutoMigrate(&RegisteredDevice{}, &IrrigationDevice{})
	require.NoError(t, err)

	// Inspect the schema to verify CASCADE constraint exists
	t.Log("=== IrrigationDevice Schema ===")
	var schema string
	err = db.Raw("SELECT sql FROM sqlite_master WHERE type='table' AND name='irrigation_devices'").Scan(&schema).Error
	require.NoError(t, err)
	t.Log(schema)

	// Check if CASCADE is in the schema
	assert.Contains(t, schema, "ON DELETE CASCADE", "Schema should contain ON DELETE CASCADE constraint")

	// Create test data
	deviceID := uuid.New()
	registered := &RegisteredDevice{
		UUID:         deviceID,
		Name:         "Test Sprinkler",
		SerialNumber: "TEST-001",
		Status:       DeviceStatusAvailable,
		WorkerType:   WorkerTypeIrrigation,
		DeviceType:   DeviceTypeSprinkler,
		Zone:         "zone-1",
	}
	err = db.Create(registered).Error
	require.NoError(t, err)

	irrigation := &IrrigationDevice{
		RegisteredDeviceID: int(registered.ID), // Foreign key to RegisteredDevice.ID
		Device:             *registered,
		GallonsPerMinute:   5.0,
	}
	err = db.Create(irrigation).Error
	require.NoError(t, err)

	// Verify both records exist
	var registeredCount, irrigationCount int64
	db.Model(&RegisteredDevice{}).Where("uuid = ?", deviceID).Count(&registeredCount)
	db.Model(&IrrigationDevice{}).Where("registered_device_id = ?", registered.ID).Count(&irrigationCount)
	assert.Equal(t, int64(1), registeredCount)
	assert.Equal(t, int64(1), irrigationCount)

	// Delete the parent (RegisteredDevice) - use Unscoped() for HARD delete
	// Soft deletes (default) don't trigger CASCADE because row isn't actually deleted
	err = db.Unscoped().Where("uuid = ?", deviceID).Delete(&RegisteredDevice{}).Error
	require.NoError(t, err)

	// Verify CASCADE deleted the child (IrrigationDevice)
	db.Model(&RegisteredDevice{}).Where("uuid = ?", deviceID).Count(&registeredCount)
	db.Model(&IrrigationDevice{}).Where("registered_device_id = ?", registered.ID).Count(&irrigationCount)

	assert.Equal(t, int64(0), registeredCount, "RegisteredDevice should be deleted")
	assert.Equal(t, int64(0), irrigationCount, "IrrigationDevice should be CASCADE deleted")

	if irrigationCount != 0 {
		t.Error("❌ CASCADE DELETE IS NOT WORKING - IrrigationDevice was not deleted")
	} else {
		t.Log("✅ CASCADE DELETE IS WORKING - IrrigationDevice was automatically deleted")
	}
}

// TestForeignKeyConstraintExists verifies the foreign key constraint is defined
func TestForeignKeyConstraintExists(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Enable foreign keys
	err = db.Exec("PRAGMA foreign_keys = ON").Error
	require.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&RegisteredDevice{}, &IrrigationDevice{})
	require.NoError(t, err)

	// Get foreign key info
	type ForeignKeyInfo struct {
		ID    int
		Seq   int
		Table string
		From  string
		To    string
		// OnUpdate and OnDelete are in separate queries
	}

	var fkInfo []ForeignKeyInfo
	err = db.Raw("PRAGMA foreign_key_list(irrigation_devices)").Scan(&fkInfo).Error
	require.NoError(t, err)

	t.Logf("=== Foreign Key Constraints for irrigation_devices ===")
	for _, fk := range fkInfo {
		t.Logf("Foreign Key: %s.%s -> %s.%s", "irrigation_devices", fk.From, fk.Table, fk.To)
	}

	// Verify at least one FK exists pointing to registered_devices
	found := false
	for _, fk := range fkInfo {
		if fk.Table == "registered_devices" {
			found = true
			break
		}
	}
	assert.True(t, found, "Should have foreign key to registered_devices table")
}
