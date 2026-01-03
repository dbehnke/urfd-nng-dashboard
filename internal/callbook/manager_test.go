package callbook

import (
	"os"
	"testing"

	"github.com/dbehnke/urfd-nng-dashboard/internal/store"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestManager_Lookup_Integration(t *testing.T) {
	// Skip if strictly unit testing without network
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test; set INTEGRATION_TEST=1 to run")
	}

	// Setup in-memory DB
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to in-memory DB: %v", err)
	}
	if err := db.AutoMigrate(&store.Callbook{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	// Create manager
	// We need to mock the store or just pass the DB if we refactor Manager to take DB?
	// The current Manager.NewManager takes *store.Store.
	// Let's create a partial store struct if possible or just inject the DB.
	// The Manager struct has `db *gorm.DB`. NewManager gets it from `s.DB`.
	// We can manually construct Manager for testing.

	mgr := &Manager{
		db:      db,
		qrzUser: os.Getenv("QRZ_USERNAME"),
		qrzPass: os.Getenv("QRZ_PASSWORD"),
	}

	// Test Case 1: Lookup a known US callsign
	t.Run("Lookup W1AW", func(t *testing.T) {
		cb, err := mgr.Lookup("W1AW")
		if err != nil {
			t.Fatalf("Lookup failed: %v", err)
		}
		if cb.Callsign != "W1AW" {
			t.Errorf("Expected W1AW, got %s", cb.Callsign)
		}
		if cb.Country != "United States" && cb.Country != "USA" {
			t.Errorf("Expected United States/USA, got %s", cb.Country)
		}
		t.Logf("Found: %+v", cb)
	})

	// Test Case 2: Cache check
	t.Run("Cache Check", func(t *testing.T) {
		var count int64
		db.Model(&store.Callbook{}).Where("callsign = ?", "W1AW").Count(&count)
		if count != 1 {
			t.Errorf("Expected 1 cached record, got %d", count)
		}
	})

	// Test Case 3: Invalid Callsign
	t.Run("Invalid Callsign", func(t *testing.T) {
		_, err := mgr.Lookup("INVALIDCALLSIGN123")
		if err == nil {
			t.Error("Expected error for invalid callsign, got nil")
		}
	})

	// Test Case 4: User Manual Verification
	userCallsigns := []string{"KF8S", "M0FXB", "VK5MD"}
	for _, cs := range userCallsigns {
		t.Run("Lookup "+cs, func(t *testing.T) {
			cb, err := mgr.Lookup(cs)
			if err != nil {
				if os.Getenv("QRZ_USERNAME") != "" {
					t.Fatalf("Lookup failed for %s: %v", cs, err)
				} else {
					t.Logf("Skipping failure for %s (no QRZ creds): %v", cs, err)
					return
				}
			}
			t.Logf("Result for %s: Name=%s, Location=%s, %s, %s (Source=%s)",
				cs, cb.Name, cb.City, cb.State, cb.Country, cb.Source)
		})
	}
}
