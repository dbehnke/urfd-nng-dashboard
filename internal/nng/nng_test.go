package nng

import (
	"encoding/json"
	"testing"
)

func TestParseEvent(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		validate func(e *Event) bool
	}{
		{
			name:  "State Event",
			input: `{"type": "state", "Clients": [{"Callsign": "N7TAE"}]}`,
			validate: func(e *Event) bool {
				return e.Type == "state" && len(e.Clients) > 0 && e.Clients[0].Callsign == "N7TAE"
			},
		},
		{
			name:  "Hearing Event",
			input: `{"type": "hearing", "my": "G4XYZ", "module": "A"}`,
			validate: func(e *Event) bool {
				return e.Type == "hearing" && e.My == "G4XYZ" && e.Module == "A"
			},
		},
		{
			name:  "Client Connect Event",
			input: `{"type": "client_connect", "callsign": "N7TAE", "module": "A"}`,
			validate: func(e *Event) bool {
				return e.Type == "client_connect" && e.Callsign == "N7TAE" && e.Module == "A"
			},
		},
		{
			name:    "Invalid JSON",
			input:   `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var event Event
			err := json.Unmarshal([]byte(tt.input), &event)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.validate != nil {
				if !tt.validate(&event) {
					t.Errorf("Validation failed for input: %s", tt.input)
				}
			}
		})
	}
}
