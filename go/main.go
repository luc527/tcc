package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"
)

const usage = `
usage:
	% server
	% client <address>
	% console <address> [log=<logname>] [rt]
		log logs all messages as csv to ./logs/<logname>_$.csv
		rt enables real-time checking
	% check
		expects log file contents from stdin
`

func prusage() {
	rep := strings.NewReplacer("%", os.Args[0], "$", logdateformat)
	fmt.Fprint(os.Stderr, rep.Replace(usage))
	os.Exit(1)
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		prusage()
	}

	switch args[0] {
	case "server":
		servermain()
		return
	case "check":
		checkmain()
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
	case "rtcheck":
		rtcheckmain(address)
		return
	}

	switch args[0] {
	case "console":
		var logname string
		var realtime bool
		for _, arg := range args[2:] {
			bef, aft, ok := strings.Cut(arg, "=")
			if bef == "log" {
				if ok && len(aft) > 0 {
					logname = aft
				} else {
					prusage()
				}
			}
			if bef == "rt" {
				realtime = true
			}
		}
		conmain(address, logname, realtime)
	default:
		prusage()
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
