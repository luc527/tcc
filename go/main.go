package main

import (
	"fmt"
	"log"
	"net"
	"os"
)

func usage() {
	path := os.Args[0]
	fmt.Printf("usage:\n")
	fmt.Printf("\t%s client address\n", path)
	fmt.Printf("\t%s server\n", path)
	os.Exit(1)
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
	}

	// TODO: take address when running as server in order to run on a remote server

	// TODO: client could also set a read deadline for when the server stops working

	switch args[0] {
	case "client":
		if len(args) == 1 {
			usage()
		}
		address := args[1]
		if err := termclient(address, os.Stdin, os.Stdout); err != nil {
			log.Fatal(err)
		}
	case "server":
		listener, err := net.Listen("tcp", "127.0.0.1:")
		if err != nil {
			log.Fatal(err)
		}
		defer listener.Close()
		log.Printf("server running on %v\n", listener.Addr().String())
		serve(listener)
	default:
		usage()
	}
}
