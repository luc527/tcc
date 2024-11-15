package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime/pprof"
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

func prof() {
	f, err := os.Create("cpu.prof")
	if err != nil {
		fmt.Printf("unable to create cpu profile file: %v\n", err)
		return
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		fmt.Printf("unable to start cpu profiling: %v\n", err)
		return
	}
	fmt.Printf("started cpu profiling\n")

	c := make(chan os.Signal, 1)
	go func() {
		<-c
		pprof.StopCPUProfile()
		fmt.Println("bye")
		os.Exit(1)
	}()

	signal.Notify(c, os.Interrupt)
}

func serverMain(args []string) {
	if len(args) == 0 {
		fmt.Println("address?")
		return
	}

	// prof()

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

	// testTest(address)
	// testIncreasingTopics(address)
	test0(address)
}
