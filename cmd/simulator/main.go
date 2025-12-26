package main

import (
	"encoding/json"
	"flag"
	"log"
	"math/rand"
	"time"

	"github.com/dbehnke/urfd-nng-dashboard/internal/nng"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/pub"
	_ "go.nanomsg.org/mangos/v3/transport/all"
)

var (
	url      = flag.String("url", "tcp://127.0.0.1:5555", "NNG publish URL")
	duration = flag.Duration("duration", 60*time.Minute, "Simulation duration")
)

type NodeState struct {
	Callsign    string
	Protocol    string
	Module      string
	ConnectTime time.Time
}

type UserState struct {
	Callsign  string
	Module    string
	LastHeard time.Time
}

type ModuleState string

const (
	StateIdle      ModuleState = "idle"
	StateTalking   ModuleState = "talking"
	StatePostTxGap ModuleState = "gap"
)

type ActiveTalker struct {
	Callsign string
	Protocol string
	EndTime  time.Time
}

type SimulatorModule struct {
	Name         string
	State        ModuleState
	ActiveTalker *ActiveTalker
	NextEvent    time.Time
}

func main() {
	flag.Parse()

	sock, err := pub.NewSocket()
	if err != nil {
		log.Fatalf("Failed to create pub socket: %v", err)
	}
	if err := sock.Listen(*url); err != nil {
		log.Fatalf("Failed to listen on %s: %v", *url, err)
	}
	defer func() {
		if err := sock.Close(); err != nil {
			log.Printf("Failed to close socket: %v", err)
		}
	}()

	log.Printf("Simulator started on %s for %v", *url, *duration)

	nodes := make(map[string]*NodeState)
	users := make(map[string]*UserState)

	modules := map[string]*SimulatorModule{
		"A": {Name: "A", State: StateIdle},
		"B": {Name: "B", State: StateIdle},
		"C": {Name: "C", State: StateIdle},
	}

	// Initialize realistic users
	realCalls := []string{"KF8S", "KI5RNN", "W8CPT", "W8EAP", "KE8VSI", "KZ8Z", "KE8RUH", "N8DBF", "KF8DRC", "W8FU", "W8VD", "K8PR", "KC8KJO", "KE8TFM", "WT8X", "AD8OD"}
	for _, call := range realCalls {
		users[call] = &UserState{
			Callsign:  call,
			Module:    string(rune('A' + rand.Intn(3))),
			LastHeard: time.Now().UTC().Add(-time.Duration(rand.Intn(60)) * time.Minute),
		}
	}

	// Initialize some nodes
	for i := 1; i <= 5; i++ {
		call := realCalls[i%len(realCalls)]
		nodes[call] = &NodeState{
			Callsign:    call,
			Protocol:    "DMR",
			Module:      string(rune('A' + rand.Intn(3))),
			ConnectTime: time.Now().UTC().Add(-time.Duration(rand.Intn(100)) * time.Minute),
		}
	}

	stop := time.After(*duration)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			log.Println("Simulation finished.")
			return
		case <-ticker.C:
			now := time.Now().UTC()

			// Manage each module independently
			for _, m := range modules {
				switch m.State {
				case StateTalking:
					if now.After(m.ActiveTalker.EndTime) {
						log.Printf("Module %s: Talker %s unkeyed", m.Name, m.ActiveTalker.Callsign)
						sendClosing(sock, m.Name, m.ActiveTalker.Callsign, m.ActiveTalker.Protocol)
						m.ActiveTalker = nil
						m.State = StatePostTxGap
						m.NextEvent = now.Add(time.Duration(2+rand.Intn(5)) * time.Second)
					}

				case StatePostTxGap:
					if now.After(m.NextEvent) {
						m.State = StateIdle
					} else {
						// During gap, high probability of someone replying (QSO)
						if rand.Float32() < 0.2 {
							startNewTalker(m, users, modules, now, sock)
						}
					}

				case StateIdle:
					// Lower probability of starting new conversation
					if rand.Float32() < 0.05 {
						startNewTalker(m, users, modules, now, sock)
					}
				}
			}

			// Randomly churn nodes
			if rand.Float32() < 0.005 {
				if len(nodes) > 3 && rand.Float32() < 0.5 {
					var dropCall string
					for c := range nodes {
						dropCall = c
						break
					}
					delete(nodes, dropCall)
					log.Printf("Node %s disconnected", dropCall)
				} else {
					call := realCalls[rand.Intn(len(realCalls))]
					nodes[call] = &NodeState{
						Callsign:    call,
						Protocol:    "D-Extra",
						Module:      string(rune('A' + rand.Intn(3))),
						ConnectTime: now,
					}
					log.Printf("Node %s connected", call)
				}
			}

			// Periodic state broadcast
			if now.Unix()%10 == 0 {
				sendState(sock, nodes, users)
			}
		}
	}
}

