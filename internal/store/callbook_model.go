package store

import (
	"time"
)

type Callbook struct {
	ID        uint      `gorm:"primaryKey" json:"id"` // DMR ID
	Callsign  string    `gorm:"index" json:"callsign"`
	Name      string    `json:"name"`    // First Name
	Surname   string    `json:"surname"` // Last Name
	City      string    `json:"city"`
	State     string    `json:"state"`
	Country   string    `json:"country"`
	Remarks   string    `json:"remarks"`
	Source    string    `json:"source"` // "radioid", "hamdb", "qrz"
	UpdatedAt time.Time `json:"updated_at"`
}
