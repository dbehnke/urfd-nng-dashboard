package store

import (
	"os"
	"testing"
)

func TestStore(t *testing.T) {
	dbPath := "test.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-shm")
	defer os.Remove(dbPath + "-wal")

	s, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Test Insert
	h := &Hearing{
		My:     "G4XYZ",
		Module: "A",
	}
	if err := s.DB.Create(h).Error; err != nil {
		t.Fatalf("Failed to create hearing: %v", err)
	}

	// Test Query
	var count int64
	s.DB.Model(&Hearing{}).Count(&count)
	if count != 1 {
		t.Errorf("Expected 1 hearing, got %d", count)
	}
}
