package main

import (
	"fmt"
	"log"
	"net"
	"os"
)

func usage() {
	fmt.Printf("\nusage:\n")
	fmt.Printf("\t%s server\n", os.Args[0])
	fmt.Printf("\t%s client <address>\n", os.Args[0])
	os.Exit(1)
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
	}

	switch args[0] {
	case "server":
		servermain()
	case "client":
		if len(args) == 1 {
			usage()
		}
		address := args[1]
		clientmain(address)
	}

}

func servermain() {
	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	log.Printf("server running on %v\n", listener.Addr().String())
	serve(listener)
}
