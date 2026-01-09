package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"

	"go.nanomsg.org/mangos/v3/protocol/req"
	_ "go.nanomsg.org/mangos/v3/transport/ipc"
	_ "go.nanomsg.org/mangos/v3/transport/tcp"
)

func main() {
	addr := flag.String("addr", "tcp://127.0.0.1:6001", "NNG Control Address")
	ip := flag.String("ip", "1.2.3.4", "IP Address to register")
	callsign := flag.String("callsign", "TESTCALL", "Callsign to register")
	flag.Parse()

	sock, err := req.NewSocket()
	if err != nil {
		log.Fatalf("can't get new req socket: %v", err)
	}
	defer sock.Close()

	if err = sock.Dial(*addr); err != nil {
		log.Fatalf("can't dial on req socket: %v", err)
	}

	cmd := map[string]string{
		"cmd":      "usrp_register",
		"ip":       *ip,
		"callsign": *callsign,
	}

	b, err := json.Marshal(cmd)
	if err != nil {
		log.Fatalf("json marshal failed: %v", err)
	}

	fmt.Printf("Sending to %s: %s\n", *addr, string(b))

	if err = sock.Send(b); err != nil {
		log.Fatalf("can't send message on req socket: %v", err)
	}

	msg, err := sock.Recv()
	if err != nil {
		log.Fatalf("can't receive date: %v", err)
	}

	fmt.Printf("Reply: %s\n", string(msg))
}
