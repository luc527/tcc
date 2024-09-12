package main

// a room handle is what a client uses to send messages to the room
// uses ctx to detect if the room closed, and to close its own connection to the room
type roomhandle struct {
	ctx
	mesc chan<- string
}

// a room message is a message the user receives from a room
type roommes struct {
	name string
	text string
}

// a client handle is what the room uses to send messages and joined/exited signals to the client
// uses ctx to detect if the client exited, or to close the client in case the room closes
type clienthandle struct {
	ctx
	mesc  chan<- roommes
	jnedc chan<- string
	exedc chan<- string
}

// the request the client sends in order to join a room
type joinroomreq struct {
	name  string
	mesc  chan<- roommes    // channel for the room to send messages in
	jnedc chan<- string     // channel for the room to send joined signals to
	exedc chan<- string     // channel for the room to send exited signals to
	resp  chan<- roomhandle // if the request succeeds, the room sends a room handle to this channel
	prob  chan<- zero       // if the request fails, zero{} is sent to this channel
}

type room struct {
	ctx
	reqc chan joinroomreq
}

func startroom(parent ctx, rid uint32, emptyc chan<- uint32) room {
	reqc := make(chan joinroomreq)
	r := room{parent.makechild(), reqc}
	go r.run(parent, rid, emptyc)
	return r
}

func (r room) join(req joinroomreq) {
	select {
	case <-r.done():
		req.prob <- zero{}
	case r.reqc <- req:
	}
}

// TODO: when sending to the client, the room maybe shouldn't only check
// whether the client closed. maybe it should also give up on sending a message
// if some time passes. otherwise one slow client could block the room
// (the room sends to the clients one by one)
// ofc using buffered channels for mesc/jnedc/exedc would help a lot, but may not be enough?
// then, what's the timeout? 1 millisecond? idk really, what's the amount that would actually help?
// but also, is it ok to create a timer for each and every message to the client?
// ! that's not necessary, could just reuse the same timer, it could even be one timer per room (maybe?)
// anyhow, with a large enough buffered channel and a decent client connection
// it's very unlikely that we'd even get to the point of losing messages like that

func (r room) run(parent ctx, rid uint32, emptyc chan<- uint32) {
	defer func() {
		r.cancel()
		select {
		case <-parent.done():
		case emptyc <- rid:
		}
	}()

	clients := make(map[string]clienthandle)

	mesc := make(chan roommes)
	exitc := make(chan string)

	for {
		select {
		case <-r.done():
			return
		case m := <-mesc:
			for _, ch := range clients {
				select {
				case <-ch.done():
				case ch.mesc <- m:
				}
			}
		case name := <-exitc:
			if _, exists := clients[name]; !exists {
				continue
			}
			delete(clients, name)
			if len(clients) == 0 {
				return
			}
			for _, ch := range clients {
				select {
				case <-ch.done():
				case ch.exedc <- name:
				}
			}
		case req := <-r.reqc:
			if _, exists := clients[req.name]; exists {
				req.prob <- zero{}
				continue
			}
			clientctx := r.makechild()
			clientmesc := make(chan string)

			go consumeclient(req.name, clientmesc, mesc, exitc, r.ctx, clientctx)
			req.resp <- roomhandle{clientctx, clientmesc}

			clients[req.name] = clienthandle{clientctx, req.mesc, req.jnedc, req.exedc}
			for _, ch := range clients {
				select {
				case <-ch.done():
				case ch.jnedc <- req.name:
				}
			}
		}
	}
}

func consumeclient(
	name string,
	inc <-chan string,
	outc chan<- roommes,
	exitc chan<- string,
	parent ctx,
	ctx ctx,
) {
	defer func() {
		select {
		case <-parent.done():
		case exitc <- name:
		}
	}()
	for {
		select {
		case <-ctx.done():
			return
		case text := <-inc:
			outc <- roommes{name, text}
		}
	}

}

func (rh roomhandle) trysend(text string) {
	select {
	case <-rh.done():
	case rh.mesc <- text:
	}
}
