package main

// a room handle is what a client uses to send messages to the room
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
	r := room{parent.childctx(), reqc}
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
		case mes := <-mesc:
			for _, ch := range clients {
				select {
				case <-ch.done():
				case ch.mesc <- mes:
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
			clientctx := r.childctx()
			clientmesc := make(chan string)

			clients[req.name] = clienthandle{clientctx, req.mesc, req.jnedc, req.exedc}
			for _, ch := range clients {
				select {
				case <-ch.done():
				case ch.jnedc <- req.name:
				}
			}
			go consumeclient(req.name, clientmesc, mesc, exitc, r.ctx, clientctx)
			req.resp <- roomhandle{clientctx, clientmesc}
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
		case mes := <-inc:
			outc <- roommes{name, mes}
		}
	}

}

func makeclientc() (mesc chan roommes, jnedc chan string, exedc chan string) {
	mesc, jnedc, exedc = make(chan roommes), make(chan string), make(chan string)
	return
}
