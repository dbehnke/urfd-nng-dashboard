package store

import "time"

// Hearing represents a voice activity event
type Hearing struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`

	My       string `json:"my" gorm:"index"`
	Ur       string `json:"ur"`
	Rpt1     string `json:"rpt1"`
	Rpt2     string `json:"rpt2"`
	Module   string `json:"module" gorm:"index"`
	Protocol string `json:"protocol"`

	// Duration of transmission (optional/computed later)
	Duration  float64 `json:"duration"`
	AudioFile string  `json:"audio_file,omitempty"`
}

// UserGainPreference stores per-user, per-module receive gain settings
type UserGainPreference struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Callsign  string    `json:"callsign" gorm:"index:idx_callsign_module,unique"`
	Module    string    `json:"module" gorm:"index:idx_callsign_module,unique"`
	Gain      int       `json:"gain"`                    // 0-1000 (percentage)
	LastHeard time.Time `json:"last_heard" gorm:"index"` // For cleanup queries
}
