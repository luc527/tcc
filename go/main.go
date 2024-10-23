package main

import (
	"fmt"
	"log"
	"net"
	"os"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		fmt.Println("cmd?")
		return
	}

	cmd, args := args[0], args[1:]

	switch cmd {
	case "server":
		serverMain(args)
	case "client":
		clientMain(args)
	default:
		fmt.Printf("unknown command %q\n", cmd)
	}
}

func serverMain(args []string) {
	if len(args) == 0 {
		fmt.Println("address?")
		return
	}
	l, err := net.Listen("tcp", args[0])
	if err != nil {
		log.Fatal(err)
	}
	_ = args[1:]

	address := l.Addr()
	fmt.Printf("listening on %v\n", address)

	s, err := newServer(numPartitions)
	if err != nil {
		log.Fatal(err)
	}
	s.start()

	serve(l, s)
}

func clientMain(args []string) {
	if len(args) == 0 {
		fmt.Println("address?")
		return
	}
	address, _ := args[0], args[1:]
	client(address)
}
