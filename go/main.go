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

type _test struct {
	s string
	f func(string) error
}

var _tests = []_test{
	{"functional", testFunctional},
	//{"rate limiting", testRateLimiting}, //doesn't really test, just shows the intervals between messages and you have to look at them to check if the rate limiting is working
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
	case "test":
		// TODO: add verbose as arg
		for _, t := range _tests {
			fmt.Printf("\n-- %-30s ", t.s)
			err := t.f(address)
			if err == nil {
				fmt.Printf("OK\n")
			} else {
				fmt.Printf("ERR: %v\n", err)
			}
		}
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
			} else if bef == "rt" {
				realtime = true
			} else {
				fmt.Println(bef, "unknown arg")
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
