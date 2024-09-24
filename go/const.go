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
	clientRateLimit  = time.Second / 4
	clientBurstLimit = 16
	clientMaxRooms   = 256
)

const (
	roomRateLimit  = time.Second
	roomBurstLimit = 64
)

const (
	roomCapacity         = 256
	roomTimeout          = 5 * time.Second
	roomBufferSize       = 16
	roomClientBufferSize = 128 // TODO: the roomclient buffer size doesn't need to be so large if the connection itself has a nice buffer size (i.e. make pc.out buffered)
	// roomClientBufferSize = 16
	// connBufferSize = 512 // mind that these will be protomes values, 21 bytes I think? -> 24 bytes aligned... around 10kb total... too much? actually very much fine? idk
)

func init() {
	if pingInterval >= connReadTimeout {
		panic("must have pingInterval < connReadTimeout")
	}
}
