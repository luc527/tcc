import * as net from 'net';
import MessageFSM from './MessageFSM.js';
import Type from './Type.js';

let serverOpts = {
    noDelay: true,
};
let server = net.createServer(serverOpts);

server.on('error', err => {
    console.log('server error', err);
});

server.on('connection', sock => {
    sock.on('end', () => {
        console.log('sock ended');
    });

    sock.on('timeout', () => {
        console.log('sock timed out');
        sock.end();
    });

    function resetTimeout() {
        // TODO: do I have to call setTimeout(0) to cancel the previous timer?
        sock.setTimeout(60_000);
    }
    resetTimeout();
    // TODO: check if calling resetTimeout() for every message received is the right way to go on
    // about this. 

    let fsm = new MessageFSM();
    fsm.onPing(() => {
        sock.write(Buffer.of(Type.ping));
        resetTimeout();
    });
    fsm.onSub((topic, b) => {

        resetTimeout();
    });
    fsm.onPub((topic, payload) => {

        resetTimeout();
    });

    sock.on('data', data => {
        for (let byte of data) {
            fsm.handle(byte);
        }
    });
});

let listenOpts = {
    host: 'localhost',
    port: 0, // ephemeral
};
server.listen(listenOpts, () => {
    let {port} = server.address();
    console.log(`listening on port ${port}`);
});