func startNewTalker(m *SimulatorModule, users map[string]*UserState, modules map[string]*SimulatorModule, now time.Time, sock mangos.Socket) {
	// Find available users (not talking on any module)
	activeCalls := make(map[string]bool)
	for _, mod := range modules {
		if mod.ActiveTalker != nil {
			activeCalls[mod.ActiveTalker.Callsign] = true
		}
	}

	var candidates []string
	for call, u := range users {
		if !activeCalls[call] && u.Module == m.Name {
			candidates = append(candidates, call)
		}
	}

	if len(candidates) == 0 {
		return
	}

	protocols := []string{"DMR", "YSF", "M17", "P25", "D-Star"}
	proto := protocols[rand.Intn(len(protocols))]

	sourceTalker := candidates[rand.Intn(len(candidates))]
	m.ActiveTalker = &ActiveTalker{
		Callsign: sourceTalker,
		Protocol: proto,
		EndTime:  now.Add(time.Duration(5+rand.Intn(30)) * time.Second),
	}
	m.State = StateTalking
	log.Printf("Module %s: Talker %s started until %s (%s)", m.Name, sourceTalker, m.ActiveTalker.EndTime.Format("15:04:05"), proto)
	sendHearing(sock, m.Name, sourceTalker, proto)
	users[sourceTalker].LastHeard = now
}

func sendHearing(sock mangos.Socket, module, callsign, protocol string) {
	ev := nng.Event{
		Type:     "hearing",
		Module:   module,
		Protocol: protocol,
		My:       callsign,
		Ur:       "CQCQCQ",
		Rpt1:     "SIMULATOR",
		Rpt2:     "URFD " + module,
	}
	data, _ := json.Marshal(ev)
	if err := sock.Send(data); err != nil {
		log.Printf("Failed to send hearing: %v", err)
	}
}

func sendClosing(sock mangos.Socket, module, callsign, protocol string) {
	ev := nng.Event{
		Type:     "closing",
		Module:   module,
		Protocol: protocol,
		My:       callsign,
	}
	data, _ := json.Marshal(ev)
	if err := sock.Send(data); err != nil {
		log.Printf("Failed to send closing: %v", err)
	}
}

func sendState(sock mangos.Socket, nodes map[string]*NodeState, users map[string]*UserState) {
	ev := nng.Event{
		Type: "state",
	}
	for _, n := range nodes {
		ev.Clients = append(ev.Clients, nng.Client{
			Callsign:    n.Callsign,
			Protocol:    n.Protocol,
			OnModule:    n.Module,
			ConnectTime: n.ConnectTime,
		})
	}
	for _, u := range users {
		ev.Users = append(ev.Users, nng.User{
			Callsign:  u.Callsign,
			OnModule:  u.Module,
			LastHeard: u.LastHeard,
			ViaPeer:   "XLX262",
		})
	}
	ev.Peers = append(ev.Peers, nng.Peer{
		Callsign:    "XLX262",
		Protocol:    "D-Extra",
		ConnectTime: time.Now().UTC().Add(-24 * time.Hour),
	})

	// Sample Modules
	ev.Modules = []nng.Module{
		{Name: "A", Description: "International / Primary"},
		{Name: "B", Description: "Local Chat / Secondary"},
		{Name: "C", Description: "Technical Discussions"},
		{Name: "D", Description: "Data & Testing"},
		{Name: "E", Description: "Emergency / Weather"},
	}

	data, _ := json.Marshal(ev)
	if err := sock.Send(data); err != nil {
		log.Printf("Failed to send state: %v", err)
	}
}
