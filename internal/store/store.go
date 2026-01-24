package store

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Store struct {
	DB *gorm.DB
}

func NewStore(dbPath string, enableSQLLogging bool) (*Store, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	logMode := logger.Silent
	if enableSQLLogging {
		logMode = logger.Info
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logMode),
	})
	if err != nil {
		return nil, err
	}

	// Enable WAL mode
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		log.Printf("Failed to set WAL mode: %v", err)
	}

	// Auto Migrate
	if err := db.AutoMigrate(&Hearing{}, &Callbook{}, &UserGainPreference{}); err != nil {
		return nil, err
	}

	return &Store{DB: db}, nil
}

// GetUserGain retrieves saved gain for callsign+module
func (s *Store) GetUserGain(callsign, module string) (*UserGainPreference, error) {
	var pref UserGainPreference
	callsign = strings.ToUpper(strings.TrimSpace(callsign))
	module = strings.ToUpper(strings.TrimSpace(module))

	err := s.DB.Where("callsign = ? AND module = ?", callsign, module).First(&pref).Error
	if err != nil {
		return nil, err
	}
	return &pref, nil
}

// SaveUserGain creates or updates gain preference
func (s *Store) SaveUserGain(callsign, module string, gain int) error {
	callsign = strings.ToUpper(strings.TrimSpace(callsign))
	module = strings.ToUpper(strings.TrimSpace(module))

	pref := UserGainPreference{
		Callsign:  callsign,
		Module:    module,
		Gain:      gain,
		LastHeard: time.Now(),
	}

	// Upsert: update if exists, insert if not
	return s.DB.Where("callsign = ? AND module = ?", callsign, module).
		Assign(UserGainPreference{Gain: gain, LastHeard: time.Now()}).
		FirstOrCreate(&pref).Error
}

// CleanupStaleGainPrefs removes entries >30 days old
func (s *Store) CleanupStaleGainPrefs() (int64, error) {
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	result := s.DB.Where("last_heard < ?", thirtyDaysAgo).Delete(&UserGainPreference{})
	return result.RowsAffected, result.Error
}
