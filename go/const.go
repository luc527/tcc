package main

import "time"

const (
	connReadTimeout = 30 * time.Second
	pingInterval    = 20 * time.Second
)

const (
	maxMessageLength = 2048
	maxNameLength    = 24
)

const (
	roomCapacity = 256
	roomTimeout  = 5 * time.Second
)

const (
	incomingRateLimit  = 256 * time.Millisecond // time.Second / 4
	incomingBurstLimit = 16
)

func init() {
	if pingInterval >= connReadTimeout {
		panic("must have pingInterval < connReadTimeout")
	}
}
