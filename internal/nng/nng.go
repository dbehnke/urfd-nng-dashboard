package nng

import (
	"encoding/json"
	"log"
	"time"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/sub"
	_ "go.nanomsg.org/mangos/v3/transport/all"
)

// Event represents the common structure of NNG messages from urfd
type Event struct {
	ID        uint      `json:"id,omitempty"`
	Type      string    `json:"type"`
	Status    string    `json:"status,omitempty"` // "active" | "ended"
	Duration  float64   `json:"duration,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	Callsign  string    `json:"callsign,omitempty"` // for client_connect/disconnect
	Module    string    `json:"module,omitempty"`
	Protocol  string    `json:"protocol,omitempty"`

	// Hearing fields
	My   string `json:"my,omitempty"`
	Ur   string `json:"ur,omitempty"`
	Rpt1 string `json:"rpt1,omitempty"`
	Rpt2 string `json:"rpt2,omitempty"`

	// State fields
	Clients []Client `json:"Clients,omitempty"`
	Users   []User   `json:"Users,omitempty"`
	Peers   []Peer   `json:"Peers,omitempty"`

	Raw json.RawMessage `json:"-"`
}

type Client struct {
	Callsign    string    `json:"Callsign"`
	Protocol    string    `json:"Protocol"`
	OnModule    string    `json:"OnModule"`
	ConnectTime time.Time `json:"ConnectTime"`
}

type User struct {
	Callsign  string    `json:"Callsign"`
	Repeater  string    `json:"Repeater"`
	OnModule  string    `json:"OnModule"`
	ViaPeer   string    `json:"ViaPeer"`
	LastHeard time.Time `json:"LastHeard"`
}

type Peer struct {
	Callsign    string    `json:"Callsign"`
	Protocol    string    `json:"Protocol"`
	ConnectTime time.Time `json:"ConnectTime"`
}

// Subscriber listens for NNG events
type Subscriber struct {
	url  string
	sock mangos.Socket
}

func NewSubscriber(url string) (*Subscriber, error) {
	sock, err := sub.NewSocket()
	if err != nil {
		return nil, err
	}

	if err := sock.Dial(url); err != nil {
		return nil, err
	}

	// Subscribe to all topics (empty prefix)
	if err := sock.SetOption(mangos.OptionSubscribe, []byte("")); err != nil {
		return nil, err
	}

	return &Subscriber{
		url:  url,
		sock: sock,
	}, nil
}

func (s *Subscriber) Listen(callback func(Event)) error {
	for {
		msg, err := s.sock.Recv()
		if err != nil {
			log.Printf("NNG Recv error: %v", err)
			continue
		}

		var event Event
		if err := json.Unmarshal(msg, &event); err != nil {
			log.Printf("JSON Unmarshal error: %v", err)
			continue
		}

		event.Raw = msg
		callback(event)
	}
}
