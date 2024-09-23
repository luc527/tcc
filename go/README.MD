# tcc/go

The server itself, along with some tools for interacting with it and validating its behaviour, is implemented in this one Go module.

## server

You can start the server with `./tccgo server`.

| file | description |
| ---- | ----------- |
| `const.go` | Defines important constants that relate to non-functional requirements. |
| `protocol.go` | Implements reading/encoding and writing/decoding protocol messages (values of the type `protomes`). |
| `conn.go` | Abstracts over a raw `net.Conn` value, i.e., it implements a connection that speaks the `"tcc"` protocol. You can write a `protomes` message `m` to the other host from a `protoconn` value `pc` via `pc.send(m)` and read one via `m := <-pc.in`. It has a `context.Context` associated with it. |
| `server.go` | Implements the server. Listens for connections, starts `protoconn`s for them, handles the `protomes`s it receives from the `in` channel. In particular, when it joins a room, it creates a "room client", which is a separate goroutine that interacts with the room on behalf of the connection. |
| `hub.go` | Responsible only for mapping room IDs to the actual rooms. |
| `room.go` | Implements the broadcasting logic of the room. |

## client

### `client.go`

A simple client for talking with the server.

## console

TODO: write this section.

### `console.go`
### `console_client.go`
### `console_army.go`
### `logger.go`

## checker

TODO: write this section.

### `simulation.go`
### `checker.go`
### `checker_realtime.go`

