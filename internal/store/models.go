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
	Duration float64 `json:"duration"`
}
