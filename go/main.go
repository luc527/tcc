package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"tccgo/chk"
	"tccgo/tests"
)

const usage = `
usage:
	$ server
	$ client <address>
	$ console <address> [log=<logname>] [rt]
		log		logs all messages as csv to ./logs/<logname>_$.csv
		rt		enables real-time checking
	$ check
		expects log file contents from stdin
`

func prusage() {
	rep := strings.NewReplacer("%", os.Args[0], "$", chk.LogDateFormat)
	fmt.Fprint(os.Stderr, rep.Replace(usage))
	os.Exit(1)
}

func climain() {
	args := os.Args[1:]
	if len(args) == 0 {
		prusage()
	}

	switch args[0] {
	case "server":
		servermain()
		return
	}

	if len(args) < 2 {
		prusage()
	}
	address := args[1]

	switch args[0] {
	case "client":
		clientmain(address)
		return
	}

	prusage()
}

func main() {
	if os.Args[1] == "server" {
		servermain()
	} else {
		// test1(os.Args[1])
		tests.Randmain(os.Args[1])
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
