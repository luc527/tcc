package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
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
	case "test":
		testMain(args)
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

	sv := makeServer(numPartitions)
	sv.start()

	serve(l, sv)
}

func clientMain(args []string) {
	if len(args) == 0 {
		fmt.Println("address?")
		return
	}
	address, _ := args[0], args[1:]
	client(address)
}

func testMain(args []string) {
	if len(args) == 0 {
		fmt.Println("address?")
		return
	}
	address, args := args[0], args[1:]
	if len(args) == 0 {
		fmt.Println("recv count sync threshold?")
		return
	}
	recvThreshold_, args := args[0], args[1:]
	recvThreshold, err := strconv.ParseInt(recvThreshold_, 10, 32)
	if err != nil {
		fmt.Printf("invalid recv count sync threshold: %v\n", err)
		return
	}
	if len(args) == 0 {
		fmt.Println("send count sync threshold?")
		return
	}
	sendThreshold_, args := args[0], args[1:]
	sendThreshold, err := strconv.ParseInt(sendThreshold_, 10, 32)
	if err != nil {
		fmt.Printf("invalid send count sync threshold: %v\n", err)
		return
	}
	_ = args
	runTestConsole(address, int(recvThreshold), int(sendThreshold))
}
