package main

type textc chan string

type room struct {
	joinreq chan rjoinreq
	closed  chan zero
}

// room message -- this is what the room sends as messages to the clients
type rmes struct {
	name string // name of the user who sent the message
	text string // text of the message
}

// room client -- this is what the room sends messages, joined and exited signals to
type rclient struct {
	name string        // client name
	out  chan<- rmes   // channel to send messages to
	jned chan<- string // channel to send joined signals to (when another client joins)
	exed chan<- string // channel to send exited signals to (when another client exits)
}

// room join request -- this is what the client sends to the room in order the room
type rjoinreq struct {
	cli  rclient
	prob chan<- zero    // if the request fails, send to this channel, signaling that a problem has occurred
	resp chan<- rhandle // if the request succeeded, respond with a handle with which the client can talk to the room
}

// room handle -- this is what the client uses to send messages to the room, provided it is still open
type rhandle struct {
	in     textc       // channel for to client to send messages to, that the room consumes
	closed <-chan zero // channel for the client to check whether the room is still open before sending a message
}

func makeroom() room {
	return room{make(chan rjoinreq), make(chan zero)}
}

func (c rclient) close() {
	// method called by the room
	close(c.out)
	close(c.jned)
	close(c.exed)
}

func (h rhandle) close() {
	// method called by the client
	close(h.in)
}

func (h rhandle) send(s string) bool {
	// method called by the client
	select {
	case h.in <- s:
		return true
	case <-h.closed:
		return false
	}
}

func (r room) close() {
	select {
	case <-r.closed:
	default:
		close(r.closed)
	}
}

func (r room) run(id uint32, emptyc chan<- uint32) {
	clients := make(map[string]rclient)

	defer func() {
		emptyc <- id
		for _, cli := range clients {
			cli.close()
		}
		r.close()
	}()

	// since the room sends to the out/jned/exed channels in sequence, in the same goroutine
	// it's very important that no two clients run in the same goroutine
	// say A and B are the two clients
	// if we do <-A.out before <-B.out in one goroutine
	// but the room does B.out<- before A.out<-
	// we get ourselves into a deadlock

	sendc := make(chan rmes)
	exitc := make(chan string)

	for {
		select {
		case <-r.closed:
		case req := <-r.joinreq:
			name := req.cli.name
			if _, exists := clients[name]; exists {
				req.prob <- zero{}
				continue
			}

			handle := rhandle{make(chan string), r.closed}
			go handle.in.consume(name, sendc, exitc)
			req.resp <- handle

			clients[name] = req.cli
			for _, cli := range clients {
				cli.jned <- name
			}
			return
		case name := <-exitc:
			cli, ok := clients[name]
			if !ok {
				continue
			}
			cli.close()
			delete(clients, name)
			if len(clients) == 0 {
				return
			}
			for _, cli := range clients {
				cli.exed <- name
			}
		case mes := <-sendc:
			for _, cli := range clients {
				cli.out <- mes
			}
		}
	}
}

func (c textc) consume(name string, sendc chan<- rmes, exitc chan<- string) {
	defer func() {
		exitc <- name
	}()
	// TODO: check whether should consider room.closed here
	for s := range c {
		sendc <- rmes{name, s}
	}
}
