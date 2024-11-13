package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"
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
	_ = args

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Minute))
	defer cancel()
	// test0(ctx, address)
	testIncreasingSubs(ctx, address)
}
