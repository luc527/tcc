package main

import "time"

const (
	pingInterval     = 20 * time.Second
	connReadTimeout  = 30 * time.Second
	connWriteTimeout = 10 * time.Second
)

const (
	connOutgoingBufferSize = 128
	connIncomingRateLimit  = time.Second / 2
	connIncomingBurstLimit = 8
)

const (
	clientMaxRooms    = 256
	clientRoomTimeout = 5 * time.Second
)

const (
	roomOutgoingBufferSize = 64
)

const (
	roomCapacity   = 256
	roomRateLimit  = time.Second / 6
	roomBurstLimit = 64
)

func init() {
	if pingInterval >= connReadTimeout {
		panic("must have pingInterval < connReadTimeout")
	}
}
